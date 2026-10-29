package procurement_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dujiao-next/internal/app/container"
	"github.com/dujiao-next/internal/app/jobs/consumer"
	orderwiring "github.com/dujiao-next/internal/bootstrap/order"
	"github.com/dujiao-next/internal/config"
	cardsecretapp "github.com/dujiao-next/internal/modules/cardsecret/application"
	"github.com/dujiao-next/internal/modules/cardsecret/infrastructure/managementclient"
	cardsecrethttp "github.com/dujiao-next/internal/modules/cardsecret/transport/http"
	downstreamreader "github.com/dujiao-next/internal/modules/downstreamcallback/infrastructure/orderreader"
	notificationsmtp "github.com/dujiao-next/internal/modules/notification/infrastructure/smtp"
	orderapp "github.com/dujiao-next/internal/modules/order/application"
	ordergormstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"
	orderhttp "github.com/dujiao-next/internal/modules/order/transport/http"
	procurementgormstore "github.com/dujiao-next/internal/modules/procurement/infrastructure/gormstore"
	"github.com/dujiao-next/internal/modules/procurement/infrastructure/redemptionclient"
	"github.com/dujiao-next/internal/queue"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	"github.com/dujiao-next/internal/constants"
	procurementapp "github.com/dujiao-next/internal/modules/procurement/application"
	procurementcontract "github.com/dujiao-next/internal/modules/procurement/contract"
	procurementdomain "github.com/dujiao-next/internal/modules/procurement/domain"
)

type procurementCallbackStatusFixture struct {
	orderNo                   string
	initialOrderStatus        string
	initialProcurementStatus  string
	callbackStatus            string
	expectedProcurementStatus string
	expectedOrderStatus       string
}

func assertProcurementCallbackStatus(t *testing.T, fixture procurementCallbackStatusFixture) {
	t.Helper()
	db := setupProcurementTestDB(t)

	order := createProcTestOrder(t, db, fixture.orderNo, fixture.initialOrderStatus, constants.FulfillmentTypeUpstream)
	proc := createTestProcurementOrder(t, db, 1, order.ID, order.OrderNo, fixture.initialProcurementStatus)

	connSvc := newTestSiteConnectionService(db, "test-key", t.TempDir())
	svc := newTestProcurementService(db, connSvc)

	if err := svc.HandleUpstreamCallback(proc.ID, fixture.callbackStatus, nil); err != nil {
		t.Fatalf("HandleUpstreamCallback: %v", err)
	}

	var updatedProc ProcurementOrder
	if err := db.First(&updatedProc, proc.ID).Error; err != nil {
		t.Fatalf("load procurement: %v", err)
	}
	if updatedProc.Status != fixture.expectedProcurementStatus {
		t.Errorf("expected procurement status %q, got %q", fixture.expectedProcurementStatus, updatedProc.Status)
	}

	var updatedOrder orderdomain.Order
	if err := db.First(&updatedOrder, order.ID).Error; err != nil {
		t.Fatalf("load order: %v", err)
	}
	if updatedOrder.Status != fixture.expectedOrderStatus {
		t.Errorf("expected order status %q, got %q", fixture.expectedOrderStatus, updatedOrder.Status)
	}
}

// ── Phase 1 tests: order rollback on procurement failure ──

func TestRejectProcurement_RollsBackOrderStatus(t *testing.T) {
	db := setupProcurementTestDB(t)

	order := createProcTestOrder(t, db, "PROC-REJECT-001", constants.OrderStatusFulfilling, constants.FulfillmentTypeUpstream)
	proc := createTestProcurementOrder(t, db, 1, order.ID, order.OrderNo, "pending")

	connSvc := newTestSiteConnectionService(db, "test-key", t.TempDir())
	svc := newTestProcurementService(db, connSvc)

	if err := svc.SubmitToUpstream(proc.ID); err != nil {
		t.Fatalf("SubmitToUpstream with missing connection: %v", err)
	}

	// 验证采购单状态 = rejected
	var updatedProc ProcurementOrder
	if err := db.First(&updatedProc, proc.ID).Error; err != nil {
		t.Fatalf("load procurement: %v", err)
	}
	if updatedProc.Status != "rejected" {
		t.Errorf("expected procurement status 'rejected', got %q", updatedProc.Status)
	}

	// 验证本地订单状态从 fulfilling 回退到 paid
	var updatedOrder orderdomain.Order
	if err := db.First(&updatedOrder, order.ID).Error; err != nil {
		t.Fatalf("load order: %v", err)
	}
	if updatedOrder.Status != constants.OrderStatusPaid {
		t.Errorf("expected order status %q, got %q", constants.OrderStatusPaid, updatedOrder.Status)
	}
}

func TestHandleUpstreamCallback_Canceled_RollsBackOrder(t *testing.T) {
	db := setupProcurementTestDB(t)

	order := createProcTestOrder(t, db, "PROC-CANCEL-001", constants.OrderStatusFulfilling, constants.FulfillmentTypeUpstream)
	proc := createTestProcurementOrder(t, db, 1, order.ID, order.OrderNo, "accepted")

	connSvc := newTestSiteConnectionService(db, "test-key", t.TempDir())
	svc := newTestProcurementService(db, connSvc)

	if err := svc.HandleUpstreamCallback(proc.ID, "canceled", nil); err != nil {
		t.Fatalf("HandleUpstreamCallback: %v", err)
	}

	// 验证采购单状态 = canceled
	var updatedProc ProcurementOrder
	if err := db.First(&updatedProc, proc.ID).Error; err != nil {
		t.Fatalf("load procurement: %v", err)
	}
	if updatedProc.Status != "canceled" {
		t.Errorf("expected procurement status 'canceled', got %q", updatedProc.Status)
	}

	// 验证本地订单状态从 fulfilling 回退到 paid
	var updatedOrder orderdomain.Order
	if err := db.First(&updatedOrder, order.ID).Error; err != nil {
		t.Fatalf("load order: %v", err)
	}
	if updatedOrder.Status != constants.OrderStatusPaid {
		t.Errorf("expected order status %q, got %q", constants.OrderStatusPaid, updatedOrder.Status)
	}
}

func TestHandleUpstreamCallback_Delivered_CreatesFulfillment(t *testing.T) {
	db := setupProcurementTestDB(t)

	order := createProcTestOrder(t, db, "PROC-DELIVER-001", constants.OrderStatusFulfilling, constants.FulfillmentTypeUpstream)
	proc := createTestProcurementOrder(t, db, 1, order.ID, order.OrderNo, "accepted")

	connSvc := newTestSiteConnectionService(db, "test-key", t.TempDir())
	svc := newTestProcurementService(db, connSvc)

	now := time.Now()
	fulfillment := &procurementcontract.Fulfillment{
		Type:        constants.FulfillmentTypeUpstream,
		Status:      constants.FulfillmentStatusDelivered,
		Payload:     "CDK-001\nCDK-002",
		DeliveredAt: &now,
	}

	if err := svc.HandleUpstreamCallback(proc.ID, "delivered", fulfillment); err != nil {
		t.Fatalf("HandleUpstreamCallback: %v", err)
	}

	// 验证采购单状态 = fulfilled
	var updatedProc ProcurementOrder
	if err := db.First(&updatedProc, proc.ID).Error; err != nil {
		t.Fatalf("load procurement: %v", err)
	}
	if updatedProc.Status != "fulfilled" {
		t.Errorf("expected procurement status 'fulfilled', got %q", updatedProc.Status)
	}

	// 验证本地订单状态 = delivered
	var updatedOrder orderdomain.Order
	if err := db.First(&updatedOrder, order.ID).Error; err != nil {
		t.Fatalf("load order: %v", err)
	}
	if updatedOrder.Status != constants.OrderStatusDelivered {
		t.Errorf("expected order status %q, got %q", constants.OrderStatusDelivered, updatedOrder.Status)
	}

	// 验证 Fulfillment 记录已创建
	var ff fulfillmentdomain.Fulfillment
	if err := db.Where("order_id = ?", order.ID).First(&ff).Error; err != nil {
		t.Fatalf("expected fulfillment record to exist: %v", err)
	}
	if ff.Payload != "CDK-001\nCDK-002" {
		t.Errorf("unexpected fulfillment payload: %q", ff.Payload)
	}
	if ff.Type != constants.FulfillmentTypeUpstream {
		t.Errorf("expected fulfillment type %q, got %q", constants.FulfillmentTypeUpstream, ff.Type)
	}
}

func TestHandleUpstreamCallback_Delivered_SynchronizesParentStatus(t *testing.T) {
	db := setupProcurementTestDB(t)
	parent := createProcTestOrder(t, db, "PROC-PARENT-DELIVERED", constants.OrderStatusFulfilling, constants.FulfillmentTypeUpstream)
	child := createProcTestOrder(t, db, "PROC-CHILD-DELIVERED", constants.OrderStatusFulfilling, constants.FulfillmentTypeUpstream)
	if err := db.Model(&child).Update("parent_id", parent.ID).Error; err != nil {
		t.Fatalf("set child parent: %v", err)
	}
	proc := createTestProcurementOrder(t, db, 1, child.ID, child.OrderNo, constants.ProcurementStatusAccepted)

	svc := newTestProcurementService(db, newTestSiteConnectionService(db, "test-key", t.TempDir()))
	if err := svc.HandleUpstreamCallback(proc.ID, "delivered", &procurementcontract.Fulfillment{Payload: "SIMULATED-PARENT-CARD"}); err != nil {
		t.Fatalf("HandleUpstreamCallback: %v", err)
	}

	var updatedParent orderdomain.Order
	if err := db.First(&updatedParent, parent.ID).Error; err != nil {
		t.Fatalf("load parent order: %v", err)
	}
	if updatedParent.Status != constants.OrderStatusDelivered {
		t.Fatalf("parent status = %q, want %q", updatedParent.Status, constants.OrderStatusDelivered)
	}
}

func TestSupplierEmptyDeliveryKeepsOrderAwaitingCard(t *testing.T) {
	db := setupProcurementTestDB(t)
	order := createProcTestOrder(t, db, "EMPTY-DELIVERY", constants.OrderStatusFulfilling, constants.FulfillmentTypeUpstream)
	proc := createTestProcurementOrder(t, db, 1, order.ID, order.OrderNo, "accepted")
	svc := newTestProcurementService(db, newTestSiteConnectionService(db, "fixture-key", t.TempDir()))
	if err := svc.HandleUpstreamCallback(proc.ID, "delivered", nil); err == nil {
		t.Fatal("empty card was marked delivered")
	}
	if err := db.First(proc, proc.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(order, order.ID).Error; err != nil {
		t.Fatal(err)
	}
	if proc.Status != "accepted" || order.Status != constants.OrderStatusFulfilling {
		t.Fatal("empty card completed order")
	}
}

func TestHandleUpstreamCallback_PartiallyRefunded_AfterFulfilledUpdatesProcurementStatus(t *testing.T) {
	assertProcurementCallbackStatus(t, procurementCallbackStatusFixture{
		orderNo:                   "PROC-REFUND-KEEP-001",
		initialOrderStatus:        constants.OrderStatusDelivered,
		initialProcurementStatus:  constants.ProcurementStatusFulfilled,
		callbackStatus:            "partially_refunded",
		expectedProcurementStatus: constants.ProcurementStatusPartiallyRefunded,
		expectedOrderStatus:       constants.OrderStatusDelivered,
	})
}

func TestHandleUpstreamCallback_PartiallyRefunded_WhileFulfillingKeepsOrderStatus(t *testing.T) {
	assertProcurementCallbackStatus(t, procurementCallbackStatusFixture{
		orderNo:                   "PROC-REFUND-FULFILLING-001",
		initialOrderStatus:        constants.OrderStatusFulfilling,
		initialProcurementStatus:  constants.ProcurementStatusAccepted,
		callbackStatus:            "partially_refunded",
		expectedProcurementStatus: constants.ProcurementStatusPartiallyRefunded,
		expectedOrderStatus:       constants.OrderStatusFulfilling,
	})
}

func TestHandleUpstreamCallback_Refunded_AfterCompletedKeepsOrderStatus(t *testing.T) {
	assertProcurementCallbackStatus(t, procurementCallbackStatusFixture{
		orderNo:                   "PROC-REFUND-COMPLETED-001",
		initialOrderStatus:        constants.OrderStatusCompleted,
		initialProcurementStatus:  constants.ProcurementStatusFulfilled,
		callbackStatus:            "refunded",
		expectedProcurementStatus: constants.ProcurementStatusRefunded,
		expectedOrderStatus:       constants.OrderStatusCompleted,
	})
}

type deliveryRewriteFixture struct {
	fail  bool
	calls int
}

func (*deliveryRewriteFixture) RequiresRewrite(*procurementdomain.LocalOrder, uint) bool { return true }

// Starts the real Node issuer, talks to it through the real shop adapter, then
// uses real order routes, the notification worker and SMTP before redeeming.
// Only supplier recharging and the receiving SMTP server are test doubles.
func TestPAIPLUSShippingToRedemptionEndToEnd(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		quantity int
		account  bool
	}{{"single", 1, false}, {"bulk", 24, false}, {"account", 1, true}} {
		t.Run(scenario.name, func(t *testing.T) {
			var sources []string
			for i := 0; i < scenario.quantity; i++ {
				sources = append(sources, fmt.Sprintf("ELLPLUS-TEST%06d-CARD%06d", i, i))
			}
			raw := strings.Join(sources, "\n") + "\n兑换地址：https://cdk.bar"
			if scenario.account {
				raw = "邮箱号：account@example.invalid邮箱密码：Test-password#1密钥2fa：TESTSEED密钥2fa在线获取：https://auth.example.invalid/2fa 兑换卡密：" + sources[0]
			}
			var target string
			var drop atomic.Bool
			drop.Store(true)
			var bindingCalls atomic.Int32
			transport := &http.Client{Timeout: 10 * time.Second}
			proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				request, err := http.NewRequestWithContext(r.Context(), r.Method, target+r.URL.RequestURI(), r.Body)
				if err != nil {
					t.Error(err)
					http.Error(w, "fixture request", 500)
					return
				}
				request.Header = r.Header.Clone()
				response, err := transport.Do(request)
				if err != nil {
					t.Error(err)
					http.Error(w, "fixture transport", 502)
					return
				}
				defer response.Body.Close()
				body, _ := io.ReadAll(response.Body)
				if r.URL.Path == "/api/integration/deliver" {
					bindingCalls.Add(1)
					if drop.Swap(false) {
						http.Error(w, "simulated lost response after binding", 502)
						return
					}
				}
				for k, v := range response.Header {
					w.Header()[k] = v
				}
				w.WriteHeader(response.StatusCode)
				_, _ = w.Write(body)
			}))
			defer proxy.Close()
			var stats string
			target, stats = startRedemptionFixture(t, proxy.URL, sources)
			db := setupProcurementTestDB(t)
			orders := ordergormstore.New(db, "fixture-guest-secret-at-least-32-characters")
			paid := time.Now()
			parent := &orderdomain.Order{OrderNo: "PAI-PARENT-" + scenario.name, Status: constants.OrderStatusFulfilling, Currency: "CNY", PaidAt: &paid, GuestEmail: "buyer@example.invalid", GuestPassword: "fixture-password", GuestLocale: "zh-CN"}
			if err := orders.Create(parent, nil); err != nil {
				t.Fatal(err)
			}
			child := &orderdomain.Order{OrderNo: "PAI-CHILD-" + scenario.name, ParentID: &parent.ID, Status: constants.OrderStatusFulfilling, Currency: "CNY", PaidAt: &paid, GuestEmail: parent.GuestEmail, GuestPassword: "fixture-password", GuestLocale: "zh-CN"}
			items := []orderdomain.OrderItem{{ProductID: 30, SKUID: 30, Quantity: scenario.quantity, FulfillmentType: constants.FulfillmentTypeUpstream, TitleJSON: jsonmap.JSON{"zh-CN": "测试充值"}, InstructionsJSON: jsonmap.JSON{"zh-CN": "旧兑换说明 https://cdk.bar " + sources[0], "en-US": "old source " + sources[0]}}}
			if err := orders.Create(child, items); err != nil {
				t.Fatal(err)
			}
			proc := createTestProcurementOrder(t, db, 1, child.ID, child.OrderNo, "accepted")
			writer, err := redemptionclient.New(proxy.URL, "fixture-integration-token-not-a-production-secret", []uint{30}, 1, child.ID, paid.Add(-time.Minute))
			if err != nil {
				t.Fatal(err)
			}
			mailQueue := &redemptionMailQueue{}
			options := procurementOptionsForTest(db)
			options.DeliveryRewriter = writer
			options.OrderLifecycle = procurementgormstore.NewLifecycle(db, mailQueue, nil, config.EmailConfig{})
			service := procurementapp.NewService(options)
			if err := service.HandleUpstreamCallback(proc.ID, "delivered", &procurementcontract.Fulfillment{Type: "auto", Status: "delivered", Payload: raw}); err == nil {
				t.Fatal("lost issuer response should suspend shipping")
			}
			var count int64
			db.Model(&fulfillmentdomain.Fulfillment{}).Count(&count)
			if count != 0 || len(mailQueue.tasks) != 0 {
				t.Fatal("unbound card reached delivery or email queue")
			}
			if redemptionStats(t, stats)["cards"] != scenario.quantity {
				t.Fatal("issuer did not bind exactly one local code per source")
			}
			// Restarting the shop service retains the pending source and stable reference.
			service = procurementapp.NewService(options)
			if err := service.PollUpstreamStatus(proc.ID); err != nil {
				t.Fatal(err)
			}
			if redemptionStats(t, stats)["cards"] != scenario.quantity || bindingCalls.Load() != 2 {
				t.Fatal("recovery duplicated a binding")
			}
			if err := service.HandleUpstreamCallback(proc.ID, "delivered", &procurementcontract.Fulfillment{Type: "auto", Payload: raw}); err != nil {
				t.Fatal(err)
			}
			if bindingCalls.Load() != 2 {
				t.Fatal("duplicate callback generated another code")
			}
			var sourceRecord ProcurementOrder
			if err := db.First(&sourceRecord, proc.ID).Error; err != nil {
				t.Fatal(err)
			}
			if sourceRecord.UpstreamPayload != raw || sourceRecord.Status != "fulfilled" {
				t.Fatal("private supplier recovery record lost")
			}
			order, err := orders.GetByID(child.ID)
			if err != nil {
				t.Fatal(err)
			}
			codes := regexp.MustCompile(`PAIPLUS-[A-HJ-NP-Z2-9]{10}-[A-HJ-NP-Z2-9]{10}`).FindAllString(order.Fulfillment.Payload, -1)
			if len(codes) != scenario.quantity {
				t.Fatal("incorrect customer code count")
			}
			assertPrivate := func(label, output string) {
				t.Helper()
				for _, secret := range append(append([]string{}, sources...), "cdk.bar") {
					if strings.Contains(output, secret) {
						t.Fatalf("%s leaked supplier data", label)
					}
				}
			}
			storedJSON, _ := json.Marshal(order)
			assertPrivate("customer order snapshot", string(storedJSON))
			if scenario.account && (!strings.Contains(order.Fulfillment.Payload, "account@example.invalid") || !strings.Contains(order.Fulfillment.Payload, "TESTSEED")) {
				t.Fatal("account delivery fields lost")
			}
			projection, err := downstreamreader.New(orders).GetByID(parent.ID)
			if err != nil {
				t.Fatal(err)
			}
			projectionJSON, _ := json.Marshal(projection)
			assertPrivate("downstream notification projection", string(projectionJSON))
			if !strings.Contains(string(projectionJSON), codes[0]) {
				t.Fatal("downstream projection lost local code")
			}
			// Real guest and member read/download handlers, with their actual order store.
			c := &container.Container{OrderStore: orders, OrderService: orderapp.NewOrderService(orderapp.OrderServiceOptions{OrderStore: orders})}
			handlers := orderwiring.New(c)
			gin.SetMode(gin.TestMode)
			router := gin.New()
			orderhttp.RegisterGuestReadRoutes(router.Group("/guest"), handlers.Guest)
			users := router.Group("/user")
			users.Use(func(ctx *gin.Context) {
				if ctx.GetHeader("X-Test-User") == "17" {
					ctx.Set("user_id", uint(17))
				}
				ctx.Next()
			})
			orderhttp.RegisterUserReadRoutes(users, handlers.User)
			guestAuth := "Guest " + base64.RawURLEncoding.EncodeToString([]byte(parent.GuestEmail+"\nfixture-password"))
			read := func(path, auth, user string) *httptest.ResponseRecorder {
				t.Helper()
				request := httptest.NewRequest("GET", path, nil)
				request.Header.Set("Authorization", auth)
				request.Header.Set("X-Test-User", user)
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, request)
				assertPrivate(path, recorder.Body.String())
				return recorder
			}
			for _, no := range []string{child.OrderNo, parent.OrderNo} {
				for _, suffix := range []string{"/fulfillment", "/fulfillment/download"} {
					path := "/guest/orders/" + no + suffix
					if strings.Contains(read(path, "", "").Body.String(), codes[0]) {
						t.Fatal("unauthorized guest read local card")
					}
					response := read(path, guestAuth, "")
					// Parent delivery is collected via children; the download is the complete export.
					if suffix == "/fulfillment/download" || no == child.OrderNo {
						if response.Code != 200 || !strings.Contains(response.Body.String(), codes[0]) {
							t.Fatalf("authorized %s missing local delivery (HTTP %d)", suffix, response.Code)
						}
					}
					if !strings.Contains(response.Header().Get("Cache-Control"), "no-store") {
						t.Fatal("delivery response can be cached")
					}
					if suffix == "/fulfillment/download" {
						for _, code := range codes {
							if !strings.Contains(response.Body.String(), code) {
								t.Fatal("download truncated cards")
							}
						}
						if !strings.Contains(response.Header().Get("Content-Disposition"), "attachment") {
							t.Fatal("download missing attachment header")
						}
					}
				}
			}
			// Real notification worker and SMTP, including the bulk .txt attachment.
			emailConfig, messages := redemptionSMTP(t)
			c.EmailSender = notificationsmtp.New(&emailConfig)
			mux := asynq.NewServeMux()
			consumer.New(c).RegisterNotifications(mux)
			if len(mailQueue.tasks) != 1 || mailQueue.tasks[0].OrderID != parent.ID {
				t.Fatal("wrong delivered notification count/order")
			}
			task, err := queue.NewOrderStatusEmailTask(mailQueue.tasks[0])
			if err != nil {
				t.Fatal(err)
			}
			if err = mux.ProcessTask(context.Background(), task); err != nil {
				t.Fatal(err)
			}
			select {
			case message := <-messages:
				decoded, attachment := decodedRedemptionMail(t, message)
				assertPrivate("SMTP body and attachment", decoded)
				for _, code := range codes {
					if !strings.Contains(decoded, code) {
						t.Fatal("email lost local card")
					}
				}
				if (scenario.quantity > 20) != attachment {
					t.Fatal("wrong bulk email attachment behavior")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("SMTP message not received")
			}
			if err := db.Model(&orderdomain.Order{}).Where("id IN ?", []uint{parent.ID, child.ID}).Update("user_id", 17).Error; err != nil {
				t.Fatal(err)
			}
			for _, suffix := range []string{"/fulfillment", "/fulfillment/download"} {
				path := "/user/orders/" + child.OrderNo + suffix
				if strings.Contains(read(path, "", "").Body.String(), codes[0]) {
					t.Fatal("non-owner read local card")
				}
				response := read(path, "", "17")
				if response.Code != 200 || !strings.Contains(response.Body.String(), codes[0]) {
					t.Fatal("member delivery unavailable")
				}
			}
			// Redeem precisely the codes received from the shop download.
			boot, err := transport.Get(proxy.URL + "/api/session")
			if err != nil {
				t.Fatal(err)
			}
			cookie := boot.Cookies()[0]
			boot.Body.Close()
			post := func(path string, input any) map[string]any {
				t.Helper()
				body, _ := json.Marshal(input)
				request, _ := http.NewRequest("POST", proxy.URL+"/api/"+path, bytes.NewReader(body))
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Origin", proxy.URL)
				request.Header.Set("X-Pai-Request", "1")
				request.AddCookie(cookie)
				response, err := transport.Do(request)
				if err != nil {
					t.Fatal(err)
				}
				defer response.Body.Close()
				out, _ := io.ReadAll(response.Body)
				assertPrivate("redemption "+path, string(out))
				if response.StatusCode != 200 {
					t.Fatalf("redemption %s HTTP %d", path, response.StatusCode)
				}
				var result struct {
					Data map[string]any `json:"data"`
				}
				if err = json.Unmarshal(out, &result); err != nil {
					t.Fatal(err)
				}
				return result.Data
			}
			for _, code := range codes {
				p := post("preview", map[string]any{"code": code})
				check := post("preflight", map[string]any{"attempt_id": p["attempt_id"], "session": `{"demo":true,"email":"buyer@example.invalid"}`})
				if check["state"] != "checked" {
					t.Fatal("account check failed")
				}
				input := map[string]any{"attempt_id": p["attempt_id"], "confirmation": check["confirmation"], "confirmed": true}
				post("redeem", input)
				post("redeem", input)
			}
			deadline := time.Now().Add(12 * time.Second)
			for _, code := range codes {
				for {
					result := post("result-by-code", map[string]any{"code": code})
					if result["state"] == "completed" {
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("redemption did not complete")
					}
					time.Sleep(200 * time.Millisecond)
				}
			}
			if got := redemptionStats(t, stats); got["cards"] != scenario.quantity || got["submits"] != scenario.quantity || got["deliveries"] != 1 {
				t.Fatalf("one-to-one/submission invariant failed: %v", got)
			}
			t.Logf("verified %d one-to-one bindings, replay/recovery, private order instructions, member/guest downloads, SMTP %t attachment, and completed redemptions", scenario.quantity, scenario.quantity > 20)
		})
	}
}

func (f *deliveryRewriteFixture) Rewrite(_ *procurementdomain.LocalOrder, _ uint, source *procurementcontract.Fulfillment) (*procurementcontract.Fulfillment, error) {
	f.calls++
	if f.fail {
		return nil, errors.New("issuer temporarily unavailable")
	}
	copy := *source
	copy.Payload = "卡密：PAIPLUS-ABCDEFGH23-ABCDEFGH45"
	copy.InstructionsOverride = jsonmap.JSON{"zh-CN": "请使用 PAIPLUS 卡密在本站兑换"}
	return &copy, nil
}

func TestRedemptionIssuerFailureKeepsAcceptedAndRecoveryDeliversOnlyStoreCode(t *testing.T) {
	db := setupProcurementTestDB(t)
	order := createProcTestOrder(t, db, "REDEMPTION-DELIVERY", constants.OrderStatusFulfilling, constants.FulfillmentTypeUpstream)
	proc := createTestProcurementOrder(t, db, 1, order.ID, order.OrderNo, "accepted")
	raw := "卡密：ELLPLUS-AAAAAAAAAA-BBBBBBBBBB"
	if err := db.Model(&orderdomain.OrderItem{}).Where("order_id = ?", order.ID).Update("instructions_json", jsonmap.JSON{"zh-CN": "旧说明 https://cdk.bar 卡密：" + raw}).Error; err != nil {
		t.Fatal(err)
	}
	writer := &deliveryRewriteFixture{fail: true}
	options := procurementOptionsForTest(db)
	options.DeliveryRewriter = writer
	service := procurementapp.NewService(options)
	source := &procurementcontract.Fulfillment{Type: "auto", Status: "delivered", Payload: raw}
	if err := service.HandleUpstreamCallback(proc.ID, "delivered", source); err == nil {
		t.Fatal("issuer failure should stop delivery")
	}
	var count int64
	db.Model(&fulfillmentdomain.Fulfillment{}).Count(&count)
	if count != 0 {
		t.Fatal("raw card was delivered on issuer failure")
	}
	var current ProcurementOrder
	db.First(&current, proc.ID)
	if current.Status != "accepted" || current.UpstreamPayload != raw || current.ErrorCode != "paiplus_binding_pending" {
		t.Fatal("lost supplier result or marked delivery completed")
	}
	disabledOptions := options
	disabledOptions.DeliveryRewriter = nil
	if err := procurementapp.NewService(disabledOptions).PollUpstreamStatus(proc.ID); err == nil {
		t.Fatal("disabling the issuer released a pending supplier code")
	}
	writer.fail = false
	if err := service.HandleUpstreamCallback(proc.ID, "delivered", &procurementcontract.Fulfillment{Type: "auto", Payload: "changed-supplier-card"}); err == nil {
		t.Fatal("conflicting callback replaced original source")
	}
	// No supplier connection exists in this fixture: recovery must use the
	// durable delivery instead of purchasing or fetching another source card.
	if err := service.PollUpstreamStatus(proc.ID); err != nil {
		t.Fatal(err)
	}
	var delivered fulfillmentdomain.Fulfillment
	db.Where("order_id=?", order.ID).First(&delivered)
	if delivered.Payload == raw || delivered.Payload != "卡密：PAIPLUS-ABCDEFGH23-ABCDEFGH45" {
		t.Fatal("wrong customer delivery")
	}
	db.First(&current, proc.ID)
	if current.UpstreamPayload != raw || current.Status != "fulfilled" || current.ErrorCode != "" {
		t.Fatal("supplier recovery record changed")
	}
	if source.Payload != raw {
		t.Fatal("rewriter mutated original supplier payload")
	}
	var item orderdomain.OrderItem
	if err := db.Where("order_id = ?", order.ID).First(&item).Error; err != nil {
		t.Fatal(err)
	}
	if item.InstructionsJSON["zh-CN"] != "请使用 PAIPLUS 卡密在本站兑换" {
		t.Fatal("order instructions still expose supplier delivery")
	}
	if err := service.HandleUpstreamCallback(proc.ID, "delivered", source); err != nil {
		t.Fatal(err)
	}
	if writer.calls != 2 {
		t.Fatal("duplicate fulfilled callback rewrote historical delivery")
	}
}

func TestPAIPLUSShopAdminShippingRevokeAndRedeem(t *testing.T) {
	sources := []string{"ELLPLUS-ABCDEFGHIJ-23456789AB", "ELLPLUS-KLMNPQRSTU-3456789ABC"}
	origin, stats := startRedemptionFixture(t, "http://127.0.0.1", sources)
	paid := time.Now()
	issuer, err := redemptionclient.New(origin, "fixture-integration-token-not-a-production-secret", []uint{30}, 1, 1, paid.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	order := &procurementdomain.LocalOrder{ID: 100, OrderNo: "ADMIN-FLOW-100", PaidAt: &paid, Items: []procurementdomain.LocalOrderItem{{ProductID: 30, SKUID: 30, Quantity: 2, Title: jsonmap.JSON{"zh-CN": "测试 Plus"}}}}
	delivery, err := issuer.Rewrite(order, 1, &procurementcontract.Fulfillment{Type: "auto", Payload: strings.Join(sources, "\n")})
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range sources {
		if strings.Contains(delivery.Payload, source) {
			t.Fatal("customer delivery leaked supplier card")
		}
	}
	client, err := managementclient.New(origin, "fixture-management-token-not-a-production-secret")
	if err != nil {
		t.Fatal(err)
	}
	// Inventory methods are outside this fixture. The real management handler,
	// HTTP client, Node registry and supplier mock run without replacing them.
	handler := cardsecrethttp.NewAdminHandler(&cardsecretapp.Service{}, client)
	router := gin.New()
	group := router.Group("/admin", func(c *gin.Context) { c.Set("admin_id", uint(12)); c.Next() })
	cardsecrethttp.RegisterAdminRoutes(group, handler)
	admin := func(action string, input any) (map[string]any, int) {
		t.Helper()
		body, _ := json.Marshal(input)
		req := httptest.NewRequest("POST", "/admin/card-secrets/paiplus/"+action, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
			t.Fatal("management response cacheable")
		}
		var envelope map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		return envelope, int(envelope["status_code"].(float64))
	}
	listed, status := admin("query", map[string]any{"query": "ADMIN-FLOW-100"})
	if status != 0 {
		t.Fatal(listed)
	}
	rows := listed["data"].(map[string]any)["cards"].([]any)
	if len(rows) != 2 {
		t.Fatal("delivery missing from admin registry")
	}
	listBytes, _ := json.Marshal(listed)
	if strings.Contains(string(listBytes), sources[0]) {
		t.Fatal("list exposed full source code")
	}
	first := rows[0].(map[string]any)
	cardID := first["id"].(string)
	view, status := admin("reveal", map[string]any{"card_id": cardID, "actor": "shop-admin:999"})
	if status != 0 {
		t.Fatal(view)
	}
	detail := view["data"].(map[string]any)
	code := detail["code"].(string)
	if !strings.Contains(delivery.Payload, code) || detail["order_no"] != "ADMIN-FLOW-100" {
		t.Fatal("wrong order/card pair")
	}
	if detail["source_code"] != sources[0] && detail["source_code"] != sources[1] {
		t.Fatal("wrong upstream mapping")
	}
	audit := detail["audit"].([]any)
	if audit[0].(map[string]any)["actor"] != "shop-admin:12" {
		t.Fatal("client forged administrator identity")
	}
	revoked, status := admin("revoke", map[string]any{"card_id": cardID, "reason": "客户退款", "confirmed": true, "request_id": "admin-flow-revoke-100"})
	if status != 0 || revoked["data"].(map[string]any)["destroyed"] != true {
		t.Fatal("revocation did not succeed")
	}
	public := func(path string, input any) (map[string]any, int) {
		t.Helper()
		body, _ := json.Marshal(input)
		req, _ := http.NewRequest("POST", origin+"/api/"+path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "http://127.0.0.1")
		req.Header.Set("X-Pai-Request", "1")
		req.AddCookie(&http.Cookie{Name: "pai_device", Value: "device-ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"})
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var envelope map[string]any
		if err = json.NewDecoder(res.Body).Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		return envelope, res.StatusCode
	}
	blocked, httpStatus := public("preview", map[string]any{"code": code})
	if httpStatus != 409 || blocked["error"].(map[string]any)["code"] != "CARD_REVOKED" {
		t.Fatal("destroyed card still usable")
	}
	secondID := rows[1].(map[string]any)["id"].(string)
	other, _ := admin("reveal", map[string]any{"card_id": secondID})
	secondCode := other["data"].(map[string]any)["code"].(string)
	p, _ := public("preview", map[string]any{"code": secondCode})
	attempt := p["data"].(map[string]any)["attempt_id"]
	p, _ = public("preflight", map[string]any{"attempt_id": attempt, "session": `{"demo":true,"email":"admin-flow@example.invalid"}`})
	confirmation := p["data"].(map[string]any)["confirmation"]
	p, httpStatus = public("redeem", map[string]any{"attempt_id": attempt, "confirmation": confirmation, "confirmed": true})
	if httpStatus != 200 {
		t.Fatal(p)
	}
	if failed, status := admin("revoke", map[string]any{"card_id": secondID, "reason": "退款", "confirmed": true, "request_id": "admin-flow-revoke-101"}); status == 0 || !strings.Contains(failed["msg"].(string), "正在充值") {
		t.Fatal("in-flight card revoked")
	}
	time.Sleep(3 * time.Second)
	p, _ = public("result", map[string]any{"attempt_id": attempt})
	if p["data"].(map[string]any)["state"] != "completed" {
		t.Fatal("recharge did not complete")
	}
	if failed, status := admin("revoke", map[string]any{"card_id": secondID, "reason": "退款", "confirmed": true, "request_id": "admin-flow-revoke-102"}); status == 0 || !strings.Contains(failed["msg"].(string), "卡密无效，销毁失败") {
		t.Fatal("used card revoked")
	}
	final, _ := admin("query", map[string]any{})
	counts := final["data"].(map[string]any)["counts"].(map[string]any)
	if counts["used"] != float64(1) || counts["revoked"] != float64(1) {
		t.Fatal("incorrect admin status counts")
	}
	if snapshot := redemptionStats(t, stats); snapshot["cards"] != 2 || snapshot["submits"] != 1 {
		t.Fatal("reissued or charged destroyed card")
	}
}
