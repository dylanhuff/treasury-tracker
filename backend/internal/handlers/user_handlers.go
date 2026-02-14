package handlers

import (
	"net/http"

	"go.uber.org/zap"
	"treasury-tracker/internal/database"
)

type UserHandler struct {
	queries database.Querier
	logger  *zap.Logger
}

func NewUserHandler(queries database.Querier, logger *zap.Logger) *UserHandler {
	return &UserHandler{queries: queries, logger: logger}
}

func (h *UserHandler) GetAllUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.queries.ListUsers(r.Context())
	if err != nil {
		h.logger.Error("Error fetching users", zap.Error(err))
		respondWithError(w, http.StatusInternalServerError, "failed to fetch users")
		return
	}

	respondWithJSON(w, http.StatusOK, users)
}
