package services

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
	"treasury-tracker/internal/models"
)

const (
	treasuryURLTemplate  = "https://home.treasury.gov/resource-center/data-chart-center/interest-rates/pages/xml?data=daily_treasury_yield_curve&field_tdr_date_value=%d"
	httpTimeout = 30 * time.Second
	cacheDuration        = 1 * time.Hour
	iso8601DateLength    = 10
)

type historicalCacheEntry struct {
	data      *models.HistoricalYieldData
	timestamp time.Time
}

type TreasuryService struct {
	cacheData      *models.YieldData
	cacheTimestamp time.Time
	cacheDuration  time.Duration
	mu             sync.RWMutex
	httpClient     *http.Client
	logger         *zap.Logger

	historicalCache map[string]*historicalCacheEntry
	historicalMu    sync.RWMutex
}

var historicalPeriods = []string{"1W", "1M", "3M", "6M", "1Y", "5Y", "10Y", "30Y"}

func NewTreasuryService(logger *zap.Logger) *TreasuryService {
	return &TreasuryService{
		cacheDuration: cacheDuration,
		httpClient: &http.Client{
			Timeout: httpTimeout,
		},
		historicalCache: make(map[string]*historicalCacheEntry),
		logger:          logger,
	}
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

func (s *TreasuryService) fetchFromAPI() (*models.TreasuryFeed, error) {
	url := fmt.Sprintf(treasuryURLTemplate, time.Now().Year())
	resp, err := s.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch treasury data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("treasury API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var feed models.TreasuryFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("failed to parse XML: %w", err)
	}

	if len(feed.Entries) == 0 {
		return nil, fmt.Errorf("no entries found in treasury feed")
	}

	return &feed, nil
}

func (s *TreasuryService) fetchFromAPIForYears(startYear, endYear int) (*models.TreasuryFeed, error) {
	yearCount := endYear - startYear + 1

	type yearResult struct {
		year    int
		entries []models.Entry
		err     error
	}
	results := make(chan yearResult, yearCount)

	for year := startYear; year <= endYear; year++ {
		go func(y int) {
			url := fmt.Sprintf(treasuryURLTemplate, y)
			resp, err := s.httpClient.Get(url)
			if err != nil {
				results <- yearResult{year: y, err: fmt.Errorf("fetch year %d: %w", y, err)}
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				results <- yearResult{year: y, err: fmt.Errorf("treasury API returned %d for year %d", resp.StatusCode, y)}
				return
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				results <- yearResult{year: y, err: fmt.Errorf("read body year %d: %w", y, err)}
				return
			}

			var feed models.TreasuryFeed
			if err := xml.Unmarshal(body, &feed); err != nil {
				results <- yearResult{year: y, err: fmt.Errorf("parse XML year %d: %w", y, err)}
				return
			}

			results <- yearResult{year: y, entries: feed.Entries}
		}(year)
	}

	yearData := make(map[int][]models.Entry)
	var fetchErrors []error

	for i := 0; i < yearCount; i++ {
		result := <-results
		if result.err != nil {
			fetchErrors = append(fetchErrors, result.err)
		} else {
			yearData[result.year] = result.entries
		}
	}

	if len(fetchErrors) > 0 {
		return nil, fetchErrors[0]
	}

	var combinedFeed models.TreasuryFeed
	for year := startYear; year <= endYear; year++ {
		combinedFeed.Entries = append(combinedFeed.Entries, yearData[year]...)
	}

	if len(combinedFeed.Entries) == 0 {
		return nil, fmt.Errorf("no entries found for years %d-%d", startYear, endYear)
	}

	return &combinedFeed, nil
}

func (s *TreasuryService) convertToYieldData(feed *models.TreasuryFeed) (*models.YieldData, error) {
	if len(feed.Entries) == 0 {
		return nil, fmt.Errorf("no entries to convert")
	}

	entry := feed.Entries[len(feed.Entries)-1]

	date := entry.Date
	if len(date) > iso8601DateLength {
		date = date[:iso8601DateLength]
	}

	return &models.YieldData{
		Date: date,
		Yields: []models.YieldPoint{
			{Term: "1M", Rate: entry.BC1Month},
			{Term: "3M", Rate: entry.BC3Month},
			{Term: "6M", Rate: entry.BC6Month},
			{Term: "1Y", Rate: entry.BC1Year},
			{Term: "2Y", Rate: entry.BC2Year},
			{Term: "5Y", Rate: entry.BC5Year},
			{Term: "10Y", Rate: entry.BC10Year},
			{Term: "30Y", Rate: entry.BC30Year},
		},
	}, nil
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

func (s *TreasuryService) convertToHistoricalData(
	feed *models.TreasuryFeed,
	startDate, endDate time.Time,
	period string,
) (*models.HistoricalYieldData, error) {
	var dataPoints []models.YieldDataPoint

	for _, entry := range feed.Entries {
		dateStr := entry.Date
		if len(dateStr) > iso8601DateLength {
			dateStr = dateStr[:iso8601DateLength]
		}

		entryDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		if entryDate.Before(startDate) || entryDate.After(endDate) {
			continue
		}

		dataPoints = append(dataPoints, models.YieldDataPoint{
			Date:     dateStr,
			Yield10Y: entry.BC10Year,
			Yield5Y:  entry.BC5Year,
			Yield2Y:  entry.BC2Year,
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

// GetHistoricalYields returns historical data with permanent caching.
// Uses double-checked locking to avoid redundant fetches under contention.
func (s *TreasuryService) GetHistoricalYields(period string) (*models.HistoricalYieldData, error) {
	s.historicalMu.RLock()
	if cached, exists := s.historicalCache[period]; exists {
		data := cached.data
		s.historicalMu.RUnlock()
		return data, nil
	}
	s.historicalMu.RUnlock()

	s.historicalMu.Lock()
	defer s.historicalMu.Unlock()

	if cached, exists := s.historicalCache[period]; exists {
		return cached.data, nil
	}

	s.logger.Info("Fetching historical yields (cache miss)", zap.String("period", period))

	startDate, endDate, err := calculateDateRange(period)
	if err != nil {
		return nil, err
	}

	var feed *models.TreasuryFeed
	startYear := startDate.Year()
	endYear := endDate.Year()

	if startYear == endYear {
		feed, err = s.fetchFromAPI()
	} else {
		feed, err = s.fetchFromAPIForYears(startYear, endYear)
	}
	if err != nil {
		return nil, err
	}

	data, err := s.convertToHistoricalData(feed, startDate, endDate, period)
	if err != nil {
		return nil, err
	}

	s.historicalCache[period] = &historicalCacheEntry{
		data:      data,
		timestamp: time.Now(),
	}

	return data, nil
}

// GetLatestYields returns current yields with 1-hour TTL caching.
func (s *TreasuryService) GetLatestYields() (*models.YieldData, error) {
	s.mu.RLock()
	if s.cacheData != nil && time.Since(s.cacheTimestamp) < s.cacheDuration {
		data := s.cacheData
		s.mu.RUnlock()
		return data, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cacheData != nil && time.Since(s.cacheTimestamp) < s.cacheDuration {
		return s.cacheData, nil
	}

	feed, err := s.fetchFromAPI()
	if err != nil {
		return nil, err
	}

	data, err := s.convertToYieldData(feed)
	if err != nil {
		return nil, err
	}

	s.cacheData = data
	s.cacheTimestamp = time.Now()

	return data, nil
}

func (s *TreasuryService) WarmCache() {
	s.logger.Info("Warming historical yield cache...")
	for _, period := range historicalPeriods {
		go func(p string) {
			start := time.Now()
			if _, err := s.GetHistoricalYields(p); err != nil {
				s.logger.Error("Cache warm failed", zap.String("period", p), zap.Error(err))
			} else {
				s.logger.Info("Cache warmed", zap.String("period", p), zap.Duration("duration", time.Since(start)))
			}
		}(period)
	}
}
