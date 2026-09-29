package epusdtadapter

import (
	"context"
	"errors"
	"testing"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/epusdt"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

func TestEpusdtAdapter_Type(t *testing.T) {
	a := NewEpusdtAdapter()
	want := constants.PaymentProviderEpusdt + ":"
	if got := a.Type(); got != want {
		t.Fatalf("Type() = %q, want %q", got, want)
	}
}

func TestEpusdtAdapter_ValidateConfig_EmptyRejected(t *testing.T) {
	a := NewEpusdtAdapter()
	err := a.ValidateConfig(jsonmap.JSON{}, "")
	if err == nil {
		t.Fatalf("expected error from empty config")
	}
	if !errors.Is(err, paymentcontract.ErrGatewayConfigInvalid) {
		t.Fatalf("expected wrapped paymentcontract.ErrGatewayConfigInvalid, got %v", err)
	}
}

func TestEpusdtAdapter_CreatePayment_ConfigInvalidMapped(t *testing.T) {
	a := NewEpusdtAdapter()
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

func TestEpusdtDisplayChannelType(t *testing.T) {
	tests := []struct {
		name string
		cfg  *epusdt.Config
		want string
	}{
		{name: "nil", cfg: nil, want: ""},
		{
			name: "cashier",
			cfg:  &epusdt.Config{OrderMode: constants.PaymentEpusdtOrderModeCashier},
			want: "",
		},
		{
			name: "tron usdt",
			cfg:  &epusdt.Config{Token: " USDT ", Network: " TRON "},
			want: "usdt.tron",
		},
		{
			name: "tron trx",
			cfg:  &epusdt.Config{Token: " trx ", Network: " tron "},
			want: "trx.tron",
		},
		{
			name: "fallback payment type",
			cfg:  &epusdt.Config{Token: "usdc", Network: "ethereum", PaymentType: "ethereum-usdc"},
			want: "usdc.ethereum",
		},
		{
			name: "unknown without payment type",
			cfg:  &epusdt.Config{Token: "usdc", Network: "ethereum"},
			want: "usdc.ethereum",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := epusdtDisplayChannelType(tc.cfg); got != tc.want {
				t.Fatalf("epusdtDisplayChannelType() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEpusdtAdapter_MapEpusdtError(t *testing.T) {
	cases := []struct {
		name string
		in   error
		want error
	}{
		{"config", epusdt.ErrConfigInvalid, paymentcontract.ErrGatewayConfigInvalid},
		{"request", epusdt.ErrRequestFailed, paymentcontract.ErrGatewayRequestFailed},
		{"response", epusdt.ErrResponseInvalid, paymentcontract.ErrGatewayResponseInvalid},
		{"signature", epusdt.ErrSignatureInvalid, paymentcontract.ErrGatewaySignatureInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapEpusdtError(tc.in)
			if !errors.Is(got, tc.want) {
				t.Fatalf("mapEpusdtError(%v) errors.Is %v = false, want true", tc.in, tc.want)
			}
		})
	}
}
