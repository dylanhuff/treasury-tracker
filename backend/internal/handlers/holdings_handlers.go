package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"treasury-tracker/internal/database"
)

type HoldingsHandlers struct {
	queries *database.Queries
}

func NewHoldingsHandlers(queries *database.Queries) *HoldingsHandlers {
	return &HoldingsHandlers{queries: queries}
}

func (h *HoldingsHandlers) GetUserHoldings(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "id")
	userID, err := strconv.ParseInt(userIDStr, 10, 32)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	holdings, err := h.queries.GetActiveHoldingsByUser(r.Context(), int32(userID))
	if err != nil {
		log.Printf("Error fetching holdings for user %d: %v", userID, err)
		respondWithError(w, http.StatusInternalServerError, "failed to fetch holdings")
		return
	}

	respondWithJSON(w, http.StatusOK, holdings)
}
