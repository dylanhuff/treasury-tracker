package services

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"treasury-tracker/internal/database"
)

func setupTreasuryTestService(t *testing.T) (*TreasuryService, *database.Queries, func()) {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testConnString)
	if err != nil {
		t.Skipf("Skipping integration test: database not available: %v", err)
	}

	// Ensure the treasury_yields table exists.
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS treasury_yields (
			date        DATE PRIMARY KEY,
			bc_1month   NUMERIC,
			bc_3month   NUMERIC,
			bc_6month   NUMERIC,
			bc_1year    NUMERIC,
			bc_2year    NUMERIC,
			bc_5year    NUMERIC,
			bc_10year   NUMERIC,
			bc_30year   NUMERIC
		)
	`)
	if err != nil {
		t.Skipf("Skipping integration test: cannot ensure treasury_yields table: %v", err)
	}

	queries := database.New(pool)
	service := NewTreasuryService(queries, pool, zap.NewNop())

	return service, queries, func() { pool.Close() }
}

func TestSyncYields(t *testing.T) {
	service, _, cleanup := setupTreasuryTestService(t)
	defer cleanup()

	ctx := context.Background()

	// SyncYields fetches from the real treasury.gov API.
	if err := service.SyncYields(ctx); err != nil {
		t.Fatalf("SyncYields failed: %v", err)
	}

	// After sync, GetLatestYields should return data.
	yieldData, err := service.GetLatestYields(ctx)
	if err != nil {
		t.Fatalf("GetLatestYields after sync failed: %v", err)
	}

	if yieldData == nil {
		t.Fatal("Expected non-nil yield data after sync")
	}
	if yieldData.Date == "" {
		t.Error("Expected non-empty date in yield data")
	}
	if len(yieldData.Yields) != 8 {
		t.Errorf("Expected 8 yield points, got %d", len(yieldData.Yields))
	}

	// Verify each yield point has a valid term.
	expectedTerms := map[string]bool{
		"1M": true, "3M": true, "6M": true, "1Y": true,
		"2Y": true, "5Y": true, "10Y": true, "30Y": true,
	}
	for _, yp := range yieldData.Yields {
		if !expectedTerms[yp.Term] {
			t.Errorf("Unexpected term: %s", yp.Term)
		}
	}
}

func TestGetHistoricalYields_AfterSync(t *testing.T) {
	service, _, cleanup := setupTreasuryTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Sync first to ensure we have data.
	if err := service.SyncYields(ctx); err != nil {
		t.Fatalf("SyncYields failed: %v", err)
	}

	// Test multiple periods.
	periods := []string{"1W", "1M", "3M"}
	for _, period := range periods {
		t.Run(period, func(t *testing.T) {
			data, err := service.GetHistoricalYields(ctx, period)
			if err != nil {
				t.Fatalf("GetHistoricalYields(%s) failed: %v", period, err)
			}

			if data == nil {
				t.Fatalf("Expected non-nil historical data for period %s", period)
			}
			if data.Period != period {
				t.Errorf("Expected period '%s', got '%s'", period, data.Period)
			}
			if data.StartDate == "" || data.EndDate == "" {
				t.Error("Expected non-empty start and end dates")
			}
			if len(data.Terms) == 0 {
				t.Error("Expected at least one term in historical data")
			}

			// For recent periods, we expect at least some data points (assuming
			// the treasury.gov sync returned data for the current year).
			t.Logf("Period %s: %d data points from %s to %s", period, len(data.Data), data.StartDate, data.EndDate)
		})
	}
}
