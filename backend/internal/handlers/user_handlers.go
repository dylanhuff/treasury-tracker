package handlers

import (
	"log"
	"net/http"

	"treasury-tracker/internal/database"
)

type UserHandler struct {
	queries *database.Queries
}

func NewUserHandler(queries *database.Queries) *UserHandler {
	return &UserHandler{queries: queries}
}

func (h *UserHandler) GetAllUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.queries.ListUsers(r.Context())
	if err != nil {
		log.Printf("Error fetching users: %v", err)
		respondWithError(w, http.StatusInternalServerError, "failed to fetch users")
		return
	}

	respondWithJSON(w, http.StatusOK, users)
}
