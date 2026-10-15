package wallethttp

import (
	"encoding/json"
	"errors"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
	"regexp"
	"strconv"
	"strings"

	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	walletpresenter "github.com/dujiao-next/internal/modules/wallet/transport/presenter"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/gin-gonic/gin"
)

type WithdrawalService interface {
	GetWithdrawPolicy() (walletcontract.WithdrawPolicy, error)
	GetUserWithdrawPolicy(uint) (walletcontract.WithdrawPolicy, error)
	ListWithdraws(walletcontract.WithdrawListFilter) ([]walletdomain.WithdrawRequest, int64, error)
	ApplyWithdraw(walletcontract.WithdrawInput) (*walletdomain.WithdrawRequest, error)
	ReviewWithdraw(walletcontract.WithdrawReviewInput) (*walletdomain.WithdrawRequest, error)
	GetAccount(uint) (*walletdomain.Account, error)
}
type WithdrawalHandler struct{ service WithdrawalService }

var withdrawalAmountPattern = regexp.MustCompile(`^[0-9]{1,14}(\.[0-9]{1,2})?$`)

func NewWithdrawalHandler(service WithdrawalService) *WithdrawalHandler {
	return &WithdrawalHandler{service: service}
}

func withdrawError(c *gin.Context, err error) {
	if errors.Is(err, walletcontract.ErrWithdrawRestricted) {
		response.Error(c, response.CodeBadRequest, err.Error())
		return
	}
	for _, known := range []error{walletcontract.ErrWithdrawDisabled, walletcontract.ErrWithdrawInvalid, walletcontract.ErrWithdrawInsufficient, walletcontract.ErrWithdrawDebt, walletcontract.ErrWithdrawState, walletcontract.ErrWithdrawNotFound, walletcontract.ErrWithdrawConflict, walletcontract.ErrEarningCurrency} {
		if errors.Is(err, known) {
			response.Error(c, response.CodeBadRequest, known.Error())
			return
		}
	}
	ginutil.RespondError(c, response.CodeInternal, "error.internal", err)
}
func (h *WithdrawalHandler) Policy(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	policy, err := h.service.GetUserWithdrawPolicy(uid)
	if err != nil {
		withdrawError(c, err)
		return
	}
	response.Success(c, policy)
}
func (h *WithdrawalHandler) UserList(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	page, size := ginutil.ParsePagination(c)
	rows, total, err := h.service.ListWithdraws(walletcontract.WithdrawListFilter{UserID: uid, Page: page, PageSize: size})
	if err != nil {
		withdrawError(c, err)
		return
	}
	if rows == nil {
		rows = []walletdomain.WithdrawRequest{}
	}
	response.SuccessWithPage(c, rows, response.BuildPagination(page, size, total))
}
func (h *WithdrawalHandler) Apply(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	var body struct {
		Amount     json.RawMessage `json:"amount"`
		Channel    string          `json:"channel"`
		Account    string          `json:"account"`
		RealName   string          `json:"real_name"`
		RequestKey string          `json:"request_key"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	raw := strings.TrimSpace(string(body.Amount))
	if strings.HasPrefix(raw, "\"") {
		if err := json.Unmarshal(body.Amount, &raw); err != nil {
			withdrawError(c, walletcontract.ErrWithdrawInvalid)
			return
		}
	}
	if len(raw) > 17 || !withdrawalAmountPattern.MatchString(raw) {
		withdrawError(c, walletcontract.ErrWithdrawInvalid)
		return
	}
	amount, err := decimal.NewFromString(raw)
	if err != nil || !amount.Equal(amount.Round(2)) || amount.Exponent() < -2 || amount.Exponent() > 0 {
		withdrawError(c, walletcontract.ErrWithdrawInvalid)
		return
	}
	input := walletcontract.WithdrawInput{UserID: uid, Amount: money.FromDecimal(amount), Channel: body.Channel, Account: body.Account, RealName: body.RealName, RequestKey: body.RequestKey}
	result, err := h.service.ApplyWithdraw(input)
	if err != nil {
		withdrawError(c, err)
		return
	}
	response.Success(c, result)
}
func (h *WithdrawalHandler) AdminList(c *gin.Context) {
	if _, ok := ginutil.GetAdminID(c); !ok {
		return
	}
	page, size := ginutil.ParsePagination(c)
	uid, _ := strconv.ParseUint(c.Query("user_id"), 10, 32)
	rows, total, err := h.service.ListWithdraws(walletcontract.WithdrawListFilter{UserID: uint(uid), Status: c.Query("status"), Page: page, PageSize: size})
	if err != nil {
		withdrawError(c, err)
		return
	}
	type item struct {
		walletdomain.WithdrawRequest
		Wallet walletpresenter.WalletAccountResp `json:"wallet"`
	}
	result := make([]item, 0, len(rows))
	for _, row := range rows {
		account, err := h.service.GetAccount(row.UserID)
		if err != nil {
			withdrawError(c, err)
			return
		}
		result = append(result, item{row, walletpresenter.NewWalletAccountResp(account)})
	}
	response.SuccessWithPage(c, result, response.BuildPagination(page, size, total))
}
func (h *WithdrawalHandler) Review(c *gin.Context) {
	aid, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		withdrawError(c, walletcontract.ErrWithdrawInvalid)
		return
	}
	var input walletcontract.WithdrawReviewInput
	if err := c.ShouldBindJSON(&input); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	input.AdminID, input.ID = aid, uint(id)
	result, err := h.service.ReviewWithdraw(input)
	if err != nil {
		withdrawError(c, err)
		return
	}
	response.Success(c, result)
}
