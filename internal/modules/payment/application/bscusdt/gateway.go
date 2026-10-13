package bscusdt

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/modules/payment/contract"
	chaincontract "github.com/dujiao-next/internal/modules/payment/contract/chain"
	"github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/shopspring/decimal"
)

// Each quote may use its rounded base through base + 0.99 USDT. Historical
// reservations remain occupied; exhaustion must never cause an unbounded uplift.
const maxAmountCandidates = 100

var ErrAmountCapacityExhausted = errors.New("chain_unique_amount_capacity_exhausted")

type Config struct {
	Network       string
	Recipient     string
	CNYPerUSDT    string
	Confirmations uint64
}

// Settle accepts only persisted, independently verified finalized transfers.
type Settle func(context.Context, domain.ChainInvoice, domain.ChainTransfer) error
type Service struct {
	store   chaincontract.Store
	chain   chaincontract.Reader
	config  Config
	rate    decimal.Decimal
	stream  string
	settle  Settle
	network chaincontract.Network
}

func New(store chaincontract.Store, chain chaincontract.Reader, cfg Config, settle Settle) (*Service, error) {
	network, err := chaincontract.ResolveNetwork(cfg.Network)
	if err != nil {
		return nil, contract.ErrGatewayConfigInvalid
	}
	cfg.Network = network.Name
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
	return &Service{store: store, chain: chain, config: cfg, rate: rate, stream: fmt.Sprintf("%d/%s/%s", network.ChainID, network.Token, cfg.Recipient), settle: settle, network: network}, nil
}
func (s *Service) Type() string { return s.network.Provider + ":" }
func (s *Service) ValidateConfig(_ jsonmap.JSON, channel string) error {
	if channel != s.network.ChannelType {
		return contract.ErrGatewayConfigInvalid
	}
	return nil
}
func (s *Service) CreatePayment(ctx context.Context, _ jsonmap.JSON, in contract.GatewayCreateInput) (*contract.GatewayCreateResult, error) {
	if in.PaymentID == 0 || strings.ToUpper(in.Currency) != "CNY" || !in.Amount.Decimal.IsPositive() {
		return nil, contract.ErrGatewayConfigInvalid
	}
	if existing, err := s.store.Invoice(ctx, in.PaymentID); err == nil {
		return s.resumeInvoice(existing, in)
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
	var expires *time.Time
	if in.OrderID == 0 {
		expires, err = s.store.RechargePayableUntil(ctx, in.PaymentID)
	} else {
		expires, err = s.store.PayableUntil(ctx, in.OrderID)
	}
	if err != nil {
		return nil, err
	}
	if expires == nil || !time.Now().Before(*expires) {
		return nil, contract.ErrGatewayConfigInvalid
	}
	// Two fiat decimals remain in payments. Chain precision is stored separately
	// as an integer string; no float or money.Amount rounding is permitted here.
	// QuoRem preserves a positive remainder even just above a cent boundary;
	// Div's finite precision could round it away before RoundCeil sees it.
	base, remainder := in.Amount.Decimal.QuoRem(s.rate, 2)
	cent := decimal.New(1, -2)
	if remainder.IsPositive() {
		base = base.Add(cent)
	}
	var invoice domain.ChainInvoice
	for attempt := int64(0); attempt < maxAmountCandidates; attempt++ {
		// The store reserves exact units atomically across all orders/recharges
		// on this stream. Always try the lowest unallocated cent first.
		amount := base.Add(cent.Mul(decimal.NewFromInt(attempt)))
		units := amount.Shift(s.network.Decimals).BigInt().String()
		invoice = domain.ChainInvoice{PaymentID: in.PaymentID, Stream: s.stream, AmountKey: s.stream + "/" + units, Units: units, Amount: amount.StringFixed(2), FiatAmount: in.Amount.String(), Currency: "CNY", CNYPerUSDT: s.rate.String(), Recipient: s.config.Recipient, StartBlock: head + 1, ExpiresAt: *expires, State: "pending"}
		created, err := s.store.CreateInvoice(ctx, &invoice, &domain.ChainCursor{Stream: s.stream, NextBlock: head + 1})
		if err != nil {
			return nil, err
		}
		if created {
			return s.invoiceResult(&invoice), nil
		}
		if existing, e := s.store.Invoice(ctx, in.PaymentID); e == nil {
			return s.resumeInvoice(existing, in)
		} else if !errors.Is(e, chaincontract.ErrNotFound) {
			return nil, e
		}
	}
	return nil, ErrAmountCapacityExhausted
}

func (s *Service) resumeInvoice(v *domain.ChainInvoice, in contract.GatewayCreateInput) (*contract.GatewayCreateResult, error) {
	fiat, err := decimal.NewFromString(v.FiatAmount)
	if err != nil || !fiat.Equal(in.Amount.Decimal) || v.Currency != strings.ToUpper(in.Currency) || v.Stream != s.stream || v.Recipient != s.config.Recipient || v.State != "pending" || !time.Now().Before(v.ExpiresAt) {
		return nil, contract.ErrGatewayConfigInvalid
	}
	return s.invoiceResult(v), nil
}

func (s *Service) invoiceResult(v *domain.ChainInvoice) *contract.GatewayCreateResult {
	return &contract.GatewayCreateResult{ExpiresAt: &v.ExpiresAt, ProviderRef: fmt.Sprintf("%s-invoice-%d", s.network.Name, v.PaymentID), QRCodeURL: v.Recipient, DisplayChannelType: s.network.ChannelType, Payload: jsonmap.JSON{"wallet_address": v.Recipient, "chain_amount": v.Amount, "chain": s.network.Name, "token_id": s.network.TokenID, "token_contract": s.network.Token, "chain_id": s.network.ChainID, "cny_per_usdt": v.CNYPerUSDT, "invoice_expires_at": v.ExpiresAt.UTC().Format(time.RFC3339)}}
}
