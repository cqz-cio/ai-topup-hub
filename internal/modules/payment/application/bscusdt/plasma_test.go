package bscusdt

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/modules/payment/contract"
	"github.com/dujiao-next/internal/modules/payment/domain"
	chainstore "github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore/chain"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

func TestPlasmaInvoiceUsesSixDecimalTokenAndCorrectInstructions(t *testing.T) {
	f := setupNetwork(t, "plasma")
	amount := decimal.RequireFromString(f.invoice.Amount)
	if f.invoice.Units != amount.Shift(6).BigInt().String() || f.invoice.Amount != "20.0000" {
		t.Fatal("wrong Plasma token amount", f.invoice.Units)
	}
	r := f.s.invoiceResult(&f.invoice)
	if r.DisplayChannelType != "usdt-plasma" || r.Payload["chain"] != "plasma" || r.Payload["chain_id"] != uint64(9745) || r.Payload["token_id"] != "USDT0" || r.Payload["token_contract"] != "0xb8ce59fc3717ada4c02eadf9682a9e934f625ebb" || r.QRCodeURL != testRecipient || f.s.Type() != "plasmausdt:" {
		t.Fatal("wrong Plasma payment instructions", r)
	}
	if f.s.ValidateConfig(nil, "usdt-bep20") == nil || f.s.ValidateConfig(nil, "usdt-plasma") != nil {
		t.Fatal("cross-network channel accepted")
	}
}

func TestPlasmaRejectsUnsafeTransfersAndWaitsForFinality(t *testing.T) {
	for _, kind := range []string{"bsc_token", "recipient", "receipt", "removed", "reorg", "unfinalized", "short", "late"} {
		t.Run(kind, func(t *testing.T) {
			f := setupNetwork(t, "plasma")
			event := f.event(f.invoice.Units, 101, 0)
			f.chain.head, f.chain.final = 120, 120
			wantError := true
			switch kind {
			case "bsc_token":
				event.Address = Token
			case "recipient":
				event.Topics[2] = addressTopic(Token)
			case "receipt":
				f.chain.badReceipt = true
			case "removed":
				event.Removed = true
			case "reorg":
				f.chain.reorg = 101
			case "unfinalized":
				f.chain.final, wantError = 100, false
			case "short":
				event = f.event(decimal.RequireFromString(f.invoice.Units).Sub(decimal.NewFromInt(1)).String(), 101, 0)
				wantError = false
			case "late":
				if err := f.db.Model(&domain.ChainInvoice{}).Where("id = ?", f.invoice.ID).Update("expires_at", time.Now().Add(-time.Hour)).Error; err != nil {
					t.Fatal(err)
				}
				wantError = false
			}
			f.chain.logs = []Log{event}
			err := f.s.RunOnce(context.Background())
			if (err != nil) != wantError || f.calls != 0 {
				t.Fatal("unsafe Plasma transfer settled", err, f.calls)
			}
		})
	}
}

func TestNetworksIsolateInvoicesCursorsAndTransactionHashes(t *testing.T) {
	bsc := setup(t)
	plasma := &testFixture{db: bsc.db, chain: &testChain{head: 100, final: 100, timestamp: bsc.chain.timestamp}}
	var err error
	plasma.s, err = New(chainstore.New(bsc.db), plasma.chain, Config{Network: "plasma", Recipient: testRecipient, CNYPerUSDT: "6.66"}, func(_ context.Context, invoice domain.ChainInvoice, transfer domain.ChainTransfer) error {
		if invoice.PaymentID != 2 || invoice.Stream != transfer.Stream || !strings.HasPrefix(transfer.Key, "9745/") {
			t.Fatal("Plasma matched another network's invoice")
		}
		plasma.calls++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	in := contract.GatewayCreateInput{PaymentID: 1, OrderID: 1, Amount: money.FromDecimal(decimal.RequireFromString("133.20")), Currency: "CNY"}
	if _, err = plasma.s.CreatePayment(context.Background(), nil, in); !errors.Is(err, contract.ErrGatewayConfigInvalid) {
		t.Fatal("Plasma reused BSC invoice", err)
	}
	in.PaymentID = 2
	if _, err = plasma.s.CreatePayment(context.Background(), nil, in); err != nil {
		t.Fatal(err)
	}
	if err = bsc.db.First(&plasma.invoice, "payment_id = ?", 2).Error; err != nil {
		t.Fatal(err)
	}
	// A transfer with the BSC invoice's exact integer amount on Plasma must
	// remain unmatched, even though both networks use the same recipient.
	plasma.chain.head, plasma.chain.final = 120, 120
	plasma.chain.logs = []Log{plasma.event(bsc.invoice.Units, 101, 0), plasma.event(plasma.invoice.Units, 102, 1)}
	plasma.run(t)
	plasma.run(t)
	if plasma.calls != 1 || bsc.calls != 0 {
		t.Fatal("cross-chain or duplicate settlement")
	}
	// Identical transaction hashes and log indexes on different chains have
	// different durable transfer keys and must each settle their own invoice.
	bsc.chain.head, bsc.chain.final = 120, 120
	bsc.chain.logs = []Log{bsc.event(bsc.invoice.Units, 102, 1)}
	bsc.run(t)
	bsc.run(t)
	if bsc.calls != 1 || plasma.calls != 1 {
		t.Fatal("network transfer keys collided")
	}
	status, err := plasma.s.Status(context.Background())
	if err != nil || status["chain_id"] != uint64(9745) || status["manual_review_count"] != int64(1) {
		t.Fatal("incorrect per-network status", status, err)
	}
}
