# Year-Based Treasury Yield Storage Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace flat treasury yield storage with PostgreSQL year-partitioned tables, parallel cache warming, pre-insertion sampling, and weekly data thinning.

**Architecture:** Partitioned `treasury_yields` table (one partition per year), `errgroup`-based parallel warmup with a `ready` channel gate, age-based `filterByAge` applied before DB insertion, weekly sampler goroutine for destructive thinning of aged data, and hourly partition check bundled with the refresh ticker. Hard cutover — all legacy code removed.

**Tech Stack:** Go 1.24, PostgreSQL (partitioning), pgx/v5, sqlc, errgroup, singleflight, zap

---

### Task 1: Update Database Schema for Partitioning

**Files:**
- Modify: `backend/db/schema.sql:66-79`

**Step 1: Update the treasury_yields table definition**

Replace the current `treasury_yields` table (lines 66-79 of `backend/db/schema.sql`) with a partitioned parent table. Remove the `PRIMARY KEY` from the column definition and add it as a table constraint (required for partitioned tables):

```sql
CREATE TABLE treasury_yields (
    date DATE NOT NULL,
    bc_1month NUMERIC(6, 3),
    bc_3month NUMERIC(6, 3),
    bc_6month NUMERIC(6, 3),
    bc_1year  NUMERIC(6, 3),
    bc_2year  NUMERIC(6, 3),
    bc_5year  NUMERIC(6, 3),
    bc_10year NUMERIC(6, 3),
    bc_30year NUMERIC(6, 3),
    PRIMARY KEY (date)
) PARTITION BY RANGE (date);

COMMENT ON TABLE treasury_yields IS 'Daily treasury yield curve data from treasury.gov, partitioned by year';
```

**Step 2: Commit**

```bash
git add backend/db/schema.sql
git commit -m "feat: convert treasury_yields to partitioned table by year"
```

---

### Task 2: Add New SQL Queries

**Files:**
- Modify: `backend/db/queries/yields.sql`

**Step 1: Add GetDistinctYieldYears, DeleteNonWeeklySamples, DeleteNonMonthlySamples**

Append these queries to `backend/db/queries/yields.sql` after the existing queries. Also remove the `GetMaxYieldDate` query (it is replaced by `GetDistinctYieldYears`):

Remove:
```sql
-- name: GetMaxYieldDate :one
SELECT COALESCE(MAX(date), '1900-01-01'::date) AS max_date
FROM treasury_yields;
```

Add:
```sql
-- name: GetDistinctYieldYears :many
SELECT DISTINCT EXTRACT(YEAR FROM date)::int AS year
FROM treasury_yields
ORDER BY year;

-- name: DeleteNonWeeklySamples :exec
DELETE FROM treasury_yields
WHERE date < $1
  AND date NOT IN (
    SELECT DISTINCT ON (date_trunc('week', date)) date
    FROM treasury_yields
    WHERE date < $1
    ORDER BY date_trunc('week', date), date DESC
  );

-- name: DeleteNonMonthlySamples :exec
DELETE FROM treasury_yields
WHERE date < $1
  AND date NOT IN (
    SELECT DISTINCT ON (date_trunc('month', date)) date
    FROM treasury_yields
    WHERE date < $1
    ORDER BY date_trunc('month', date), date DESC
  );
```

**Step 2: Run sqlc generate**

```bash
cd backend && sqlc generate
```

Verify: `internal/database/yields.sql.go` should contain new functions `GetDistinctYieldYears`, `DeleteNonWeeklySamples`, `DeleteNonMonthlySamples`, and no longer contain `GetMaxYieldDate`. `internal/database/querier.go` should reflect the same changes.

**Step 3: Commit**

```bash
git add backend/db/queries/yields.sql backend/internal/database/
git commit -m "feat: add year-based queries and remove GetMaxYieldDate"
```

---

### Task 3: Rewrite TreasuryService — Struct, Constructor, and Partition Management

**Files:**
- Modify: `backend/internal/services/treasury.go:28-43`

**Step 1: Update the TreasuryService struct and constructor**

Add the `ready` channel to the struct and initialize it in the constructor. The full struct and constructor should be:

```go
type TreasuryService struct {
	queries    *database.Queries
	pool       *pgxpool.Pool
	httpClient *http.Client
	logger     *zap.Logger
	sfGroup    singleflight.Group
	ready      chan struct{} // closed when warmup completes
}

func NewTreasuryService(queries *database.Queries, pool *pgxpool.Pool, logger *zap.Logger) *TreasuryService {
	return &TreasuryService{
		queries:    queries,
		pool:       pool,
		httpClient: &http.Client{Timeout: httpTimeout},
		logger:     logger,
		ready:      make(chan struct{}),
	}
}
```

**Step 2: Add ensurePartitionExists method**

Add this method to `treasury.go`. It uses raw SQL via the pool because partition DDL is dynamic and cannot go through sqlc:

```go
// ensurePartitionExists creates a year partition if it doesn't already exist.
// Uses IF NOT EXISTS for idempotency.
func (s *TreasuryService) ensurePartitionExists(ctx context.Context, year int) error {
	query := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS treasury_yields_%d PARTITION OF treasury_yields FOR VALUES FROM ('%d-01-01') TO ('%d-01-01')`,
		year, year, year+1,
	)
	_, err := s.pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("ensure partition for year %d: %w", year, err)
	}
	return nil
}
```

**Step 3: Add maybeCreateNextYearPartition method**

```go
// maybeCreateNextYearPartition creates next year's partition if we're within 30 days of Jan 1.
func (s *TreasuryService) maybeCreateNextYearPartition(ctx context.Context) {
	now := time.Now()
	nextYear := now.Year() + 1
	nextJan1 := time.Date(nextYear, 1, 1, 0, 0, 0, 0, time.UTC)

	if nextJan1.Sub(now) <= 30*24*time.Hour {
		if err := s.ensurePartitionExists(ctx, nextYear); err != nil {
			s.logger.Error("failed to create next year partition", zap.Int("year", nextYear), zap.Error(err))
		} else {
			s.logger.Info("ensured next year partition exists", zap.Int("year", nextYear))
		}
	}
}
```

**Step 4: Commit**

```bash
git add backend/internal/services/treasury.go
git commit -m "feat: add ready channel, partition management to TreasuryService"
```

---

### Task 4: Implement filterByAge

**Files:**
- Modify: `backend/internal/services/treasury.go`

**Step 1: Write unit test for filterByAge**

Create test file `backend/internal/services/filter_test.go`:

```go
package services

import (
	"testing"
	"time"

	"treasury-tracker/internal/models"
)

func TestFilterByAge_RecentDataKeptDaily(t *testing.T) {
	now := time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC)
	entries := []models.Entry{
		{Date: "2025-06-01T00:00:00", BC1Month: 4.5, BC10Year: 4.1},
		{Date: "2025-06-02T00:00:00", BC1Month: 4.5, BC10Year: 4.1},
		{Date: "2025-06-03T00:00:00", BC1Month: 4.5, BC10Year: 4.1},
	}
	result := filterByAge(entries, now)
	if len(result) != 3 {
		t.Errorf("expected 3 entries (all within 1Y), got %d", len(result))
	}
}

func TestFilterByAge_OldDataSampledWeekly(t *testing.T) {
	now := time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC)
	// Data from Jan 2024 — about 2 years old, should be weekly sampled
	entries := []models.Entry{
		{Date: "2024-01-01T00:00:00", BC10Year: 4.0}, // Mon (week 1)
		{Date: "2024-01-02T00:00:00", BC10Year: 4.1}, // Tue (week 1)
		{Date: "2024-01-03T00:00:00", BC10Year: 4.2}, // Wed (week 1)
		{Date: "2024-01-08T00:00:00", BC10Year: 4.3}, // Mon (week 2)
		{Date: "2024-01-09T00:00:00", BC10Year: 4.4}, // Tue (week 2)
	}
	result := filterByAge(entries, now)
	// Should keep 2 entries: latest per week (Jan 3 and Jan 9)
	if len(result) != 2 {
		t.Errorf("expected 2 entries (weekly sample), got %d", len(result))
	}
}

func TestFilterByAge_VeryOldDataSampledMonthly(t *testing.T) {
	now := time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC)
	// Data from 2019 — about 7 years old, should be monthly sampled
	entries := []models.Entry{
		{Date: "2019-01-02T00:00:00", BC10Year: 3.0},
		{Date: "2019-01-15T00:00:00", BC10Year: 3.1},
		{Date: "2019-01-30T00:00:00", BC10Year: 3.2},
		{Date: "2019-02-05T00:00:00", BC10Year: 3.3},
		{Date: "2019-02-20T00:00:00", BC10Year: 3.4},
	}
	result := filterByAge(entries, now)
	// Should keep 2 entries: latest per month (Jan 30 and Feb 20)
	if len(result) != 2 {
		t.Errorf("expected 2 entries (monthly sample), got %d", len(result))
	}
}

func TestFilterByAge_EmptyInput(t *testing.T) {
	now := time.Now()
	result := filterByAge(nil, now)
	if len(result) != 0 {
		t.Errorf("expected 0 entries for nil input, got %d", len(result))
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd backend && go test ./internal/services/ -run TestFilterByAge -v -short
```

Expected: compilation failure — `filterByAge` not defined.

**Step 3: Implement filterByAge**

Add to `backend/internal/services/treasury.go`:

```go
// filterByAge applies age-based sampling to entries before DB insertion.
// Recent data (<= 1Y) kept at daily granularity.
// Older data (1-5Y) sampled to weekly (latest per ISO week).
// Very old data (> 5Y) sampled to monthly (latest per calendar month).
func filterByAge(entries []models.Entry, now time.Time) []models.Entry {
	if len(entries) == 0 {
		return entries
	}

	oneYearAgo := now.AddDate(-1, 0, 0)
	fiveYearsAgo := now.AddDate(-5, 0, 0)

	var daily []models.Entry
	weeklyMap := make(map[string]models.Entry)  // ISO week key -> latest entry
	monthlyMap := make(map[string]models.Entry) // YYYY-MM key -> latest entry

	for _, entry := range entries {
		dateStr := entry.Date
		if len(dateStr) > iso8601DateLength {
			dateStr = dateStr[:iso8601DateLength]
		}
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		if date.After(oneYearAgo) || date.Equal(oneYearAgo) {
			daily = append(daily, entry)
		} else if date.After(fiveYearsAgo) {
			year, week := date.ISOWeek()
			key := fmt.Sprintf("%d-W%02d", year, week)
			if existing, ok := weeklyMap[key]; ok {
				existDate := existing.Date[:iso8601DateLength]
				if dateStr > existDate {
					weeklyMap[key] = entry
				}
			} else {
				weeklyMap[key] = entry
			}
		} else {
			key := date.Format("2006-01")
			if existing, ok := monthlyMap[key]; ok {
				existDate := existing.Date[:iso8601DateLength]
				if dateStr > existDate {
					monthlyMap[key] = entry
				}
			} else {
				monthlyMap[key] = entry
			}
		}
	}

	result := make([]models.Entry, 0, len(daily)+len(weeklyMap)+len(monthlyMap))
	result = append(result, daily...)
	for _, e := range weeklyMap {
		result = append(result, e)
	}
	for _, e := range monthlyMap {
		result = append(result, e)
	}
	return result
}
```

**Step 4: Run test to verify it passes**

```bash
cd backend && go test ./internal/services/ -run TestFilterByAge -v -short
```

Expected: all 4 tests PASS.

**Step 5: Commit**

```bash
git add backend/internal/services/treasury.go backend/internal/services/filter_test.go
git commit -m "feat: add filterByAge for pre-insertion sampling"
```

---

### Task 5: Rewrite SyncYields — Parallel Warmup with Smart Skip

**Files:**
- Modify: `backend/internal/services/treasury.go:47-95` (replace SyncYields)
- Modify: `backend/internal/services/treasury.go:99-166` (update fetchAndStoreYear)

**Step 1: Rewrite SyncYields**

Replace the existing `SyncYields` method (lines 47-95) with the parallel version. Add `"golang.org/x/sync/errgroup"` to imports (already in go.mod):

```go
// SyncYields creates partitions and fetches missing years in parallel.
// Skips years that already have data in the DB. Always fetches current year.
// Closes the ready channel when complete.
func (s *TreasuryService) SyncYields(ctx context.Context) error {
	defer close(s.ready)

	currentYear := time.Now().Year()
	startYear := currentYear - 30

	existingYears, err := s.queries.GetDistinctYieldYears(ctx)
	if err != nil {
		s.logger.Warn("failed to get existing years, will fetch all", zap.Error(err))
		existingYears = nil
	}

	existing := make(map[int32]bool, len(existingYears))
	for _, y := range existingYears {
		existing[y] = true
	}

	g, gCtx := errgroup.WithContext(ctx)

	for year := startYear; year <= currentYear; year++ {
		y := year
		if y != currentYear && existing[int32(y)] {
			s.logger.Debug("skipping year with existing data", zap.Int("year", y))
			continue
		}

		g.Go(func() error {
			if err := s.ensurePartitionExists(gCtx, y); err != nil {
				return err
			}
			return s.fetchAndStoreYear(gCtx, y)
		})
	}

	if err := g.Wait(); err != nil {
		s.logger.Error("sync yields completed with errors", zap.Error(err))
	}
	return nil
}
```

Note: `SyncYields` does NOT return the errgroup error as fatal — it logs it and proceeds. This matches the previous behavior where partial failures were tolerated. The `ready` channel is always closed via `defer` regardless of errors.

**Step 2: Update fetchAndStoreYear to call filterByAge before insertion**

Replace the existing `fetchAndStoreYear` method (lines 99-166) with:

```go
// fetchAndStoreYear fetches one year of data from treasury.gov, applies age-based
// sampling, and upserts surviving rows into the DB.
// Wrapped in singleflight to prevent duplicate concurrent fetches of the same year.
func (s *TreasuryService) fetchAndStoreYear(ctx context.Context, year int) error {
	key := fmt.Sprintf("fetch_year_%d", year)

	_, err, _ := s.sfGroup.Do(key, func() (interface{}, error) {
		s.logger.Info("Fetching treasury data for year", zap.Int("year", year))

		url := fmt.Sprintf(treasuryURLTemplate, year)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("create request for year %d: %w", year, err)
		}

		resp, err := s.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetch year %d: %w", year, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("treasury API returned %d for year %d", resp.StatusCode, year)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read body year %d: %w", year, err)
		}

		var feed models.TreasuryFeed
		if err := xml.Unmarshal(body, &feed); err != nil {
			return nil, fmt.Errorf("parse XML year %d: %w", year, err)
		}

		filtered := filterByAge(feed.Entries, time.Now())
		s.logger.Info("Parsed and filtered entries for year",
			zap.Int("year", year),
			zap.Int("raw", len(feed.Entries)),
			zap.Int("filtered", len(filtered)),
		)

		for _, entry := range filtered {
			dateStr := entry.Date
			if len(dateStr) > iso8601DateLength {
				dateStr = dateStr[:iso8601DateLength]
			}

			parsed, err := time.Parse("2006-01-02", dateStr)
			if err != nil {
				s.logger.Warn("Skipping entry with bad date", zap.String("date", dateStr), zap.Error(err))
				continue
			}

			params := database.UpsertTreasuryYieldParams{
				Date:     pgtype.Date{Time: parsed, Valid: true},
				Bc1month: numericFromFloat(entry.BC1Month),
				Bc3month: numericFromFloat(entry.BC3Month),
				Bc6month: numericFromFloat(entry.BC6Month),
				Bc1year:  numericFromFloat(entry.BC1Year),
				Bc2year:  numericFromFloat(entry.BC2Year),
				Bc5year:  numericFromFloat(entry.BC5Year),
				Bc10year: numericFromFloat(entry.BC10Year),
				Bc30year: numericFromFloat(entry.BC30Year),
			}

			if err := s.queries.UpsertTreasuryYield(ctx, params); err != nil {
				s.logger.Warn("Failed to upsert yield", zap.String("date", dateStr), zap.Error(err))
			}
		}

		return nil, nil
	})

	return err
}
```

**Step 3: Add errgroup import**

Ensure the import block includes:
```go
"golang.org/x/sync/errgroup"
```

**Step 4: Commit**

```bash
git add backend/internal/services/treasury.go
git commit -m "feat: rewrite SyncYields for parallel warmup with smart skip"
```

---

### Task 6: Rewrite Background Goroutines

**Files:**
- Modify: `backend/internal/services/treasury.go:225-245` (replace StartRefreshTicker)

**Step 1: Rewrite StartRefreshTicker with ready gate and partition check**

Replace lines 225-245 with:

```go
// StartRefreshTicker starts a goroutine that fetches the current year's data hourly
// and checks whether next year's partition needs to be created.
// Waits for warmup to complete before starting.
func (s *TreasuryService) StartRefreshTicker(ctx context.Context) {
	go func() {
		<-s.ready // wait for warmup to complete
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				s.logger.Info("Refresh ticker stopped")
				return
			case <-ticker.C:
				s.logger.Info("Hourly refresh: fetching current year data")
				currentYear := time.Now().Year()
				if err := s.ensurePartitionExists(ctx, currentYear); err != nil {
					s.logger.Error("failed to ensure current year partition", zap.Int("year", currentYear), zap.Error(err))
				}
				if err := s.fetchAndStoreYear(ctx, currentYear); err != nil {
					s.logger.Error("Hourly refresh failed", zap.Error(err))
				}
				s.maybeCreateNextYearPartition(ctx)
			}
		}
	}()
}
```

**Step 2: Add StartWeeklySampler**

Add new method:

```go
// StartWeeklySampler starts a goroutine that runs weekly to thin out old data.
// Data older than 1 year is reduced to weekly granularity.
// Data older than 5 years is reduced to monthly granularity.
// Waits for warmup to complete before starting.
func (s *TreasuryService) StartWeeklySampler(ctx context.Context) {
	go func() {
		<-s.ready // wait for warmup to complete
		ticker := time.NewTicker(7 * 24 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				s.logger.Info("Weekly sampler stopped")
				return
			case <-ticker.C:
				s.logger.Info("Weekly sampler: thinning old data")
				s.sampleOldData(ctx)
			}
		}
	}()
}

// sampleOldData deletes excess rows for data that has aged past sampling thresholds.
// Phase 1: data > 5Y old → keep only latest per month (run first to avoid re-processing)
// Phase 2: data 1-5Y old → keep only latest per ISO week
func (s *TreasuryService) sampleOldData(ctx context.Context) {
	now := time.Now()

	// Phase 1: monthly sampling for data > 5 years old
	fiveYearsAgo := pgtype.Date{Time: now.AddDate(-5, 0, 0), Valid: true}
	if err := s.queries.DeleteNonMonthlySamples(ctx, fiveYearsAgo); err != nil {
		s.logger.Error("monthly sampling failed", zap.Error(err))
	} else {
		s.logger.Info("monthly sampling complete")
	}

	// Phase 2: weekly sampling for data 1-5 years old
	// Only thin data older than 1Y (but the monthly pass already handled >5Y)
	oneYearAgo := pgtype.Date{Time: now.AddDate(-1, 0, 0), Valid: true}
	if err := s.queries.DeleteNonWeeklySamples(ctx, oneYearAgo); err != nil {
		s.logger.Error("weekly sampling failed", zap.Error(err))
	} else {
		s.logger.Info("weekly sampling complete")
	}
}
```

**Step 3: Commit**

```bash
git add backend/internal/services/treasury.go
git commit -m "feat: add StartWeeklySampler and rewrite StartRefreshTicker with ready gate"
```

---

### Task 7: Simplify GetHistoricalYields and Remove Legacy Code

**Files:**
- Modify: `backend/internal/services/treasury.go:168-201` (GetHistoricalYields)
- Delete: `sampleDataPoints` function (lines 292-348)
- Delete: `GetMaxYieldDate` references

**Step 1: Simplify GetHistoricalYields**

Replace lines 168-201 with (no more `sampleDataPoints` call):

```go
// GetHistoricalYields queries the DB for yields in the given period.
// Data is already at the correct density from pre-insertion sampling and weekly thinning.
func (s *TreasuryService) GetHistoricalYields(ctx context.Context, period string) (*models.HistoricalYieldData, error) {
	startDate, endDate, err := calculateDateRange(period)
	if err != nil {
		return nil, err
	}

	rows, err := s.queries.GetYieldsByDateRange(ctx, database.GetYieldsByDateRangeParams{
		Date:   pgtype.Date{Time: startDate, Valid: true},
		Date_2: pgtype.Date{Time: endDate, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("query yields by date range: %w", err)
	}

	dataPoints := make([]models.YieldDataPoint, 0, len(rows))
	for _, row := range rows {
		dataPoints = append(dataPoints, models.YieldDataPoint{
			Date:     row.Date.Time.Format("2006-01-02"),
			Yield2Y:  numericToDecimalSafe(row.Bc2year),
			Yield5Y:  numericToDecimalSafe(row.Bc5year),
			Yield10Y: numericToDecimalSafe(row.Bc10year),
		})
	}

	return &models.HistoricalYieldData{
		Period:    period,
		StartDate: startDate.Format("2006-01-02"),
		EndDate:   endDate.Format("2006-01-02"),
		Terms:     []string{"10Y", "5Y", "2Y"},
		Data:      dataPoints,
	}, nil
}
```

**Step 2: Delete sampleDataPoints entirely**

Remove the entire `sampleDataPoints` function (lines 292-348). This is the old query-time sampling logic — no longer needed.

**Step 3: Commit**

```bash
git add backend/internal/services/treasury.go
git commit -m "feat: simplify GetHistoricalYields, remove sampleDataPoints"
```

---

### Task 8: Update main.go

**Files:**
- Modify: `backend/cmd/server/main.go:87-91`

**Step 1: Add StartWeeklySampler call**

Replace lines 87-91 with:

```go
	treasuryService := services.NewTreasuryService(queries, pool, logger)
	if err := treasuryService.SyncYields(ctx); err != nil {
		logger.Error("initial yield sync failed", zap.Error(err))
	}
	treasuryService.StartRefreshTicker(ctx)
	treasuryService.StartWeeklySampler(ctx)
```

**Step 2: Commit**

```bash
git add backend/cmd/server/main.go
git commit -m "feat: start weekly sampler goroutine on startup"
```

---

### Task 9: Update deploy-fresh-schema.sh

**Files:**
- Modify: `backend/db/deploy-fresh-schema.sh`

**Step 1: Add treasury_yields to the verification step**

After the schema is applied (step 4), the verification section should also show the treasury_yields table status. Add after line 87:

```bash
echo -e "\n${GREEN}Treasury yields table (partitioned):${NC}"
run_psql -d "$DB_NAME" -c "\d+ treasury_yields"
```

No other changes needed — the schema.sql now creates the partitioned parent table, and Go code handles partition creation.

**Step 2: Commit**

```bash
git add backend/db/deploy-fresh-schema.sh
git commit -m "feat: add partition verification to deploy script"
```

---

### Task 10: Update Tests

**Files:**
- Modify: `backend/internal/services/treasury_test.go`
- Modify: `backend/internal/handlers/e2e_test.go`

**Step 1: Update setupTreasuryTestService in treasury_test.go**

The test setup currently creates a non-partitioned table. Update to create the partitioned table and a test partition:

```go
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

	// Create partitions for the last 31 years.
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
```

Add `"fmt"` and `"time"` to imports if not already present.

**Step 2: Update E2E test in e2e_test.go**

The E2E test calls `treasuryService.SyncYields(ctx)` which now closes the `ready` channel. This should work as-is since the `ready` channel is only closed once. No code changes needed — verify it compiles and the `SyncYields` call still works in the test context.

**Step 3: Run all tests**

```bash
cd backend && go test ./... -short -v
```

Expected: all unit tests pass. Integration tests skip (require DB).

**Step 4: Commit**

```bash
git add backend/internal/services/treasury_test.go backend/internal/handlers/e2e_test.go
git commit -m "test: update tests for partitioned table and new warmup logic"
```

---

### Task 11: Verify Build and Clean Up

**Files:**
- All modified files

**Step 1: Verify the project compiles**

```bash
cd backend && go build ./...
```

Expected: clean build with no errors.

**Step 2: Verify no references to removed code**

Search for any remaining references to `sampleDataPoints` or `GetMaxYieldDate`:

```bash
grep -r "sampleDataPoints\|GetMaxYieldDate" backend/
```

Expected: no matches.

**Step 3: Run go vet**

```bash
cd backend && go vet ./...
```

Expected: no issues.

**Step 4: Final commit if any cleanup was needed**

```bash
git add -A backend/
git commit -m "chore: clean up unused references after yield storage rewrite"
```

---

### Task Summary

| Task | Description | Files |
|------|-------------|-------|
| 1 | Schema: partitioned table | `db/schema.sql` |
| 2 | SQL queries: add new, remove old | `db/queries/yields.sql`, sqlc regen |
| 3 | Service struct: ready channel, partition mgmt | `services/treasury.go` |
| 4 | filterByAge: pre-insertion sampling | `services/treasury.go`, `services/filter_test.go` |
| 5 | SyncYields: parallel warmup with smart skip | `services/treasury.go` |
| 6 | Background goroutines: refresh + sampler | `services/treasury.go` |
| 7 | Simplify queries, delete legacy | `services/treasury.go` |
| 8 | main.go: wire up StartWeeklySampler | `cmd/server/main.go` |
| 9 | Deploy script: verify partitions | `db/deploy-fresh-schema.sh` |
| 10 | Tests: update for partitioned table | `services/treasury_test.go`, `handlers/e2e_test.go` |
| 11 | Build verification and cleanup | All |
