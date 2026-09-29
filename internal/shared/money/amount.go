// Package money provides the shared, database-safe monetary value object used
// across bounded contexts.
package money

import (
	"database/sql/driver"
	"encoding/json"

	"github.com/shopspring/decimal"
)

// Amount stores monetary values at two-decimal precision.
type Amount struct {
	decimal.Decimal
}

// FromDecimal creates an Amount rounded to two decimal places.
func FromDecimal(amount decimal.Decimal) Amount {
	return Amount{Decimal: amount.Round(2)}
}

// MarshalJSON emits a fixed two-decimal string to avoid floating-point loss.
func (m Amount) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.Decimal.Round(2).StringFixed(2))
}

// UnmarshalJSON accepts either a JSON string or a JSON number.
func (m *Amount) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if data[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		parsed, err := decimal.NewFromString(value)
		if err != nil {
			return err
		}
		m.Decimal = parsed.Round(2)
		return nil
	}
	var value float64
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	m.Decimal = decimal.NewFromFloat(value).Round(2)
	return nil
}

// Value implements driver.Valuer.
func (m Amount) Value() (driver.Value, error) {
	return m.Decimal.Round(2).Value()
}

// Scan implements sql.Scanner.
func (m *Amount) Scan(value any) error {
	if err := m.Decimal.Scan(value); err != nil {
		return err
	}
	m.Decimal = m.Decimal.Round(2)
	return nil
}

// String returns a fixed two-decimal representation.
func (m Amount) String() string {
	return m.Decimal.Round(2).StringFixed(2)
}
