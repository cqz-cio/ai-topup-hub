package bscusdtbootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/app/container"
	jobconsumer "github.com/dujiao-next/internal/app/jobs/consumer"
	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"
	mappingstore "github.com/dujiao-next/internal/modules/catalog/mapping/infrastructure/gormstore"
	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	orderstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"
	orderpresenter "github.com/dujiao-next/internal/modules/order/transport/presenter"
	paymentapp "github.com/dujiao-next/internal/modules/payment/application"
	"github.com/dujiao-next/internal/modules/payment/application/bscusdt"
	chaincontract "github.com/dujiao-next/internal/modules/payment/contract/chain"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	chainstore "github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore/chain"
	procurementapp "github.com/dujiao-next/internal/modules/procurement/application"
	procurementdomain "github.com/dujiao-next/internal/modules/procurement/domain"
	procurementstore "github.com/dujiao-next/internal/modules/procurement/infrastructure/gormstore"
	procurementmapping "github.com/dujiao-next/internal/modules/procurement/infrastructure/mappingreader"
	procurementorder "github.com/dujiao-next/internal/modules/procurement/infrastructure/orderreader"
	procurementupstream "github.com/dujiao-next/internal/modules/procurement/infrastructure/upstreamgateway"
	connectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
	"github.com/dujiao-next/internal/queue"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/dujiao-next/internal/upstream"
	"github.com/hibiken/asynq"
	"github.com/shopspring/decimal"
)

// The chain reader, queue transport and supplier wire are the only fakes.
// Payment transitions, durable procurement, the storefront consumer, supplier
// gateway and customer fulfillment all use the production implementations.
type purchaseFlowSupplier struct {
	upstream.Adapter
	mode      string
	calls     int
	purchases int
	request   upstream.CreateUpstreamOrderReq
	queryRef  string
}

func (*purchaseFlowSupplier) RequiresSingleSubmission() bool { return true }
func (s *purchaseFlowSupplier) CreateOrder(_ context.Context, request upstream.CreateUpstreamOrderReq) (*upstream.CreateUpstreamOrderResp, error) {
	s.calls++
	s.request = request
	if s.mode == "unknown_result" {
		s.purchases++
		return nil, errors.New("simulated response lost after supplier charge")
	}
	if s.mode == "insufficient_balance" {
		return &upstream.CreateUpstreamOrderResp{ErrorCode: "insufficient_balance", ErrorMessage: "simulated empty merchant balance"}, nil
	}
	s.purchases++
	result := &upstream.CreateUpstreamOrderResp{OK: true, OrderNo: "001234567890123456", Amount: "33.00", Currency: "CNY"}
	if s.mode != "delayed_card" {
		result.Fulfillment = purchaseFlowCard()
	}
	return result, nil
}
func (s *purchaseFlowSupplier) GetOrderByReference(_ context.Context, reference string) (*upstream.UpstreamOrderDetail, error) {
	s.queryRef = reference
	return &upstream.UpstreamOrderDetail{OrderNo: reference, Status: "delivered", Fulfillment: purchaseFlowCard()}, nil
}
func purchaseFlowCard() *upstream.UpstreamFulfillment {
	return &upstream.UpstreamFulfillment{Type: "auto", Status: "delivered", Payload: "MOCK-PAID-SUPPLIER-CARD"}
}

type purchaseFlowProvider struct{ supplier *purchaseFlowSupplier }

func (*purchaseFlowProvider) GetByID(id uint) (*connectiondomain.Connection, error) {
	return &connectiondomain.Connection{ID: id, Protocol: constants.ConnectionProtocolACGFaka, Status: "active", RetryMax: 2, RetryIntervals: "[30,60]"}, nil
}
func (p *purchaseFlowProvider) GetAdapter(*connectiondomain.Connection) (upstream.Adapter, error) {
	return p.supplier, nil
}

type purchaseFlowQueue struct {
	unavailable bool
	submit      []uint
	poll        []uint
}

func (q *purchaseFlowQueue) EnqueueSubmit(id uint, _ time.Duration) error {
	if q.unavailable {
		return errors.New("simulated Redis outage")
	}
	q.submit = append(q.submit, id)
	return nil
}
func (q *purchaseFlowQueue) EnqueuePoll(id uint, _ time.Duration) error {
	q.poll = append(q.poll, id)
	return nil
}

func TestConfirmedUSDTTransferRunsStorefrontProcurementAndCustomerDelivery(t *testing.T) {
	for _, networkName := range []string{"bsc", "plasma"} {
		for _, scenario := range []string{"immediate_card", "delayed_card", "queue_recovery", "insufficient_balance", "unknown_result", "retry_after_topup"} {
			t.Run(networkName+"/"+scenario, func(t *testing.T) {
				payments, db, registry := setupPaymentService(t)
				sqlDB, _ := db.DB()
				sqlDB.SetMaxOpenConns(1)
				t.Cleanup(func() { sqlDB.Close() })
				if err := db.AutoMigrate(&paymentdomain.ChainInvoice{}, &paymentdomain.ChainCursor{}, &paymentdomain.ChainTransfer{}, &procurementdomain.Order{}, &connectiondomain.Connection{}, &mappingdomain.Mapping{}, &mappingdomain.SKUMapping{}); err != nil {
					t.Fatal(err)
				}
				productMap := mappingdomain.Mapping{ConnectionID: 1, LocalProductID: 1, UpstreamProductID: 4, IsActive: true}
				if err := db.Create(&productMap).Error; err != nil {
					t.Fatal(err)
				}
				if err := db.Create(&mappingdomain.SKUMapping{ProductMappingID: productMap.ID, LocalSKUID: 1, UpstreamSKUID: 4, UpstreamIsActive: true}).Error; err != nil {
					t.Fatal(err)
				}
				orders := orderstore.New(db, "test-guest-key")
				supplier := &purchaseFlowSupplier{mode: scenario}
				if scenario == "retry_after_topup" {
					supplier.mode = "insufficient_balance"
				}
				queued := &purchaseFlowQueue{unavailable: scenario == "queue_recovery"}
				procurements := procurementapp.NewService(procurementapp.Options{
					Repository: procurementstore.New(db), Orders: procurementorder.New(orders),
					ProductMappings: procurementmapping.NewProducts(mappingstore.NewMappingStore(db)),
					SKUMappings:     procurementmapping.NewSKUs(mappingstore.NewSKUMappingStore(db)),
					Connections:     procurementupstream.New(&purchaseFlowProvider{supplier: supplier}),
					Queue:           queued,
					OrderLifecycle:  procurementstore.NewLifecycle(db, nil, nil, config.EmailConfig{}),
				})
				payments.SetProcurementService(procurements)
				network, err := chaincontract.ResolveNetwork(networkName)
				if err != nil {
					t.Fatal(err)
				}
				chain := &bscTestChain{head: 100, timestamp: time.Now().UTC().Truncate(time.Second)}
				settlements := 0
				gateway, err := bscusdt.New(chainstore.New(db), chain, bscusdt.Config{Network: networkName, Recipient: "0x66a8bab15067eab1efc948ac03684f0397c8416f", CNYPerUSDT: "6.66", Confirmations: 15}, func(_ context.Context, invoice paymentdomain.ChainInvoice, transfer paymentdomain.ChainTransfer) error {
					settlements++
					_, err := payments.HandleCallback(paymentapp.PaymentCallbackInput{PaymentID: invoice.PaymentID, Status: constants.PaymentStatusSuccess, Amount: money.FromDecimal(decimal.RequireFromString(invoice.FiatAmount)), Currency: invoice.Currency, PaidAt: &transfer.PaidAt, ProviderRef: transfer.TxHash})
					return err
				})
				if err != nil {
					t.Fatal(err)
				}
				registry.Register(network.Provider, "", gateway)
				channel := paymentdomain.PaymentChannel{Name: network.ChannelName, ProviderType: network.Provider, ChannelType: network.ChannelType, InteractionMode: constants.PaymentInteractionQR, IsActive: true, ConfigJSON: jsonmap.JSON{}}
				if err = db.Create(&channel).Error; err != nil {
					t.Fatal(err)
				}
				expires := time.Now().Add(15 * time.Minute)
				amount := money.FromDecimal(decimal.RequireFromString("133.20"))
				order := orderdomain.Order{OrderNo: "MOCK-USDT-PURCHASE", Status: constants.OrderStatusPendingPayment, Currency: "CNY", OriginalAmount: amount, TotalAmount: amount, ExpiresAt: &expires, Items: []orderdomain.OrderItem{{ProductID: 1, SKUID: 1, Quantity: 1, FulfillmentType: constants.FulfillmentTypeUpstream, TitleJSON: jsonmap.JSON{"zh-CN": "Mock supplier item"}, CostPrice: money.FromDecimal(decimal.RequireFromString("33.00"))}}}
				if err = db.Create(&order).Error; err != nil {
					t.Fatal(err)
				}
				payment, err := payments.CreatePayment(paymentapp.CreatePaymentInput{OrderID: order.ID, ChannelID: channel.ID, Context: context.Background()})
				if err != nil {
					t.Fatal(err)
				}
				var invoice paymentdomain.ChainInvoice
				if err = db.First(&invoice, "payment_id = ?", payment.Payment.ID).Error; err != nil {
					t.Fatal(err)
				}
				chain.event = &chaincontract.Log{Address: network.Token, Topics: []string{bscusdt.TransferTopic, "0x" + strings.Repeat("0", 24) + strings.Repeat("1", 40), "0x" + strings.Repeat("0", 24) + invoice.Recipient[2:]}, Data: fmt.Sprintf("0x%064x", decimal.RequireFromString(invoice.Units).BigInt()), BlockNumber: "0x65", BlockHash: fmt.Sprintf("0x%064x", 101), TransactionHash: "0x" + strings.Repeat("a", 64), LogIndex: "0x0"}
				chain.head = 110
				if err = gateway.RunOnce(context.Background()); err != nil {
					t.Fatal(err)
				}
				var count int64
				if err = db.Model(&procurementdomain.Order{}).Count(&count).Error; err != nil || count != 0 || supplier.calls != 0 {
					t.Fatal("unconfirmed transfer created procurement", err, count)
				}
				chain.head = 120
				for range 2 {
					if err = gateway.RunOnce(context.Background()); err != nil {
						t.Fatal(err)
					}
				}
				var procurement procurementdomain.Order
				if err = db.First(&procurement, "local_order_id = ?", order.ID).Error; err != nil || settlements != 1 || supplier.calls != 0 {
					t.Fatal("confirmed payment did not persist one pending procurement", err, settlements)
				}
				if scenario == "queue_recovery" {
					if len(queued.submit) != 0 {
						t.Fatal("unavailable queue accepted work")
					}
					queued.unavailable = false
					if err = db.Model(&procurement).Update("updated_at", time.Now().Add(-3*time.Minute)).Error; err != nil {
						t.Fatal(err)
					}
					if err = procurements.RecoverPaidOrders(); err != nil {
						t.Fatal(err)
					}
				}
				if len(queued.submit) != 1 || queued.submit[0] != procurement.ID {
					t.Fatal("paid procurement was not enqueued exactly once", queued.submit)
				}
				mux := asynq.NewServeMux()
				jobconsumer.New(&container.Container{ProcurementOrderService: procurements}).RegisterStorefront(mux)
				work := func(taskType string) {
					t.Helper()
					payload, _ := json.Marshal(queue.ProcurementSubmitPayload{ProcurementOrderID: procurement.ID})
					task := asynq.NewTask(taskType, payload)
					handler, pattern := mux.Handler(task)
					if pattern != taskType {
						t.Fatal("storefront does not handle procurement task")
					}
					if err := handler.ProcessTask(context.Background(), task); err != nil {
						t.Fatal(err)
					}
				}
				work(queue.TaskProcurementSubmit)
				expectedCalls := 1
				if scenario == "retry_after_topup" {
					if err = db.First(&procurement, procurement.ID).Error; err != nil || procurement.Status != "rejected" || supplier.purchases != 0 {
						t.Fatal("empty balance must reject without a supplier purchase", err, procurement.Status, supplier.purchases)
					}
					unfulfilled, err := orders.GetByID(order.ID)
					if err != nil || unfulfilled.Status != constants.OrderStatusPaid || unfulfilled.Fulfillment != nil {
						t.Fatal("rejected procurement changed the paid order or delivered a card", err)
					}
					supplier.mode = "immediate_card" // The merchant has now topped up.
					work(queue.TaskProcurementSubmit)
					if supplier.calls != 1 {
						t.Fatal("a rejected purchase was automatically retried")
					}
					if err = procurements.RetryManual(procurement.ID); err != nil {
						t.Fatal(err)
					}
					if len(queued.submit) != 2 || queued.submit[1] != procurement.ID {
						t.Fatal("manual retry did not enqueue the original procurement")
					}
					work(queue.TaskProcurementSubmit)
					expectedCalls = 2
				}
				if scenario == "delayed_card" {
					if len(queued.poll) != 1 || queued.poll[0] != procurement.ID {
						t.Fatal("accepted supplier order did not schedule card retrieval")
					}
					work(queue.TaskProcurementPollStatus)
					if supplier.queryRef != "001234567890123456" {
						t.Fatal("supplier order reference lost leading zeroes")
					}
				}
				work(queue.TaskProcurementSubmit) // Redelivery must not purchase again.
				if supplier.calls != expectedCalls || supplier.request.MaxAmount != "33.00" || supplier.request.Currency != "CNY" || supplier.request.SKUID != 4 || supplier.request.Quantity != 1 {
					t.Fatal("supplier request duplicated or lost purchase constraints", supplier.calls)
				}
				if err = db.First(&procurement, procurement.ID).Error; err != nil {
					t.Fatal(err)
				}
				got, err := orders.GetByID(order.ID)
				if err != nil {
					t.Fatal(err)
				}
				if err = db.Model(&fulfillmentdomain.Fulfillment{}).Where("order_id = ?", order.ID).Count(&count).Error; err != nil {
					t.Fatal(err)
				}
				if scenario == "insufficient_balance" || scenario == "unknown_result" {
					want := "rejected"
					if scenario == "unknown_result" {
						want = "manual_review"
					}
					if procurement.Status != want || got.Status != constants.OrderStatusPaid || count != 0 {
						t.Fatal("failed or unknown purchase falsely delivered a card", procurement.Status, got.Status, count)
					}
					return
				}
				var delivery fulfillmentdomain.Fulfillment
				if err = db.First(&delivery, "order_id = ?", order.ID).Error; err != nil || count != 1 || delivery.Payload != purchaseFlowCard().Payload || procurement.Status != "fulfilled" || got.Status != constants.OrderStatusDelivered {
					t.Fatal("confirmed payment did not deliver exactly one supplier card", err, procurement.Status, got.Status, count)
				}
				customerOrder := orderpresenter.NewOrderDetail(got)
				if customerOrder.Fulfillment == nil || customerOrder.Fulfillment.Payload != purchaseFlowCard().Payload || supplier.purchases != 1 {
					t.Fatal("delivered supplier card is missing from the customer order response")
				}
			})
		}
	}
}
