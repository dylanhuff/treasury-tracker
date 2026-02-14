package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"treasury-tracker/internal/database"
	"treasury-tracker/internal/services"
	"treasury-tracker/internal/testutil"
)

func TestE2E_FundBuySellWithdraw(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E test in short mode")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testConnString)
	if err != nil {
		t.Skipf("Skipping E2E test: database not available: %v", err)
	}
	defer pool.Close()

	logger := zap.NewNop()
	queries := database.New(pool)
	txService := services.NewTransactionService(queries, pool, logger)
	treasuryService := services.NewTreasuryService(queries, pool, logger)
	handler := NewTransactionHandlers(txService, queries, treasuryService, logger)

	if err := treasuryService.SyncYields(ctx); err != nil {
		t.Fatalf("SyncYields failed: %v", err)
	}

	testUser, err := queries.CreateUser(ctx, database.CreateUserParams{
		Name:    "E2E Test User",
		Balance: testutil.MustNumeric("0.00"),
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	defer testutil.CleanupUser(t, ctx, queries, testUser.ID)

	// Step 1: Fund $100,000
	t.Run("Fund", func(t *testing.T) {
		fundReq := TransactionRequest{UserID: testUser.ID, Amount: 100000.00}
		body, _ := json.Marshal(fundReq)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/fund", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.FundHandler(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Fund failed: status %d, body: %s", w.Code, w.Body.String())
		}

		var resp TransactionResponse
		json.NewDecoder(w.Body).Decode(&resp)
		if !resp.Success || resp.User == nil {
			t.Fatal("Fund response missing success or user")
		}

		balance := testutil.MustFloat64(resp.User.Balance)
		if balance != 100000.00 {
			t.Fatalf("Expected balance 100000.00 after fund, got %f", balance)
		}
		t.Logf("Funded: balance = $%.2f", balance)
	})

	// Step 2: Buy a 6M T-Bill with $50,000 face value
	var holdingID int32
	t.Run("Buy", func(t *testing.T) {
		buyReq := BuyRequest{UserID: testUser.ID, Term: "6M", FaceValue: 50000.00}
		body, _ := json.Marshal(buyReq)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/buy", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.BuyHandler(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Buy failed: status %d, body: %s", w.Code, w.Body.String())
		}

		var resp BuyResponse
		json.NewDecoder(w.Body).Decode(&resp)
		if !resp.Success || resp.User == nil {
			t.Fatal("Buy response missing success or user")
		}

		balance := testutil.MustFloat64(resp.User.Balance)
		if balance >= 100000.00 {
			t.Fatalf("Expected balance < 100000 after buy, got %f", balance)
		}
		t.Logf("Bought 6M T-Bill: face=%s, price=%s, discount=%s, balance=$%.2f",
			resp.FaceValue, resp.PurchasePrice, resp.Discount, balance)

		holdings, err := queries.GetActiveHoldingsByUser(ctx, testUser.ID)
		if err != nil {
			t.Fatalf("Failed to get holdings: %v", err)
		}
		if len(holdings) == 0 {
			t.Fatal("Expected at least 1 holding after buy")
		}
		holdingID = holdings[0].ID
	})

	// Step 3: Sell the holding
	t.Run("Sell", func(t *testing.T) {
		if holdingID == 0 {
			t.Skip("No holding ID from buy step")
		}

		sellReq := SellRequest{UserID: testUser.ID, HoldingID: holdingID, Amount: 50000.00}
		body, _ := json.Marshal(sellReq)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sell", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.SellHandler(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Sell failed: status %d, body: %s", w.Code, w.Body.String())
		}

		var resp TransactionResponse
		json.NewDecoder(w.Body).Decode(&resp)
		if !resp.Success || resp.User == nil {
			t.Fatal("Sell response missing success or user")
		}

		balance := testutil.MustFloat64(resp.User.Balance)
		t.Logf("Sold holding: balance = $%.2f", balance)
	})

	// Step 4: Withdraw remaining balance and verify
	t.Run("Withdraw", func(t *testing.T) {
		user, err := queries.GetUser(ctx, testUser.ID)
		if err != nil {
			t.Fatalf("Failed to get user: %v", err)
		}
		currentBalance := testutil.MustFloat64(user.Balance)
		if currentBalance <= 0 {
			t.Fatalf("Expected positive balance before withdraw, got %f", currentBalance)
		}

		withdrawReq := TransactionRequest{UserID: testUser.ID, Amount: currentBalance}
		body, _ := json.Marshal(withdrawReq)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/withdraw", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.WithdrawHandler(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Withdraw failed: status %d, body: %s", w.Code, w.Body.String())
		}

		var resp TransactionResponse
		json.NewDecoder(w.Body).Decode(&resp)
		if !resp.Success || resp.User == nil {
			t.Fatal("Withdraw response missing success or user")
		}

		finalBalance := testutil.MustFloat64(resp.User.Balance)
		if finalBalance != 0.00 {
			t.Errorf("Expected final balance 0.00 after full withdraw, got %f", finalBalance)
		}
		t.Logf("Withdrew $%.2f: final balance = $%.2f", currentBalance, finalBalance)
	})

	transactions, err := queries.GetTransactionsByUser(ctx, testUser.ID)
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if len(transactions) != 4 {
		t.Errorf("Expected 4 transactions (fund, buy, sell, withdraw), got %d", len(transactions))
	}
	for _, tx := range transactions {
		t.Logf("Transaction: type=%s, amount=%v", tx.Type, tx.Amount)
	}
}
