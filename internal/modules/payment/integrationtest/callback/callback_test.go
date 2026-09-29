package paymentcallback_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	okpayadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/okpay"

	paymentapp "github.com/dujiao-next/internal/modules/payment/application"
	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
	paymentgormstore "github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	ordercontract "github.com/dujiao-next/internal/modules/order/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	ordergormstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	productgormstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/constants"
	paymentprovider "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/provider"
	paymentcallback "github.com/dujiao-next/internal/modules/payment/transport/http/callback"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type okpayCallbackFixture struct {
	orderRepo   ordercontract.Store
	paymentRepo paymentcontract.Store
	handler     *paymentcallback.Handler
	order       *orderdomain.Order
	payment     *paymentdomain.Payment
}

type callbackServiceTestAdapter struct {
	payments *paymentapp.PaymentService
}

func (a callbackServiceTestAdapter) HandleSyncCallback(channel *paymentdomain.PaymentChannel, form map[string][]string, body []byte) (*paymentdomain.Payment, error) {
	return a.payments.HandleSyncCallback(channel, form, body)
}

func (a callbackServiceTestAdapter) HandleWechatWebhook(input paymentcallback.WechatWebhookInput) (*paymentdomain.Payment, string, error) {
	return a.payments.HandleWechatWebhook(paymentapp.WebhookCallbackInput{
		ChannelID: input.ChannelID,
		Headers:   input.Headers,
		Body:      input.Body,
		Context:   input.Context,
	})
}

func newOkpayCallbackFixture(t *testing.T) *okpayCallbackFixture {
	t.Helper()

	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:payment_callback_okpay_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&userdomain.User{},
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&orderdomain.Order{},
		&orderdomain.OrderItem{},
		&fulfillmentdomain.Fulfillment{},
		&paymentdomain.PaymentChannel{},
		&paymentdomain.Payment{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	user := &userdomain.User{
		Email:        "okpay-callback@example.com",
		PasswordHash: "hash",
		Status:       constants.UserStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	order := &orderdomain.Order{
		OrderNo:                 "DJOKPAYCALLBACK001",
		UserID:                  user.ID,
		Status:                  constants.OrderStatusPendingPayment,
		Currency:                "CNY",
		OriginalAmount:          money.FromDecimal(decimal.NewFromInt(88)),
		DiscountAmount:          money.FromDecimal(decimal.Zero),
		PromotionDiscountAmount: money.FromDecimal(decimal.Zero),
		TotalAmount:             money.FromDecimal(decimal.NewFromInt(88)),
		WalletPaidAmount:        money.FromDecimal(decimal.Zero),
		OnlinePaidAmount:        money.FromDecimal(decimal.NewFromInt(88)),
		RefundedAmount:          money.FromDecimal(decimal.Zero),
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}
	channel := &paymentdomain.PaymentChannel{
		Name:            "OKPAY",
		ProviderType:    constants.PaymentProviderOkpay,
		ChannelType:     constants.PaymentChannelTypeUsdt,
		InteractionMode: constants.PaymentInteractionQR,
		FeeRate:         money.FromDecimal(decimal.Zero),
		ConfigJSON: jsonmap.JSON{
			"merchant_id":    "shop-1",
			"merchant_token": "token-1",
			"return_url":     "https://example.com/pay",
			"callback_url":   "https://api.example.com/api/v1/payments/callback",
			"exchange_rate":  "7",
		},
		IsActive:  true,
		SortOrder: 10,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Create(channel).Error; err != nil {
		t.Fatalf("create channel failed: %v", err)
	}
	// okpay 是加密货币网关，payment 实际以 USDT 计价（P1.2c Task 1 后 CurrencySent 写入 DB）
	payment := &paymentdomain.Payment{
		OrderID:         order.ID,
		ChannelID:       channel.ID,
		ProviderType:    channel.ProviderType,
		ChannelType:     channel.ChannelType,
		InteractionMode: channel.InteractionMode,
		Amount:          money.FromDecimal(decimal.NewFromFloat(616)),
		FeeRate:         money.FromDecimal(decimal.Zero),
		FeeAmount:       money.FromDecimal(decimal.Zero),
		Currency:        "USDT",
		Status:          constants.PaymentStatusPending,
		ProviderRef:     "OKPAY-ORDER-1",
		GatewayOrderNo:  "DJP9001",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(payment).Error; err != nil {
		t.Fatalf("create payment failed: %v", err)
	}

	orderRepo := ordergormstore.New(db, "test-guest-credential-secret-with-32-bytes")
	paymentRepo := paymentgormstore.New(db, "test-guest-credential-secret-with-32-bytes")
	channelRepo := paymentgormstore.NewChannelStore(db)
	productRepo := productgormstore.NewProductStore(db)
	productSKURepo := productgormstore.NewSKUStore(db)

	registry := paymentprovider.NewRegistry()
	registry.Register(constants.PaymentProviderOkpay, "", okpayadapter.NewOkpayAdapter())

	paymentService := paymentapp.NewPaymentService(paymentapp.PaymentServiceOptions{
		OrderStore:              orderRepo,
		ProductRepo:             productRepo,
		ProductSKURepo:          productSKURepo,
		PaymentStore:            paymentRepo,
		ChannelStore:            channelRepo,
		ExpireMinutes:           15,
		PaymentProviderRegistry: registry,
	})

	return &okpayCallbackFixture{
		orderRepo:   orderRepo,
		paymentRepo: paymentRepo,
		handler:     paymentcallback.NewHandler(callbackServiceTestAdapter{payments: paymentService}, paymentRepo, channelRepo, nil),
		order:       order,
		payment:     payment,
	}
}

func TestPaymentCallbackRejectsOkpayFormEncodedBody(t *testing.T) {
	fixture := newOkpayCallbackFixture(t)
	bodyWithoutSign := "code=200&data.order_id=OKPAY-ORDER-1&data.unique_id=DJP9001&data.pay_user_id=7238234930&data.amount=616.00000000&data.coin=USDT&data.status=1&data.type=deposit&id=shop-1&status=success"
	sign := hmacSHA256HexUpper(bodyWithoutSign, "token-1")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/callback", strings.NewReader(bodyWithoutSign+"&sign="+sign))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	fixture.handler.PaymentCallback(c)

	// 新协议下 OKPay 只发 JSON 回调,form-encoded body 不应被任何 handler 识别。
	if w.Code != http.StatusNotFound {
		t.Fatalf("form-encoded okpay callback should be unrecognized, got status %d body %s", w.Code, w.Body.String())
	}
}

func TestPaymentCallbackHandlesOkpayJSON(t *testing.T) {
	fixture := newOkpayCallbackFixture(t)
	body := signedOkpayJSONCallback("616.00000000", "USDT", "token-1")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/callback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	fixture.handler.PaymentCallback(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if strings.TrimSpace(w.Body.String()) != constants.OkpayCallbackSuccess {
		t.Fatalf("unexpected response body: %s", w.Body.String())
	}

	updatedPayment, err := fixture.paymentRepo.GetByID(fixture.payment.ID)
	if err != nil {
		t.Fatalf("reload payment failed: %v", err)
	}
	if updatedPayment == nil || updatedPayment.Status != constants.PaymentStatusSuccess {
		t.Fatalf("payment status not updated: %+v", updatedPayment)
	}
	updatedOrder, err := fixture.orderRepo.GetByID(fixture.order.ID)
	if err != nil {
		t.Fatalf("reload order failed: %v", err)
	}
	if updatedOrder == nil || updatedOrder.Status != constants.OrderStatusPaid {
		t.Fatalf("order status not updated: %+v", updatedOrder)
	}
}

func TestPaymentCallbackRejectsOkpayJSONInvalidSignature(t *testing.T) {
	fixture := newOkpayCallbackFixture(t)
	body := strings.Replace(signedOkpayJSONCallback("616.00000000", "USDT", "token-1"), `"sign":"`, `"sign":"BAD`, 1)
	w := performOkpayJSONCallback(t, fixture, body)
	assertOkpayCallbackRejected(t, fixture, w)
}

func TestPaymentCallbackRejectsOkpayJSONAmountMismatch(t *testing.T) {
	fixture := newOkpayCallbackFixture(t)
	body := signedOkpayJSONCallback("615.00000000", "USDT", "token-1")
	w := performOkpayJSONCallback(t, fixture, body)
	assertOkpayCallbackRejected(t, fixture, w)
}

func TestPaymentCallbackRejectsOkpayJSONCurrencyMismatch(t *testing.T) {
	fixture := newOkpayCallbackFixture(t)
	body := signedOkpayJSONCallback("616.00000000", "TRX", "token-1")
	w := performOkpayJSONCallback(t, fixture, body)
	assertOkpayCallbackRejected(t, fixture, w)
}

func performOkpayJSONCallback(t *testing.T, fixture *okpayCallbackFixture, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/callback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	fixture.handler.PaymentCallback(c)
	return w
}

func assertOkpayCallbackRejected(t *testing.T, fixture *okpayCallbackFixture, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 for okpay retry contract, got %d", w.Code)
	}
	if strings.TrimSpace(w.Body.String()) != constants.OkpayCallbackFail {
		t.Fatalf("expected okpay fail response, got %s", w.Body.String())
	}
	updatedPayment, err := fixture.paymentRepo.GetByID(fixture.payment.ID)
	if err != nil {
		t.Fatalf("reload payment failed: %v", err)
	}
	if updatedPayment == nil || updatedPayment.Status != constants.PaymentStatusPending {
		t.Fatalf("payment should remain pending: %+v", updatedPayment)
	}
	updatedOrder, err := fixture.orderRepo.GetByID(fixture.order.ID)
	if err != nil {
		t.Fatalf("reload order failed: %v", err)
	}
	if updatedOrder == nil || updatedOrder.Status != constants.OrderStatusPendingPayment {
		t.Fatalf("order should remain pending payment: %+v", updatedOrder)
	}
}

func signedOkpayJSONCallback(amount string, coin string, token string) string {
	bodyWithoutSign := fmt.Sprintf(
		`{"code":200,"data":{"order_id":"OKPAY-ORDER-1","unique_id":"DJP9001","pay_user_id":"7238234930","amount":%q,"coin":%q,"status":1,"type":"deposit"},"id":"shop-1","status":"success"}`,
		amount,
		coin,
	)
	// 新协议签名原文用点号展开嵌套 data,不做 URL 编码。
	signBase := fmt.Sprintf(
		`code=200&data.amount=%s&data.coin=%s&data.order_id=OKPAY-ORDER-1&data.pay_user_id=7238234930&data.status=1&data.type=deposit&data.unique_id=DJP9001&id=shop-1&status=success`,
		amount,
		coin,
	)
	sign := hmacSHA256HexUpper(signBase, token)
	return strings.TrimSuffix(bodyWithoutSign, "}") + `,"sign":"` + sign + `"}`
}

func hmacSHA256HexUpper(base string, token string) string {
	mac := hmac.New(sha256.New, []byte(token))
	mac.Write([]byte(base))
	return strings.ToUpper(hex.EncodeToString(mac.Sum(nil)))
}
