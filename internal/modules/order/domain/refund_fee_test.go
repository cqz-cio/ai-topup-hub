package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestCalculatePaymentFeeRefundAmountAllocatesAndAbsorbsRounding(t *testing.T) {
	paymentAmount := decimal.RequireFromString("100.00")
	paymentFee := decimal.RequireFromString("3.01")

	first := CalculatePaymentFeeRefundAmount(
		paymentAmount,
		paymentFee,
		decimal.Zero,
		decimal.Zero,
		decimal.RequireFromString("33.33"),
	)
	if !first.Equal(decimal.RequireFromString("1.00")) {
		t.Fatalf("first fee refund = %s, want 1.00", first)
	}

	second := CalculatePaymentFeeRefundAmount(
		paymentAmount,
		paymentFee,
		decimal.RequireFromString("33.33"),
		first,
		decimal.RequireFromString("66.67"),
	)
	if !second.Equal(decimal.RequireFromString("2.01")) {
		t.Fatalf("final fee refund = %s, want 2.01", second)
	}
}

func TestCalculatePaymentFeeRefundAmountCapsAtOriginalFee(t *testing.T) {
	got := CalculatePaymentFeeRefundAmount(
		decimal.RequireFromString("80.00"),
		decimal.RequireFromString("2.40"),
		decimal.RequireFromString("60.00"),
		decimal.RequireFromString("1.80"),
		decimal.RequireFromString("50.00"),
	)
	if !got.Equal(decimal.RequireFromString("0.60")) {
		t.Fatalf("capped fee refund = %s, want 0.60", got)
	}
}
