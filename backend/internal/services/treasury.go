package services

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
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
}

func NewTreasuryService(queries *database.Queries, pool *pgxpool.Pool, logger *zap.Logger) *TreasuryService {
	return &TreasuryService{
		queries:    queries,
		pool:       pool,
		httpClient: &http.Client{Timeout: httpTimeout},
		logger:     logger,
	}
}

// SyncYields checks the max date in the DB and fetches any missing years from the API.
// If the DB is empty (max date is 1900-01-01), it starts from 30 years ago.
func (s *TreasuryService) SyncYields(ctx context.Context) error {
	maxDateRaw, err := s.queries.GetMaxYieldDate(ctx)
	if err != nil {
		return fmt.Errorf("get max yield date: %w", err)
	}

	var maxDate time.Time
	switch v := maxDateRaw.(type) {
	case time.Time:
		maxDate = v
	case pgtype.Date:
		if v.Valid {
			maxDate = v.Time
		} else {
			maxDate = time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
		}
	default:
		return fmt.Errorf("unexpected type for max_date: %T", maxDateRaw)
	}

	sentinel := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	now := time.Now()

	var startYear int
	if maxDate.Equal(sentinel) || maxDate.Before(sentinel.AddDate(0, 0, 1)) {
		startYear = now.Year() - 30
		s.logger.Info("DB empty, syncing from 30 years ago", zap.Int("startYear", startYear))
	} else {
		startYear = maxDate.Year()
		s.logger.Info("Syncing from max date year", zap.Time("maxDate", maxDate), zap.Int("startYear", startYear))
	}

	endYear := now.Year()

	failures := 0
	total := 0
	for year := startYear; year <= endYear; year++ {
		total++
		if err := s.fetchAndStoreYear(ctx, year); err != nil {
			s.logger.Error("failed to fetch year", zap.Int("year", year), zap.Error(err))
			failures++
		}
	}

	if failures > 0 && failures == total {
		return fmt.Errorf("sync failed for all %d years", total)
	}
	return nil
}

// fetchAndStoreYear fetches one year of data from treasury.gov and upserts into the DB.
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

		s.logger.Info("Parsed entries for year", zap.Int("year", year), zap.Int("count", len(feed.Entries)))

		for _, entry := range feed.Entries {
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

// GetHistoricalYields queries the DB for yields in the given period, converts to model types,
// and applies density reduction via sampleDataPoints.
func (s *TreasuryService) GetHistoricalYields(period string) (*models.HistoricalYieldData, error) {
	ctx := context.Background()

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
			Yield2Y:  numericToFloat(row.Bc2year),
			Yield5Y:  numericToFloat(row.Bc5year),
			Yield10Y: numericToFloat(row.Bc10year),
		})
	}

	return &models.HistoricalYieldData{
		Period:    period,
		StartDate: startDate.Format("2006-01-02"),
		EndDate:   endDate.Format("2006-01-02"),
		Terms:     []string{"10Y", "5Y", "2Y"},
		Data:      sampleDataPoints(dataPoints, period),
	}, nil
}

// GetLatestYields queries the DB for the most recent yield row and converts to models.YieldData.
func (s *TreasuryService) GetLatestYields() (*models.YieldData, error) {
	ctx := context.Background()

	row, err := s.queries.GetLatestYield(ctx)
	if err != nil {
		return nil, fmt.Errorf("get latest yield: %w", err)
	}

	return &models.YieldData{
		Date: row.Date.Time.Format("2006-01-02"),
		Yields: []models.YieldPoint{
			{Term: "1M", Rate: numericToFloat(row.Bc1month)},
			{Term: "3M", Rate: numericToFloat(row.Bc3month)},
			{Term: "6M", Rate: numericToFloat(row.Bc6month)},
			{Term: "1Y", Rate: numericToFloat(row.Bc1year)},
			{Term: "2Y", Rate: numericToFloat(row.Bc2year)},
			{Term: "5Y", Rate: numericToFloat(row.Bc5year)},
			{Term: "10Y", Rate: numericToFloat(row.Bc10year)},
			{Term: "30Y", Rate: numericToFloat(row.Bc30year)},
		},
	}, nil
}

// StartRefreshTicker starts a goroutine that fetches the current year's data hourly.
// It respects context cancellation for graceful shutdown.
func (s *TreasuryService) StartRefreshTicker(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				s.logger.Info("Refresh ticker stopped")
				return
			case <-ticker.C:
				s.logger.Info("Hourly refresh: fetching current year data")
				if err := s.fetchAndStoreYear(ctx, time.Now().Year()); err != nil {
					s.logger.Error("Hourly refresh failed", zap.Error(err))
				}
			}
		}
	}()
}

// numericFromFloat converts a float64 to pgtype.Numeric.
func numericFromFloat(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	n.Scan(fmt.Sprintf("%.3f", f))
	return n
}

// numericToFloat converts a pgtype.Numeric to float64.
func numericToFloat(n pgtype.Numeric) float64 {
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
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

// sampleDataPoints reduces density for long periods (30Y: monthly, 10Y/5Y: weekly).
func sampleDataPoints(dataPoints []models.YieldDataPoint, period string) []models.YieldDataPoint {
	switch period {
	case "1W", "1M", "3M", "6M", "1Y":
		return dataPoints
	}

	if len(dataPoints) == 0 {
		return dataPoints
	}

	var samplingInterval int
	switch period {
	case "30Y":
		samplingInterval = 30
	case "10Y", "5Y":
		samplingInterval = 7
	default:
		return dataPoints
	}

	intervalMap := make(map[string]models.YieldDataPoint)

	for _, point := range dataPoints {
		date, err := time.Parse("2006-01-02", point.Date)
		if err != nil {
			continue
		}

		var intervalKey string
		if samplingInterval == 30 {
			intervalKey = date.Format("2006-01")
		} else {
			year, week := date.ISOWeek()
			intervalKey = fmt.Sprintf("%d-W%02d", year, week)
		}

		if existing, exists := intervalMap[intervalKey]; exists {
			if point.Date > existing.Date {
				intervalMap[intervalKey] = point
			}
		} else {
			intervalMap[intervalKey] = point
		}
	}

	sampled := make([]models.YieldDataPoint, 0, len(intervalMap))
	for _, point := range intervalMap {
		sampled = append(sampled, point)
	}

	sort.Slice(sampled, func(i, j int) bool {
		return sampled[i].Date < sampled[j].Date
	})

	return sampled
}
