package procurement_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"

	"github.com/dujiao-next/internal/constants"
	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	ordergormstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"
	paymentapp "github.com/dujiao-next/internal/modules/payment/application"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	paymentgormstore "github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore"
	procurementapp "github.com/dujiao-next/internal/modules/procurement/application"
	procurementdomain "github.com/dujiao-next/internal/modules/procurement/domain"
	procurementupstream "github.com/dujiao-next/internal/modules/procurement/infrastructure/upstreamgateway"
	"github.com/dujiao-next/internal/upstream"
)

type workflowQueue struct {
	submits []uint
	polls   []uint
	delays  []time.Duration
	err     error
}

func (q *workflowQueue) EnqueueSubmit(id uint, delay time.Duration) error {
	if q.err != nil {
		return q.err
	}
	q.submits = append(q.submits, id)
	q.delays = append(q.delays, delay)
	return nil
}

func (q *workflowQueue) EnqueuePoll(id uint, _ time.Duration) error {
	q.polls = append(q.polls, id)
	return q.err
}

// All supplier calls use this in-process fixture; no payment or purchase
// requests can reach the production site.
func TestConfirmedBEP20PaymentAutomaticallyPurchasesAndDeliversSplitOrder(t *testing.T) {
	adapter := &acgProcurementAdapter{result: &upstream.CreateUpstreamOrderResp{OK: true, OrderNo: "0001039FIXTURE", Amount: "33.00", Currency: "CNY", Fulfillment: &upstream.UpstreamFulfillment{Type: "auto", Status: "delivered", Payload: "SIMULATED-CARD"}}}
	db, _, placeholder, child := acgProcurementFixture(t, constants.OrderStatusPendingPayment, adapter)
	if err := db.Delete(placeholder).Error; err != nil {
		t.Fatal(err)
	}
	parent := &orderdomain.Order{OrderNo: "BEP20-PARENT-FIXTURE", Status: constants.OrderStatusPendingPayment, Currency: "CNY", TotalAmount: child.TotalAmount, OriginalAmount: child.OriginalAmount}
	if err := db.Create(parent).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(child).Update("parent_id", parent.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&paymentdomain.Payment{}, &paymentdomain.PaymentChannel{}); err != nil {
		t.Fatal(err)
	}
	payment := &paymentdomain.Payment{OrderID: parent.ID, ChannelID: 1, ProviderType: "bscusdt", Status: constants.PaymentStatusPending, Amount: parent.TotalAmount, Currency: "CNY"}
	if err := db.Create(payment).Error; err != nil {
		t.Fatal(err)
	}
	q := &workflowQueue{}
	options := procurementOptionsForTest(db)
	options.Queue = q
	options.Connections = procurementupstream.New(&acgProcurementProvider{adapter: adapter})
	procurement := procurementapp.NewService(options)
	payments := paymentapp.NewPaymentService(paymentapp.PaymentServiceOptions{OrderStore: ordergormstore.New(db, "test-guest-credential-secret-with-32-bytes"), PaymentStore: paymentgormstore.New(db, "test-guest-credential-secret-with-32-bytes"), ChannelStore: paymentgormstore.NewChannelStore(db)})
	payments.SetProcurementService(procurement)
	input := paymentapp.PaymentCallbackInput{PaymentID: payment.ID, OrderNo: parent.OrderNo, ChannelID: 1, Status: constants.PaymentStatusSuccess, Amount: parent.TotalAmount, Currency: "CNY", ProviderRef: "fixture-confirmed-chain-transfer"}
	for i := 0; i < 2; i++ {
		if _, err := payments.HandleCallback(input); err != nil {
			t.Fatal(err)
		}
	}
	if len(q.submits) != 1 {
		t.Fatalf("payment did not enqueue one purchase: %v", q.submits)
	}
	for i := 0; i < 2; i++ {
		if err := procurement.SubmitToUpstream(q.submits[0]); err != nil {
			t.Fatal(err)
		}
	}
	var actual procurementdomain.Order
	if err := db.First(&actual, q.submits[0]).Error; err != nil {
		t.Fatal(err)
	}
	var fulfilled fulfillmentdomain.Fulfillment
	if err := db.Where("order_id = ?", child.ID).First(&fulfilled).Error; err != nil {
		t.Fatal(err)
	}
	for _, id := range []uint{parent.ID, child.ID} {
		var order orderdomain.Order
		if err := db.First(&order, id).Error; err != nil {
			t.Fatal(err)
		}
		if order.Status != constants.OrderStatusDelivered {
			t.Fatalf("order %d not delivered: %s", id, order.Status)
		}
	}
	if actual.Status != "fulfilled" || fulfilled.Payload != "SIMULATED-CARD" || adapter.calls.Load() != 1 {
		t.Fatal("automatic card delivery or purchase idempotency failed")
	}
}

func TestProcurementRecoveryRepairsQueueOutageWithoutUnpaidPurchases(t *testing.T) {
	adapter := &acgProcurementAdapter{}
	db, _, proc, paid := acgProcurementFixture(t, constants.OrderStatusPaid, adapter)
	if err := db.Delete(proc).Error; err != nil {
		t.Fatal(err)
	}
	unpaid := createProcTestOrder(t, db, "UNPAID-RECOVERY", constants.OrderStatusPendingPayment, constants.FulfillmentTypeUpstream)
	q := &workflowQueue{err: errors.New("queue unavailable")}
	options := procurementOptionsForTest(db)
	options.Queue = q
	options.Connections = procurementupstream.New(&acgProcurementProvider{adapter: adapter})
	svc := procurementapp.NewService(options)
	if err := svc.CreateForOrder(unpaid.ID); err == nil {
		t.Fatal("unpaid order created procurement")
	}
	if err := svc.CreateForOrder(paid.ID); err == nil {
		t.Fatal("enqueue failure was hidden")
	}
	var pending procurementdomain.Order
	if err := db.Where("local_order_id = ?", paid.ID).First(&pending).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&pending).Update("updated_at", time.Now().Add(-3*time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	q.err = nil
	if err := svc.RecoverPaidOrders(); err != nil {
		t.Fatal(err)
	}
	if len(q.submits) != 1 || q.submits[0] != pending.ID || adapter.calls.Load() != 0 {
		t.Fatal("lost pending task was not safely recovered")
	}
	if err := db.Model(&pending).Updates(map[string]any{"status": "submitting", "updated_at": time.Now().Add(-3 * time.Minute)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.RecoverPaidOrders(); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&pending, pending.ID).Error; err != nil {
		t.Fatal(err)
	}
	if pending.Status != "manual_review" || len(q.submits) != 1 || adapter.calls.Load() != 0 {
		t.Fatal("uncertain submission was re-purchased")
	}
}

func TestProcurementConcurrentCallbacksCreateOneIntent(t *testing.T) {
	db := setupProcurementTestDB(t)
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	order := createProcTestOrder(t, db, "CONCURRENT-INTENT", constants.OrderStatusPaid, constants.FulfillmentTypeUpstream)
	if err := db.Create(&mappingdomain.Mapping{ConnectionID: 1, LocalProductID: 1, UpstreamProductID: 4, IsActive: true}).Error; err != nil {
		t.Fatal(err)
	}
	svc := newTestProcurementService(db, newTestSiteConnectionService(db, "fixture-key", t.TempDir()))
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() { defer group.Done(); _ = svc.CreateForOrder(order.ID) }()
	}
	group.Wait()
	var count int64
	if err := db.Model(&procurementdomain.Order{}).Where("local_order_id = ?", order.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("parallel callbacks created %d purchases", count)
	}
}

func TestRecoveryCreatesMissingPaidIntentAndRecoversPersistedCard(t *testing.T) {
	adapter := &acgProcurementAdapter{}
	db, _, proc, order := acgProcurementFixture(t, constants.OrderStatusPaid, adapter)
	if err := db.Delete(proc).Error; err != nil {
		t.Fatal(err)
	}
	q := &workflowQueue{}
	options := procurementOptionsForTest(db)
	options.Queue = q
	options.Connections = procurementupstream.New(&acgProcurementProvider{adapter: adapter})
	svc := procurementapp.NewService(options)
	if err := svc.RecoverPaidOrders(); err != nil {
		t.Fatal(err)
	}
	if len(q.submits) != 1 {
		t.Fatal("missing paid intent not recovered")
	}
	id := q.submits[0]
	if err := db.Model(&procurementdomain.Order{}).Where("id = ?", id).Updates(map[string]any{"status": "accepted", "upstream_order_no": "FIXTURE-SUPPLIER-REF", "upstream_payload": "RECOVERED-FIXTURE-CARD", "updated_at": time.Now().Add(-3 * time.Minute)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.RecoverPaidOrders(); err != nil {
		t.Fatal(err)
	}
	if len(q.polls) != 1 || q.polls[0] != id {
		t.Fatal("accepted purchase not queued for recovery")
	}
	if err := svc.PollUpstreamStatus(id); err != nil {
		t.Fatal(err)
	}
	if err := db.First(order, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if order.Status != constants.OrderStatusDelivered || adapter.calls.Load() != 0 {
		t.Fatal("persisted card recovery sent a new purchase")
	}
}

func TestParentRecoverySkipsExistingChildAndPurchasesRemainingChild(t *testing.T) {
	db := setupProcurementTestDB(t)
	parent := createProcTestOrder(t, db, "PARTIAL-PARENT", constants.OrderStatusPaid, constants.FulfillmentTypeUpstream)
	first := createProcTestOrder(t, db, "PARTIAL-CHILD-1", constants.OrderStatusPaid, constants.FulfillmentTypeUpstream)
	second := createProcTestOrder(t, db, "PARTIAL-CHILD-2", constants.OrderStatusPaid, constants.FulfillmentTypeUpstream)
	if err := db.Model(&orderdomain.Order{}).Where("id IN ?", []uint{first.ID, second.ID}).Update("parent_id", parent.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&mappingdomain.Mapping{ConnectionID: 1, LocalProductID: 1, UpstreamProductID: 4, IsActive: true}).Error; err != nil {
		t.Fatal(err)
	}
	createTestProcurementOrder(t, db, 1, first.ID, first.OrderNo, "pending")
	svc := newTestProcurementService(db, newTestSiteConnectionService(db, "fixture-key", t.TempDir()))
	if err := svc.CreateForOrder(parent.ID); err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Model(&procurementdomain.Order{}).Where("local_order_id IN ?", []uint{first.ID, second.ID}).Count(&count)
	if count != 2 {
		t.Fatal("existing first child prevented remaining child procurement")
	}
}

// ── CreateForOrder tests ──

func TestCreateForOrder_SkipsNonUpstreamItems(t *testing.T) {
	db := setupProcurementTestDB(t)

	order := createProcTestOrder(t, db, "PROC-SKIP-001", constants.OrderStatusPaid, constants.FulfillmentTypeAuto)

	connSvc := newTestSiteConnectionService(db, "test-key", t.TempDir())
	svc := newTestProcurementService(db, connSvc)

	if err := svc.CreateForOrder(order.ID); err != nil {
		t.Fatalf("CreateForOrder: %v", err)
	}

	// 验证没有创建采购单
	var count int64
	db.Model(&ProcurementOrder{}).Count(&count)
	if count != 0 {
		t.Errorf("expected no procurement orders for auto fulfillment, got %d", count)
	}
}

func TestCreateForOrder_IdempotentSkipsDuplicate(t *testing.T) {
	db := setupProcurementTestDB(t)

	order := createProcTestOrder(t, db, "PROC-DUP-001", constants.OrderStatusPaid, constants.FulfillmentTypeUpstream)
	pm := &mappingdomain.Mapping{ConnectionID: 1, LocalProductID: 1, UpstreamProductID: 101, IsActive: true}
	db.Create(pm)

	connSvc := newTestSiteConnectionService(db, "test-key", t.TempDir())
	svc := newTestProcurementService(db, connSvc)

	// 第一次创建成功
	if err := svc.CreateForOrder(order.ID); err != nil {
		t.Fatalf("first CreateForOrder: %v", err)
	}

	// 第二次应该返回 ErrExists
	err := svc.CreateForOrder(order.ID)
	if err != ErrExists {
		t.Errorf("expected ErrExists on duplicate, got: %v", err)
	}
}
