package bscusdtbootstrap

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	rechargebootstrap "github.com/dujiao-next/internal/bootstrap/autorecharge"
	"github.com/dujiao-next/internal/constants"
	rechargeapp "github.com/dujiao-next/internal/modules/autorecharge/application"
	rechargecontract "github.com/dujiao-next/internal/modules/autorecharge/contract"
	rechargedomain "github.com/dujiao-next/internal/modules/autorecharge/domain"
	"github.com/dujiao-next/internal/modules/autorecharge/infrastructure/aicdk"
	rechargestore "github.com/dujiao-next/internal/modules/autorecharge/infrastructure/gormstore"
	fulfillmentapp "github.com/dujiao-next/internal/modules/fulfillment/application"
	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	fulfillmentstore "github.com/dujiao-next/internal/modules/fulfillment/infrastructure/gormstore"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	orderstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"
	paymentapp "github.com/dujiao-next/internal/modules/payment/application"
	"github.com/dujiao-next/internal/modules/payment/application/bscusdt"
	chaincontract "github.com/dujiao-next/internal/modules/payment/contract/chain"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	chainstore "github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore/chain"
	"github.com/dujiao-next/internal/modules/payment/transport/presenter"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

type bscTestChain struct {
	head      uint64
	timestamp time.Time
	event     *chaincontract.Log
}

func (c *bscTestChain) Check(context.Context) error { return nil }
func (c *bscTestChain) Header(_ context.Context, tag string) (*chaincontract.Header, error) {
	n := c.head
	if tag != "latest" && tag != "finalized" {
		fmt.Sscanf(tag, "0x%x", &n)
	}
	return &chaincontract.Header{Number: fmt.Sprintf("0x%x", n), Hash: fmt.Sprintf("0x%064x", n), Timestamp: fmt.Sprintf("0x%x", c.timestamp.Unix())}, nil
}
func (c *bscTestChain) Logs(_ context.Context, from, to uint64, _ string) ([]chaincontract.Log, error) {
	if c.event != nil && from <= 101 && to >= 101 {
		return []chaincontract.Log{*c.event}, nil
	}
	return []chaincontract.Log{}, nil
}
func (c *bscTestChain) Receipt(context.Context, string) (*chaincontract.Receipt, error) {
	return &chaincontract.Receipt{Status: "0x1", TransactionHash: c.event.TransactionHash, BlockNumber: c.event.BlockNumber, BlockHash: c.event.BlockHash, Logs: []chaincontract.Log{*c.event}}, nil
}

func TestBSCConfirmedTransferPaysGuestOrderAndDeliversOneBoundCode(t *testing.T) {
	svc, db, registry := setupPaymentService(t)
	sql, _ := db.DB()
	sql.SetMaxOpenConns(1)
	t.Cleanup(func() { sql.Close() })
	if err := db.AutoMigrate(&paymentdomain.ChainInvoice{}, &paymentdomain.ChainCursor{}, &paymentdomain.ChainTransfer{}, &rechargedomain.Task{}); err != nil {
		t.Fatal(err)
	}
	store := orderstore.New(db, "test-secret-for-bsc-code-fulfillment")
	recharge, err := rechargeapp.New(rechargeapp.Options{Store: rechargestore.New(db), Orders: &rechargebootstrap.Orders{DB: db, Store: store, Fulfillment: fulfillmentapp.New(fulfillmentapp.Options{OrderStore: store, FulfillmentStore: fulfillmentstore.New(db)}), RechargeSKUs: map[uint]bool{9: true}, RedeemURL: "https://shop.example.test/redeem"}, Partner: aicdk.New(""), Secret: strings.Repeat("k", 32), Bindings: []rechargecontract.Binding{{SKUID: 9, Plan: "chatgptprolite", Region: "US", Channel: "3", CardRule: "pro5-cards"}}})
	if err != nil {
		t.Fatal(err)
	}
	svc.SetAutoRechargeService(recharge)
	chain := &bscTestChain{head: 100, timestamp: time.Now().UTC().Truncate(time.Second)}
	settlements := 0
	gateway, err := bscusdt.New(chainstore.New(db), chain, bscusdt.Config{Recipient: "0x66a8bab15067eab1efc948ac03684f0397c8416f", CNYPerUSDT: "6.66"}, func(_ context.Context, i paymentdomain.ChainInvoice, transfer paymentdomain.ChainTransfer) error {
		settlements++
		_, e := svc.HandleCallback(paymentapp.PaymentCallbackInput{PaymentID: i.PaymentID, Status: constants.PaymentStatusSuccess, Amount: money.FromDecimal(decimal.RequireFromString(i.FiatAmount)), Currency: i.Currency, PaidAt: &transfer.PaidAt, ProviderRef: transfer.TxHash})
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	registry.Register(bscusdt.Provider, "", gateway)
	channel := paymentdomain.PaymentChannel{Name: "BSC USDT", ProviderType: bscusdt.Provider, ChannelType: "usdt-bep20", InteractionMode: constants.PaymentInteractionQR, IsActive: true, ConfigJSON: jsonmap.JSON{}}
	if err = db.Create(&channel).Error; err != nil {
		t.Fatal(err)
	}
	expiry := time.Now().Add(15 * time.Minute)
	order := orderdomain.Order{OrderNo: "BSC-GUEST-ORDER", Status: constants.OrderStatusPendingPayment, Currency: "CNY", OriginalAmount: money.FromDecimal(decimal.RequireFromString("133.20")), TotalAmount: money.FromDecimal(decimal.RequireFromString("133.20")), ExpiresAt: &expiry, Items: []orderdomain.OrderItem{{SKUID: 9, Quantity: 1, FulfillmentType: constants.FulfillmentTypeManual, TitleJSON: jsonmap.JSON{"zh-CN": "Pro 5x"}}}}
	if err = db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	result, err := svc.CreatePayment(paymentapp.CreatePaymentInput{OrderID: order.ID, ChannelID: channel.ID, Context: context.Background()})
	if err != nil {
		t.Fatal(err)
	}
	response := presenter.NewCreatePaymentResp(&presenter.CreatePaymentResultView{Payment: result.Payment, Channel: result.Channel})
	if response.Chain != "bsc" || response.WalletAddress == "" || response.ChainAmount == "" || response.ExpiresAt == nil || response.Currency != "CNY" {
		t.Fatalf("incomplete cashier response: %+v", response)
	}
	var invoice paymentdomain.ChainInvoice
	if err = db.First(&invoice, "payment_id = ?", result.Payment.ID).Error; err != nil {
		t.Fatal(err)
	}
	var tasks int64
	db.Model(&rechargedomain.Task{}).Count(&tasks)
	if tasks != 0 {
		t.Fatal("unpaid order issued a code")
	}
	chain.event = &chaincontract.Log{Address: bscusdt.Token, Topics: []string{bscusdt.TransferTopic, "0x" + strings.Repeat("0", 24) + strings.Repeat("1", 40), "0x" + strings.Repeat("0", 24) + invoice.Recipient[2:]}, Data: fmt.Sprintf("0x%064x", decimal.RequireFromString(invoice.Units).BigInt()), BlockNumber: "0x65", BlockHash: fmt.Sprintf("0x%064x", 101), TransactionHash: "0x" + strings.Repeat("a", 64), LogIndex: "0x0"}
	chain.head = 120
	for i := 0; i < 2; i++ {
		if err = gateway.RunOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
		if err = recharge.Advance(context.Background(), order.ID); err != nil {
			t.Fatal(err)
		}
	}
	var paid paymentdomain.Payment
	db.First(&paid, result.Payment.ID)
	var task rechargedomain.Task
	db.First(&task, "order_id = ?", order.ID)
	got, err := store.GetByID(order.ID)
	if err != nil {
		t.Fatal(err)
	}
	var deliveries int64
	db.Model(&fulfillmentdomain.Fulfillment{}).Where("order_id = ?", order.ID).Count(&deliveries)
	if settlements != 1 || paid.Status != constants.PaymentStatusSuccess || got.Status != constants.OrderStatusDelivered || deliveries != 1 || task.State != "issued" || task.Plan != "chatgptprolite" || task.ConfirmedAt != nil {
		t.Fatalf("incorrect paid-to-code flow: settlements=%d payment=%s order=%s deliveries=%d task=%s plan=%s", settlements, paid.Status, got.Status, deliveries, task.State, task.Plan)
	}
}
