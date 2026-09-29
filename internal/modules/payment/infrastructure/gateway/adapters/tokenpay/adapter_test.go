package tokenpayadapter

import (
	"context"
	"errors"
	"testing"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/tokenpay"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

func TestTokenpayAdapter_Type(t *testing.T) {
	a := NewTokenpayAdapter()
	want := constants.PaymentProviderTokenpay + ":"
	if got := a.Type(); got != want {
		t.Fatalf("Type() = %q, want %q", got, want)
	}
}

func TestTokenpayAdapter_ValidateConfig_EmptyRejected(t *testing.T) {
	a := NewTokenpayAdapter()
	err := a.ValidateConfig(jsonmap.JSON{}, "")
	if err == nil {
		t.Fatalf("expected error from empty config")
	}
	if !errors.Is(err, paymentcontract.ErrGatewayConfigInvalid) {
		t.Fatalf("expected wrapped paymentcontract.ErrGatewayConfigInvalid, got %v", err)
	}
}

func TestTokenpayAdapter_CreatePayment_ConfigInvalidMapped(t *testing.T) {
	a := NewTokenpayAdapter()
	_, err := a.CreatePayment(context.Background(), jsonmap.JSON{}, paymentcontract.GatewayCreateInput{
		OrderNo:  "ORDER_1",
		Currency: "USDT",
	})
	if err == nil {
		t.Fatalf("expected error from empty config")
	}
	if !errors.Is(err, paymentcontract.ErrGatewayConfigInvalid) {
		t.Fatalf("expected wrapped paymentcontract.ErrGatewayConfigInvalid, got %v", err)
	}
}

func TestTokenpayAdapter_MapTokenpayError(t *testing.T) {
	cases := []struct {
		name string
		in   error
		want error
	}{
		{"config", tokenpay.ErrConfigInvalid, paymentcontract.ErrGatewayConfigInvalid},
		{"request", tokenpay.ErrRequestFailed, paymentcontract.ErrGatewayRequestFailed},
		{"response", tokenpay.ErrResponseInvalid, paymentcontract.ErrGatewayResponseInvalid},
		{"signature", tokenpay.ErrSignatureInvalid, paymentcontract.ErrGatewaySignatureInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapTokenpayError(tc.in)
			if !errors.Is(got, tc.want) {
				t.Fatalf("mapTokenpayError(%v) errors.Is %v = false, want true", tc.in, tc.want)
			}
		})
	}
}
