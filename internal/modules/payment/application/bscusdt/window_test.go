package bscusdt

import (
	"context"
	"fmt"
	"github.com/dujiao-next/internal/modules/payment/domain"
	"sync"
	"testing"
	"time"
)

func TestPaymentWindowsAcrossNetworks(t *testing.T) {
	for _, network := range []string{"bsc", "plasma", "tron"} {
		t.Run(network, func(t *testing.T) {
			f := setupNetwork(t, network)
			now := time.Now().UTC().Truncate(time.Second)
			old := f.invoice
			old.CreatedAt = now.Add(-30 * time.Minute)
			old.ExpiresAt = now.Add(-15 * time.Minute)
			if e := f.db.Save(&old).Error; e != nil {
				t.Fatal(e)
			}
			f.chain.head = 200
			result, e := f.s.CreatePayment(context.Background(), nil, amountInput(2, "133.20"))
			if e != nil {
				t.Fatal(e)
			}
			if result.Payload["chain_amount"] != "20.0000" {
				t.Fatal("amount not reused")
			}
			current, e := f.s.store.Invoice(context.Background(), 2)
			if e != nil {
				t.Fatal(e)
			}
			if current.AmountKey == old.AmountKey {
				t.Fatal("history key reused")
			}
			cursor, e := f.s.store.Cursor(context.Background(), f.s.stream)
			if e != nil {
				t.Fatal(e)
			}
			transfers := []domain.ChainTransfer{
				{Key: "old-timely-" + network, Stream: f.s.stream, TxHash: "old", Block: 101, Units: old.Units, PaidAt: old.ExpiresAt.Add(-time.Second), State: "manual_review"},
				{Key: "new-timely-" + network, Stream: f.s.stream, TxHash: "new", Block: 201, Units: old.Units, PaidAt: current.CreatedAt.Truncate(time.Second), State: "manual_review"},
			}
			if e = f.s.store.ApplyScan(context.Background(), *cursor, 220, "hash", transfers); e != nil {
				t.Fatal(e)
			}
			for i, id := range []uint{1, 2} {
				invoice, e := f.s.store.Invoice(context.Background(), id)
				if e != nil {
					t.Fatal(e)
				}
				if invoice.State != "confirmed" || invoice.TransferKey == nil || *invoice.TransferKey != transfers[i].Key {
					t.Fatalf("wrong match: %+v", invoice)
				}
			}
			// Duplicate scanner replay cannot create another transfer or change matching.
			if e = f.s.store.ApplyScan(context.Background(), *cursor, 220, "hash", transfers); e != nil {
				t.Fatal(e)
			}
			var count int64
			f.db.Model(&domain.ChainTransfer{}).Count(&count)
			if count != 2 {
				t.Fatal("duplicate transfers")
			}
		})
	}
}

func TestPaymentWindowBoundaryAndExpiredGap(t *testing.T) {
	for _, kind := range []string{"before", "end", "gap", "new", "different_block"} {
		t.Run(kind, func(t *testing.T) {
			f := setup(t)
			boundary := time.Now().UTC().Truncate(time.Second).Add(-time.Minute)
			old := f.invoice
			old.CreatedAt = boundary.Add(-15 * time.Minute)
			old.ExpiresAt = boundary
			if e := f.db.Save(&old).Error; e != nil {
				t.Fatal(e)
			}
			f.chain.head = 200
			if _, e := f.s.CreatePayment(context.Background(), nil, amountInput(2, "133.20")); e != nil {
				t.Fatal(e)
			}
			current, _ := f.s.store.Invoice(context.Background(), 2)
			paid, block, want := boundary.Add(-time.Second), uint64(101), uint(1)
			switch kind {
			case "end":
				paid = boundary
				want = 0
			case "gap":
				paid = boundary.Add(30 * time.Second)
				want = 0
			case "new":
				paid = current.CreatedAt.Truncate(time.Second)
				block = 201
				want = 2
			case "different_block":
				paid = current.CreatedAt.Truncate(time.Second)
				block = 100
				want = 0
			}
			cursor, _ := f.s.store.Cursor(context.Background(), f.s.stream)
			v := domain.ChainTransfer{Key: "boundary-" + kind, Stream: f.s.stream, Block: block, Units: old.Units, PaidAt: paid, State: "manual_review"}
			if e := f.s.store.ApplyScan(context.Background(), *cursor, 220, "hash", []domain.ChainTransfer{v}); e != nil {
				t.Fatal(e)
			}
			var stored domain.ChainTransfer
			if e := f.db.First(&stored, "key = ?", v.Key).Error; e != nil {
				t.Fatal(e)
			}
			if stored.PaymentID != want {
				t.Fatalf("got payment %d want %d", stored.PaymentID, want)
			}
			if want == 0 && stored.State == "ready" {
				t.Fatal("expired transfer accepted")
			}
		})
	}
}

func TestPaidAmountReservedUntilDeadlineAndReusedConcurrently(t *testing.T) {
	f := setup(t)
	if e := f.db.Model(&domain.ChainInvoice{}).Where("payment_id = ?", 1).Update("state", "confirmed").Error; e != nil {
		t.Fatal(e)
	}
	r, e := f.s.CreatePayment(context.Background(), nil, amountInput(2, "133.20"))
	if e != nil || r.Payload["chain_amount"] != "20.0001" {
		t.Fatal("paid amount released too early", e)
	}
	// Simulate the end of the complete old payment windows, preserving history.
	if e = f.db.Model(&domain.ChainInvoice{}).Where("payment_id IN ?", []uint{1, 2}).
		Updates(map[string]any{"created_at": time.Now().Add(-30 * time.Minute), "expires_at": time.Now().Add(-time.Minute)}).Error; e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for id := uint(3); id < 23; id++ {
		wg.Add(1)
		go func(id uint) {
			defer wg.Done()
			_, e := f.s.CreatePayment(context.Background(), nil, amountInput(id, "133.20"))
			errs <- e
		}(id)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	var invoices []domain.ChainInvoice
	if e = f.db.Where("payment_id >= ?", 3).Order("amount").Find(&invoices).Error; e != nil {
		t.Fatal(e)
	}
	if len(invoices) != 20 {
		t.Fatal("missing concurrent invoices")
	}
	for i, v := range invoices {
		if v.Amount != fmt.Sprintf("20.%04d", i) {
			t.Fatal("amount gap/duplicate", v.Amount)
		}
	}
	var historical int64
	f.db.Model(&domain.ChainInvoice{}).Where("payment_id IN ?", []uint{1, 2}).Count(&historical)
	if historical != 2 {
		t.Fatal("history deleted")
	}
}

func TestOverlappingHistoricalWindowsFailClosed(t *testing.T) {
	f := setup(t)
	v := f.invoice
	v.ID = 0
	v.PaymentID = 2
	v.AmountKey += "/legacy-overlap"
	if e := f.db.Create(&v).Error; e != nil {
		t.Fatal(e)
	}
	cursor, _ := f.s.store.Cursor(context.Background(), f.s.stream)
	transfer := domain.ChainTransfer{Key: "overlap", Stream: f.s.stream, Block: 101, Units: v.Units, PaidAt: v.CreatedAt.Truncate(time.Second), State: "manual_review"}
	if e := f.s.store.ApplyScan(context.Background(), *cursor, 110, "hash", []domain.ChainTransfer{transfer}); e == nil {
		t.Fatal("ambiguous window accepted")
	}
	after, _ := f.s.store.Cursor(context.Background(), f.s.stream)
	if after.NextBlock != cursor.NextBlock {
		t.Fatal("cursor advanced on ambiguous match")
	}
}
