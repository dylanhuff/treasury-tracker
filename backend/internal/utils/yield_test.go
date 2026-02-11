package utils

import (
	"math"
	"testing"
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
		faceValue float64
		yieldRate float64
		term      string
		expected  float64
		wantErr   bool
	}{
		{
			name:      "Standard scenario: 6M bill at 4.5% yield",
			faceValue: 10000.0,
			yieldRate: 4.5,
			term:      "6M",
			expected:  9775.0, // 10000 × (1 - (4.5/100 × 180)/360) = 10000 × 0.9775
			wantErr:   false,
		},
		{
			name:      "Zero yield edge case",
			faceValue: 10000.0,
			yieldRate: 0.0,
			term:      "6M",
			expected:  10000.0, // Price should equal face value when yield is zero
			wantErr:   false,
		},
		{
			name:      "High yield scenario: 10% yield",
			faceValue: 10000.0,
			yieldRate: 10.0,
			term:      "6M",
			expected:  9500.0, // 10000 × (1 - (10/100 × 180)/360) = 10000 × 0.95
			wantErr:   false,
		},
		{
			name:      "Short term: 1M at 3% yield",
			faceValue: 10000.0,
			yieldRate: 3.0,
			term:      "1M",
			expected:  9975.0, // 10000 × (1 - (3/100 × 30)/360) = 10000 × 0.9975
			wantErr:   false,
		},
		{
			name:      "Long term: 1Y at 5% yield",
			faceValue: 10000.0,
			yieldRate: 5.0,
			term:      "1Y",
			expected:  9493.06, // 10000 × (1 - (5/100 × 365)/360) = 10000 × 0.9493056
			wantErr:   false,
		},
		{
			name:      "Different face value: $50,000 at 4% for 3M",
			faceValue: 50000.0,
			yieldRate: 4.0,
			term:      "3M",
			expected:  49500.0, // 50000 × (1 - (4/100 × 90)/360) = 50000 × 0.99
			wantErr:   false,
		},
		{
			name:      "Validation: negative yield should error",
			faceValue: 10000.0,
			yieldRate: -1.0,
			term:      "6M",
			expected:  0.0,
			wantErr:   true,
		},
		{
			name:      "Validation: yield over 100 should error",
			faceValue: 10000.0,
			yieldRate: 101.0,
			term:      "6M",
			expected:  0.0,
			wantErr:   true,
		},
		{
			name:      "Validation: invalid term should error",
			faceValue: 10000.0,
			yieldRate: 4.5,
			term:      "2Y", // 2Y is not valid for T-Bills (only 1M, 3M, 6M, 1Y)
			expected:  0.0,
			wantErr:   true,
		},
		{
			name:      "Validation: invalid term format should error",
			faceValue: 10000.0,
			yieldRate: 4.5,
			term:      "invalid",
			expected:  0.0,
			wantErr:   true,
		},
		{
			name:      "Validation: zero face value should error",
			faceValue: 0.0,
			yieldRate: 4.5,
			term:      "6M",
			expected:  0.0,
			wantErr:   true,
		},
		{
			name:      "Validation: negative face value should error",
			faceValue: -10000.0,
			yieldRate: 4.5,
			term:      "6M",
			expected:  0.0,
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
				// Use a small epsilon for floating point comparison
				if math.Abs(result-tt.expected) > 0.01 {
					t.Errorf("CalculateBillPrice() = %f, want %f", result, tt.expected)
				}
			}
		})
	}
}

func TestCalculateBillDiscount(t *testing.T) {
	tests := []struct {
		name          string
		faceValue     float64
		purchasePrice float64
		expected      float64
	}{
		{
			name:          "Standard discount calculation",
			faceValue:     10000.0,
			purchasePrice: 9775.0,
			expected:      225.0,
		},
		{
			name:          "Zero discount (par purchase)",
			faceValue:     10000.0,
			purchasePrice: 10000.0,
			expected:      0.0,
		},
		{
			name:          "Large discount",
			faceValue:     100000.0,
			purchasePrice: 95000.0,
			expected:      5000.0,
		},
		{
			name:          "Small face value",
			faceValue:     1000.0,
			purchasePrice: 975.0,
			expected:      25.0,
		},
		{
			name:          "Cents precision",
			faceValue:     10000.0,
			purchasePrice: 9775.55,
			expected:      224.45,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateBillDiscount(tt.faceValue, tt.purchasePrice)
			if math.Abs(result-tt.expected) > 0.01 {
				t.Errorf("CalculateBillDiscount() = %f, want %f", result, tt.expected)
			}
		})
	}
}

func TestCalculateBillPriceAllTerms(t *testing.T) {
	faceValue := 10000.0
	yieldRate := 4.5

	// Only T-Bill terms (1M, 3M, 6M, 1Y)
	terms := []string{"1M", "3M", "6M", "1Y"}

	for _, term := range terms {
		t.Run(term, func(t *testing.T) {
			price, err := CalculateBillPrice(faceValue, yieldRate, term)
			if err != nil {
				t.Errorf("CalculateBillPrice failed for %s: %v", term, err)
				return
			}

			// Price should always be less than or equal to face value
			if price > faceValue {
				t.Errorf("Price (%f) should not exceed face value (%f) for term %s", price, faceValue, term)
			}

			// Price should be positive
			if price <= 0 {
				t.Errorf("Price should be positive for term %s, got %f", term, price)
			}

			// Discount should increase with term length
			discount := CalculateBillDiscount(faceValue, price)
			if discount < 0 {
				t.Errorf("Discount should be non-negative for term %s, got %f", term, discount)
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
		faceValue float64
		yieldRate float64
		term      string
		expected  float64
		wantErr   bool
	}{
		{
			name:      "2Y Note at par",
			faceValue: 10000.0,
			yieldRate: 3.5,
			term:      "2Y",
			expected:  10000.0,
			wantErr:   false,
		},
		{
			name:      "5Y Note at par",
			faceValue: 50000.0,
			yieldRate: 3.8,
			term:      "5Y",
			expected:  50000.0,
			wantErr:   false,
		},
		{
			name:      "10Y Note at par",
			faceValue: 100000.0,
			yieldRate: 4.0,
			term:      "10Y",
			expected:  100000.0,
			wantErr:   false,
		},
		{
			name:      "30Y Bond at par",
			faceValue: 250000.0,
			yieldRate: 4.5,
			term:      "30Y",
			expected:  250000.0,
			wantErr:   false,
		},
		{
			name:      "Zero yield - still at par",
			faceValue: 10000.0,
			yieldRate: 0.0,
			term:      "2Y",
			expected:  10000.0,
			wantErr:   false,
		},
		{
			name:      "Small face value",
			faceValue: 1000.0,
			yieldRate: 3.5,
			term:      "5Y",
			expected:  1000.0,
			wantErr:   false,
		},
		{
			name:      "Validation: negative face value",
			faceValue: -10000.0,
			yieldRate: 3.5,
			term:      "2Y",
			expected:  0.0,
			wantErr:   true,
		},
		{
			name:      "Validation: zero face value",
			faceValue: 0.0,
			yieldRate: 3.5,
			term:      "2Y",
			expected:  0.0,
			wantErr:   true,
		},
		{
			name:      "Validation: negative yield rate",
			faceValue: 10000.0,
			yieldRate: -1.0,
			term:      "2Y",
			expected:  0.0,
			wantErr:   true,
		},
		{
			name:      "Validation: yield rate over 100",
			faceValue: 10000.0,
			yieldRate: 101.0,
			term:      "2Y",
			expected:  0.0,
			wantErr:   true,
		},
		{
			name:      "Validation: invalid term for Notes/Bonds",
			faceValue: 10000.0,
			yieldRate: 3.5,
			term:      "1M",
			expected:  0.0,
			wantErr:   true,
		},
		{
			name:      "Validation: invalid term format",
			faceValue: 10000.0,
			yieldRate: 3.5,
			term:      "6Y",
			expected:  0.0,
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
				// Use a small epsilon for floating point comparison
				if math.Abs(result-tt.expected) > 0.01 {
					t.Errorf("CalculateNoteBondPrice() = %f, want %f", result, tt.expected)
				}
			}
		})
	}
}

func TestCalculateNoteBondMaturityValue(t *testing.T) {
	tests := []struct {
		name      string
		principal float64
		yieldRate float64
		daysHeld  int
		expected  float64
		wantErr   bool
	}{
		{
			name:      "2Y note at 3.5% held full term (730 days)",
			principal: 10000.0,
			yieldRate: 3.5,
			daysHeld:  730,
			expected:  10700.0, // 10000 + (10000 × 0.035 × (730/365)) = 10000 + 700
			wantErr:   false,
		},
		{
			name:      "10Y note at 4.0% held 1 year (365 days)",
			principal: 10000.0,
			yieldRate: 4.0,
			daysHeld:  365,
			expected:  10400.0, // 10000 + (10000 × 0.04 × (365/365)) = 10000 + 400
			wantErr:   false,
		},
		{
			name:      "30Y bond at 4.5% held 6 months (180 days)",
			principal: 10000.0,
			yieldRate: 4.5,
			daysHeld:  180,
			expected:  10221.92, // 10000 + (10000 × 0.045 × (180/365)) = 10000 + 221.92
			wantErr:   false,
		},
		{
			name:      "5Y note at 3.8% held full term (1825 days)",
			principal: 50000.0,
			yieldRate: 3.8,
			daysHeld:  1825,
			expected:  59500.0, // 50000 + (50000 × 0.038 × (1825/365)) = 50000 + 9500
			wantErr:   false,
		},
		{
			name:      "Large principal: $1M at 4% for 1 year",
			principal: 1000000.0,
			yieldRate: 4.0,
			daysHeld:  365,
			expected:  1040000.0, // 1000000 + 40000
			wantErr:   false,
		},
		{
			name:      "Zero days held",
			principal: 10000.0,
			yieldRate: 3.5,
			daysHeld:  0,
			expected:  10000.0, // No interest accrued
			wantErr:   false,
		},
		{
			name:      "Zero yield rate",
			principal: 10000.0,
			yieldRate: 0.0,
			daysHeld:  730,
			expected:  10000.0, // No interest at 0% yield
			wantErr:   false,
		},
		{
			name:      "Partial year: 100 days",
			principal: 10000.0,
			yieldRate: 5.0,
			daysHeld:  100,
			expected:  10136.99, // 10000 + (10000 × 0.05 × (100/365))
			wantErr:   false,
		},
		{
			name:      "Very short holding: 1 day",
			principal: 100000.0,
			yieldRate: 4.0,
			daysHeld:  1,
			expected:  100010.96, // 100000 + (100000 × 0.04 × (1/365))
			wantErr:   false,
		},
		{
			name:      "High yield scenario: 10%",
			principal: 10000.0,
			yieldRate: 10.0,
			daysHeld:  365,
			expected:  11000.0, // 10000 + 1000
			wantErr:   false,
		},
		{
			name:      "Validation: negative principal",
			principal: -10000.0,
			yieldRate: 3.5,
			daysHeld:  730,
			expected:  0.0,
			wantErr:   true,
		},
		{
			name:      "Validation: zero principal",
			principal: 0.0,
			yieldRate: 3.5,
			daysHeld:  730,
			expected:  0.0,
			wantErr:   true,
		},
		{
			name:      "Validation: negative yield rate",
			principal: 10000.0,
			yieldRate: -1.0,
			daysHeld:  730,
			expected:  0.0,
			wantErr:   true,
		},
		{
			name:      "Validation: yield rate over 100",
			principal: 10000.0,
			yieldRate: 101.0,
			daysHeld:  730,
			expected:  0.0,
			wantErr:   true,
		},
		{
			name:      "Validation: negative days held",
			principal: 10000.0,
			yieldRate: 3.5,
			daysHeld:  -10,
			expected:  0.0,
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
				// Use a small epsilon for floating point comparison
				if math.Abs(result-tt.expected) > 0.01 {
					t.Errorf("CalculateNoteBondMaturityValue() = %f, want %f", result, tt.expected)
				}
			}
		})
	}
}
