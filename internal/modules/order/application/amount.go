package application

import "github.com/shopspring/decimal"

func normalizeOrderAmount(amount decimal.Decimal) decimal.Decimal {
	normalized := amount.Round(2)
	if normalized.LessThan(decimal.Zero) {
		return decimal.Zero
	}
	return normalized
}
