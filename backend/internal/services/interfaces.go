package services

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"treasury-tracker/internal/database"
	"treasury-tracker/internal/models"
)

type TreasuryServiceInterface interface {
	GetLatestYields(ctx context.Context) (*models.YieldData, error)
	GetHistoricalYields(ctx context.Context, period string) (*models.HistoricalYieldData, error)
}

type TransactionServiceInterface interface {
	FundAccount(ctx context.Context, userID int32, amount pgtype.Numeric) (*database.User, error)
	WithdrawAccount(ctx context.Context, userID int32, amount pgtype.Numeric) (*database.User, error)
	BuyTreasury(ctx context.Context, userID int32, term string, faceValue pgtype.Numeric, currentYield pgtype.Numeric) (*PurchaseResult, error)
	SellTreasury(ctx context.Context, userID int32, holding database.Holding, amount pgtype.Numeric, currentYield pgtype.Numeric) (*database.User, error)
}

// Compile-time interface satisfaction checks.
var _ TreasuryServiceInterface = (*TreasuryService)(nil)
var _ TransactionServiceInterface = (*TransactionService)(nil)
