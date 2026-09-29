package bscusdt

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	"github.com/dujiao-next/internal/modules/payment/contract"
	chaincontract "github.com/dujiao-next/internal/modules/payment/contract/chain"
	"github.com/dujiao-next/internal/modules/payment/domain"
	chainstore "github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore/chain"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const testRecipient = "0x66a8bab15067eab1efc948ac03684f0397c8416f"

type Header = chaincontract.Header
type Log = chaincontract.Log
type Receipt = chaincontract.Receipt

type testChain struct {
	head, final uint64
	timestamp   time.Time
	logs        []Log
	err         error
	badReceipt  bool
	reorg       uint64
}

func (c *testChain) Check(context.Context) error { return c.err }
func (c *testChain) Header(_ context.Context, tag string) (*Header, error) {
	if c.err != nil {
		return nil, c.err
	}
	var n uint64
	switch tag {
	case "latest":
		n = c.head
	case "finalized":
		n = c.final
	default:
		var e error
		n, e = quantity(tag)
		if e != nil {
			return nil, e
		}
	}
	hash := fmt.Sprintf("0x%064x", n)
	if n == c.reorg {
		hash = "0x" + strings.Repeat("f", 64)
	}
	return &Header{Number: hexBlock(n), Hash: hash, Timestamp: hexBlock(uint64(c.timestamp.Unix()))}, nil
}
func (c *testChain) Logs(_ context.Context, from, to uint64, _ string) ([]Log, error) {
	if c.err != nil {
		return nil, c.err
	}
	out := []Log{}
	for _, v := range c.logs {
		n, _ := quantity(v.BlockNumber)
		if n >= from && n <= to {
			out = append(out, v)
		}
	}
	return out, nil
}
func (c *testChain) Receipt(_ context.Context, hash string) (*Receipt, error) {
	for _, v := range c.logs {
		if v.TransactionHash == hash {
			status := "0x1"
			if c.badReceipt {
				status = "0x0"
			}
			return &Receipt{Status: status, TransactionHash: hash, BlockHash: v.BlockHash, BlockNumber: v.BlockNumber, Logs: []Log{v}}, nil
		}
	}
	return nil, ErrRPC
}

type testFixture struct {
	s         *Service
	db        *gorm.DB
	chain     *testChain
	calls     int
	settleErr error
	invoice   domain.ChainInvoice
}

func setup(t *testing.T) *testFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "chain.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sql, _ := db.DB()
	sql.SetMaxOpenConns(1)
	t.Cleanup(func() { sql.Close() })
	if err = db.AutoMigrate(&domain.ChainInvoice{}, &domain.ChainCursor{}, &domain.ChainTransfer{}, &orderdomain.Order{}); err != nil {
		t.Fatal(err)
	}
	f := &testFixture{db: db, chain: &testChain{head: 100, final: 100, timestamp: time.Now().UTC().Truncate(time.Second)}}
	f.s, err = New(chainstore.New(db), f.chain, Config{Recipient: testRecipient, CNYPerUSDT: "6.66", Confirmations: 15}, func(_ context.Context, i domain.ChainInvoice, v domain.ChainTransfer) error {
		f.calls++
		return f.settleErr
	})
	if err != nil {
		t.Fatal(err)
	}
	expires := time.Now().Add(15 * time.Minute)
	if err = db.Create(&orderdomain.Order{ID: 1, OrderNo: "ORDER-1", Status: constants.OrderStatusPendingPayment, ExpiresAt: &expires}).Error; err != nil {
		t.Fatal(err)
	}
	result, err := f.s.CreatePayment(context.Background(), nil, contract.GatewayCreateInput{PaymentID: 1, OrderID: 1, Amount: money.FromDecimal(decimal.RequireFromString("133.20")), Currency: "CNY"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExpiresAt == nil || result.QRCodeURL != testRecipient {
		t.Fatal("missing payment instructions")
	}
	if err = db.First(&f.invoice, "payment_id = ?", 1).Error; err != nil {
		t.Fatal(err)
	}
	return f
}
func (f *testFixture) event(units string, block uint64, index uint64) Log {
	amount := decimal.RequireFromString(units).BigInt()
	return Log{Address: Token, Topics: []string{TransferTopic, addressTopic("0x" + strings.Repeat("1", 40)), addressTopic(testRecipient)}, Data: fmt.Sprintf("0x%064x", amount), BlockNumber: hexBlock(block), BlockHash: fmt.Sprintf("0x%064x", block), TransactionHash: fmt.Sprintf("0x%064x", index+1000), LogIndex: hexBlock(index)}
}
func (f *testFixture) run(t *testing.T) {
	t.Helper()
	if err := f.s.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestInvoiceLocksRateAndSixDecimalAmount(t *testing.T) {
	f := setup(t)
	amount := decimal.RequireFromString(f.invoice.Amount)
	if !amount.GreaterThan(decimal.NewFromInt(20)) || !amount.LessThan(decimal.RequireFromString("20.01")) || f.invoice.CNYPerUSDT != "6.66" || f.invoice.Units != amount.Shift(18).BigInt().String() {
		t.Fatal("incorrect conversion or precision")
	}
	f.s.rate = decimal.NewFromInt(9)
	result, err := f.s.CreatePayment(context.Background(), nil, contract.GatewayCreateInput{PaymentID: 1, OrderID: 1, Amount: money.FromDecimal(decimal.RequireFromString("133.20")), Currency: "CNY"})
	if err != nil || result.Payload["chain_amount"] != f.invoice.Amount || result.Payload["cny_per_usdt"] != "6.66" {
		t.Fatal("retry changed invoice", err)
	}
}
func TestOnlyFinalizedExactTransferSettlesOnce(t *testing.T) {
	f := setup(t)
	f.chain.logs = []Log{f.event(f.invoice.Units, 101, 0)}
	f.chain.head = 114
	f.chain.final = 114
	f.run(t)
	if f.calls != 0 {
		t.Fatal("not enough confirmations")
	}
	f.chain.head = 120
	f.chain.final = 100
	f.run(t)
	if f.calls != 0 {
		t.Fatal("unfinalized event settled")
	}
	f.chain.final = 120
	f.run(t)
	f.run(t)
	if f.calls != 1 {
		t.Fatal("duplicate or missing settlement", f.calls)
	}
	var invoice domain.ChainInvoice
	f.db.First(&invoice, 1)
	if invoice.State != "confirmed" || invoice.TransferKey == nil {
		t.Fatal("missing confirmed evidence")
	}
}
func TestWrongAmountLateAndDuplicateTransfersAreRecordedForReview(t *testing.T) {
	for _, kind := range []string{"short", "over", "late", "before", "duplicate"} {
		t.Run(kind, func(t *testing.T) {
			f := setup(t)
			units := f.invoice.Units
			switch kind {
			case "short":
				units = decimal.RequireFromString(units).Sub(decimal.NewFromInt(1)).String()
			case "over":
				units = decimal.RequireFromString(units).Add(decimal.NewFromInt(1)).String()
			case "late":
				f.db.Model(&domain.ChainInvoice{}).Where("id = ?", f.invoice.ID).Update("expires_at", time.Now().Add(-time.Hour))
			case "before":
				f.db.Model(&domain.ChainInvoice{}).Where("id = ?", f.invoice.ID).Update("start_block", uint64(102))
			}
			f.chain.logs = []Log{f.event(units, 101, 0)}
			want := 0
			if kind == "duplicate" {
				f.chain.logs = append(f.chain.logs, f.event(units, 102, 1))
				want = 1
			}
			f.chain.head = 120
			f.chain.final = 120
			f.run(t)
			if f.calls != want {
				t.Fatal("incorrect automatic settlement")
			}
			var count int64
			f.db.Model(&domain.ChainTransfer{}).Where("state = ?", "manual_review").Count(&count)
			if count != 1 {
				t.Fatal("missing review record")
			}
		})
	}
}
func TestMalformedEventsNeverAdvanceCursor(t *testing.T) {
	for _, kind := range []string{"token", "recipient", "topic", "removed", "receipt", "reorg"} {
		t.Run(kind, func(t *testing.T) {
			f := setup(t)
			event := f.event(f.invoice.Units, 101, 0)
			switch kind {
			case "token":
				event.Address = testRecipient
			case "recipient":
				event.Topics[2] = addressTopic(Token)
			case "topic":
				event.Topics[0] = "0x" + strings.Repeat("0", 64)
			case "removed":
				event.Removed = true
			case "receipt":
				f.chain.badReceipt = true
			case "reorg":
				f.chain.reorg = 101
			}
			f.chain.logs = []Log{event}
			f.chain.head = 120
			f.chain.final = 120
			if err := f.s.RunOnce(context.Background()); err == nil {
				t.Fatal("invalid RPC event accepted")
			}
			var cursor domain.ChainCursor
			f.db.First(&cursor)
			if cursor.NextBlock != 101 || f.calls != 0 {
				t.Fatal("invalid page committed")
			}
		})
	}
}
func TestRPCRecoveryAndSettlementOutboxSurviveRestart(t *testing.T) {
	f := setup(t)
	f.chain.logs = []Log{f.event(f.invoice.Units, 101, 0)}
	f.chain.head = 120
	f.chain.final = 120
	f.chain.err = ErrRPC
	if err := f.s.RunOnce(context.Background()); err == nil {
		t.Fatal("RPC failure ignored")
	}
	f.chain.err = nil
	f.settleErr = errors.New("database temporarily down")
	if err := f.s.RunOnce(context.Background()); err == nil {
		t.Fatal("settlement failure ignored")
	}
	var pending domain.ChainTransfer
	f.db.First(&pending)
	if pending.State != "ready" {
		t.Fatal("outbox not persisted")
	}
	f.settleErr = nil
	f.db.Model(&domain.ChainTransfer{}).Where("key = ?", pending.Key).Update("next_attempt_at", nil)
	restarted, err := New(chainstore.New(f.db), f.chain, f.s.config, f.s.settle)
	if err != nil {
		t.Fatal(err)
	}
	f.s = restarted
	f.run(t)
	f.run(t)
	if f.calls != 2 {
		t.Fatal("outbox lost or replayed after delivery", f.calls)
	}
}
func TestConcurrentInvoiceCreationKeepsSameAmount(t *testing.T) {
	f := setup(t)
	errs := make(chan error, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := f.s.CreatePayment(context.Background(), nil, contract.GatewayCreateInput{PaymentID: 2, OrderID: 1, Amount: money.FromDecimal(decimal.RequireFromString("133.20")), Currency: "CNY"})
			if e == nil && r == nil {
				e = errors.New("empty result")
			}
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	var invoices []domain.ChainInvoice
	f.db.Order("id").Find(&invoices)
	if len(invoices) != 2 || invoices[0].Units == invoices[1].Units {
		t.Fatal("same amount was assigned to two payments")
	}
}
func TestFinalizedHistoryChangeStopsListener(t *testing.T) {
	f := setup(t)
	f.chain.head = 120
	f.chain.final = 120
	f.run(t)
	f.chain.reorg = 106
	f.chain.head = 130
	f.chain.final = 130
	if err := f.s.RunOnce(context.Background()); !errors.Is(err, ErrReorg) {
		t.Fatal("history rewrite ignored", err)
	}
}
