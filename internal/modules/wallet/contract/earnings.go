package contract

import (
	"errors"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/shared/money"
)

var (
	ErrWithdrawRestricted   = errors.New("推广或分销账户已被停用或暂停结算，请联系客服核查后再提现")
	ErrWithdrawDisabled     = errors.New("钱包提现暂未开放")
	ErrWithdrawInvalid      = errors.New("提现金额、渠道或收款信息不正确")
	ErrWithdrawInsufficient = errors.New("可提现收益不足；充值和赠送余额不可提现")
	ErrWithdrawDebt         = errors.New("存在待追回收益，请先处理退款冲回后再提现或打款")
	ErrWithdrawState        = errors.New("提现状态已变化，请刷新；须先审核通过，实际打款后才能标记已打款")
	ErrWithdrawNotFound     = errors.New("提现申请不存在")
	ErrWithdrawConflict     = errors.New("重复请求的提现信息不一致，请刷新后重试")
	ErrEarningCurrency      = errors.New("收益币种与人民币钱包不一致，不能自动折算")
	ErrEarningSource        = errors.New("钱包收益来源或资金记录不一致")
	ErrUnifiedWallet        = errors.New("收益已统一到钱包，请在我的钱包申请提现")
)

type EarningsRepository interface {
	CheckWithdrawalAllowed(userID uint) error
	GetEarningSettlement(reference string) (*walletdomain.EarningSettlement, error)
	SaveEarningSettlement(*walletdomain.EarningSettlement) error
	GetOrderFunding(orderID uint) (*walletdomain.OrderFunding, error)
	SaveOrderFunding(*walletdomain.OrderFunding) error
	CreateWithdraw(*walletdomain.WithdrawRequest) error
	GetWithdraw(id uint, lock bool) (*walletdomain.WithdrawRequest, error)
	GetWithdrawByKey(userID uint, key string) (*walletdomain.WithdrawRequest, error)
	UpdateWithdraw(*walletdomain.WithdrawRequest) error
	ListWithdraws(WithdrawListFilter) ([]walletdomain.WithdrawRequest, int64, error)
}

type EarningInput struct {
	UserID    uint
	Reference string
	Currency  string
	Amount    money.Amount // cumulative signed source amount, never a replayable delta
	Type      string
	Remark    string
}
type WithdrawPolicy struct {
	BlockedReason string       `json:"blocked_reason,omitempty"`
	Enabled       bool         `json:"enabled"`
	Minimum       money.Amount `json:"minimum"`
	Channels      []string     `json:"channels"`
	Currency      string       `json:"currency"`
}
type WithdrawPolicyReader interface {
	GetWalletWithdrawPolicy() (WithdrawPolicy, error)
}
type WithdrawInput struct {
	UserID     uint
	Amount     money.Amount `json:"amount"`
	Channel    string       `json:"channel"`
	Account    string       `json:"account"`
	RealName   string       `json:"real_name"`
	RequestKey string       `json:"request_key"`
}
type WithdrawReviewInput struct {
	ID              uint
	AdminID         uint
	Action          string `json:"action"`
	Remark          string `json:"remark"`
	PayoutReference string `json:"payout_reference"`
}
type WithdrawListFilter struct {
	UserID   uint
	Status   string
	Page     int
	PageSize int
}

// RefundFunding is supplied only by the authenticated order refund workflow.
// RootOrderID identifies the actual wallet debit for split orders.
type RefundFunding struct {
	RootOrderID    uint
	RootTotal      money.Amount
	RefundedBefore money.Amount
	RefundedAfter  money.Amount
}
