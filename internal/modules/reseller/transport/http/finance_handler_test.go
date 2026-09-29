package resellerhttp_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"

	resellermodule "github.com/dujiao-next/internal/modules/reseller/contract"
	adminhttp "github.com/dujiao-next/internal/modules/reseller/transport/http/admin"
	userhttp "github.com/dujiao-next/internal/modules/reseller/transport/http/user"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

type financeServiceStub struct {
	dashboard resellermodule.UserFinanceDashboard
	err       error
}

func (s financeServiceStub) GetUserFinanceDashboard(userID uint) (resellermodule.UserFinanceDashboard, error) {
	return s.dashboard, s.err
}

func (s financeServiceStub) ListUserBalanceAccounts(userID uint, filter resellermodule.UserBalanceAccountListFilter) ([]resellerdomain.BalanceAccount, int64, error) {
	return nil, 0, s.err
}

func (s financeServiceStub) ListUserLedgerEntries(userID uint, filter resellermodule.UserLedgerListFilter) ([]resellerdomain.LedgerEntry, int64, error) {
	return nil, 0, s.err
}

func (s financeServiceStub) ListUserWithdrawRequests(userID uint, filter resellermodule.UserWithdrawListFilter) ([]resellerdomain.WithdrawRequest, int64, error) {
	return nil, 0, s.err
}

func (s financeServiceStub) ApplyUserWithdraw(userID uint, input resellermodule.WithdrawApplyInput) (*resellerdomain.WithdrawRequest, error) {
	return nil, s.err
}

func (s financeServiceStub) ListAdminLedgerEntries(filter resellermodule.AdminLedgerListFilter) ([]resellerdomain.LedgerEntry, int64, error) {
	return nil, 0, s.err
}

func (s financeServiceStub) ListAdminBalanceAccounts(filter resellermodule.AdminBalanceAccountListFilter) ([]resellerdomain.BalanceAccount, int64, error) {
	return nil, 0, s.err
}

func (s financeServiceStub) ListAdminWithdrawRequests(filter resellermodule.AdminWithdrawListFilter) ([]resellerdomain.WithdrawRequest, int64, error) {
	return nil, 0, s.err
}

func (s financeServiceStub) ReviewWithdraw(adminID, withdrawID uint, action, reason string) (*resellerdomain.WithdrawRequest, error) {
	return nil, s.err
}

func TestUserFinanceHandlerMapsWithdrawError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := financeServiceStub{err: resellermodule.ErrWithdrawInsufficient}
	h := userhttp.NewUserFinanceHandler(stub, stub)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/reseller/withdraws", bytes.NewReader([]byte(`{"amount":"10","currency":"USD","channel":"usdt","account":"T"}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("user_id", uint(9))

	h.ApplyWithdraw(c)

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

func TestAdminFinanceHandlerMapsWithdrawStatusInvalid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := financeServiceStub{err: resellermodule.ErrWithdrawStatusInvalid}
	h := adminhttp.NewAdminFinanceHandler(stub, stub)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/admin/resellers/withdraws/1/pay", nil)
	c.Set("admin_id", uint(1))
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	h.PayWithdraw(c)

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

func TestAdminFinanceHandlerMapsNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := financeServiceStub{err: productcontract.ErrNotFound}
	h := adminhttp.NewAdminFinanceHandler(stub, stub)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/admin/resellers/withdraws/1/reject", bytes.NewReader([]byte(`{"reason":"x"}`)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("admin_id", uint(1))
	c.Params = gin.Params{{Key: "id", Value: "1"}}

	h.RejectWithdraw(c)

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
