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
	"github.com/shopspring/decimal"
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
	PurchasePrice decimal.Decimal
	FaceValue     decimal.Decimal
	Discount      decimal.Decimal
}

func NewTransactionService(queries *database.Queries, pool *pgxpool.Pool, logger *zap.Logger) *TransactionService {
	return &TransactionService{
		queries: queries,
		pool:    pool,
		logger:  logger,
	}
}

func (s *TransactionService) FundAccount(ctx context.Context, userID int32, amount pgtype.Numeric) (*database.User, error) {
	amountDec, err := numericToDecimal(amount)
	if err != nil {
		return nil, &ValidationError{Message: "invalid amount format"}
	}
	if amountDec.LessThanOrEqual(decimal.Zero) {
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
	amountDec, err := numericToDecimal(amount)
	if err != nil {
		return nil, &ValidationError{Message: "invalid amount format"}
	}
	if amountDec.LessThanOrEqual(decimal.Zero) {
		return nil, &ValidationError{Message: "amount must be greater than zero"}
	}

	var updatedUser *database.User

	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := s.queries.WithTx(tx)

		currentUser, err := qtx.GetUserForUpdate(ctx, userID)
		if err != nil {
			return fmt.Errorf("failed to get user: %w", err)
		}

		currentBalanceDec, err := numericToDecimal(currentUser.Balance)
		if err != nil {
			return fmt.Errorf("invalid balance format: %w", err)
		}
		if currentBalanceDec.LessThan(amountDec) {
			return ErrInsufficientBalance
		}

		negativeAmount := pgtype.Numeric{}
		if err := negativeAmount.Scan("-" + amountDec.StringFixed(2)); err != nil {
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

	faceValueDec, err := numericToDecimal(faceValue)
	if err != nil {
		return nil, &ValidationError{Message: "invalid face value format"}
	}
	if faceValueDec.LessThanOrEqual(decimal.Zero) {
		return nil, &ValidationError{Message: "face value must be greater than zero"}
	}

	yieldRateDec, err := numericToDecimal(currentYield)
	if err != nil {
		return nil, &ValidationError{Message: "invalid yield rate format"}
	}
	if yieldRateDec.LessThan(decimal.Zero) {
		return nil, &ValidationError{Message: "yield rate must be non-negative"}
	}

	purchasePriceDec, err := calculatePurchasePrice(securityType, faceValueDec, yieldRateDec, term)
	if err != nil {
		return nil, err
	}

	purchasePrice := pgtype.Numeric{}
	if err := purchasePrice.Scan(purchasePriceDec.StringFixed(2)); err != nil {
		return nil, fmt.Errorf("failed to create purchase price: %w", err)
	}

	var updatedUser *database.User

	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := s.queries.WithTx(tx)

		currentUser, err := qtx.GetUserForUpdate(ctx, userID)
		if err != nil {
			return fmt.Errorf("failed to get user: %w", err)
		}

		currentBalanceDec, err := numericToDecimal(currentUser.Balance)
		if err != nil {
			return fmt.Errorf("invalid balance format: %w", err)
		}
		if currentBalanceDec.LessThan(purchasePriceDec) {
			return fmt.Errorf("%w: need %s, have %s", ErrInsufficientBalance,
				purchasePriceDec.StringFixed(2), currentBalanceDec.StringFixed(2))
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
		if err := negativePurchasePrice.Scan("-" + purchasePriceDec.StringFixed(2)); err != nil {
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
		PurchasePrice: purchasePriceDec,
		FaceValue:     faceValueDec,
		Discount:      faceValueDec.Sub(purchasePriceDec),
	}, nil
}

// Caller is responsible for fetching and validating the holding exists.
func (s *TransactionService) SellTreasury(
	ctx context.Context,
	userID int32,
	holding database.Holding,
	amount pgtype.Numeric,
	currentYield pgtype.Numeric,
) (*database.User, error) {
	sellAmountDec, err := numericToDecimal(amount)
	if err != nil {
		return nil, &ValidationError{Message: "invalid amount format"}
	}
	if sellAmountDec.LessThanOrEqual(decimal.Zero) {
		return nil, &ValidationError{Message: "amount must be greater than zero"}
	}

	if holding.UserID != userID {
		return nil, ErrUnauthorized
	}

	remainingDec, err := numericToDecimal(holding.RemainingAmount)
	if err != nil {
		return nil, fmt.Errorf("invalid remaining amount format: %w", err)
	}
	if sellAmountDec.GreaterThan(remainingDec) {
		return nil, &ValidationError{Message: fmt.Sprintf(
			"insufficient remaining amount: requested %s, available %s",
			sellAmountDec.StringFixed(2), remainingDec.StringFixed(2))}
	}

	totalProceedsDec, err := calculateSellProceeds(holding, sellAmountDec)
	if err != nil {
		return nil, err
	}

	var updatedUser *database.User

	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		qtx := s.queries.WithTx(tx)

		newRemainingDec := remainingDec.Sub(sellAmountDec)
		newRemaining := pgtype.Numeric{}
		if err := newRemaining.Scan(newRemainingDec.StringFixed(2)); err != nil {
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
		if err := proceedsAmount.Scan(totalProceedsDec.StringFixed(2)); err != nil {
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

func calculatePurchasePrice(securityType string, faceValue, yieldRate decimal.Decimal, term string) (decimal.Decimal, error) {
	switch securityType {
	case utils.SecurityTypeBill:
		return utils.CalculateBillPrice(faceValue, yieldRate, term)
	case utils.SecurityTypeNote, utils.SecurityTypeBond:
		return utils.CalculateNoteBondPrice(faceValue, yieldRate, term)
	default:
		return decimal.Zero, fmt.Errorf("unknown security type: %s", securityType)
	}
}

func calculateSellProceeds(holding database.Holding, sellAmount decimal.Decimal) (decimal.Decimal, error) {
	securityType := holding.SecurityType.String
	if !holding.SecurityType.Valid || securityType == "" {
		inferred, err := utils.GetSecurityType(holding.Term)
		if err != nil {
			return decimal.Zero, fmt.Errorf("cannot determine security type for term %s: %w", holding.Term, err)
		}
		securityType = inferred
	}

	if securityType == utils.SecurityTypeBill {
		return sellAmount, nil
	}

	daysHeld := int(time.Since(holding.PurchaseDate.Time).Hours() / 24)
	if daysHeld < 0 {
		return decimal.Zero, errors.New("invalid holding: purchase date is in the future")
	}

	yieldRateDec, err := numericToDecimal(holding.YieldAtPurchase)
	if err != nil {
		return decimal.Zero, fmt.Errorf("invalid yield rate for holding: %w", err)
	}

	maturityValue, err := utils.CalculateNoteBondMaturityValue(sellAmount, yieldRateDec, daysHeld)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to calculate maturity value: %w", err)
	}

	zap.L().Info("Selling holding",
		zap.String("security_type", securityType),
		zap.String("principal", sellAmount.StringFixed(2)),
		zap.String("yield", yieldRateDec.String()),
		zap.Int("days_held", daysHeld),
		zap.String("proceeds", maturityValue.StringFixed(2)),
	)

	return maturityValue, nil
}

func numericToDecimal(n pgtype.Numeric) (decimal.Decimal, error) {
	if !n.Valid || n.NaN {
		return decimal.Zero, fmt.Errorf("invalid numeric")
	}
	if n.Int == nil {
		return decimal.Zero, nil
	}
	return decimal.NewFromBigInt(n.Int, n.Exp), nil
}

func handleBalanceConstraint(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23514" {
		return ErrInsufficientBalance
	}
	return fmt.Errorf("failed to update balance: %w", err)
}
