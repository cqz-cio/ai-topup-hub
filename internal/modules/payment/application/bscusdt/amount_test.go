package bscusdt

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/dujiao-next/internal/modules/payment/contract"
	chaincontract "github.com/dujiao-next/internal/modules/payment/contract/chain"
	"github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

func centInput(id uint, fiat string) contract.GatewayCreateInput {
	return contract.GatewayCreateInput{PaymentID: id, OrderID: 1, Amount: money.FromDecimal(decimal.RequireFromString(fiat)), Currency: "CNY"}
}

func TestInvoiceRoundsUpToCentsOnBothNetworks(t *testing.T) {
	for _, network := range []string{"bsc", "plasma"} {
		for _, tc := range []struct{ name, fiat, rate, want string }{
			{"current_rate", "100.00", "6.649820455", "15.04"},
			{"exact_cent", "66.60", "6.66", "10.00"},
			{"round_up", "100.00", "6.66", "15.02"},
			{"carry", "66.59", "6.66", "10.00"},
			{"small", "0.01", "10", "0.01"},
			{"exact_small", "0.01", "1", "0.01"},
			{"above_boundary", "1.00", "0.99999999999999999999", "1.01"},
		} {
			t.Run(network+"/"+tc.name, func(t *testing.T) {
				f := setupNetwork(t, network)
				f.s.rate = decimal.RequireFromString(tc.rate)
				r, err := f.s.CreatePayment(context.Background(), nil, centInput(2, tc.fiat))
				if err != nil {
					t.Fatal(err)
				}
				v, err := f.s.store.Invoice(context.Background(), 2)
				if err != nil {
					t.Fatal(err)
				}
				if v.Amount != tc.want || r.Payload["chain_amount"] != tc.want || v.Units != decimal.RequireFromString(tc.want).Shift(f.s.network.Decimals).BigInt().String() {
					t.Fatalf("wrong quote: %+v", v)
				}
				if v.FiatAmount != tc.fiat || v.CNYPerUSDT != tc.rate {
					t.Fatal("fiat or rate snapshot changed")
				}
			})
		}
	}
}

func TestConcurrentInvoicesReserveLowestAvailableCents(t *testing.T) {
	f := setup(t) // 20.00 already reserved.
	const count = 16
	failures := make(chan error, count)
	var wg sync.WaitGroup
	for id := uint(2); id <= count+1; id++ {
		wg.Add(1)
		go func(id uint) {
			defer wg.Done()
			_, err := f.s.CreatePayment(context.Background(), nil, centInput(id, "133.20"))
			failures <- err
		}(id)
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	var invoices []domain.ChainInvoice
	if err := f.db.Order("units").Find(&invoices).Error; err != nil {
		t.Fatal(err)
	}
	if len(invoices) != count+1 {
		t.Fatal("missing invoices", len(invoices))
	}
	for n, invoice := range invoices {
		if invoice.Amount != fmt.Sprintf("20.%02d", n) {
			t.Fatal("amount gap or duplicate", invoice.Amount)
		}
	}
	// A neighboring fiat quote shares the same stream-wide reservation pool.
	r, err := f.s.CreatePayment(context.Background(), nil, centInput(100, "133.21"))
	if err != nil || r.Payload["chain_amount"] != "20.17" {
		t.Fatal("neighboring quote reused occupied cents", err)
	}
}

func TestHistoricalCentAmountsAreNeverReused(t *testing.T) {
	for _, state := range []string{"pending", "confirmed", "settled"} {
		t.Run(state, func(t *testing.T) {
			f := setup(t)
			if err := f.db.Model(&domain.ChainInvoice{}).Where("id = ?", f.invoice.ID).Updates(map[string]any{"state": state, "expires_at": time.Now().Add(-time.Hour)}).Error; err != nil {
				t.Fatal(err)
			}
			r, err := f.s.CreatePayment(context.Background(), nil, centInput(2, "133.20"))
			if err != nil || r.Payload["chain_amount"] != "20.01" {
				t.Fatal("historical amount reused", err)
			}
			f.chain.head, f.chain.final = 120, 120
			f.chain.logs = []Log{f.event(f.invoice.Units, 101, 0)}
			f.run(t)
			if f.calls != 0 {
				t.Fatal("late old transfer paid a new invoice")
			}
		})
	}
}

func TestCentCapacityStopsAtNinetyNineCentUplift(t *testing.T) {
	f := setup(t)
	for n := 1; n < maxAmountCandidates; n++ {
		v := f.invoice
		v.ID, v.PaymentID = 0, uint(n+1)
		v.Amount = fmt.Sprintf("20.%02d", n)
		v.Units = decimal.RequireFromString(v.Amount).Shift(f.s.network.Decimals).BigInt().String()
		v.AmountKey = v.Stream + "/" + v.Units
		v.ExpiresAt = time.Now().Add(-time.Hour)
		if err := f.db.Create(&v).Error; err != nil {
			t.Fatal(err)
		}
	}
	r, err := f.s.CreatePayment(context.Background(), nil, centInput(500, "133.20"))
	if r != nil || !errors.Is(err, ErrAmountCapacityExhausted) {
		t.Fatal("unbounded amount allocation", err)
	}
	if _, err := f.s.store.Invoice(context.Background(), 500); !errors.Is(err, chaincontract.ErrNotFound) {
		t.Fatal("failed quote persisted", err)
	}
}

func TestLegacySixDecimalInvoiceResumesAndSettlesUnchanged(t *testing.T) {
	for _, network := range []string{"bsc", "plasma"} {
		t.Run(network, func(t *testing.T) {
			f := setupNetwork(t, network)
			f.invoice.Amount = "20.003517"
			f.invoice.Units = decimal.RequireFromString(f.invoice.Amount).Shift(f.s.network.Decimals).BigInt().String()
			f.invoice.AmountKey = f.invoice.Stream + "/" + f.invoice.Units
			if err := f.db.Save(&f.invoice).Error; err != nil {
				t.Fatal(err)
			}
			f.s.rate = decimal.RequireFromString("6.649820455")
			r, err := f.s.CreatePayment(context.Background(), nil, centInput(1, "133.20"))
			if err != nil || r.Payload["chain_amount"] != "20.003517" || r.Payload["cny_per_usdt"] != "6.66" {
				t.Fatal("legacy quote changed", err)
			}
			f.chain.head, f.chain.final = 120, 120
			f.chain.logs = []Log{f.event(f.invoice.Units, 101, 0)}
			f.run(t)
			f.run(t)
			if f.calls != 1 {
				t.Fatal("legacy transfer not settled exactly once", f.calls)
			}
		})
	}
}
