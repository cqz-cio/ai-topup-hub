package userhttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	resellermodule "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

type orderServiceStub struct {
	err error
}

func (s orderServiceStub) ListUserOrders(userID uint, input resellermodule.OrderListInput) ([]resellermodule.OrderListItem, int64, error) {
	return nil, 0, s.err
}

func (s orderServiceStub) GetUserOrderDetail(userID uint, orderNo string) (*resellermodule.OrderDetail, error) {
	return nil, s.err
}

func (s orderServiceStub) StatsUserOrders(userID uint, input resellermodule.OrderListInput) (resellermodule.OrderStats, error) {
	return resellermodule.OrderStats{}, s.err
}

func TestUserOrderHandlerMapsOrderNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewUserOrderHandler(orderServiceStub{err: resellermodule.ErrOrderNotFound})
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/reseller/orders/missing", nil)
	c.Set("user_id", uint(9))
	c.Params = gin.Params{{Key: "order_no", Value: "missing"}}

	h.GetOrderDetail(c)

	var resp struct {
		StatusCode int `json:"status_code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if resp.StatusCode != response.CodeNotFound {
		t.Fatalf("expected not found, body=%s", recorder.Body.String())
	}
}

func TestUserOrderHandlerMapsProfileInactive(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewUserOrderHandler(orderServiceStub{err: resellermodule.ErrProfileInactive})
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/reseller/orders", nil)
	c.Set("user_id", uint(9))

	h.ListOrders(c)

	var resp struct {
		StatusCode int `json:"status_code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if resp.StatusCode != response.CodeBadRequest {
		t.Fatalf("expected bad request, body=%s", recorder.Body.String())
	}
}
