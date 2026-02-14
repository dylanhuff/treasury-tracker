package services

import (
	"context"
	"fmt"
	"testing"
	"time"

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

	// Drop and recreate as partitioned table for test isolation.
	_, _ = pool.Exec(ctx, "DROP TABLE IF EXISTS treasury_yields CASCADE")
	_, err = pool.Exec(ctx, `
		CREATE TABLE treasury_yields (
			date        DATE NOT NULL,
			bc_1month   NUMERIC,
			bc_3month   NUMERIC,
			bc_6month   NUMERIC,
			bc_1year    NUMERIC,
			bc_2year    NUMERIC,
			bc_5year    NUMERIC,
			bc_10year   NUMERIC,
			bc_30year   NUMERIC,
			PRIMARY KEY (date)
		) PARTITION BY RANGE (date)
	`)
	if err != nil {
		t.Skipf("Skipping integration test: cannot create partitioned table: %v", err)
	}

	// Create partitions for the last 31 years + next year.
	currentYear := time.Now().Year()
	for y := currentYear - 30; y <= currentYear+1; y++ {
		_, err := pool.Exec(ctx, fmt.Sprintf(
			"CREATE TABLE IF NOT EXISTS treasury_yields_%d PARTITION OF treasury_yields FOR VALUES FROM ('%d-01-01') TO ('%d-01-01')",
			y, y, y+1,
		))
		if err != nil {
			t.Skipf("Skipping integration test: cannot create partition for %d: %v", y, err)
		}
	}

	queries := database.New(pool)
	service := NewTreasuryService(queries, pool, zap.NewNop())

	return service, queries, func() {
		pool.Exec(context.Background(), "DROP TABLE IF EXISTS treasury_yields CASCADE")
		pool.Close()
	}
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
