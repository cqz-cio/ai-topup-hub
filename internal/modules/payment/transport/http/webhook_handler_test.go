package paymenthttp

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

func TestStripeWebhookQueryBind(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest("POST", "/api/v1/payments/webhook/stripe?channel_id=12", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	var query StripeWebhookQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		t.Fatalf("bind stripe query failed: %v", err)
	}
	if query.ChannelID != 12 {
		t.Fatalf("expected channel id 12, got %d", query.ChannelID)
	}
}

func TestRespondPaymentCallbackError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		err  error
		code int
		msg  string
	}{
		{
			name: "status invalid",
			err:  ErrPaymentStatusInvalid,
			code: response.CodeBadRequest,
			msg:  "支付状态不合法",
		},
		{
			name: "gateway response invalid",
			err:  ErrPaymentGatewayResponseInvalid,
			code: response.CodeBadRequest,
			msg:  "支付网关响应异常",
		},
		{
			name: "unknown error",
			err:  errors.New("boom"),
			code: response.CodeInternal,
			msg:  "支付回调处理失败",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

			respondPaymentCallbackError(c, tt.err)

			if recorder.Code != http.StatusOK {
				t.Fatalf("HTTP status = %d, want %d", recorder.Code, http.StatusOK)
			}
			var body response.Response
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.StatusCode != tt.code {
				t.Fatalf("status_code = %d, want %d; body=%s", body.StatusCode, tt.code, recorder.Body.String())
			}
			if body.Msg != tt.msg {
				t.Fatalf("msg = %q, want %q; body=%s", body.Msg, tt.msg, recorder.Body.String())
			}
		})
	}
}

func TestReadWebhookBodyRejectsOversizedPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", maxWebhookBodyBytes+1)))

	if _, err := readWebhookBody(c); err == nil {
		t.Fatalf("oversized webhook body must be rejected")
	}
}
