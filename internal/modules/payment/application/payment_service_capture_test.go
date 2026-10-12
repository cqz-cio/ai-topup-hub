package application

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/provider"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

// recordingCapturer 捕获传入 ValidateConfig 的第二参数,用于回归测试。
type recordingCapturer struct {
	receivedMode string
}

func (r *recordingCapturer) Type() string { return "test_capturer" }

func (r *recordingCapturer) ValidateConfig(_ jsonmap.JSON, mode string) error {
	r.receivedMode = mode
	return errors.New("stop-test")
}

func (r *recordingCapturer) CreatePayment(_ context.Context, _ jsonmap.JSON, _ paymentcontract.GatewayCreateInput) (*paymentcontract.GatewayCreateResult, error) {
	return nil, errors.New("not-implemented")
}

func (r *recordingCapturer) QueryPayment(_ context.Context, _ jsonmap.JSON, _ string) (*paymentcontract.GatewayQueryResult, error) {
	return nil, errors.New("not-implemented")
}

// 防止重新引入 stripe/wechat capture 全部失败的回归:
// captureViaRegistry 必须把 channel.InteractionMode(而非 channel.ChannelType)
// 传给 Capturer.ValidateConfig,否则 stripe/wechat adapter 会判定 mode 非法。
func TestCaptureViaRegistry_PassesInteractionModeNotChannelType(t *testing.T) {
	cap := &recordingCapturer{}
	reg := provider.NewRegistry()
	reg.Register("test_provider", "test_channel", cap)

	svc := &PaymentService{paymentProviderRegistry: reg}

	channel := &paymentdomain.PaymentChannel{
		ID:              1,
		ProviderType:    "test_provider",
		ChannelType:     "test_channel",
		InteractionMode: constants.PaymentInteractionQR,
	}
	payment := &paymentdomain.Payment{
		ID:          1,
		ChannelID:   1,
		ProviderRef: "ref-123",
	}

	_, _ = svc.captureViaRegistry(CapturePaymentInput{}, payment, channel)

	if cap.receivedMode != channel.InteractionMode {
		t.Fatalf("ValidateConfig received mode=%q, want %q (channel.InteractionMode)",
			cap.receivedMode, channel.InteractionMode)
	}
	if cap.receivedMode == channel.ChannelType {
		t.Fatalf("regression: ValidateConfig received channel.ChannelType=%q", cap.receivedMode)
	}
}

// Uses only an in-memory database and a local HTTP gateway; no real purchase or payment.
func TestAlipayQuerySettlesThroughExistingPaymentValidation(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, orderNo, amount, status string
		tamper                        bool
		wantErr                       error
		wantStatus                    string
	}{
		{"paid", "DJP-ALIPAY-1", "1160.00", "TRADE_SUCCESS", false, nil, constants.PaymentStatusSuccess},
		{"finished", "DJP-ALIPAY-1", "1160.00", "TRADE_FINISHED", false, nil, constants.PaymentStatusSuccess},
		{"pending", "DJP-ALIPAY-1", "1160.00", "WAIT_BUYER_PAY", false, nil, constants.PaymentStatusPending},
		{"wrong amount", "DJP-ALIPAY-1", "0.01", "TRADE_SUCCESS", false, ErrPaymentAmountMismatch, constants.PaymentStatusPending},
		{"wrong order", "DJP-OTHER", "1160.00", "TRADE_SUCCESS", false, ErrPaymentGatewayResponseInvalid, constants.PaymentStatusPending},
		{"tampered", "DJP-ALIPAY-1", "1160.00", "TRADE_SUCCESS", true, ErrPaymentGatewayResponseInvalid, constants.PaymentStatusPending},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, db := setupPaymentServiceWalletTest(t)
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				if err := r.ParseForm(); err != nil {
					t.Error(err)
					return
				}
				var biz map[string]string
				if json.Unmarshal([]byte(r.Form.Get("biz_content")), &biz) != nil || biz["out_trade_no"] != "DJP-ALIPAY-1" {
					t.Error("query used ProviderRef instead of stored GatewayOrderNo")
				}
				payload, _ := json.Marshal(map[string]string{
					"code": "10000", "out_trade_no": tc.orderNo, "trade_no": "ALIPAY-TRADE-1",
					"total_amount": tc.amount, "trade_status": tc.status, "send_pay_date": "2026-10-12 12:00:00",
				})
				sum := sha256.Sum256(payload)
				signature, signErr := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
				if signErr != nil {
					t.Error(signErr)
					return
				}
				if tc.tamper {
					signature[0] ^= 1
				}
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"alipay_trade_query_response": json.RawMessage(payload), "sign": base64.StdEncoding.EncodeToString(signature),
				})
			}))
			defer server.Close()
			channel := &paymentdomain.PaymentChannel{
				Name: "Alipay test", ProviderType: constants.PaymentProviderOfficial, ChannelType: constants.PaymentChannelTypeAlipay,
				InteractionMode: constants.PaymentInteractionPage,
				ConfigJSON: jsonmap.JSON{
					"app_id": "2026000000000000", "seller_id": "2088000000000000", "gateway_url": server.URL, "sign_type": "RSA2",
					"private_key":       string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})),
					"alipay_public_key": string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})),
					"notify_url":        "https://example.com/api/v1/payments/callback", "return_url": "https://example.com/pay",
				},
			}
			if err := db.Create(channel).Error; err != nil {
				t.Fatal(err)
			}
			order := &orderdomain.Order{OrderNo: "SHOP-ALIPAY-1", Status: constants.OrderStatusPendingPayment,
				Currency: "CNY", TotalAmount: money.FromDecimal(decimal.NewFromInt(1160))}
			if err := db.Create(order).Error; err != nil {
				t.Fatal(err)
			}
			payment := &paymentdomain.Payment{
				OrderID: order.ID, ChannelID: channel.ID, ProviderType: channel.ProviderType, ChannelType: channel.ChannelType,
				InteractionMode: channel.InteractionMode, Amount: order.TotalAmount, Currency: "CNY", Status: constants.PaymentStatusPending,
				GatewayOrderNo: "DJP-ALIPAY-1", ProviderRef: "DIFFERENT-PROVIDER-REF",
			}
			if err := db.Create(payment).Error; err != nil {
				t.Fatal(err)
			}
			_, err = svc.CapturePayment(CapturePaymentInput{PaymentID: payment.ID, Context: context.Background()})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("capture error=%v, want %v", err, tc.wantErr)
			}
			if err := db.First(payment, payment.ID).Error; err != nil {
				t.Fatal(err)
			}
			if payment.Status != tc.wantStatus {
				t.Fatalf("payment status=%s, want %s", payment.Status, tc.wantStatus)
			}
			if err := db.First(order, order.ID).Error; err != nil {
				t.Fatal(err)
			}
			if tc.wantStatus != constants.PaymentStatusSuccess {
				if order.Status != constants.OrderStatusPendingPayment || order.PaidAt != nil {
					t.Fatal("unverified payment changed order")
				}
				return
			}
			if order.Status != constants.OrderStatusPaid || payment.ProviderRef != "ALIPAY-TRADE-1" {
				t.Fatal("verified payment did not settle")
			}
			if payment.PaidAt == nil || !payment.PaidAt.Equal(time.Date(2026, 10, 12, 4, 0, 0, 0, time.UTC)) {
				t.Fatal("Alipay payment timezone was lost")
			}
			paidAt := *order.PaidAt
			if _, err := svc.CapturePayment(CapturePaymentInput{PaymentID: payment.ID}); err != nil {
				t.Fatal(err)
			}
			if requests.Load() != 1 {
				t.Fatal("already paid order was queried again")
			}
			if err := db.First(order, order.ID).Error; err != nil || !order.PaidAt.Equal(paidAt) {
				t.Fatal("duplicate settlement changed order")
			}
		})
	}
}
