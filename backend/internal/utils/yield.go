package utils

import (
	"fmt"
	"math"
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
func CalculateBillPrice(faceValue float64, yieldRate float64, term string) (float64, error) {
	securityType, err := GetSecurityType(term)
	if err != nil {
		return 0, err
	}

	if securityType != SecurityTypeBill {
		return 0, fmt.Errorf("CalculateBillPrice only applies to Treasury Bills (1M-1Y). For %s securities (%s), use CalculateNoteBondPrice", securityType, term)
	}

	if faceValue <= 0 {
		return 0, fmt.Errorf("face value must be greater than 0, got: %f", faceValue)
	}

	if yieldRate < 0 || yieldRate > 100 {
		return 0, fmt.Errorf("yield rate must be between 0 and 100, got: %f", yieldRate)
	}

	days, err := TermDurationDays(term)
	if err != nil {
		return 0, err
	}

	discountFactor := (yieldRate / 100.0 * float64(days)) / 360.0
	price := faceValue * (1.0 - discountFactor)
	price = math.Round(price*100) / 100

	return price, nil
}

func CalculateBillDiscount(faceValue float64, purchasePrice float64) float64 {
	discount := faceValue - purchasePrice
	return math.Round(discount*100) / 100
}

// CalculateNoteBondPrice returns par value (Notes/Bonds trade at face value).
func CalculateNoteBondPrice(faceValue float64, yieldRate float64, term string) (float64, error) {
	if faceValue <= 0 {
		return 0, fmt.Errorf("face value must be greater than 0, got: %f", faceValue)
	}

	if yieldRate < 0 || yieldRate > 100 {
		return 0, fmt.Errorf("yield rate must be between 0 and 100, got: %f", yieldRate)
	}

	securityType, err := GetSecurityType(term)
	if err != nil {
		return 0, err
	}
	if securityType != SecurityTypeNote && securityType != SecurityTypeBond {
		return 0, fmt.Errorf("invalid Note/Bond term: %s (must be 2Y, 5Y, 10Y, or 30Y)", term)
	}

	return math.Round(faceValue*100) / 100, nil
}

// CalculateNoteBondMaturityValue returns principal + simple interest (365-day convention).
func CalculateNoteBondMaturityValue(principal float64, yieldRate float64, daysHeld int) (float64, error) {
	if principal <= 0 {
		return 0, fmt.Errorf("principal must be greater than 0, got: %f", principal)
	}

	if yieldRate < 0 || yieldRate > 100 {
		return 0, fmt.Errorf("yield rate must be between 0 and 100, got: %f", yieldRate)
	}

	if daysHeld < 0 {
		return 0, fmt.Errorf("days held must be non-negative, got: %d", daysHeld)
	}

	simpleInterest := principal * (yieldRate / 100.0) * (float64(daysHeld) / 365.0)
	maturityValue := principal + simpleInterest
	return math.Round(maturityValue*100) / 100, nil
}
