package services

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"treasury-tracker/internal/database"
	"treasury-tracker/internal/testutil"
)

const testConnString = "postgres://postgres:postgres@localhost:5432/treasury_db?sslmode=disable"

func setupTestService(t *testing.T) (*TransactionService, *database.Queries, func()) {
	t.Helper()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, testConnString)
	if err != nil {
		t.Skipf("Skipping integration test: database not available: %v", err)
	}

	queries := database.New(pool)
	service := NewTransactionService(queries, pool)

	return service, queries, func() { pool.Close() }
}

func TestBuyTreasury_Success(t *testing.T) {
	service, queries, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	testUser, err := queries.CreateUser(ctx, database.CreateUserParams{
		Name:    "Test User - Buy Success",
		Balance: testutil.MustNumeric("500000.00"),
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	defer testutil.CleanupUser(t, ctx, queries, testUser.ID)

	amount := testutil.MustNumeric("100000.00")
	currentYield := testutil.MustNumeric("4.50")
	result, err := service.BuyTreasury(ctx, testUser.ID, "6M", amount, currentYield)
	if err != nil {
		t.Fatalf("BuyTreasury failed: %v", err)
	}
	if result == nil || result.User == nil {
		t.Fatal("Expected purchase result with user, got nil")
	}

	// Purchase price = $100,000 * (1 - (4.50/100 * 180)/360) = $97,750
	expectedBalance := 402250.00
	actualBalance := testutil.MustFloat64(result.User.Balance)
	if actualBalance != expectedBalance {
		t.Errorf("Expected balance %f, got %f", expectedBalance, actualBalance)
	}

	holdings, err := queries.GetHoldingsByUser(ctx, testUser.ID)
	if err != nil {
		t.Fatalf("Failed to get holdings: %v", err)
	}
	if len(holdings) != 1 {
		t.Fatalf("Expected 1 holding, got %d", len(holdings))
	}

	holding := holdings[0]
	if holding.Term != "6M" {
		t.Errorf("Expected term '6M', got '%s'", holding.Term)
	}
	if testutil.MustFloat64(holding.Amount) != 100000.00 {
		t.Errorf("Expected holding amount 100000.00, got %f", testutil.MustFloat64(holding.Amount))
	}
	if testutil.MustFloat64(holding.YieldAtPurchase) != 4.50 {
		t.Errorf("Expected yield 4.50, got %f", testutil.MustFloat64(holding.YieldAtPurchase))
	}

	transactions, err := queries.GetTransactionsByUser(ctx, testUser.ID)
	if err != nil {
		t.Fatalf("Failed to get transactions: %v", err)
	}
	if len(transactions) != 1 {
		t.Fatalf("Expected 1 transaction, got %d", len(transactions))
	}

	tx := transactions[0]
	if tx.Type != database.TransactionTypeBuy {
		t.Errorf("Expected transaction type 'buy', got '%s'", tx.Type)
	}
	if testutil.MustFloat64(tx.Amount) != 97750.00 {
		t.Errorf("Expected transaction amount 97750.00, got %f", testutil.MustFloat64(tx.Amount))
	}
}

func TestBuyTreasury_InsufficientBalance(t *testing.T) {
	service, queries, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	testUser, err := queries.CreateUser(ctx, database.CreateUserParams{
		Name:    "Test User - Insufficient Balance",
		Balance: testutil.MustNumeric("50000.00"),
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	defer testutil.CleanupUser(t, ctx, queries, testUser.ID)

	amount := testutil.MustNumeric("100000.00")
	currentYield := testutil.MustNumeric("4.50")
	_, err = service.BuyTreasury(ctx, testUser.ID, "6M", amount, currentYield)

	if err == nil {
		t.Fatal("Expected insufficient balance error, got nil")
	}
	if !strings.Contains(err.Error(), "insufficient balance") {
		t.Errorf("Expected error containing 'insufficient balance', got: %v", err)
	}

	holdings, err := queries.GetHoldingsByUser(ctx, testUser.ID)
	if err != nil {
		t.Fatalf("Failed to get holdings: %v", err)
	}
	if len(holdings) != 0 {
		t.Errorf("Expected 0 holdings, got %d", len(holdings))
	}
}

func TestBuyTreasury_InvalidAmount(t *testing.T) {
	service, queries, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	testUser, err := queries.CreateUser(ctx, database.CreateUserParams{
		Name:    "Test User - Invalid Amount",
		Balance: testutil.MustNumeric("500000.00"),
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	defer testutil.CleanupUser(t, ctx, queries, testUser.ID)

	for _, tc := range []struct {
		name   string
		amount string
	}{
		{"Zero amount", "0.00"},
		{"Negative amount", "-1000.00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			amount := testutil.MustNumeric(tc.amount)
			currentYield := testutil.MustNumeric("4.50")
			_, err := service.BuyTreasury(ctx, testUser.ID, "6M", amount, currentYield)

			if err == nil {
				t.Fatalf("Expected error for %s, got nil", tc.name)
			}
			if !strings.Contains(err.Error(), "face value must be greater than zero") {
				t.Errorf("Expected 'face value must be greater than zero', got: %v", err)
			}
		})
	}
}

func TestBuyTreasury_AtomicTransaction(t *testing.T) {
	service, queries, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	testUser, err := queries.CreateUser(ctx, database.CreateUserParams{
		Name:    "Test User - Atomic Test",
		Balance: testutil.MustNumeric("100000.00"),
	})
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	defer testutil.CleanupUser(t, ctx, queries, testUser.ID)

	// Face value of $102,500 at 4.50% yield costs ~$100,194 (exceeds $100,000 balance)
	amount := testutil.MustNumeric("102500.00")
	currentYield := testutil.MustNumeric("4.50")
	_, err = service.BuyTreasury(ctx, testUser.ID, "6M", amount, currentYield)

	if err == nil {
		t.Fatal("Expected insufficient balance error, got nil")
	}

	holdings, _ := queries.GetHoldingsByUser(ctx, testUser.ID)
	if len(holdings) != 0 {
		t.Errorf("Expected 0 holdings after failed transaction, got %d", len(holdings))
	}

	transactions, _ := queries.GetTransactionsByUser(ctx, testUser.ID)
	if len(transactions) != 0 {
		t.Errorf("Expected 0 transactions after failed transaction, got %d", len(transactions))
	}

	user, _ := queries.GetUser(ctx, testUser.ID)
	if testutil.MustFloat64(user.Balance) != 100000.00 {
		t.Errorf("Expected balance unchanged at 100000.00, got %f", testutil.MustFloat64(user.Balance))
	}
}
