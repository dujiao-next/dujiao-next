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

// CalculatePaymentFeeRefundAmount 按累计退款比例分摊手续费，最终退款吸收尾差且不超过原手续费。
func CalculatePaymentFeeRefundAmount(
	paymentAmount, paymentFee, refundedPrincipalBefore, refundedFeeBefore, refundAmount decimal.Decimal,
) decimal.Decimal {
	paymentAmount = paymentAmount.Round(2)
	paymentFee = paymentFee.Round(2)
	refundedPrincipalBefore = refundedPrincipalBefore.Round(2)
	refundedFeeBefore = refundedFeeBefore.Round(2)
	refundAmount = refundAmount.Round(2)
	if paymentAmount.LessThanOrEqual(decimal.Zero) ||
		paymentFee.LessThanOrEqual(decimal.Zero) ||
		refundAmount.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero
	}

	if refundedPrincipalBefore.IsNegative() {
		refundedPrincipalBefore = decimal.Zero
	}
	if refundedFeeBefore.IsNegative() {
		refundedFeeBefore = decimal.Zero
	}
	if refundedPrincipalBefore.GreaterThan(paymentAmount) {
		refundedPrincipalBefore = paymentAmount
	}
	if refundedFeeBefore.GreaterThan(paymentFee) {
		refundedFeeBefore = paymentFee
	}

	cumulativePrincipal := refundedPrincipalBefore.Add(refundAmount)
	if cumulativePrincipal.GreaterThan(paymentAmount) {
		cumulativePrincipal = paymentAmount
	}
	targetRefundedFee := paymentFee.Mul(cumulativePrincipal).Div(paymentAmount).Round(2)
	if cumulativePrincipal.GreaterThanOrEqual(paymentAmount) {
		targetRefundedFee = paymentFee
	}

	current := targetRefundedFee.Sub(refundedFeeBefore).Round(2)
	if current.IsNegative() {
		return decimal.Zero
	}
	remaining := paymentFee.Sub(refundedFeeBefore).Round(2)
	if current.GreaterThan(remaining) {
		return remaining
	}
	return current
}
