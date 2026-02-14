package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"treasury-tracker/internal/database"
	"treasury-tracker/internal/services"
	"treasury-tracker/internal/testutil"
)

const testConnString = "postgres://postgres:postgres@localhost:5432/treasury_db?sslmode=disable"

func setupTestHandler(t *testing.T) (*TransactionHandlers, *database.Queries, func()) {
	t.Helper()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, testConnString)
	if err != nil {
		t.Skipf("Skipping integration test: database not available: %v", err)
	}

	logger := zap.NewNop()
	queries := database.New(pool)
	txService := services.NewTransactionService(queries, pool, logger)
	treasuryService := services.NewTreasuryService(queries, pool, logger)
	handler := NewTransactionHandlers(txService, queries, treasuryService, logger)

	return handler, queries, func() { pool.Close() }
}

func TestBuyHandler_Success(t *testing.T) {
	handler, queries, cleanup := setupTestHandler(t)
	defer cleanup()
	ctx := context.Background()

	testUser, err := queries.CreateUser(ctx, database.CreateUserParams{
		Name:    "Test User - Handler Success",
		Balance: testutil.MustNumeric("500000.00"),
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	defer testutil.CleanupUser(t, ctx, queries, testUser.ID)

	buyReq := BuyRequest{
		UserID:    testUser.ID,
		Term:      "6M",
		FaceValue: 100000.00,
	}
	body, _ := json.Marshal(buyReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/buy", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.BuyHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d, body: %s", w.Code, w.Body.String())
	}

	var resp BuyResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if resp.User == nil {
		t.Error("Expected user in response")
	}
}

func TestBuyHandler_InvalidTerm(t *testing.T) {
	handler, queries, cleanup := setupTestHandler(t)
	defer cleanup()
	ctx := context.Background()

	testUser, err := queries.CreateUser(ctx, database.CreateUserParams{
		Name:    "Test User - Invalid Term",
		Balance: testutil.MustNumeric("500000.00"),
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	defer testutil.CleanupUser(t, ctx, queries, testUser.ID)

	buyReq := BuyRequest{
		UserID:    testUser.ID,
		Term:      "INVALID",
		FaceValue: 100000.00,
	}
	body, _ := json.Marshal(buyReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/buy", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.BuyHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp errorResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error == "" {
		t.Error("Expected error message")
	}
}

func TestBuyHandler_InsufficientBalance(t *testing.T) {
	handler, queries, cleanup := setupTestHandler(t)
	defer cleanup()
	ctx := context.Background()

	testUser, err := queries.CreateUser(ctx, database.CreateUserParams{
		Name:    "Test User - Insufficient Balance Handler",
		Balance: testutil.MustNumeric("50000.00"),
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	defer testutil.CleanupUser(t, ctx, queries, testUser.ID)

	buyReq := BuyRequest{
		UserID:    testUser.ID,
		Term:      "6M",
		FaceValue: 100000.00,
	}
	body, _ := json.Marshal(buyReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/buy", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.BuyHandler(w, req)

	// May return 400 (insufficient balance) or 500 (yield fetch timeout)
	if w.Code != http.StatusBadRequest && w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 400 or 500, got %d", w.Code)
	}

	transactions, _ := queries.GetTransactionsByUser(ctx, testUser.ID)
	if len(transactions) != 0 {
		t.Errorf("Expected 0 transactions, got %d", len(transactions))
	}

	user, _ := queries.GetUser(ctx, testUser.ID)
	if testutil.MustFloat64(user.Balance) != 50000.00 {
		t.Errorf("Expected balance unchanged at 50000.00, got %f", testutil.MustFloat64(user.Balance))
	}
}

func TestBuyHandler_InvalidJSON(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/buy", bytes.NewReader([]byte(`{"invalid`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.BuyHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestBuyHandler_AllValidTerms(t *testing.T) {
	handler, queries, cleanup := setupTestHandler(t)
	defer cleanup()
	ctx := context.Background()

	testUser, err := queries.CreateUser(ctx, database.CreateUserParams{
		Name:    "Test User - All Terms",
		Balance: testutil.MustNumeric("10000000.00"),
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	defer testutil.CleanupUser(t, ctx, queries, testUser.ID)

	for _, term := range []string{"1M", "3M", "6M", "1Y", "2Y", "5Y", "10Y", "30Y"} {
		t.Run(term, func(t *testing.T) {
			buyReq := BuyRequest{
				UserID:    testUser.ID,
				Term:      term,
				FaceValue: 10000.00,
			}
			body, _ := json.Marshal(buyReq)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/buy", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.BuyHandler(w, req)

			if w.Code != http.StatusOK {
				var resp errorResponse
				json.NewDecoder(w.Body).Decode(&resp)
				t.Errorf("Expected 200 for term %s, got %d: %s", term, w.Code, resp.Error)
			}
		})
	}

	transactions, _ := queries.GetTransactionsByUser(ctx, testUser.ID)
	if len(transactions) == 0 {
		t.Error("Expected at least some transactions")
	}
	t.Logf("Created %d transactions", len(transactions))
}

// ---------------------------------------------------------------------------
// Mock-based unit tests (no database required)
// ---------------------------------------------------------------------------

func TestFundHandler_Success_Mock(t *testing.T) {
	user := sampleUser()
	txMock := &mockTransactionService{fundResult: user}
	handler := NewTransactionHandlers(txMock, nil, nil, zap.NewNop())

	fundReq := TransactionRequest{UserID: 1, Amount: 5000.00}
	body, _ := json.Marshal(fundReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/fund", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.FundHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp TransactionResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Error("Expected success=true")
	}
	if resp.User == nil {
		t.Error("Expected user in response")
	}
	if resp.User.ID != 1 {
		t.Errorf("Expected user ID 1, got %d", resp.User.ID)
	}
}

func TestFundHandler_InvalidJSON_Mock(t *testing.T) {
	handler := NewTransactionHandlers(nil, nil, nil, zap.NewNop())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/fund", bytes.NewReader([]byte(`not json`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.FundHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp errorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}
	if resp.Error == "" {
		t.Error("Expected non-empty error message")
	}
}

func TestBuyHandler_Success_Mock(t *testing.T) {
	user := sampleUser()
	treasuryMock := &mockTreasuryService{
		latestYields: sampleYieldData(),
	}
	txMock := &mockTransactionService{
		buyResult: &services.PurchaseResult{
			User:          user,
			FaceValue:     decimal.NewFromFloat(50000.00),
			PurchasePrice: decimal.NewFromFloat(48912.50),
			Discount:      decimal.NewFromFloat(1087.50),
		},
	}
	handler := NewTransactionHandlers(txMock, nil, treasuryMock, zap.NewNop())

	buyReq := BuyRequest{
		UserID:    1,
		Term:      "6M",
		FaceValue: 50000.00,
	}
	body, _ := json.Marshal(buyReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/buy", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.BuyHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp BuyResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Error("Expected success=true")
	}
	if resp.User == nil {
		t.Error("Expected user in response")
	}
	if !resp.FaceValue.Equal(decimal.NewFromFloat(50000.00)) {
		t.Errorf("Expected face value 50000.00, got %s", resp.FaceValue)
	}
	if !resp.PurchasePrice.Equal(decimal.NewFromFloat(48912.50)) {
		t.Errorf("Expected purchase price 48912.50, got %s", resp.PurchasePrice)
	}
	if !resp.Discount.Equal(decimal.NewFromFloat(1087.50)) {
		t.Errorf("Expected discount 1087.50, got %s", resp.Discount)
	}
}

func TestBuyHandler_InvalidTerm_Mock(t *testing.T) {
	// No services needed -- handler should reject before calling any service
	handler := NewTransactionHandlers(nil, nil, nil, zap.NewNop())

	buyReq := BuyRequest{
		UserID:    1,
		Term:      "INVALID",
		FaceValue: 50000.00,
	}
	body, _ := json.Marshal(buyReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/buy", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.BuyHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp errorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}
	if resp.Error == "" {
		t.Error("Expected non-empty error message for invalid term")
	}
}
