package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"treasury-tracker/internal/database"
	"treasury-tracker/internal/utils"
)

type TransactionService struct {
	queries *database.Queries
	pool    *pgxpool.Pool
	logger  *zap.Logger
}

type PurchaseResult struct {
	User          *database.User
	PurchasePrice float64
	FaceValue     float64
	Discount      float64
}

func NewTransactionService(queries *database.Queries, pool *pgxpool.Pool, logger *zap.Logger) *TransactionService {
	return &TransactionService{
		queries: queries,
		pool:    pool,
		logger:  logger,
	}
}

func (s *TransactionService) FundAccount(ctx context.Context, userID int32, amount pgtype.Numeric) (*database.User, error) {
	amountFloat, err := amount.Float64Value()
	if err != nil {
		return nil, &ValidationError{Message: "invalid amount format"}
	}
	if !amountFloat.Valid || amountFloat.Float64 <= 0 {
		return nil, &ValidationError{Message: "amount must be greater than zero"}
	}

	var updatedUser *database.User

	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := s.queries.WithTx(tx)

		user, err := qtx.UpdateUserBalance(ctx, database.UpdateUserBalanceParams{
			Balance: amount,
			ID:      userID,
		})
		if err != nil {
			return fmt.Errorf("failed to update balance: %w", err)
		}

		_, err = qtx.CreateTransaction(ctx, database.CreateTransactionParams{
			UserID:             userID,
			Type:               database.TransactionTypeFund,
			Term:               pgtype.Text{Valid: false},
			Amount:             amount,
			YieldAtTransaction: pgtype.Numeric{Valid: false},
			BalanceAfter:       user.Balance,
			HoldingID:          pgtype.Int4{Valid: false},
		})
		if err != nil {
			return fmt.Errorf("failed to create transaction record: %w", err)
		}

		updatedUser = &user
		return nil
	})

	return updatedUser, err
}

func (s *TransactionService) WithdrawAccount(ctx context.Context, userID int32, amount pgtype.Numeric) (*database.User, error) {
	amountFloat, err := amount.Float64Value()
	if err != nil {
		return nil, &ValidationError{Message: "invalid amount format"}
	}
	if !amountFloat.Valid || amountFloat.Float64 <= 0 {
		return nil, &ValidationError{Message: "amount must be greater than zero"}
	}

	var updatedUser *database.User

	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := s.queries.WithTx(tx)

		currentUser, err := qtx.GetUserForUpdate(ctx, userID)
		if err != nil {
			return fmt.Errorf("failed to get user: %w", err)
		}

		currentBalanceFloat, err := currentUser.Balance.Float64Value()
		if err != nil {
			return fmt.Errorf("invalid balance format: %w", err)
		}
		if !currentBalanceFloat.Valid || currentBalanceFloat.Float64 < amountFloat.Float64 {
			return ErrInsufficientBalance
		}

		negativeAmount := pgtype.Numeric{}
		if err := negativeAmount.Scan(fmt.Sprintf("-%.2f", amountFloat.Float64)); err != nil {
			return fmt.Errorf("failed to create negative amount: %w", err)
		}

		user, err := qtx.UpdateUserBalance(ctx, database.UpdateUserBalanceParams{
			Balance: negativeAmount,
			ID:      userID,
		})
		if err != nil {
			return handleBalanceConstraint(err)
		}

		_, err = qtx.CreateTransaction(ctx, database.CreateTransactionParams{
			UserID:             userID,
			Type:               database.TransactionTypeWithdraw,
			Term:               pgtype.Text{Valid: false},
			Amount:             amount,
			YieldAtTransaction: pgtype.Numeric{Valid: false},
			BalanceAfter:       user.Balance,
			HoldingID:          pgtype.Int4{Valid: false},
		})
		if err != nil {
			return fmt.Errorf("failed to create transaction record: %w", err)
		}

		updatedUser = &user
		return nil
	})

	return updatedUser, err
}

// BuyTreasury purchases a treasury security atomically.
// T-Bills (1M-1Y) use discount pricing; Notes/Bonds (2Y-30Y) use par pricing.
func (s *TransactionService) BuyTreasury(
	ctx context.Context,
	userID int32,
	term string,
	faceValue pgtype.Numeric,
	currentYield pgtype.Numeric,
) (*PurchaseResult, error) {
	securityType, err := utils.GetSecurityType(term)
	if err != nil {
		return nil, fmt.Errorf("invalid term: %w", err)
	}

	faceValueFloat, err := faceValue.Float64Value()
	if err != nil {
		return nil, &ValidationError{Message: "invalid face value format"}
	}
	if !faceValueFloat.Valid || faceValueFloat.Float64 <= 0 {
		return nil, &ValidationError{Message: "face value must be greater than zero"}
	}

	yieldRateFloat, err := currentYield.Float64Value()
	if err != nil {
		return nil, &ValidationError{Message: "invalid yield rate format"}
	}
	if !yieldRateFloat.Valid || yieldRateFloat.Float64 < 0 {
		return nil, &ValidationError{Message: "yield rate must be non-negative"}
	}

	purchasePriceFloat, err := calculatePurchasePrice(securityType, faceValueFloat.Float64, yieldRateFloat.Float64, term)
	if err != nil {
		return nil, err
	}

	purchasePrice := pgtype.Numeric{}
	if err := purchasePrice.Scan(fmt.Sprintf("%.2f", purchasePriceFloat)); err != nil {
		return nil, fmt.Errorf("failed to create purchase price: %w", err)
	}

	var updatedUser *database.User

	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := s.queries.WithTx(tx)

		currentUser, err := qtx.GetUserForUpdate(ctx, userID)
		if err != nil {
			return fmt.Errorf("failed to get user: %w", err)
		}

		currentBalanceFloat, err := currentUser.Balance.Float64Value()
		if err != nil {
			return fmt.Errorf("invalid balance format: %w", err)
		}
		if !currentBalanceFloat.Valid || currentBalanceFloat.Float64 < purchasePriceFloat {
			return fmt.Errorf("%w: need %.2f, have %.2f", ErrInsufficientBalance,
				purchasePriceFloat, currentBalanceFloat.Float64)
		}

		holding, err := qtx.CreateHolding(ctx, database.CreateHoldingParams{
			UserID:          userID,
			Term:            term,
			Amount:          faceValue,
			YieldAtPurchase: currentYield,
			PurchaseDate:    pgtype.Timestamp{Time: time.Now(), Valid: true},
			RemainingAmount: faceValue,
			FaceValue:       faceValue,
			PurchasePrice:   purchasePrice,
			SecurityType:    pgtype.Text{String: securityType, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("failed to create holding: %w", err)
		}

		negativePurchasePrice := pgtype.Numeric{}
		if err := negativePurchasePrice.Scan(fmt.Sprintf("-%.2f", purchasePriceFloat)); err != nil {
			return fmt.Errorf("failed to create negative purchase price: %w", err)
		}

		user, err := qtx.UpdateUserBalance(ctx, database.UpdateUserBalanceParams{
			Balance: negativePurchasePrice,
			ID:      userID,
		})
		if err != nil {
			return handleBalanceConstraint(err)
		}

		_, err = qtx.CreateTransaction(ctx, database.CreateTransactionParams{
			UserID:             userID,
			Type:               database.TransactionTypeBuy,
			Term:               pgtype.Text{String: term, Valid: true},
			Amount:             purchasePrice,
			YieldAtTransaction: currentYield,
			BalanceAfter:       user.Balance,
			HoldingID:          pgtype.Int4{Int32: holding.ID, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("failed to create transaction record: %w", err)
		}

		updatedUser = &user
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &PurchaseResult{
		User:          updatedUser,
		PurchasePrice: purchasePriceFloat,
		FaceValue:     faceValueFloat.Float64,
		Discount:      faceValueFloat.Float64 - purchasePriceFloat,
	}, nil
}

// SellTreasury sells a treasury holding (full or partial) and returns proceeds.
// The caller is responsible for fetching and validating the holding exists.
func (s *TransactionService) SellTreasury(
	ctx context.Context,
	userID int32,
	holding database.Holding,
	amount pgtype.Numeric,
	currentYield pgtype.Numeric,
) (*database.User, error) {
	amountFloat, err := amount.Float64Value()
	if err != nil {
		return nil, &ValidationError{Message: "invalid amount format"}
	}
	if !amountFloat.Valid || amountFloat.Float64 <= 0 {
		return nil, &ValidationError{Message: "amount must be greater than zero"}
	}

	if holding.UserID != userID {
		return nil, ErrUnauthorized
	}

	remainingFloat, err := holding.RemainingAmount.Float64Value()
	if err != nil {
		return nil, fmt.Errorf("invalid remaining amount format: %w", err)
	}
	if !remainingFloat.Valid || amountFloat.Float64 > remainingFloat.Float64 {
		return nil, &ValidationError{Message: fmt.Sprintf(
			"insufficient remaining amount: requested %.2f, available %.2f",
			amountFloat.Float64, remainingFloat.Float64)}
	}

	totalProceeds, err := calculateSellProceeds(holding, amountFloat.Float64)
	if err != nil {
		return nil, err
	}

	var updatedUser *database.User

	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := s.queries.WithTx(tx)

		newRemaining := pgtype.Numeric{}
		if err := newRemaining.Scan(fmt.Sprintf("%.2f", remainingFloat.Float64-amountFloat.Float64)); err != nil {
			return fmt.Errorf("failed to create new remaining amount: %w", err)
		}

		_, err = qtx.UpdateHoldingRemainingAmount(ctx, database.UpdateHoldingRemainingAmountParams{
			ID:              holding.ID,
			RemainingAmount: newRemaining,
		})
		if err != nil {
			return fmt.Errorf("failed to update holding: %w", err)
		}

		proceedsAmount := pgtype.Numeric{}
		if err := proceedsAmount.Scan(fmt.Sprintf("%.2f", totalProceeds)); err != nil {
			return fmt.Errorf("failed to create proceeds amount: %w", err)
		}

		user, err := qtx.UpdateUserBalance(ctx, database.UpdateUserBalanceParams{
			Balance: proceedsAmount,
			ID:      userID,
		})
		if err != nil {
			return fmt.Errorf("failed to update balance: %w", err)
		}

		_, err = qtx.CreateTransaction(ctx, database.CreateTransactionParams{
			UserID:             userID,
			Type:               database.TransactionTypeSell,
			Term:               pgtype.Text{String: holding.Term, Valid: true},
			Amount:             amount,
			YieldAtTransaction: currentYield,
			BalanceAfter:       user.Balance,
			HoldingID:          pgtype.Int4{Int32: holding.ID, Valid: true},
		})
		if err != nil {
			return fmt.Errorf("failed to create transaction record: %w", err)
		}

		updatedUser = &user
		return nil
	})

	return updatedUser, err
}

func calculatePurchasePrice(securityType string, faceValue, yieldRate float64, term string) (float64, error) {
	switch securityType {
	case utils.SecurityTypeBill:
		return utils.CalculateBillPrice(faceValue, yieldRate, term)
	case utils.SecurityTypeNote, utils.SecurityTypeBond:
		return utils.CalculateNoteBondPrice(faceValue, yieldRate, term)
	default:
		return 0, fmt.Errorf("unknown security type: %s", securityType)
	}
}

func calculateSellProceeds(holding database.Holding, sellAmount float64) (float64, error) {
	securityType := holding.SecurityType.String
	if !holding.SecurityType.Valid || securityType == "" {
		inferred, err := utils.GetSecurityType(holding.Term)
		if err != nil {
			return 0, fmt.Errorf("cannot determine security type for term %s: %w", holding.Term, err)
		}
		securityType = inferred
	}

	if securityType == utils.SecurityTypeBill {
		return sellAmount, nil
	}

	daysHeld := int(time.Since(holding.PurchaseDate.Time).Hours() / 24)
	if daysHeld < 0 {
		return 0, errors.New("invalid holding: purchase date is in the future")
	}

	yieldRateFloat, err := holding.YieldAtPurchase.Float64Value()
	if err != nil || !yieldRateFloat.Valid {
		return 0, fmt.Errorf("invalid yield rate for holding: %w", err)
	}

	maturityValue, err := utils.CalculateNoteBondMaturityValue(sellAmount, yieldRateFloat.Float64, daysHeld)
	if err != nil {
		return 0, fmt.Errorf("failed to calculate maturity value: %w", err)
	}

	zap.L().Info("Selling holding",
		zap.String("security_type", securityType),
		zap.Float64("principal", sellAmount),
		zap.Float64("yield", yieldRateFloat.Float64),
		zap.Int("days_held", daysHeld),
		zap.Float64("proceeds", maturityValue),
	)

	return maturityValue, nil
}

func handleBalanceConstraint(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23514" {
		return ErrInsufficientBalance
	}
	return fmt.Errorf("failed to update balance: %w", err)
}
