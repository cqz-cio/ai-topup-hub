package bscusdt

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/dujiao-next/internal/modules/payment/contract"
	chaincontract "github.com/dujiao-next/internal/modules/payment/contract/chain"
	"github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/shopspring/decimal"
)

type Config struct {
	Recipient     string
	CNYPerUSDT    string
	Confirmations uint64
}

// Settle accepts only persisted, independently verified finalized transfers.
type Settle func(context.Context, domain.ChainInvoice, domain.ChainTransfer) error
type Service struct {
	store  chaincontract.Store
	chain  chaincontract.Reader
	config Config
	rate   decimal.Decimal
	stream string
	settle Settle
}

func New(store chaincontract.Store, chain chaincontract.Reader, cfg Config, settle Settle) (*Service, error) {
	cfg.Recipient = strings.ToLower(strings.TrimSpace(cfg.Recipient))
	rate, err := decimal.NewFromString(cfg.CNYPerUSDT)
	if store == nil || chain == nil || err != nil || !rate.IsPositive() || rate.GreaterThan(decimal.NewFromInt(1000000)) || !isHex(cfg.Recipient, 20) || cfg.Recipient == "0x"+strings.Repeat("0", 40) {
		return nil, contract.ErrGatewayConfigInvalid
	}
	if cfg.Confirmations == 0 {
		cfg.Confirmations = 15
	}
	if cfg.Confirmations > 1000 {
		return nil, contract.ErrGatewayConfigInvalid
	}
	return &Service{store: store, chain: chain, config: cfg, rate: rate, stream: fmt.Sprintf("%d/%s/%s", ChainID, Token, cfg.Recipient), settle: settle}, nil
}
func (s *Service) Type() string { return Provider + ":" }
func (s *Service) ValidateConfig(_ jsonmap.JSON, channel string) error {
	if channel != "usdt-bep20" {
		return contract.ErrGatewayConfigInvalid
	}
	return nil
}
func (s *Service) CreatePayment(ctx context.Context, _ jsonmap.JSON, in contract.GatewayCreateInput) (*contract.GatewayCreateResult, error) {
	if in.OrderID == 0 || in.PaymentID == 0 || strings.ToUpper(in.Currency) != "CNY" || !in.Amount.Decimal.IsPositive() {
		return nil, contract.ErrGatewayConfigInvalid
	}
	if existing, err := s.store.Invoice(ctx, in.PaymentID); err == nil {
		if existing.State != "pending" || !time.Now().Before(existing.ExpiresAt) {
			return nil, contract.ErrGatewayConfigInvalid
		}
		return invoiceResult(existing), nil
	} else if !errors.Is(err, chaincontract.ErrNotFound) {
		return nil, err
	}
	if err := s.chain.Check(ctx); err != nil {
		return nil, err
	}
	latest, err := s.chain.Header(ctx, "latest")
	if err != nil {
		return nil, err
	}
	head, err := quantity(latest.Number)
	if err != nil {
		return nil, err
	}
	stamp, err := quantity(latest.Timestamp)
	if err != nil || time.Since(time.Unix(int64(stamp), 0)) > 2*time.Minute || time.Unix(int64(stamp), 0).After(time.Now().Add(30*time.Second)) {
		return nil, ErrRPC
	}
	expires, err := s.store.PayableUntil(ctx, in.OrderID)
	if err != nil {
		return nil, err
	}
	if expires == nil || !time.Now().Before(*expires) {
		return nil, contract.ErrGatewayConfigInvalid
	}
	// Two fiat decimals remain in payments. Chain precision is stored separately
	// as an integer string; no float or money.Amount rounding is permitted here.
	base := in.Amount.Decimal.Div(s.rate).RoundCeil(2)
	seed, err := rand.Int(rand.Reader, big.NewInt(9999))
	if err != nil {
		return nil, err
	}
	var invoice domain.ChainInvoice
	for attempt := int64(0); attempt < 9999; attempt++ {
		amount := base.Add(decimal.NewFromInt(1 + (seed.Int64()+attempt)%9999).Shift(-6))
		units := amount.Shift(18).BigInt().String()
		invoice = domain.ChainInvoice{PaymentID: in.PaymentID, Stream: s.stream, AmountKey: s.stream + "/" + units, Units: units, Amount: amount.StringFixed(6), FiatAmount: in.Amount.String(), Currency: "CNY", CNYPerUSDT: s.rate.String(), Recipient: s.config.Recipient, StartBlock: head + 1, ExpiresAt: *expires, State: "pending"}
		created, err := s.store.CreateInvoice(ctx, &invoice, &domain.ChainCursor{Stream: s.stream, NextBlock: head + 1})
		if err != nil {
			return nil, err
		}
		if created {
			return invoiceResult(&invoice), nil
		}
		if existing, e := s.store.Invoice(ctx, in.PaymentID); e == nil {
			return invoiceResult(existing), nil
		} else if !errors.Is(e, chaincontract.ErrNotFound) {
			return nil, e
		}
	}
	return nil, errors.New("bsc_unique_amount_capacity_exhausted")
}

func invoiceResult(v *domain.ChainInvoice) *contract.GatewayCreateResult {
	return &contract.GatewayCreateResult{ExpiresAt: &v.ExpiresAt, ProviderRef: fmt.Sprintf("bsc-invoice-%d", v.PaymentID), QRCodeURL: v.Recipient, DisplayChannelType: "usdt-bep20", Payload: jsonmap.JSON{"wallet_address": v.Recipient, "chain_amount": v.Amount, "chain": "bsc", "token_id": "USDT", "token_contract": Token, "chain_id": ChainID, "cny_per_usdt": v.CNYPerUSDT, "invoice_expires_at": v.ExpiresAt.UTC().Format(time.RFC3339)}}
}
