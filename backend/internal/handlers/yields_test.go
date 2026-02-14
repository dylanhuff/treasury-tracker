package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
	"treasury-tracker/internal/models"
)

func TestGetYields_Success(t *testing.T) {
	mock := &mockTreasuryService{
		latestYields: sampleYieldData(),
	}
	handler := NewYieldHandler(mock, zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/yields", nil)
	w := httptest.NewRecorder()

	handler.GetYields(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp models.YieldData
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Date != "2026-02-14" {
		t.Errorf("Expected date '2026-02-14', got '%s'", resp.Date)
	}
	if len(resp.Yields) != 8 {
		t.Errorf("Expected 8 yield points, got %d", len(resp.Yields))
	}

	// Verify first and last yield points
	if resp.Yields[0].Term != "1M" {
		t.Errorf("Expected first term '1M', got '%s'", resp.Yields[0].Term)
	}
	if !resp.Yields[0].Rate.Equal(sampleYieldData().Yields[0].Rate) {
		t.Errorf("Expected 1M rate %s, got %s", sampleYieldData().Yields[0].Rate, resp.Yields[0].Rate)
	}
	if resp.Yields[7].Term != "30Y" {
		t.Errorf("Expected last term '30Y', got '%s'", resp.Yields[7].Term)
	}
}

func TestGetYields_ServiceError(t *testing.T) {
	mock := &mockTreasuryService{
		latestErr: fmt.Errorf("database connection failed"),
	}
	handler := NewYieldHandler(mock, zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/yields", nil)
	w := httptest.NewRecorder()

	handler.GetYields(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("Expected status 500, got %d", w.Code)
	}

	var resp errorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}
	if resp.Error == "" {
		t.Error("Expected non-empty error message")
	}
}

func TestGetHistoricalYields_DefaultPeriod(t *testing.T) {
	mock := &mockTreasuryService{
		historicalData: sampleHistoricalData("3M"),
	}
	handler := NewYieldHandler(mock, zap.NewNop())

	// No period query param -- handler should default to "3M"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/yields/historical", nil)
	w := httptest.NewRecorder()

	handler.GetHistoricalYields(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp models.HistoricalYieldData
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Period != "3M" {
		t.Errorf("Expected period '3M' (default), got '%s'", resp.Period)
	}
	if len(resp.Data) != 3 {
		t.Errorf("Expected 3 data points, got %d", len(resp.Data))
	}
	if len(resp.Terms) != 3 {
		t.Errorf("Expected 3 terms, got %d", len(resp.Terms))
	}
}

func TestGetHistoricalYields_ValidPeriod(t *testing.T) {
	mock := &mockTreasuryService{
		historicalData: sampleHistoricalData("1Y"),
	}
	handler := NewYieldHandler(mock, zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/yields/historical?period=1Y", nil)
	w := httptest.NewRecorder()

	handler.GetHistoricalYields(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp models.HistoricalYieldData
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Period != "1Y" {
		t.Errorf("Expected period '1Y', got '%s'", resp.Period)
	}
}

func TestGetHistoricalYields_InvalidPeriod(t *testing.T) {
	mock := &mockTreasuryService{}
	handler := NewYieldHandler(mock, zap.NewNop())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/yields/historical?period=INVALID", nil)
	w := httptest.NewRecorder()

	handler.GetHistoricalYields(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp errorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}
	if resp.Error == "" {
		t.Error("Expected non-empty error message for invalid period")
	}
}

func TestGetHistoricalYields_AllValidPeriods(t *testing.T) {
	validPeriodsList := []string{"1W", "1M", "3M", "6M", "1Y", "5Y", "10Y", "30Y"}

	for _, period := range validPeriodsList {
		t.Run(period, func(t *testing.T) {
			mock := &mockTreasuryService{
				historicalData: sampleHistoricalData(period),
			}
			handler := NewYieldHandler(mock, zap.NewNop())

			req := httptest.NewRequest(http.MethodGet, "/api/v1/yields/historical?period="+period, nil)
			w := httptest.NewRecorder()

			handler.GetHistoricalYields(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected 200 for period %s, got %d: %s", period, w.Code, w.Body.String())
			}
		})
	}
}
