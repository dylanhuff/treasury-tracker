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
	entries := []models.Entry{
		{Date: "2024-01-01T00:00:00", BC10Year: 4.0},
		{Date: "2024-01-02T00:00:00", BC10Year: 4.1},
		{Date: "2024-01-03T00:00:00", BC10Year: 4.2},
		{Date: "2024-01-08T00:00:00", BC10Year: 4.3},
		{Date: "2024-01-09T00:00:00", BC10Year: 4.4},
	}
	result := filterByAge(entries, now)
	if len(result) != 2 {
		t.Errorf("expected 2 entries (weekly sample), got %d", len(result))
	}
}

func TestFilterByAge_VeryOldDataSampledMonthly(t *testing.T) {
	now := time.Date(2026, 2, 14, 0, 0, 0, 0, time.UTC)
	entries := []models.Entry{
		{Date: "2019-01-02T00:00:00", BC10Year: 3.0},
		{Date: "2019-01-15T00:00:00", BC10Year: 3.1},
		{Date: "2019-01-30T00:00:00", BC10Year: 3.2},
		{Date: "2019-02-05T00:00:00", BC10Year: 3.3},
		{Date: "2019-02-20T00:00:00", BC10Year: 3.4},
	}
	result := filterByAge(entries, now)
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
