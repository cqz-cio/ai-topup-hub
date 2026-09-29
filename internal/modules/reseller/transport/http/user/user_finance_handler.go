package userhttp

import (
	"strings"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	resellermodule "github.com/dujiao-next/internal/modules/reseller/contract"
	dto "github.com/dujiao-next/internal/modules/reseller/transport/http/presenter"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/shopspring/decimal"

	"github.com/gin-gonic/gin"
)

// UserFinanceQueryService 是用户中心分销财务查询端点所需的最小用例接口。
type UserFinanceQueryService interface {
	GetUserFinanceDashboard(userID uint) (resellermodule.UserFinanceDashboard, error)
	ListUserBalanceAccounts(userID uint, filter resellermodule.UserBalanceAccountListFilter) ([]resellerdomain.BalanceAccount, int64, error)
	ListUserLedgerEntries(userID uint, filter resellermodule.UserLedgerListFilter) ([]resellerdomain.LedgerEntry, int64, error)
	ListUserWithdrawRequests(userID uint, filter resellermodule.UserWithdrawListFilter) ([]resellerdomain.WithdrawRequest, int64, error)
}

// UserWithdrawService 是用户中心提现写操作所需的最小用例接口。
type UserWithdrawService interface {
	ApplyUserWithdraw(userID uint, input resellermodule.WithdrawApplyInput) (*resellerdomain.WithdrawRequest, error)
}

// UserFinanceHandler 处理用户中心分销财务请求。
type UserFinanceHandler struct {
	query    UserFinanceQueryService
	withdraw UserWithdrawService
}

func NewUserFinanceHandler(query UserFinanceQueryService, withdraw UserWithdrawService) *UserFinanceHandler {
	if query == nil {
		panic("reseller user finance handler: query is nil")
	}
	if withdraw == nil {
		panic("reseller user finance handler: withdraw is nil")
	}
	return &UserFinanceHandler{query: query, withdraw: withdraw}
}

type withdrawApplyRequest struct {
	Amount   string `json:"amount" binding:"required"`
	Currency string `json:"currency" binding:"required"`
	Channel  string `json:"channel" binding:"required"`
	Account  string `json:"account" binding:"required"`
}

// GetDashboard 获取当前用户的分销商财务看板。
func (h *UserFinanceHandler) GetDashboard(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	data, err := h.query.GetUserFinanceDashboard(uid)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.Success(c, dto.NewResellerDashboardResp(data.Opened, data.Profile, data.Balances, data.WithdrawEnabled, data.WithdrawDisabledReason))
}

// ListBalanceAccounts 查询当前用户的分销余额账户。
func (h *UserFinanceHandler) ListBalanceAccounts(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	rows, total, err := h.query.ListUserBalanceAccounts(uid, resellermodule.UserBalanceAccountListFilter{
		Page:     page,
		PageSize: pageSize,
		Status:   strings.TrimSpace(c.Query("status")),
	})
	if err != nil {
		respondUserFinanceError(c, err, "error.user_fetch_failed")
		return
	}
	response.SuccessWithPage(c, dto.NewResellerBalanceRespList(rows), response.BuildPagination(page, pageSize, total))
}

// ListLedgerEntries 查询当前用户的分销账务流水。
func (h *UserFinanceHandler) ListLedgerEntries(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	orderID, err := ginutil.ParseQueryUint(c.Query("order_id"), false)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	rows, total, err := h.query.ListUserLedgerEntries(uid, resellermodule.UserLedgerListFilter{
		Page:     page,
		PageSize: pageSize,
		Type:     strings.TrimSpace(c.Query("type")),
		Status:   strings.TrimSpace(c.Query("status")),
		OrderID:  orderID,
	})
	if err != nil {
		respondUserFinanceError(c, err, "error.user_fetch_failed")
		return
	}
	response.SuccessWithPage(c, dto.NewResellerLedgerRespList(rows), response.BuildPagination(page, pageSize, total))
}

// ListWithdraws 查询当前用户的分销提现申请。
func (h *UserFinanceHandler) ListWithdraws(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	rows, total, err := h.query.ListUserWithdrawRequests(uid, resellermodule.UserWithdrawListFilter{
		Page:     page,
		PageSize: pageSize,
		Status:   strings.TrimSpace(c.Query("status")),
	})
	if err != nil {
		respondUserFinanceError(c, err, "error.user_fetch_failed")
		return
	}
	response.SuccessWithPage(c, dto.NewResellerWithdrawRespList(rows), response.BuildPagination(page, pageSize, total))
}

// ApplyWithdraw 提交当前用户的分销提现申请。
func (h *UserFinanceHandler) ApplyWithdraw(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	var req withdrawApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	amount, err := decimal.NewFromString(strings.TrimSpace(req.Amount))
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	row, err := h.withdraw.ApplyUserWithdraw(uid, resellermodule.WithdrawApplyInput{
		Amount:   amount,
		Currency: strings.TrimSpace(req.Currency),
		Channel:  strings.TrimSpace(req.Channel),
		Account:  strings.TrimSpace(req.Account),
	})
	if err != nil {
		respondUserFinanceError(c, err, "error.save_failed")
		return
	}
	response.Success(c, dto.NewResellerWithdrawResp(row))
}
