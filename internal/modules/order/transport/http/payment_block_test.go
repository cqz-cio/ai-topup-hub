package orderhttp

import (
	"encoding/base64"
	"encoding/json"
	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	orderpresenter "github.com/dujiao-next/internal/modules/order/transport/presenter"
	pc "github.com/dujiao-next/internal/modules/procurement/contract"
	reseller "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type blockedOrderPayment struct{}

func (blockedOrderPayment) CreatePayment(CreatePaymentInput) (*CreatePaymentResult, error) {
	return nil, &pc.PaymentBlock{Code: pc.BalanceInsufficient, NotificationStatus: "pending", NoticeID: 7}
}

func TestCreateAndPayReturnsRecoverableOrderWithoutPaymentInstructions(t *testing.T) {
	h := &CreateHandler{pay: blockedOrderPayment{}}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/orders/create-and-pay", nil)
	h.respondCreateAndPay(c, &orderdomain.Order{ID: 1, OrderNo: "MOCK-BLOCKED"}, orderpresenter.OrderDetail{}, 3, false)
	var body struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data["order_no"] != "MOCK-BLOCKED" || body.Data["payment_block"] == nil {
		t.Fatalf("lost retry details: %s", w.Body.String())
	}
	for _, key := range []string{"payment_id", "qr_code", "pay_url", "wallet_address", "chain_amount"} {
		if _, exists := body.Data[key]; exists {
			t.Fatalf("exposed %s", key)
		}
	}
}

type deliveryQuery struct {
	UserOrderQuery
	GuestOrderQuery
	OrderQuery
	order      *orderdomain.Order
	seenTenant reseller.TenantContext
}

func (q *deliveryQuery) GetAnyOrderByUserOrderNoForTenant(tenant reseller.TenantContext, no string, uid uint) (*orderdomain.Order, error) {
	q.seenTenant = tenant
	if uid != 17 || no != q.order.OrderNo {
		return nil, ErrOrderNotFound
	}
	return q.order, nil
}
func (q *deliveryQuery) GetAnyOrderByGuestOrderNoForTenant(tenant reseller.TenantContext, no, email, password string) (*orderdomain.Order, error) {
	q.seenTenant = tenant
	if no != q.order.OrderNo || email != "fixture@example.test" || password != "fixture-password" {
		return nil, ErrGuestOrderNotFound
	}
	return q.order, nil
}
func (q *deliveryQuery) GetOrderForAdmin(uint) (*orderdomain.Order, error) { return q.order, nil }
func (q *deliveryQuery) ListOrdersForAdmin(OrderListFilter) ([]orderdomain.Order, int64, error) {
	return []orderdomain.Order{*q.order}, 1, nil
}

func deliveryFixture() *deliveryQuery {
	now := time.Now()
	return &deliveryQuery{order: &orderdomain.Order{ID: 8, OrderNo: "FIXTURE-CHILD", PaidAt: &now,
		Fulfillment: &fulfillmentdomain.Fulfillment{Status: "delivered", Payload: "TEST-NOT-REDEEMABLE"},
	}}
}

func TestGuestDeliveryRevalidatesCredentialsForViewAndDownload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, suffix := range []string{"/fulfillment", "/fulfillment/download"} {
		for _, credential := range []string{"", "fixture@example.test\nwrong-password", "wrong@example.test\nfixture-password", "fixture@example.test\nfixture-password"} {
			t.Run(suffix+"/"+credential, func(t *testing.T) {
				q := deliveryFixture()
				r := gin.New()
				RegisterGuestReadRoutes(r.Group("/guest"), NewGuestHandler(q, nil, nil, nil))
				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/guest/orders/FIXTURE-CHILD"+suffix, nil)
				if credential != "" {
					req.Header.Set("Authorization", "Guest "+base64.RawURLEncoding.EncodeToString([]byte(credential)))
				}
				r.ServeHTTP(w, req)
				want := credential == "fixture@example.test\nfixture-password"
				if strings.Contains(w.Body.String(), "TEST-NOT-REDEEMABLE") != want {
					t.Fatal("guest delivery authorization failed")
				}
				if !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
					t.Fatal("delivery response may be cached")
				}
			})
		}
	}
}

func TestUserDeliveryRevalidatesOwnerAndWithholdsUnpaidContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, uid := range []uint{0, 17, 18} {
		for _, paid := range []bool{false, true} {
			q := deliveryFixture()
			if !paid {
				q.order.PaidAt = nil
			}
			r := gin.New()
			r.Use(func(c *gin.Context) {
				if uid != 0 {
					c.Set("user_id", uid)
				}
				c.Next()
			})
			h := &UserHandler{orders: q}
			RegisterUserReadRoutes(r, h)
			for _, suffix := range []string{"/fulfillment", "/fulfillment/download"} {
				w := httptest.NewRecorder()
				r.ServeHTTP(w, httptest.NewRequest("GET", "/orders/FIXTURE-CHILD"+suffix, nil))
				if strings.Contains(w.Body.String(), "TEST-NOT-REDEEMABLE") != (uid == 17 && paid) {
					t.Fatalf("uid=%d paid=%t path=%s", uid, paid, suffix)
				}
				if !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
					t.Fatal("missing no-store")
				}
			}
		}
	}
}

func TestAdminListNeverPreloadsDelivery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	q := deliveryFixture()
	q.order.Children = []orderdomain.Order{*q.order}
	h := &AdminHandler{orders: q}
	r := gin.New()
	RegisterAdminRoutes(r, h)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/orders", nil))
	if strings.Contains(w.Body.String(), "TEST-NOT-REDEEMABLE") {
		t.Fatal("admin list preloaded parent or child card data")
	}
	if !strings.Contains(w.Body.String(), "delivered") {
		t.Fatal("delivery metadata missing")
	}
}
