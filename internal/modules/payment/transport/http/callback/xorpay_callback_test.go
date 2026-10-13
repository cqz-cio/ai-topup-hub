package paymentcallbackhttp

import (
	"errors"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type xorpayHTTPFixture struct {
	failure  bool
	missing  bool
	provider string
	called   bool
}

func (f *xorpayHTTPFixture) GetByGatewayOrderNo(no string) (*paymentdomain.Payment, error) {
	if f.missing {
		return nil, nil
	}
	return &paymentdomain.Payment{ID: 1, ChannelID: 2, GatewayOrderNo: no}, nil
}
func (*xorpayHTTPFixture) GetLatestByProviderRef(string) (*paymentdomain.Payment, error) {
	return nil, nil
}
func (f *xorpayHTTPFixture) GetByID(uint) (*paymentdomain.PaymentChannel, error) {
	return &paymentdomain.PaymentChannel{ID: 2, ProviderType: f.provider}, nil
}
func (f *xorpayHTTPFixture) HandleSyncCallback(_ *paymentdomain.PaymentChannel, form map[string][]string, _ []byte) (*paymentdomain.Payment, error) {
	f.called = true
	if f.failure {
		return nil, errors.New("verification or transaction failed")
	}
	return &paymentdomain.Payment{ID: 1, Status: "success"}, nil
}
func (*xorpayHTTPFixture) HandleWechatWebhook(WechatWebhookInput) (*paymentdomain.Payment, string, error) {
	return nil, "", errors.New("not supported")
}

func TestXorpayAcknowledgementRequiresCommittedPayment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name             string
		failure, missing bool
		provider         string
		status           int
	}{
		{"committed", false, false, "xorpay", 200},
		{"failed", true, false, "xorpay", 500},
		{"unknown-order", false, true, "xorpay", 404},
		{"wrong-provider", false, false, "epay", 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &xorpayHTTPFixture{failure: tc.failure, missing: tc.missing, provider: tc.provider}
			handler := NewHandler(f, f, f, nil)
			values := url.Values{"aoid": {"ao-1"}, "order_id": {"DJP-1"}, "pay_price": {"12.30"}, "pay_time": {"2026-10-13 00:00:00"}, "sign": {"fixture"}}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/callback", strings.NewReader(values.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req
			handler.PaymentCallback(c)
			if w.Code != tc.status {
				t.Fatalf("expected HTTP %d, got %d", tc.status, w.Code)
			}
			if tc.status == 200 && (!f.called || w.Body.String() != "ok") {
				t.Fatal("acknowledged without callback commit")
			}
			if (tc.missing || tc.provider != "xorpay") && f.called {
				t.Fatal("wrong target reached settlement")
			}
		})
	}
}
