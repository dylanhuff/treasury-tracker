package handlers

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
	"treasury-tracker/internal/database"
	"treasury-tracker/internal/models"
	"treasury-tracker/internal/services"
)

type mockTreasuryService struct {
	latestYields   *models.YieldData
	latestErr      error
	historicalData *models.HistoricalYieldData
	historicalErr  error
}

func (m *mockTreasuryService) GetLatestYields(ctx context.Context) (*models.YieldData, error) {
	return m.latestYields, m.latestErr
}

func (m *mockTreasuryService) GetHistoricalYields(ctx context.Context, period string) (*models.HistoricalYieldData, error) {
	return m.historicalData, m.historicalErr
}

type mockTransactionService struct {
	fundResult     *database.User
	fundErr        error
	withdrawResult *database.User
	withdrawErr    error
	buyResult      *services.PurchaseResult
	buyErr         error
	sellResult     *database.User
	sellErr        error
}

func (m *mockTransactionService) FundAccount(ctx context.Context, userID int32, amount pgtype.Numeric) (*database.User, error) {
	return m.fundResult, m.fundErr
}

func (m *mockTransactionService) WithdrawAccount(ctx context.Context, userID int32, amount pgtype.Numeric) (*database.User, error) {
	return m.withdrawResult, m.withdrawErr
}

func (m *mockTransactionService) BuyTreasury(ctx context.Context, userID int32, term string, faceValue, currentYield pgtype.Numeric) (*services.PurchaseResult, error) {
	return m.buyResult, m.buyErr
}

func (m *mockTransactionService) SellTreasury(ctx context.Context, userID int32, holding database.Holding, amount, currentYield pgtype.Numeric) (*database.User, error) {
	return m.sellResult, m.sellErr
}

func sampleYieldData() *models.YieldData {
	return &models.YieldData{
		Date: "2026-02-14",
		Yields: []models.YieldPoint{
			{Term: "1M", Rate: decimal.NewFromFloat(4.50)},
			{Term: "3M", Rate: decimal.NewFromFloat(4.48)},
			{Term: "6M", Rate: decimal.NewFromFloat(4.35)},
			{Term: "1Y", Rate: decimal.NewFromFloat(4.20)},
			{Term: "2Y", Rate: decimal.NewFromFloat(4.05)},
			{Term: "5Y", Rate: decimal.NewFromFloat(3.95)},
			{Term: "10Y", Rate: decimal.NewFromFloat(4.10)},
			{Term: "30Y", Rate: decimal.NewFromFloat(4.45)},
		},
	}
}

func sampleHistoricalData(period string) *models.HistoricalYieldData {
	return &models.HistoricalYieldData{
		Period:    period,
		StartDate: "2025-11-14",
		EndDate:   "2026-02-14",
		Terms:     []string{"10Y", "5Y", "2Y"},
		Data: []models.YieldDataPoint{
			{
				Date:     "2025-12-01",
				Yield2Y:  decimal.NewFromFloat(4.00),
				Yield5Y:  decimal.NewFromFloat(3.90),
				Yield10Y: decimal.NewFromFloat(4.05),
			},
			{
				Date:     "2026-01-15",
				Yield2Y:  decimal.NewFromFloat(4.02),
				Yield5Y:  decimal.NewFromFloat(3.92),
				Yield10Y: decimal.NewFromFloat(4.08),
			},
			{
				Date:     "2026-02-14",
				Yield2Y:  decimal.NewFromFloat(4.05),
				Yield5Y:  decimal.NewFromFloat(3.95),
				Yield10Y: decimal.NewFromFloat(4.10),
			},
		},
	}
}

func sampleUser() *database.User {
	balance := pgtype.Numeric{}
	balance.Scan("100000.00")
	return &database.User{
		ID:      1,
		Name:    "Test User",
		Balance: balance,
	}
}
