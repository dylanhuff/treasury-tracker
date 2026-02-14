package handlers

import (
	"log"
	"net/http"

	"treasury-tracker/internal/services"
)

type YieldHandler struct {
	treasuryService *services.TreasuryService
}

func NewYieldHandler(treasuryService *services.TreasuryService) *YieldHandler {
	return &YieldHandler{treasuryService: treasuryService}
}

var validPeriods = map[string]struct{}{
	"1W": {}, "1M": {}, "3M": {}, "6M": {},
	"1Y": {}, "5Y": {}, "10Y": {}, "30Y": {},
}

func (h *YieldHandler) GetYields(w http.ResponseWriter, r *http.Request) {
	yieldData, err := h.treasuryService.GetLatestYields()
	if err != nil {
		log.Printf("Error fetching treasury yields: %v", err)
		respondWithError(w, http.StatusInternalServerError, "failed to fetch treasury data")
		return
	}

	respondWithJSON(w, http.StatusOK, yieldData)
}

func (h *YieldHandler) GetHistoricalYields(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "3M"
	}

	if _, ok := validPeriods[period]; !ok {
		respondWithError(w, http.StatusBadRequest, "invalid period: must be one of 1W, 1M, 3M, 6M, 1Y, 5Y, 10Y, 30Y")
		return
	}

	data, err := h.treasuryService.GetHistoricalYields(period)
	if err != nil {
		log.Printf("Error fetching historical yields: %v", err)
		respondWithError(w, http.StatusInternalServerError, "failed to fetch historical treasury data")
		return
	}

	respondWithJSON(w, http.StatusOK, data)
}
