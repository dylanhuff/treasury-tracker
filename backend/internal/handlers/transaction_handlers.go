package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"treasury-tracker/internal/database"
	"treasury-tracker/internal/models"
	"treasury-tracker/internal/services"
)

type TransactionHandlers struct {
	txService       *services.TransactionService
	queries         *database.Queries
	treasuryService *services.TreasuryService
}

func NewTransactionHandlers(
	txService *services.TransactionService,
	queries *database.Queries,
	treasuryService *services.TreasuryService,
) *TransactionHandlers {
	return &TransactionHandlers{
		txService:       txService,
		queries:         queries,
		treasuryService: treasuryService,
	}
}

type TransactionRequest struct {
	UserID int32   `json:"user_id"`
	Amount float64 `json:"amount"`
}

type BuyRequest struct {
	UserID    int32   `json:"user_id"`
	Term      string  `json:"term"`
	FaceValue float64 `json:"face_value"`
}

type SellRequest struct {
	UserID    int32   `json:"user_id"`
	HoldingID int32   `json:"holding_id"`
	Amount    float64 `json:"amount"`
}

type TransactionResponse struct {
	Success bool           `json:"success"`
	User    *database.User `json:"user,omitempty"`
	Error   string         `json:"error,omitempty"`
}

type BuyResponse struct {
	Success       bool           `json:"success"`
	User          *database.User `json:"user"`
	FaceValue     float64        `json:"face_value"`
	PurchasePrice float64        `json:"purchase_price"`
	Discount      float64        `json:"discount"`
}

var validTerms = map[string]bool{
	"1M": true, "3M": true, "6M": true, "1Y": true,
	"2Y": true, "5Y": true, "10Y": true, "30Y": true,
}

func (h *TransactionHandlers) FundHandler(w http.ResponseWriter, r *http.Request) {
	var req TransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	amount := pgtype.Numeric{}
	if err := amount.Scan(fmt.Sprintf("%.2f", req.Amount)); err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid amount format")
		return
	}

	user, err := h.txService.FundAccount(r.Context(), req.UserID, amount)
	if err != nil {
		log.Printf("Error funding account for user %d: %v", req.UserID, err)
		respondWithServiceError(w, err)
		return
	}

	respondWithJSON(w, http.StatusOK, TransactionResponse{Success: true, User: user})
}

func (h *TransactionHandlers) WithdrawHandler(w http.ResponseWriter, r *http.Request) {
	var req TransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	amount := pgtype.Numeric{}
	if err := amount.Scan(fmt.Sprintf("%.2f", req.Amount)); err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid amount format")
		return
	}

	user, err := h.txService.WithdrawAccount(r.Context(), req.UserID, amount)
	if err != nil {
		log.Printf("Error withdrawing from account for user %d: %v", req.UserID, err)
		respondWithServiceError(w, err)
		return
	}

	respondWithJSON(w, http.StatusOK, TransactionResponse{Success: true, User: user})
}

func (h *TransactionHandlers) GetUserTransactions(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "userId")
	userID, err := strconv.ParseInt(userIDStr, 10, 32)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	transactions, err := h.queries.GetTransactionsByUser(r.Context(), int32(userID))
	if err != nil {
		log.Printf("Error fetching transactions for user %d: %v", userID, err)
		respondWithError(w, http.StatusInternalServerError, "failed to fetch transactions")
		return
	}

	respondWithJSON(w, http.StatusOK, transactions)
}

func (h *TransactionHandlers) BuyHandler(w http.ResponseWriter, r *http.Request) {
	var req BuyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !validTerms[req.Term] {
		respondWithError(w, http.StatusBadRequest, "invalid term: must be one of 1M, 3M, 6M, 1Y, 2Y, 5Y, 10Y, 30Y")
		return
	}

	yieldData, err := h.treasuryService.GetLatestYields()
	if err != nil {
		log.Printf("Error fetching yield data: %v", err)
		respondWithError(w, http.StatusInternalServerError, "failed to fetch current yield data")
		return
	}

	yieldRate, ok := findYieldRate(yieldData.Yields, req.Term)
	if !ok {
		respondWithError(w, http.StatusInternalServerError, "yield data not available for selected term")
		return
	}

	faceValueNumeric := pgtype.Numeric{}
	if err := faceValueNumeric.Scan(fmt.Sprintf("%.2f", req.FaceValue)); err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid face value format")
		return
	}

	currentYield := pgtype.Numeric{}
	if err := currentYield.Scan(fmt.Sprintf("%.2f", yieldRate)); err != nil {
		respondWithError(w, http.StatusInternalServerError, "invalid yield format")
		return
	}

	result, err := h.txService.BuyTreasury(r.Context(), req.UserID, req.Term, faceValueNumeric, currentYield)
	if err != nil {
		log.Printf("Error executing buy for user %d: %v", req.UserID, err)
		switch {
		case errors.Is(err, services.ErrInsufficientBalance):
			respondWithError(w, http.StatusBadRequest, err.Error())
		default:
			respondWithServiceError(w, err)
		}
		return
	}

	respondWithJSON(w, http.StatusOK, BuyResponse{
		Success:       true,
		User:          result.User,
		FaceValue:     result.FaceValue,
		PurchasePrice: result.PurchasePrice,
		Discount:      result.Discount,
	})
}

func (h *TransactionHandlers) SellHandler(w http.ResponseWriter, r *http.Request) {
	var req SellRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	amount := pgtype.Numeric{}
	if err := amount.Scan(fmt.Sprintf("%.2f", req.Amount)); err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid amount format")
		return
	}

	yieldData, err := h.treasuryService.GetLatestYields()
	if err != nil {
		log.Printf("Error fetching yield data for sell: %v", err)
		respondWithError(w, http.StatusInternalServerError, "failed to fetch current yield data")
		return
	}

	holding, err := h.queries.GetHoldingByID(r.Context(), req.HoldingID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "holding not found")
		return
	}

	yieldRate, ok := findYieldRate(yieldData.Yields, holding.Term)
	if !ok {
		respondWithError(w, http.StatusInternalServerError, "yield data not available for holding term")
		return
	}

	currentYield := pgtype.Numeric{}
	if err := currentYield.Scan(fmt.Sprintf("%.2f", yieldRate)); err != nil {
		respondWithError(w, http.StatusInternalServerError, "invalid yield format")
		return
	}

	user, err := h.txService.SellTreasury(r.Context(), req.UserID, holding, amount, currentYield)
	if err != nil {
		log.Printf("Error executing sell for user %d: %v", req.UserID, err)
		respondWithServiceError(w, err)
		return
	}

	respondWithJSON(w, http.StatusOK, TransactionResponse{Success: true, User: user})
}

func findYieldRate(yields []models.YieldPoint, term string) (float64, bool) {
	for _, y := range yields {
		if y.Term == term {
			return y.Rate, true
		}
	}
	return 0, false
}
