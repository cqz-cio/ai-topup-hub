package procurement_test

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"encoding/json"
	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	procurementapp "github.com/dujiao-next/internal/modules/procurement/application"
	procurementcontract "github.com/dujiao-next/internal/modules/procurement/contract"
	procurementdomain "github.com/dujiao-next/internal/modules/procurement/domain"
	procurementupstream "github.com/dujiao-next/internal/modules/procurement/infrastructure/upstreamgateway"
	"github.com/dujiao-next/internal/upstream"
	"net/http"
	"net/http/httptest"
	"testing"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"

	"github.com/dujiao-next/internal/constants"
	siteconnectionapp "github.com/dujiao-next/internal/modules/siteconnection/application"
)

// ── SubmitToUpstream tests ──

func TestSubmitToUpstream_Success(t *testing.T) {
	// This test uses only its httptest supplier and isolated database.
	t.Setenv("UPSTREAM_PURCHASE_ENABLED", "true")
	db := setupProcurementTestDB(t)

	order := createProcTestOrder(t, db, "PROC-SUBMIT-001", constants.OrderStatusPaid, constants.FulfillmentTypeUpstream)
	// 创建 product mapping 和 sku mapping
	pm := &mappingdomain.Mapping{
		ConnectionID:      1,
		LocalProductID:    1,
		UpstreamProductID: 101,
		IsActive:          true,
	}
	db.Create(pm)
	sm := &mappingdomain.SKUMapping{
		ProductMappingID: pm.ID,
		LocalSKUID:       1,
		UpstreamSKUID:    201,
		UpstreamIsActive: true,
	}
	db.Create(sm)

	// mock upstream server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":       true,
			"order_id": 999,
			"order_no": "UP-999",
			"status":   "accepted",
			"amount":   "50.00",
			"currency": "CNY",
		})
	}))
	defer server.Close()

	connSvc := newTestSiteConnectionService(db, "test-key", t.TempDir())
	conn, err := connSvc.Create(siteconnectionapp.CreateInput{
		Name:      "test-upstream",
		BaseURL:   server.URL,
		ApiKey:    "key",
		ApiSecret: "secret",
		Protocol:  constants.ConnectionProtocolDujiaoNext,
	})
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}

	proc := createTestProcurementOrder(t, db, conn.ID, order.ID, order.OrderNo, "pending")

	svc := newTestProcurementService(db, connSvc)

	if err := svc.SubmitToUpstream(proc.ID); err != nil {
		t.Fatalf("SubmitToUpstream: %v", err)
	}

	// 验证采购单状态 = accepted
	var updatedProc ProcurementOrder
	db.First(&updatedProc, proc.ID)
	if updatedProc.Status != "accepted" {
		t.Errorf("expected procurement status 'accepted', got %q", updatedProc.Status)
	}
	if updatedProc.UpstreamOrderID != 999 {
		t.Errorf("expected upstream_order_id=999, got %d", updatedProc.UpstreamOrderID)
	}

	// 验证本地订单状态 = fulfilling
	var updatedOrder orderdomain.Order
	db.First(&updatedOrder, order.ID)
	if updatedOrder.Status != constants.OrderStatusFulfilling {
		t.Errorf("expected order status %q, got %q", constants.OrderStatusFulfilling, updatedOrder.Status)
	}
}

func TestSubmitToUpstream_NonRetryableError_Rejects(t *testing.T) {
	// This test uses only its httptest supplier and isolated database.
	t.Setenv("UPSTREAM_PURCHASE_ENABLED", "true")
	db := setupProcurementTestDB(t)

	order := createProcTestOrder(t, db, "PROC-NONRETRY-001", constants.OrderStatusFulfilling, constants.FulfillmentTypeUpstream)
	pm := &mappingdomain.Mapping{ConnectionID: 1, LocalProductID: 1, UpstreamProductID: 101, IsActive: true}
	db.Create(pm)
	sm := &mappingdomain.SKUMapping{ProductMappingID: pm.ID, LocalSKUID: 1, UpstreamSKUID: 201, UpstreamIsActive: true}
	db.Create(sm)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":            false,
			"error_code":    "product_out_of_stock",
			"error_message": "product out of stock",
		})
	}))
	defer server.Close()

	connSvc := newTestSiteConnectionService(db, "test-key", t.TempDir())
	conn, _ := connSvc.Create(siteconnectionapp.CreateInput{
		Name: "test-upstream", BaseURL: server.URL,
		ApiKey: "key", ApiSecret: "secret", Protocol: constants.ConnectionProtocolDujiaoNext,
	})

	proc := createTestProcurementOrder(t, db, conn.ID, order.ID, order.OrderNo, "pending")
	svc := newTestProcurementService(db, connSvc)

	// 不可重试错误应返回 error
	_ = svc.SubmitToUpstream(proc.ID)

	// 验证采购单状态 = rejected
	var updatedProc ProcurementOrder
	db.First(&updatedProc, proc.ID)
	if updatedProc.Status != "rejected" {
		t.Errorf("expected procurement status 'rejected', got %q", updatedProc.Status)
	}

	// 验证本地订单状态回退到 paid
	var updatedOrder orderdomain.Order
	db.First(&updatedOrder, order.ID)
	if updatedOrder.Status != constants.OrderStatusPaid {
		t.Errorf("expected order status %q after rejection, got %q", constants.OrderStatusPaid, updatedOrder.Status)
	}
}

func TestSubmitToUpstream_RetryableError_Retries(t *testing.T) {
	// This test uses only its httptest supplier and isolated database.
	t.Setenv("UPSTREAM_PURCHASE_ENABLED", "true")
	db := setupProcurementTestDB(t)

	order := createProcTestOrder(t, db, "PROC-RETRY-001", constants.OrderStatusFulfilling, constants.FulfillmentTypeUpstream)
	pm := &mappingdomain.Mapping{ConnectionID: 1, LocalProductID: 1, UpstreamProductID: 101, IsActive: true}
	db.Create(pm)
	sm := &mappingdomain.SKUMapping{ProductMappingID: pm.ID, LocalSKUID: 1, UpstreamSKUID: 201, UpstreamIsActive: true}
	db.Create(sm)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":            false,
			"error_code":    "server_error",
			"error_message": "temporary failure",
		})
	}))
	defer server.Close()

	connSvc := newTestSiteConnectionService(db, "test-key", t.TempDir())
	conn, _ := connSvc.Create(siteconnectionapp.CreateInput{
		Name: "test-upstream", BaseURL: server.URL,
		ApiKey: "key", ApiSecret: "secret", Protocol: constants.ConnectionProtocolDujiaoNext,
		RetryMax: 3,
	})

	proc := createTestProcurementOrder(t, db, conn.ID, order.ID, order.OrderNo, "pending")
	svc := newTestProcurementService(db, connSvc)

	// 可重试错误不应返回 error（已入队重试）
	if err := svc.SubmitToUpstream(proc.ID); err != nil {
		t.Fatalf("expected no error for retryable failure, got: %v", err)
	}

	// 验证采购单状态 = failed（而非 rejected）
	var updatedProc ProcurementOrder
	db.First(&updatedProc, proc.ID)
	if updatedProc.Status != "failed" {
		t.Errorf("expected procurement status 'failed', got %q", updatedProc.Status)
	}
	if updatedProc.RetryCount != 1 {
		t.Errorf("expected retry_count=1, got %d", updatedProc.RetryCount)
	}
}

func TestHandleSubmitFailure_MaxRetriesExhausted(t *testing.T) {
	// This test uses only its httptest supplier and isolated database.
	t.Setenv("UPSTREAM_PURCHASE_ENABLED", "true")
	db := setupProcurementTestDB(t)

	order := createProcTestOrder(t, db, "PROC-MAXRETRY-001", constants.OrderStatusFulfilling, constants.FulfillmentTypeUpstream)
	productMapping := &mappingdomain.Mapping{ConnectionID: 1, LocalProductID: 1, UpstreamProductID: 101, IsActive: true}
	db.Create(productMapping)
	db.Create(&mappingdomain.SKUMapping{
		ProductMappingID: productMapping.ID,
		LocalSKUID:       1,
		UpstreamSKUID:    201,
		UpstreamIsActive: true,
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":            false,
			"error_code":    "server_error",
			"error_message": "timeout after retries",
		})
	}))
	defer server.Close()

	connSvc := newTestSiteConnectionService(db, "test-key", t.TempDir())
	conn, err := connSvc.Create(siteconnectionapp.CreateInput{
		Name: "test-upstream", BaseURL: server.URL,
		ApiKey: "key", ApiSecret: "secret", Protocol: constants.ConnectionProtocolDujiaoNext,
		RetryMax: 2, RetryIntervals: "[30,60]",
	})
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}

	proc := createTestProcurementOrder(t, db, conn.ID, order.ID, order.OrderNo, "failed")
	// 设置 retry_count 已达上限
	db.Model(proc).Update("retry_count", 2)

	svc := newTestProcurementService(db, connSvc)

	// 通过公开提交入口验证：可重试错误在次数耗尽后仍必须转为 rejected。
	_ = svc.SubmitToUpstream(proc.ID)

	// 验证采购单状态 = rejected
	var updatedProc ProcurementOrder
	db.First(&updatedProc, proc.ID)
	if updatedProc.Status != "rejected" {
		t.Errorf("expected procurement status 'rejected', got %q", updatedProc.Status)
	}

	// 验证本地订单回退到 paid
	var updatedOrder orderdomain.Order
	db.First(&updatedOrder, order.ID)
	if updatedOrder.Status != constants.OrderStatusPaid {
		t.Errorf("expected order status %q, got %q", constants.OrderStatusPaid, updatedOrder.Status)
	}
}

func TestACGProcurementPaidOrderDeliversCardOnce(t *testing.T) {
	adapter := &acgProcurementAdapter{result: &upstream.CreateUpstreamOrderResp{OK: true, OrderNo: "001234567890123456", Amount: "33.00", Currency: "CNY", Fulfillment: &upstream.UpstreamFulfillment{Type: "auto", Status: "delivered", Payload: "PAID-CARD"}}}
	db, svc, proc, order := acgProcurementFixture(t, constants.OrderStatusPaid, adapter)
	if err := svc.SubmitToUpstream(proc.ID); err != nil {
		t.Fatal(err)
	}
	_ = svc.SubmitToUpstream(proc.ID) // Redelivered queue jobs cannot purchase again.
	var actual procurementdomain.Order
	db.First(&actual, proc.ID)
	var local orderdomain.Order
	db.First(&local, order.ID)
	var fulfillment fulfillmentdomain.Fulfillment
	db.Where("order_id = ?", order.ID).First(&fulfillment)
	if adapter.calls.Load() != 1 || actual.Status != "fulfilled" || actual.UpstreamOrderNo != "001234567890123456" || local.Status != constants.OrderStatusDelivered || fulfillment.Payload != "PAID-CARD" {
		t.Fatalf("purchase/delivery not exactly once: calls=%d proc=%#v local=%s payload=%q", adapter.calls.Load(), actual, local.Status, fulfillment.Payload)
	}
	if adapter.request.MaxAmount != "33.00" || adapter.request.Currency != "CNY" || adapter.request.SKUID != 4 {
		t.Fatalf("purchase lost cost ceiling: %#v", adapter.request)
	}
}

func TestACGProcurementUnpaidOrderNeverPurchases(t *testing.T) {
	adapter := &acgProcurementAdapter{}
	db, svc, proc, _ := acgProcurementFixture(t, constants.OrderStatusPendingPayment, adapter)
	if err := svc.SubmitToUpstream(proc.ID); err != nil {
		t.Fatal(err)
	}
	var actual procurementdomain.Order
	db.First(&actual, proc.ID)
	if adapter.calls.Load() != 0 || actual.Status != "rejected" {
		t.Fatalf("unpaid order reached supplier: %d %s", adapter.calls.Load(), actual.Status)
	}
}

func TestACGProcurementUncertainChargeCannotBeRetriedOrCanceled(t *testing.T) {
	adapter := &acgProcurementAdapter{err: errors.New("response lost after charge")}
	db, svc, proc, _ := acgProcurementFixture(t, constants.OrderStatusPaid, adapter)
	if err := svc.SubmitToUpstream(proc.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.SubmitToUpstream(proc.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.RetryManual(proc.ID); err == nil {
		t.Fatal("ambiguous purchase allowed a manual resubmit")
	}
	if err := svc.CancelManual(proc.ID); err == nil {
		t.Fatal("ambiguous supplier charge was marked canceled")
	}
	var actual procurementdomain.Order
	db.First(&actual, proc.ID)
	if adapter.calls.Load() != 1 || actual.Status != "manual_review" {
		t.Fatalf("ambiguous charge was resent: %d %s", adapter.calls.Load(), actual.Status)
	}
}

func TestACGProcurementRecoversDelayedCardByStringOrderNumber(t *testing.T) {
	adapter := &acgProcurementAdapter{result: &upstream.CreateUpstreamOrderResp{OK: true, OrderNo: "001234567890123456", Amount: "33.00", Currency: "CNY"}}
	db, svc, proc, order := acgProcurementFixture(t, constants.OrderStatusPaid, adapter)
	if err := svc.SubmitToUpstream(proc.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.PollUpstreamStatus(proc.ID); err != nil {
		t.Fatal(err)
	}
	var actual orderdomain.Order
	db.First(&actual, order.ID)
	if adapter.calls.Load() != 1 || adapter.reference != "001234567890123456" || actual.Status != constants.OrderStatusDelivered {
		t.Fatalf("string order reference was lost: %s status=%s", adapter.reference, actual.Status)
	}
}

func TestACGProcurementConcurrentWorkersClaimOnePurchase(t *testing.T) {
	adapter := &acgProcurementAdapter{result: &upstream.CreateUpstreamOrderResp{OK: true, OrderNo: "001234567890123456", Amount: "33.00", Currency: "CNY"}}
	_, svc, proc, _ := acgProcurementFixture(t, constants.OrderStatusPaid, adapter)
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() { defer group.Done(); _ = svc.SubmitToUpstream(proc.ID) }()
	}
	group.Wait()
	if adapter.calls.Load() != 1 {
		t.Fatalf("parallel workers purchased %d times", adapter.calls.Load())
	}
}

type delayedPreparationFailure struct {
	procurementcontract.ConnectionProvider
	calls   atomic.Int32
	started chan struct{}
	release chan struct{}
	missing bool
}

func (p *delayedPreparationFailure) Open(id uint) (procurementcontract.UpstreamConnection, error) {
	if p.calls.Add(1) == 1 {
		close(p.started)
		<-p.release
		if p.missing {
			return nil, nil
		}
		return nil, errors.New("temporary connection read failure")
	}
	return p.ConnectionProvider.Open(id)
}

func TestACGStalePreparationCannotReopenUncertainPurchase(t *testing.T) {
	for _, missing := range []bool{false, true} {
		name := "transient error"
		if missing {
			name = "missing connection"
		}
		t.Run(name, func(t *testing.T) {
			adapter := &acgProcurementAdapter{err: errors.New("response lost after charge")}
			db, _, proc, _ := acgProcurementFixture(t, constants.OrderStatusPaid, adapter)
			provider := &delayedPreparationFailure{
				ConnectionProvider: procurementupstream.New(&acgProcurementProvider{adapter: adapter}),
				started:            make(chan struct{}), release: make(chan struct{}), missing: missing,
			}
			options := procurementOptionsForTest(db)
			options.Connections = provider
			svc := procurementapp.NewService(options)
			done := make(chan error, 1)
			go func() { done <- svc.SubmitToUpstream(proc.ID) }()
			var releaseOnce sync.Once
			defer releaseOnce.Do(func() { close(provider.release) })
			select {
			case <-provider.started:
			case <-time.After(5 * time.Second):
				t.Fatal("first worker did not reach preparation")
			}
			if err := svc.SubmitToUpstream(proc.ID); err != nil {
				t.Fatal(err)
			}
			releaseOnce.Do(func() { close(provider.release) })
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("stale worker did not finish")
			}
			if err := db.First(proc, proc.ID).Error; err != nil {
				t.Fatal(err)
			}
			if proc.Status != "manual_review" {
				t.Fatalf("stale preparation overwrote uncertain purchase with %q", proc.Status)
			}
			if err := svc.SubmitToUpstream(proc.ID); err != nil {
				t.Fatal(err)
			}
			if adapter.calls.Load() != 1 {
				t.Fatal("stale preparation allowed a second balance purchase")
			}
		})
	}
}

type staleManualRetryRead struct {
	procurementcontract.Repository
	afterRead func() error
}

func (r *staleManualRetryRead) GetByID(id uint) (*procurementdomain.Order, error) {
	order, err := r.Repository.GetByID(id)
	if err == nil && order != nil && r.afterRead != nil {
		callback := r.afterRead
		r.afterRead = nil
		err = callback()
	}
	return order, err
}

func TestACGManualRetryCannotResetAnAlreadyClaimedPurchase(t *testing.T) {
	adapter := &acgProcurementAdapter{}
	db, _, proc, _ := acgProcurementFixture(t, constants.OrderStatusPaid, adapter)
	if err := db.Model(proc).Update("status", "rejected").Error; err != nil {
		t.Fatal(err)
	}
	options := procurementOptionsForTest(db)
	options.Repository = &staleManualRetryRead{
		Repository: options.Repository,
		// Another manual retry won and its worker claimed the purchase while
		// this request still held the old rejected snapshot.
		afterRead: func() error { return db.Model(proc).Update("status", "submitting").Error },
	}
	options.Connections = procurementupstream.New(&acgProcurementProvider{adapter: adapter})
	svc := procurementapp.NewService(options)
	if err := svc.RetryManual(proc.ID); !errors.Is(err, procurementcontract.ErrStatusInvalid) {
		t.Fatalf("stale retry was not refused: %v", err)
	}
	if err := db.First(proc, proc.ID).Error; err != nil {
		t.Fatal(err)
	}
	if proc.Status != "submitting" {
		t.Fatal("manual retry reopened a claimed balance purchase")
	}
}

func TestACGCancelDoesNotProceedWhenConnectionCannotBeRead(t *testing.T) {
	adapter := &acgProcurementAdapter{}
	db, _, proc, _ := acgProcurementFixture(t, constants.OrderStatusPaid, adapter)
	provider := &delayedPreparationFailure{
		started: make(chan struct{}), release: make(chan struct{}),
	}
	close(provider.release)
	options := procurementOptionsForTest(db)
	options.Connections = provider
	svc := procurementapp.NewService(options)
	if err := svc.CancelManual(proc.ID); err == nil {
		t.Fatal("cancellation continued without knowing the supplier's capabilities")
	}
	if err := db.First(proc, proc.ID).Error; err != nil {
		t.Fatal(err)
	}
	if proc.Status != "pending" {
		t.Fatal("unavailable connection caused a local-only supplier cancellation")
	}
}

func TestACGProcurementPendingBalancePurchaseCannotBeCanceled(t *testing.T) {
	adapter := &acgProcurementAdapter{}
	db, svc, proc, _ := acgProcurementFixture(t, constants.OrderStatusPaid, adapter)
	if err := svc.CancelManual(proc.ID); err == nil {
		t.Fatal("pending balance purchase allowed a cancellation race")
	}
	var actual procurementdomain.Order
	db.First(&actual, proc.ID)
	if actual.Status != "pending" || adapter.calls.Load() != 0 {
		t.Fatalf("cancellation changed supplier purchase: %s %d", actual.Status, adapter.calls.Load())
	}
}

func TestACGInterruptedSubmissionIsReconciledWithoutResend(t *testing.T) {
	adapter := &acgProcurementAdapter{}
	db, svc, proc, _ := acgProcurementFixture(t, constants.OrderStatusPaid, adapter)
	db.Model(proc).Updates(map[string]any{"status": "submitting", "updated_at": time.Now().Add(-10 * time.Minute)})
	svc.SyncAcceptedOrders()
	var actual procurementdomain.Order
	db.First(&actual, proc.ID)
	if actual.Status != "manual_review" || adapter.calls.Load() != 0 {
		t.Fatalf("interrupted purchase was resubmitted: %s %d", actual.Status, adapter.calls.Load())
	}
}

func TestACGReadOnlyPreflightFailureRetriesOnlyAfterDelay(t *testing.T) {
	adapter := &acgProcurementAdapter{result: &upstream.CreateUpstreamOrderResp{ErrorCode: "supplier_unavailable", ErrorMessage: "read-only quote unavailable"}}
	db, _, proc, order := acgProcurementFixture(t, constants.OrderStatusPaid, adapter)
	q := &workflowQueue{}
	options := procurementOptionsForTest(db)
	options.Queue = q
	options.Connections = procurementupstream.New(&acgProcurementProvider{adapter: adapter})
	svc := procurementapp.NewService(options)
	if err := svc.SubmitToUpstream(proc.ID); err != nil {
		t.Fatal(err)
	}
	if len(q.delays) != 1 || q.delays[0] != 30*time.Second {
		t.Fatal("read-only retry delay was lost")
	}
	if err := svc.SubmitToUpstream(proc.ID); err != nil {
		t.Fatal(err)
	}
	if adapter.calls.Load() != 1 {
		t.Fatal("preflight retried before deadline")
	}
	if err := db.Model(proc).Update("next_retry_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	adapter.result = &upstream.CreateUpstreamOrderResp{OK: true, OrderNo: "RETRY-FIXTURE", Amount: "33.00", Currency: "CNY", Fulfillment: &upstream.UpstreamFulfillment{Payload: "RETRY-FIXTURE-CARD"}}
	if err := svc.SubmitToUpstream(proc.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.First(order, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if adapter.calls.Load() != 2 || order.Status != constants.OrderStatusDelivered {
		t.Fatal("safe delayed retry did not deliver")
	}
}

type failDeliveryOrderRead struct {
	procurementcontract.OrderRepository
	reads int
}

func (r *failDeliveryOrderRead) GetByID(id uint) (*procurementdomain.LocalOrder, error) {
	r.reads++
	if r.reads == 2 {
		return nil, errors.New("temporary order read failure after card persistence")
	}
	return r.OrderRepository.GetByID(id)
}

func TestPersistedSupplierCardRecoversAfterOrderReadFailureWithoutNewPurchase(t *testing.T) {
	adapter := &acgProcurementAdapter{result: &upstream.CreateUpstreamOrderResp{OK: true, OrderNo: "DELIVERY-RECOVERY-FIXTURE", Amount: "33.00", Currency: "CNY", Fulfillment: &upstream.UpstreamFulfillment{Payload: "PERSISTED-FIXTURE-CARD"}}}
	db, _, proc, order := acgProcurementFixture(t, constants.OrderStatusPaid, adapter)
	options := procurementOptionsForTest(db)
	options.Orders = &failDeliveryOrderRead{OrderRepository: options.Orders}
	options.Connections = procurementupstream.New(&acgProcurementProvider{adapter: adapter})
	svc := procurementapp.NewService(options)
	if err := svc.SubmitToUpstream(proc.ID); err == nil {
		t.Fatal("delivery read failure was hidden")
	}
	if err := db.First(proc, proc.ID).Error; err != nil {
		t.Fatal(err)
	}
	if proc.Status != "accepted" {
		t.Fatal("delivery failure prematurely completed procurement")
	}
	if err := svc.SubmitToUpstream(proc.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.First(proc, proc.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(order, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if proc.Status != "fulfilled" || order.Status != constants.OrderStatusDelivered || adapter.calls.Load() != 1 {
		t.Fatal("delivery recovery sent another purchase")
	}
}
