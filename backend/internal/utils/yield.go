package utils

import (
	"fmt"

	"github.com/shopspring/decimal"
)

const (
	SecurityTypeBill = "bill"
	SecurityTypeNote = "note"
	SecurityTypeBond = "bond"
)

func TermDurationDays(term string) (int, error) {
	termMap := map[string]int{
		"1M": 30, "3M": 90, "6M": 180, "1Y": 365,
		"2Y": 730, "5Y": 1825, "10Y": 3650, "30Y": 10950,
	}

	days, exists := termMap[term]
	if !exists {
		return 0, fmt.Errorf("invalid term: %s", term)
	}

	return days, nil
}

// GetSecurityType classifies by maturity: bill (<=1Y), note (2-10Y), bond (30Y).
func GetSecurityType(term string) (string, error) {
	switch term {
	case "1M", "3M", "6M", "1Y":
		return SecurityTypeBill, nil
	case "2Y", "5Y", "10Y":
		return SecurityTypeNote, nil
	case "30Y":
		return SecurityTypeBond, nil
	default:
		return "", fmt.Errorf("invalid term: %s (valid terms: 1M, 3M, 6M, 1Y, 2Y, 5Y, 10Y, 30Y)", term)
	}
}

// CalculateBillPrice uses 360-day discount convention: price = faceValue * (1 - yield*days/360)
func CalculateBillPrice(faceValue, yieldRate decimal.Decimal, term string) (decimal.Decimal, error) {
	securityType, err := GetSecurityType(term)
	if err != nil {
		return decimal.Zero, err
	}

	if securityType != SecurityTypeBill {
		return decimal.Zero, fmt.Errorf("CalculateBillPrice only applies to Treasury Bills (1M-1Y). For %s securities (%s), use CalculateNoteBondPrice", securityType, term)
	}

	if faceValue.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, fmt.Errorf("face value must be greater than 0, got: %s", faceValue.String())
	}

	if yieldRate.LessThan(decimal.Zero) || yieldRate.GreaterThan(decimal.NewFromInt(100)) {
		return decimal.Zero, fmt.Errorf("yield rate must be between 0 and 100, got: %s", yieldRate.String())
	}

	days, err := TermDurationDays(term)
	if err != nil {
		return decimal.Zero, err
	}

	hundred := decimal.NewFromInt(100)
	threeSixty := decimal.NewFromInt(360)
	daysDecimal := decimal.NewFromInt(int64(days))

	discountFactor := yieldRate.Div(hundred).Mul(daysDecimal).Div(threeSixty)
	price := faceValue.Mul(decimal.NewFromInt(1).Sub(discountFactor))
	price = price.Round(2)

	return price, nil
}

func CalculateBillDiscount(faceValue, purchasePrice decimal.Decimal) decimal.Decimal {
	discount := faceValue.Sub(purchasePrice)
	return discount.Round(2)
}

// CalculateNoteBondPrice returns par value (Notes/Bonds trade at face value).
func CalculateNoteBondPrice(faceValue, yieldRate decimal.Decimal, term string) (decimal.Decimal, error) {
	if faceValue.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, fmt.Errorf("face value must be greater than 0, got: %s", faceValue.String())
	}

	if yieldRate.LessThan(decimal.Zero) || yieldRate.GreaterThan(decimal.NewFromInt(100)) {
		return decimal.Zero, fmt.Errorf("yield rate must be between 0 and 100, got: %s", yieldRate.String())
	}

	securityType, err := GetSecurityType(term)
	if err != nil {
		return decimal.Zero, err
	}
	if securityType != SecurityTypeNote && securityType != SecurityTypeBond {
		return decimal.Zero, fmt.Errorf("invalid Note/Bond term: %s (must be 2Y, 5Y, 10Y, or 30Y)", term)
	}

	return faceValue.Round(2), nil
}

// CalculateNoteBondMaturityValue returns principal + simple interest (365-day convention).
func CalculateNoteBondMaturityValue(principal, yieldRate decimal.Decimal, daysHeld int) (decimal.Decimal, error) {
	if principal.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, fmt.Errorf("principal must be greater than 0, got: %s", principal.String())
	}

	if yieldRate.LessThan(decimal.Zero) || yieldRate.GreaterThan(decimal.NewFromInt(100)) {
		return decimal.Zero, fmt.Errorf("yield rate must be between 0 and 100, got: %s", yieldRate.String())
	}

	if daysHeld < 0 {
		return decimal.Zero, fmt.Errorf("days held must be non-negative, got: %d", daysHeld)
	}

	hundred := decimal.NewFromInt(100)
	threeSixtyFive := decimal.NewFromInt(365)
	daysDecimal := decimal.NewFromInt(int64(daysHeld))

	simpleInterest := principal.Mul(yieldRate.Div(hundred)).Mul(daysDecimal.Div(threeSixtyFive))
	maturityValue := principal.Add(simpleInterest)
	return maturityValue.Round(2), nil
}
