package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	"treasury-tracker/internal/database"
)

type HoldingsHandlers struct {
	queries database.Querier
	logger  *zap.Logger
}

func NewHoldingsHandlers(queries database.Querier, logger *zap.Logger) *HoldingsHandlers {
	return &HoldingsHandlers{queries: queries, logger: logger}
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
		h.logger.Error("Error fetching holdings", zap.Int64("user_id", userID), zap.Error(err))
		respondWithError(w, http.StatusInternalServerError, "failed to fetch holdings")
		return
	}

	respondWithJSON(w, http.StatusOK, holdings)
}
