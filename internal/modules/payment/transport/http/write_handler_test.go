package paymenthttp

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

func TestRespondPaymentCreateError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		err  error
		code int
		msg  string
	}{
		{
			name: "gateway response invalid",
			err:  ErrPaymentGatewayResponseInvalid,
			code: response.CodeBadRequest,
			msg:  "支付网关响应异常",
		},
		{
			name: "recharge channel not allowed",
			err:  ErrPaymentChannelNotAllowedForRecharge,
			code: response.CodeBadRequest,
			msg:  "钱包充值不支持此支付渠道",
		},
		{
			name: "unknown error",
			err:  errors.New("boom"),
			code: response.CodeInternal,
			msg:  "创建支付失败",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, recorder := newResponseTestContext()

			respondPaymentCreateError(c, tt.err)

			assertErrorResponse(t, recorder, tt.code, tt.msg)
		})
	}
}

func TestRespondPaymentCaptureError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		err  error
		code int
		msg  string
	}{
		{
			name: "amount mismatch",
			err:  ErrPaymentAmountMismatch,
			code: response.CodeBadRequest,
			msg:  "支付金额不匹配",
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
			c, recorder := newResponseTestContext()

			respondPaymentCaptureError(c, tt.err)

			assertErrorResponse(t, recorder, tt.code, tt.msg)
		})
	}
}

func newResponseTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	return c, recorder
}

func assertErrorResponse(t *testing.T, recorder *httptest.ResponseRecorder, wantCode int, wantMsg string) {
	t.Helper()
	if recorder.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var body response.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.StatusCode != wantCode {
		t.Fatalf("status_code = %d, want %d; body=%s", body.StatusCode, wantCode, recorder.Body.String())
	}
	if body.Msg != wantMsg {
		t.Fatalf("msg = %q, want %q; body=%s", body.Msg, wantMsg, recorder.Body.String())
	}
}
