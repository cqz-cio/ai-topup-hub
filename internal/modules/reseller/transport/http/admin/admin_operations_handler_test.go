package adminhttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	reportingdomain "github.com/dujiao-next/internal/modules/reporting/domain"
	resellermodule "github.com/dujiao-next/internal/modules/reseller/application"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

type operationsServiceStub struct {
	overview *resellermodule.OperationsOverviewResponse
	finance  *resellermodule.OperationsFinanceResponse
	err      error
}

func (s operationsServiceStub) GetOverview(ctx context.Context, input reportingdomain.Query) (*resellermodule.OperationsOverviewResponse, error) {
	return s.overview, s.err
}

func (s operationsServiceStub) GetFinance(ctx context.Context, input reportingdomain.Query) (*resellermodule.OperationsFinanceResponse, error) {
	return s.finance, s.err
}

func TestAdminOperationsHandlerOverview(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewAdminOperationsHandler(operationsServiceStub{
		overview: &resellermodule.OperationsOverviewResponse{
			Lifecycle: resellermodule.OperationsLifecycleResponse{ProfilesTotal: 2, ProfilesActive: 1},
			Orders:    resellermodule.OperationsOrdersResponse{OrdersTotal: 3, PaidOrders: 2},
		},
	})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/resellers/operations/overview?range=today&tz=Asia/Shanghai", nil)

	h.GetOverview(c)

	if w.Code != http.StatusOK {
		t.Fatalf("http status want 200 got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		StatusCode int `json:"status_code"`
		Data       struct {
			Lifecycle struct {
				ProfilesTotal int64 `json:"profiles_total"`
			} `json:"lifecycle"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if resp.StatusCode != response.CodeOK || resp.Data.Lifecycle.ProfilesTotal != 2 {
		t.Fatalf("unexpected payload: %+v", resp)
	}
}

func TestAdminOperationsHandlerFinanceMapsInvalidRange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewAdminOperationsHandler(operationsServiceStub{err: reportingdomain.ErrRangeInvalid})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin/resellers/operations/finance?range=today&tz=Asia/Shanghai", nil)

	h.GetFinance(c)

	if w.Code != http.StatusOK {
		t.Fatalf("http status want 200 got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		StatusCode int `json:"status_code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if resp.StatusCode != response.CodeBadRequest {
		t.Fatalf("expected bad request for invalid range, got %+v body=%s", resp, w.Body.String())
	}
}
