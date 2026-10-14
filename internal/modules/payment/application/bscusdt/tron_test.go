package bscusdt

import (
	"context"
	chaincontract "github.com/dujiao-next/internal/modules/payment/contract/chain"
	"github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/shopspring/decimal"
	"strings"
	"testing"
	"time"
)

func TestTronInvoiceAndConfirmedSettlement(t *testing.T) {
	f := setupNetwork(t, "tron")
	r := f.s.invoiceResult(&f.invoice)
	if r.DisplayChannelType != "usdt-trc20" || r.Payload["chain"] != "tron" || r.Payload["token_contract"] != chaincontract.TronUSDTContract || r.QRCodeURL != f.invoice.Recipient || !strings.HasPrefix(r.QRCodeURL, "T") || f.invoice.Units != "20000000" {
		t.Fatal("incorrect TRON invoice", r)
	}
	f.chain.head, f.chain.final = 120, 100
	f.chain.logs = []Log{f.event(f.invoice.Units, 101, 0)}
	f.run(t)
	if f.calls != 0 {
		t.Fatal("unfinalized transfer settled")
	}
	f.chain.final = 120
	f.run(t)
	f.run(t)
	if f.calls != 1 {
		t.Fatal("missing or duplicate settlement")
	}
	var transfer domain.ChainTransfer
	f.db.First(&transfer)
	if !strings.HasPrefix(transfer.Sender, "T") || !strings.HasPrefix(transfer.Key, "728126428/") {
		t.Fatal("incorrect TRON identity")
	}
}

func TestTronUnsafeTransfersNeverSettle(t *testing.T) {
	for _, kind := range []string{"wrong_token", "wrong_recipient", "failed_receipt", "short", "over", "late"} {
		t.Run(kind, func(t *testing.T) {
			f := setupNetwork(t, "tron")
			event := f.event(f.invoice.Units, 101, 0)
			invalid := false
			switch kind {
			case "wrong_token":
				event.Address = Token
				invalid = true
			case "wrong_recipient":
				event.Topics[2] = addressTopic(Token)
				invalid = true
			case "failed_receipt":
				f.chain.badReceipt = true
				invalid = true
			case "short":
				event = f.event(decimal.RequireFromString(f.invoice.Units).Sub(decimal.NewFromInt(1)).String(), 101, 0)
			case "over":
				event = f.event(decimal.RequireFromString(f.invoice.Units).Add(decimal.NewFromInt(1)).String(), 101, 0)
			case "late":
				if err := f.db.Model(&domain.ChainInvoice{}).Where("id = ?", f.invoice.ID).Update("expires_at", time.Now().Add(-time.Hour)).Error; err != nil {
					t.Fatal(err)
				}
			}
			f.chain.head, f.chain.final = 120, 120
			f.chain.logs = []Log{event}
			err := f.s.RunOnce(context.Background())
			if (err != nil) != invalid || f.calls != 0 {
				t.Fatal("unsafe TRON transfer settled", err)
			}
		})
	}
}
