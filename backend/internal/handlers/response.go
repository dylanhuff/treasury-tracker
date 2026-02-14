package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"go.uber.org/zap"
	"treasury-tracker/internal/services"
)

type errorResponse struct {
	Error string `json:"error"`
}

func respondWithJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		zap.L().Error("Error encoding response", zap.Error(err))
	}
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	respondWithJSON(w, code, errorResponse{Error: msg})
}

func respondWithServiceError(w http.ResponseWriter, err error) {
	var validationErr *services.ValidationError
	switch {
	case errors.Is(err, services.ErrInsufficientBalance):
		respondWithError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, services.ErrHoldingNotFound):
		respondWithError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, services.ErrUnauthorized):
		respondWithError(w, http.StatusForbidden, err.Error())
	case errors.As(err, &validationErr):
		respondWithError(w, http.StatusBadRequest, err.Error())
	default:
		respondWithError(w, http.StatusInternalServerError, "internal server error")
	}
}
