package models

import (
	"encoding/xml"

	"github.com/shopspring/decimal"
)

type YieldPoint struct {
	Term string          `json:"term"`
	Rate decimal.Decimal `json:"rate"`
}

type YieldData struct {
	Date   string       `json:"date"`
	Yields []YieldPoint `json:"yields"`
}

type TreasuryFeed struct {
	XMLName xml.Name `xml:"feed"`
	Entries []Entry  `xml:"entry"`
}

type Entry struct {
	Date     string  `xml:"content>properties>NEW_DATE"`
	BC1Month float64 `xml:"content>properties>BC_1MONTH"`
	BC3Month float64 `xml:"content>properties>BC_3MONTH"`
	BC6Month float64 `xml:"content>properties>BC_6MONTH"`
	BC1Year  float64 `xml:"content>properties>BC_1YEAR"`
	BC2Year  float64 `xml:"content>properties>BC_2YEAR"`
	BC5Year  float64 `xml:"content>properties>BC_5YEAR"`
	BC10Year float64 `xml:"content>properties>BC_10YEAR"`
	BC30Year float64 `xml:"content>properties>BC_30YEAR"`
}

type YieldDataPoint struct {
	Date     string          `json:"date"`
	Yield2Y  decimal.Decimal `json:"2Y"`
	Yield5Y  decimal.Decimal `json:"5Y"`
	Yield10Y decimal.Decimal `json:"10Y"`
}

type HistoricalYieldData struct {
	Period    string           `json:"period"`
	StartDate string           `json:"startDate"`
	EndDate   string           `json:"endDate"`
	Terms     []string         `json:"terms"`
	Data      []YieldDataPoint `json:"data"`
}
