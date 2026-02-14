package services

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"

	"treasury-tracker/internal/database"
	"treasury-tracker/internal/models"
)

const (
	treasuryURLTemplate = "https://home.treasury.gov/resource-center/data-chart-center/interest-rates/pages/xml?data=daily_treasury_yield_curve&field_tdr_date_value=%d"
	httpTimeout         = 30 * time.Second
	iso8601DateLength   = 10
)

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

// ensurePartitionExists creates a yearly partition table if it does not already exist.
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

// maybeCreateNextYearPartition proactively creates next year's partition
// when we are within 30 days of January 1st.
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

// filterByAge applies age-based sampling to XML entries before DB insertion:
//   - Row age <= 1Y: keep all (daily)
//   - Row age 1-5Y: keep latest per ISO week
//   - Row age > 5Y: keep latest per calendar month
func filterByAge(entries []models.Entry, now time.Time) []models.Entry {
	if len(entries) == 0 {
		return entries
	}

	oneYearAgo := now.AddDate(-1, 0, 0)
	fiveYearsAgo := now.AddDate(-5, 0, 0)

	var daily []models.Entry
	weeklyMap := make(map[string]models.Entry)
	monthlyMap := make(map[string]models.Entry)

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
				existDate := existing.Date
				if len(existDate) > iso8601DateLength {
					existDate = existDate[:iso8601DateLength]
				}
				if dateStr > existDate {
					weeklyMap[key] = entry
				}
			} else {
				weeklyMap[key] = entry
			}
		} else {
			key := date.Format("2006-01")
			if existing, ok := monthlyMap[key]; ok {
				existDate := existing.Date
				if len(existDate) > iso8601DateLength {
					existDate = existDate[:iso8601DateLength]
				}
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

// SyncYields fetches treasury data for all years in parallel, skipping years
// that already have data (except the current year which is always refreshed).
// It closes the ready channel when complete to unblock background goroutines.
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

	g := new(errgroup.Group)

	for year := startYear; year <= currentYear; year++ {
		y := year
		if y != currentYear && existing[int32(y)] {
			s.logger.Debug("skipping year with existing data", zap.Int("year", y))
			continue
		}

		g.Go(func() error {
			if err := s.ensurePartitionExists(ctx, y); err != nil {
				s.logger.Error("failed to create partition", zap.Int("year", y), zap.Error(err))
				return nil // don't fail other goroutines
			}
			if err := s.fetchAndStoreYear(ctx, y); err != nil {
				s.logger.Error("failed to fetch year", zap.Int("year", y), zap.Error(err))
				return nil // don't fail other goroutines
			}
			return nil
		})
	}

	g.Wait()
	return nil
}

// fetchAndStoreYear fetches one year of data from treasury.gov, applies
// age-based filtering, and upserts into the DB.
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

// GetHistoricalYields queries the DB for yields in the given period and converts
// to model types. No query-time sampling is needed since data is pre-sampled
// at insertion time and by the weekly sampler.
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

// GetLatestYields queries the DB for the most recent yield row and converts to models.YieldData.
func (s *TreasuryService) GetLatestYields(ctx context.Context) (*models.YieldData, error) {
	row, err := s.queries.GetLatestYield(ctx)
	if err != nil {
		return nil, fmt.Errorf("get latest yield: %w", err)
	}

	return &models.YieldData{
		Date: row.Date.Time.Format("2006-01-02"),
		Yields: []models.YieldPoint{
			{Term: "1M", Rate: numericToDecimalSafe(row.Bc1month)},
			{Term: "3M", Rate: numericToDecimalSafe(row.Bc3month)},
			{Term: "6M", Rate: numericToDecimalSafe(row.Bc6month)},
			{Term: "1Y", Rate: numericToDecimalSafe(row.Bc1year)},
			{Term: "2Y", Rate: numericToDecimalSafe(row.Bc2year)},
			{Term: "5Y", Rate: numericToDecimalSafe(row.Bc5year)},
			{Term: "10Y", Rate: numericToDecimalSafe(row.Bc10year)},
			{Term: "30Y", Rate: numericToDecimalSafe(row.Bc30year)},
		},
	}, nil
}

// StartRefreshTicker starts a goroutine that fetches the current year's data hourly.
// It waits for the initial warmup to complete before starting the ticker.
func (s *TreasuryService) StartRefreshTicker(ctx context.Context) {
	go func() {
		<-s.ready
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

// StartWeeklySampler starts a goroutine that thins old data on a weekly schedule.
// It waits for the initial warmup to complete before starting.
func (s *TreasuryService) StartWeeklySampler(ctx context.Context) {
	go func() {
		<-s.ready
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

// sampleOldData deletes non-representative rows for old data:
//   - Data older than 5 years: keep only one row per month
//   - Data older than 1 year: keep only one row per week
func (s *TreasuryService) sampleOldData(ctx context.Context) {
	now := time.Now()

	fiveYearsAgo := pgtype.Date{Time: now.AddDate(-5, 0, 0), Valid: true}
	if err := s.queries.DeleteNonMonthlySamples(ctx, fiveYearsAgo); err != nil {
		s.logger.Error("monthly sampling failed", zap.Error(err))
	} else {
		s.logger.Info("monthly sampling complete")
	}

	oneYearAgo := pgtype.Date{Time: now.AddDate(-1, 0, 0), Valid: true}
	if err := s.queries.DeleteNonWeeklySamples(ctx, oneYearAgo); err != nil {
		s.logger.Error("weekly sampling failed", zap.Error(err))
	} else {
		s.logger.Info("weekly sampling complete")
	}
}

// numericFromFloat converts a float64 to pgtype.Numeric.
func numericFromFloat(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	if err := n.Scan(fmt.Sprintf("%.3f", f)); err != nil {
		zap.L().Error("failed to scan numeric from float", zap.Float64("value", f), zap.Error(err))
	}
	return n
}

// numericToDecimalSafe converts a pgtype.Numeric to decimal.Decimal without
// intermediate float64 conversion, returning Zero on error.
func numericToDecimalSafe(n pgtype.Numeric) decimal.Decimal {
	if !n.Valid || n.NaN || n.Int == nil {
		return decimal.Zero
	}
	return decimal.NewFromBigInt(n.Int, n.Exp)
}

func calculateDateRange(period string) (startDate, endDate time.Time, err error) {
	endDate = time.Now()

	switch period {
	case "1W":
		startDate = endDate.AddDate(0, 0, -7)
	case "1M":
		startDate = endDate.AddDate(0, -1, 0)
	case "3M":
		startDate = endDate.AddDate(0, -3, 0)
	case "6M":
		startDate = endDate.AddDate(0, -6, 0)
	case "1Y":
		startDate = endDate.AddDate(-1, 0, 0)
	case "5Y":
		startDate = endDate.AddDate(-5, 0, 0)
	case "10Y":
		startDate = endDate.AddDate(-10, 0, 0)
	case "30Y":
		startDate = endDate.AddDate(-30, 0, 0)
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("invalid period: %s", period)
	}

	return startDate, endDate, nil
}
