package carthttp

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	cartcontract "github.com/dujiao-next/internal/modules/cart/contract"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

func TestRespondCartItemUpdateError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		err  error
		code int
		msg  string
	}{
		{
			name: "invalid fulfillment",
			err:  cartcontract.ErrFulfillmentInvalid,
			code: response.CodeBadRequest,
			msg:  "交付信息不合法",
		},
		{
			name: "manual stock insufficient",
			err:  cartcontract.ErrManualStockInsufficient,
			code: response.CodeBadRequest,
			msg:  "人工库存不足",
		},
		{
			name: "unknown error",
			err:  errors.New("boom"),
			code: response.CodeInternal,
			msg:  "更新订单失败",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/cart/items", nil)

			respondCartItemUpdateError(c, tt.err)

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
