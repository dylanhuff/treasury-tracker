package utils

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestTermDurationDays(t *testing.T) {
	tests := []struct {
		name     string
		term     string
		expected int
		wantErr  bool
	}{
		{"1 Month", "1M", 30, false},
		{"3 Months", "3M", 90, false},
		{"6 Months", "6M", 180, false},
		{"1 Year", "1Y", 365, false},
		{"2 Years", "2Y", 730, false},
		{"5 Years", "5Y", 1825, false},
		{"10 Years", "10Y", 3650, false},
		{"30 Years", "30Y", 10950, false},
		{"Invalid term", "6Y", 0, true},
		{"Empty term", "", 0, true},
		{"Invalid format", "1m", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			days, err := TermDurationDays(tt.term)
			if (err != nil) != tt.wantErr {
				t.Errorf("TermDurationDays(%s) error = %v, wantErr %v", tt.term, err, tt.wantErr)
				return
			}
			if days != tt.expected {
				t.Errorf("TermDurationDays(%s) = %d, want %d", tt.term, days, tt.expected)
			}
		})
	}
}

func TestCalculateBillPrice(t *testing.T) {
	tests := []struct {
		name      string
		faceValue decimal.Decimal
		yieldRate decimal.Decimal
		term      string
		expected  decimal.Decimal
		wantErr   bool
	}{
		{
			name:      "Standard scenario: 6M bill at 4.5% yield",
			faceValue: decimal.NewFromFloat(10000.0),
			yieldRate: decimal.NewFromFloat(4.5),
			term:      "6M",
			expected:  decimal.RequireFromString("9775.00"),
			wantErr:   false,
		},
		{
			name:      "Zero yield edge case",
			faceValue: decimal.NewFromFloat(10000.0),
			yieldRate: decimal.NewFromFloat(0.0),
			term:      "6M",
			expected:  decimal.RequireFromString("10000.00"),
			wantErr:   false,
		},
		{
			name:      "High yield scenario: 10% yield",
			faceValue: decimal.NewFromFloat(10000.0),
			yieldRate: decimal.NewFromFloat(10.0),
			term:      "6M",
			expected:  decimal.RequireFromString("9500.00"),
			wantErr:   false,
		},
		{
			name:      "Short term: 1M at 3% yield",
			faceValue: decimal.NewFromFloat(10000.0),
			yieldRate: decimal.NewFromFloat(3.0),
			term:      "1M",
			expected:  decimal.RequireFromString("9975.00"),
			wantErr:   false,
		},
		{
			name:      "Long term: 1Y at 5% yield",
			faceValue: decimal.NewFromFloat(10000.0),
			yieldRate: decimal.NewFromFloat(5.0),
			term:      "1Y",
			expected:  decimal.RequireFromString("9493.06"),
			wantErr:   false,
		},
		{
			name:      "Different face value: $50,000 at 4% for 3M",
			faceValue: decimal.NewFromFloat(50000.0),
			yieldRate: decimal.NewFromFloat(4.0),
			term:      "3M",
			expected:  decimal.RequireFromString("49500.00"),
			wantErr:   false,
		},
		{
			name:      "Validation: negative yield should error",
			faceValue: decimal.NewFromFloat(10000.0),
			yieldRate: decimal.NewFromFloat(-1.0),
			term:      "6M",
			expected:  decimal.Zero,
			wantErr:   true,
		},
		{
			name:      "Validation: yield over 100 should error",
			faceValue: decimal.NewFromFloat(10000.0),
			yieldRate: decimal.NewFromFloat(101.0),
			term:      "6M",
			expected:  decimal.Zero,
			wantErr:   true,
		},
		{
			name:      "Validation: invalid term should error",
			faceValue: decimal.NewFromFloat(10000.0),
			yieldRate: decimal.NewFromFloat(4.5),
			term:      "2Y",
			expected:  decimal.Zero,
			wantErr:   true,
		},
		{
			name:      "Validation: invalid term format should error",
			faceValue: decimal.NewFromFloat(10000.0),
			yieldRate: decimal.NewFromFloat(4.5),
			term:      "invalid",
			expected:  decimal.Zero,
			wantErr:   true,
		},
		{
			name:      "Validation: zero face value should error",
			faceValue: decimal.NewFromFloat(0.0),
			yieldRate: decimal.NewFromFloat(4.5),
			term:      "6M",
			expected:  decimal.Zero,
			wantErr:   true,
		},
		{
			name:      "Validation: negative face value should error",
			faceValue: decimal.NewFromFloat(-10000.0),
			yieldRate: decimal.NewFromFloat(4.5),
			term:      "6M",
			expected:  decimal.Zero,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CalculateBillPrice(tt.faceValue, tt.yieldRate, tt.term)
			if (err != nil) != tt.wantErr {
				t.Errorf("CalculateBillPrice() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if !result.Equal(tt.expected) {
					t.Errorf("CalculateBillPrice() = %s, want %s", result.String(), tt.expected.String())
				}
			}
		})
	}
}

func TestCalculateBillDiscount(t *testing.T) {
	tests := []struct {
		name          string
		faceValue     decimal.Decimal
		purchasePrice decimal.Decimal
		expected      decimal.Decimal
	}{
		{
			name:          "Standard discount calculation",
			faceValue:     decimal.NewFromFloat(10000.0),
			purchasePrice: decimal.NewFromFloat(9775.0),
			expected:      decimal.RequireFromString("225.00"),
		},
		{
			name:          "Zero discount (par purchase)",
			faceValue:     decimal.NewFromFloat(10000.0),
			purchasePrice: decimal.NewFromFloat(10000.0),
			expected:      decimal.RequireFromString("0.00"),
		},
		{
			name:          "Large discount",
			faceValue:     decimal.NewFromFloat(100000.0),
			purchasePrice: decimal.NewFromFloat(95000.0),
			expected:      decimal.RequireFromString("5000.00"),
		},
		{
			name:          "Small face value",
			faceValue:     decimal.NewFromFloat(1000.0),
			purchasePrice: decimal.NewFromFloat(975.0),
			expected:      decimal.RequireFromString("25.00"),
		},
		{
			name:          "Cents precision",
			faceValue:     decimal.NewFromFloat(10000.0),
			purchasePrice: decimal.NewFromFloat(9775.55),
			expected:      decimal.RequireFromString("224.45"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateBillDiscount(tt.faceValue, tt.purchasePrice)
			if !result.Equal(tt.expected) {
				t.Errorf("CalculateBillDiscount() = %s, want %s", result.String(), tt.expected.String())
			}
		})
	}
}

func TestCalculateBillPriceAllTerms(t *testing.T) {
	faceValue := decimal.NewFromFloat(10000.0)
	yieldRate := decimal.NewFromFloat(4.5)

	// Only T-Bill terms (1M, 3M, 6M, 1Y)
	terms := []string{"1M", "3M", "6M", "1Y"}

	for _, term := range terms {
		t.Run(term, func(t *testing.T) {
			price, err := CalculateBillPrice(faceValue, yieldRate, term)
			if err != nil {
				t.Errorf("CalculateBillPrice failed for %s: %v", term, err)
				return
			}

			if price.GreaterThan(faceValue) {
				t.Errorf("Price (%s) should not exceed face value (%s) for term %s", price.String(), faceValue.String(), term)
			}

			if price.LessThanOrEqual(decimal.Zero) {
				t.Errorf("Price should be positive for term %s, got %s", term, price.String())
			}

			discount := CalculateBillDiscount(faceValue, price)
			if discount.LessThan(decimal.Zero) {
				t.Errorf("Discount should be non-negative for term %s, got %s", term, discount.String())
			}
		})
	}
}

func TestGetSecurityType(t *testing.T) {
	tests := []struct {
		name         string
		term         string
		expectedType string
		wantErr      bool
	}{
		// Treasury Bills (1M - 1Y)
		{"1 Month Bill", "1M", "bill", false},
		{"3 Month Bill", "3M", "bill", false},
		{"6 Month Bill", "6M", "bill", false},
		{"1 Year Bill", "1Y", "bill", false},

		// Treasury Notes (2Y - 10Y)
		{"2 Year Note", "2Y", "note", false},
		{"5 Year Note", "5Y", "note", false},
		{"10 Year Note", "10Y", "note", false},

		// Treasury Bonds (30Y)
		{"30 Year Bond", "30Y", "bond", false},

		// Invalid terms
		{"Invalid term - 6Y", "6Y", "", true},
		{"Invalid term - 15Y", "15Y", "", true},
		{"Invalid term - empty", "", "", true},
		{"Invalid term - lowercase", "1m", "", true},
		{"Invalid term - text", "invalid", "", true},
		{"Invalid term - 20Y", "20Y", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			secType, err := GetSecurityType(tt.term)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSecurityType(%s) error = %v, wantErr %v", tt.term, err, tt.wantErr)
				return
			}
			if !tt.wantErr && secType != tt.expectedType {
				t.Errorf("GetSecurityType(%s) = %s, want %s", tt.term, secType, tt.expectedType)
			}
		})
	}
}

func TestCalculateNoteBondPrice(t *testing.T) {
	tests := []struct {
		name      string
		faceValue decimal.Decimal
		yieldRate decimal.Decimal
		term      string
		expected  decimal.Decimal
		wantErr   bool
	}{
		{
			name:      "2Y Note at par",
			faceValue: decimal.NewFromFloat(10000.0),
			yieldRate: decimal.NewFromFloat(3.5),
			term:      "2Y",
			expected:  decimal.RequireFromString("10000.00"),
			wantErr:   false,
		},
		{
			name:      "5Y Note at par",
			faceValue: decimal.NewFromFloat(50000.0),
			yieldRate: decimal.NewFromFloat(3.8),
			term:      "5Y",
			expected:  decimal.RequireFromString("50000.00"),
			wantErr:   false,
		},
		{
			name:      "10Y Note at par",
			faceValue: decimal.NewFromFloat(100000.0),
			yieldRate: decimal.NewFromFloat(4.0),
			term:      "10Y",
			expected:  decimal.RequireFromString("100000.00"),
			wantErr:   false,
		},
		{
			name:      "30Y Bond at par",
			faceValue: decimal.NewFromFloat(250000.0),
			yieldRate: decimal.NewFromFloat(4.5),
			term:      "30Y",
			expected:  decimal.RequireFromString("250000.00"),
			wantErr:   false,
		},
		{
			name:      "Zero yield - still at par",
			faceValue: decimal.NewFromFloat(10000.0),
			yieldRate: decimal.NewFromFloat(0.0),
			term:      "2Y",
			expected:  decimal.RequireFromString("10000.00"),
			wantErr:   false,
		},
		{
			name:      "Small face value",
			faceValue: decimal.NewFromFloat(1000.0),
			yieldRate: decimal.NewFromFloat(3.5),
			term:      "5Y",
			expected:  decimal.RequireFromString("1000.00"),
			wantErr:   false,
		},
		{
			name:      "Validation: negative face value",
			faceValue: decimal.NewFromFloat(-10000.0),
			yieldRate: decimal.NewFromFloat(3.5),
			term:      "2Y",
			expected:  decimal.Zero,
			wantErr:   true,
		},
		{
			name:      "Validation: zero face value",
			faceValue: decimal.NewFromFloat(0.0),
			yieldRate: decimal.NewFromFloat(3.5),
			term:      "2Y",
			expected:  decimal.Zero,
			wantErr:   true,
		},
		{
			name:      "Validation: negative yield rate",
			faceValue: decimal.NewFromFloat(10000.0),
			yieldRate: decimal.NewFromFloat(-1.0),
			term:      "2Y",
			expected:  decimal.Zero,
			wantErr:   true,
		},
		{
			name:      "Validation: yield rate over 100",
			faceValue: decimal.NewFromFloat(10000.0),
			yieldRate: decimal.NewFromFloat(101.0),
			term:      "2Y",
			expected:  decimal.Zero,
			wantErr:   true,
		},
		{
			name:      "Validation: invalid term for Notes/Bonds",
			faceValue: decimal.NewFromFloat(10000.0),
			yieldRate: decimal.NewFromFloat(3.5),
			term:      "1M",
			expected:  decimal.Zero,
			wantErr:   true,
		},
		{
			name:      "Validation: invalid term format",
			faceValue: decimal.NewFromFloat(10000.0),
			yieldRate: decimal.NewFromFloat(3.5),
			term:      "6Y",
			expected:  decimal.Zero,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CalculateNoteBondPrice(tt.faceValue, tt.yieldRate, tt.term)
			if (err != nil) != tt.wantErr {
				t.Errorf("CalculateNoteBondPrice() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if !result.Equal(tt.expected) {
					t.Errorf("CalculateNoteBondPrice() = %s, want %s", result.String(), tt.expected.String())
				}
			}
		})
	}
}

func TestCalculateNoteBondMaturityValue(t *testing.T) {
	tests := []struct {
		name      string
		principal decimal.Decimal
		yieldRate decimal.Decimal
		daysHeld  int
		expected  decimal.Decimal
		wantErr   bool
	}{
		{
			name:      "2Y note at 3.5% held full term (730 days)",
			principal: decimal.NewFromFloat(10000.0),
			yieldRate: decimal.NewFromFloat(3.5),
			daysHeld:  730,
			expected:  decimal.RequireFromString("10700.00"),
			wantErr:   false,
		},
		{
			name:      "10Y note at 4.0% held 1 year (365 days)",
			principal: decimal.NewFromFloat(10000.0),
			yieldRate: decimal.NewFromFloat(4.0),
			daysHeld:  365,
			expected:  decimal.RequireFromString("10400.00"),
			wantErr:   false,
		},
		{
			name:      "30Y bond at 4.5% held 6 months (180 days)",
			principal: decimal.NewFromFloat(10000.0),
			yieldRate: decimal.NewFromFloat(4.5),
			daysHeld:  180,
			expected:  decimal.RequireFromString("10221.92"),
			wantErr:   false,
		},
		{
			name:      "5Y note at 3.8% held full term (1825 days)",
			principal: decimal.NewFromFloat(50000.0),
			yieldRate: decimal.NewFromFloat(3.8),
			daysHeld:  1825,
			expected:  decimal.RequireFromString("59500.00"),
			wantErr:   false,
		},
		{
			name:      "Large principal: $1M at 4% for 1 year",
			principal: decimal.NewFromFloat(1000000.0),
			yieldRate: decimal.NewFromFloat(4.0),
			daysHeld:  365,
			expected:  decimal.RequireFromString("1040000.00"),
			wantErr:   false,
		},
		{
			name:      "Zero days held",
			principal: decimal.NewFromFloat(10000.0),
			yieldRate: decimal.NewFromFloat(3.5),
			daysHeld:  0,
			expected:  decimal.RequireFromString("10000.00"),
			wantErr:   false,
		},
		{
			name:      "Zero yield rate",
			principal: decimal.NewFromFloat(10000.0),
			yieldRate: decimal.NewFromFloat(0.0),
			daysHeld:  730,
			expected:  decimal.RequireFromString("10000.00"),
			wantErr:   false,
		},
		{
			name:      "Partial year: 100 days",
			principal: decimal.NewFromFloat(10000.0),
			yieldRate: decimal.NewFromFloat(5.0),
			daysHeld:  100,
			expected:  decimal.RequireFromString("10136.99"),
			wantErr:   false,
		},
		{
			name:      "Very short holding: 1 day",
			principal: decimal.NewFromFloat(100000.0),
			yieldRate: decimal.NewFromFloat(4.0),
			daysHeld:  1,
			expected:  decimal.RequireFromString("100010.96"),
			wantErr:   false,
		},
		{
			name:      "High yield scenario: 10%",
			principal: decimal.NewFromFloat(10000.0),
			yieldRate: decimal.NewFromFloat(10.0),
			daysHeld:  365,
			expected:  decimal.RequireFromString("11000.00"),
			wantErr:   false,
		},
		{
			name:      "Validation: negative principal",
			principal: decimal.NewFromFloat(-10000.0),
			yieldRate: decimal.NewFromFloat(3.5),
			daysHeld:  730,
			expected:  decimal.Zero,
			wantErr:   true,
		},
		{
			name:      "Validation: zero principal",
			principal: decimal.NewFromFloat(0.0),
			yieldRate: decimal.NewFromFloat(3.5),
			daysHeld:  730,
			expected:  decimal.Zero,
			wantErr:   true,
		},
		{
			name:      "Validation: negative yield rate",
			principal: decimal.NewFromFloat(10000.0),
			yieldRate: decimal.NewFromFloat(-1.0),
			daysHeld:  730,
			expected:  decimal.Zero,
			wantErr:   true,
		},
		{
			name:      "Validation: yield rate over 100",
			principal: decimal.NewFromFloat(10000.0),
			yieldRate: decimal.NewFromFloat(101.0),
			daysHeld:  730,
			expected:  decimal.Zero,
			wantErr:   true,
		},
		{
			name:      "Validation: negative days held",
			principal: decimal.NewFromFloat(10000.0),
			yieldRate: decimal.NewFromFloat(3.5),
			daysHeld:  -10,
			expected:  decimal.Zero,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CalculateNoteBondMaturityValue(tt.principal, tt.yieldRate, tt.daysHeld)
			if (err != nil) != tt.wantErr {
				t.Errorf("CalculateNoteBondMaturityValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if !result.Equal(tt.expected) {
					t.Errorf("CalculateNoteBondMaturityValue() = %s, want %s", result.String(), tt.expected.String())
				}
			}
		})
	}
}
