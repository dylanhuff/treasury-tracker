# Year-Based Treasury Yield Storage Design

## Overview

Redesign treasury yield storage from a flat single table to PostgreSQL range-partitioned
table organized by year. Includes parallel cache warming, pre-insertion sampling,
weekly data thinning, and automatic partition management. This is a hard cutover —
all legacy code is removed.

## Goals

1. Store yield data by year using PostgreSQL partitioning
2. Parallelize cache warming (one goroutine per year, all concurrent)
3. Sample data before DB insertion (not at query time)
4. Keep data fresh with daily refresh
5. Thin aging data weekly via a background goroutine
6. Auto-create partitions for upcoming years via the daily refresh ticker
7. Prevent redundant treasury.gov fetches during warmup

## Database Schema

### Partitioned Parent Table

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
    bc_30year NUMERIC(6, 3)
) PARTITION BY RANGE (date);
```

The parent table holds no data directly. Each year gets a partition:

```sql
CREATE TABLE treasury_yields_2024 PARTITION OF treasury_yields
    FOR VALUES FROM ('2024-01-01') TO ('2025-01-01');
```

Partitions are created dynamically by Go code via `ensurePartitionExists()`, which
uses `CREATE TABLE IF NOT EXISTS ... PARTITION OF ...` for idempotency.

All existing queries (GetYieldsByDateRange, GetLatestYield, UpsertTreasuryYield)
work unchanged against the parent table. PostgreSQL handles partition routing and
pruning automatically.

### New SQL Queries

```sql
-- Get which years already have data (for smart startup)
-- name: GetDistinctYieldYears :many
SELECT DISTINCT EXTRACT(YEAR FROM date)::int AS year
FROM treasury_yields ORDER BY year;

-- Delete non-weekly samples for data older than threshold
-- name: DeleteNonWeeklySamples :exec
DELETE FROM treasury_yields
WHERE date < $1
  AND date NOT IN (
    SELECT DISTINCT ON (date_trunc('week', date)) date
    FROM treasury_yields
    WHERE date < $1
    ORDER BY date_trunc('week', date), date DESC
  );

-- Delete non-monthly samples for data older than threshold
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

## Parallel Cache Warming (SyncYields)

On startup, SyncYields:

1. Queries DB for which years already have data (`GetDistinctYieldYears`)
2. Skips years that already exist; always fetches current year
3. Launches one goroutine per missing year via `errgroup` (no concurrency limit)
4. Each goroutine:
   a. Creates the year's partition if missing (`ensurePartitionExists`)
   b. Fetches XML from treasury.gov for that year
   c. Applies age-based sampling to filter rows before insertion (`filterByAge`)
   d. Inserts only the surviving rows into the DB
5. Closes the `ready` channel when all goroutines complete

On a fresh DB: ~31 concurrent goroutines. On restart with data: only current year.

### Ready Channel (Concurrency Gate)

```go
type TreasuryService struct {
    ready   chan struct{} // closed when warmup completes
    queries *database.Queries
    pool    *pgxpool.Pool
    // ...
}
```

- `SyncYields()` calls `defer close(s.ready)` at the start
- `StartRefreshTicker()` and `StartWeeklySampler()` both `<-s.ready` before
  entering their loops
- API requests query the DB directly (may get partial data during warmup,
  never trigger treasury.gov fetches)
- After warmup, `<-s.ready` returns instantly (closed channels never block)

## Pre-Insertion Sampling (filterByAge)

Each cache warming goroutine applies sampling before inserting rows:

| Row Age        | Resolution | Logic                              |
|----------------|------------|------------------------------------|
| <= 1 year      | Daily      | Keep all rows                      |
| 1Y to 5Y       | Weekly     | Keep latest row per ISO week       |
| > 5 years      | Monthly    | Keep latest row per calendar month |

A single year can span multiple thresholds. For example, in Feb 2026, year 2025
data from Jan 2025 is >1Y old (weekly) while data from Mar 2025 is <1Y old (daily).
The per-row age check handles this correctly.

This replaces the current `sampleDataPoints()` function which is deleted.
`GetHistoricalYields()` returns data directly from the DB with no post-processing.

## Background Goroutines

### 1. Daily Refresh Ticker (modified)

- Waits on `<-s.ready` before starting
- Runs every hour
- Fetches current year data from treasury.gov (picks up new daily yields)
- Checks if next year is within 30 days — creates partition if so

### 2. Weekly Sampler (new)

- Waits on `<-s.ready` before starting
- Runs every 7 days
- Thins data that has aged past sampling thresholds:
  - Phase 1: data now >1Y old → delete all except latest per ISO week
  - Phase 2: data now >5Y old → delete all except latest per month
- Uses `DeleteNonWeeklySamples` and `DeleteNonMonthlySamples` queries
- Destructive: raw daily rows are permanently removed for old data

### Goroutine Summary

| Goroutine       | Lifecycle    | Interval | Purpose                                      |
|-----------------|-------------|----------|----------------------------------------------|
| SyncYields      | Startup only | Once     | Parallel fetch + sample + insert all years   |
| Refresh ticker  | Persistent   | Hourly   | Fetch current year + partition check         |
| Weekly sampler  | Persistent   | Weekly   | Thin old data past sampling thresholds       |

All respect `ctx.Done()` for graceful shutdown.

## Query Layer

`GetHistoricalYields()` simplifies to a date range query with no sampling:

1. `calculateDateRange(period)` computes start/end dates (unchanged)
2. `GetYieldsByDateRange(start, end)` queries the partitioned table
3. PostgreSQL prunes irrelevant partitions automatically
4. Returns rows directly — DB already has the right density

## API Contract

No changes. Frontend is unaffected.

- `GET /api/yields` — latest yield (unchanged)
- `GET /api/yields/historical?period=5Y` — historical by period (unchanged)

## Hard Cutover — Removed Code

- `sampleDataPoints()` — deleted entirely
- Old sequential `SyncYields()` loop — replaced with errgroup parallel version
- Old `StartRefreshTicker()` without ready channel — rewritten
- `deploy-fresh-schema.sh` — updated to create partitioned parent table
- Any tests referencing old sampling or sequential warmup — rewritten

No feature flags, no fallback paths, no backwards-compatibility shims.

## Files Changed

| File | Change |
|------|--------|
| `db/schema.sql` | `treasury_yields` becomes `PARTITION BY RANGE (date)` |
| `db/queries/yields.sql` | Add `GetDistinctYieldYears`, `DeleteNonWeeklySamples`, `DeleteNonMonthlySamples` |
| `internal/services/treasury.go` | Parallel warmup, ready channel, filterByAge, StartWeeklySampler, maybeCreateNextYearPartition, remove sampleDataPoints |
| `internal/database/*.go` | Regenerated by sqlc |
| `cmd/server/main.go` | Call StartWeeklySampler after StartRefreshTicker |
| `internal/services/interfaces.go` | Update interface if method signatures change |
| `db/deploy-fresh-schema.sh` | Create partitioned parent table |
| Tests | Rewrite to match new architecture |
