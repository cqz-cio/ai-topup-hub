package domain

import (
	"github.com/dujiao-next/internal/shared/money"
	"time"
)

// EarningSettlement records the cumulative amount transferred from a source
// ledger. Replaying a settlement cannot create a second credit.
type EarningSettlement struct {
	ID        uint         `gorm:"primarykey" json:"id"`
	Reference string       `gorm:"size:120;not null;uniqueIndex" json:"reference"`
	UserID    uint         `gorm:"not null;index" json:"user_id"`
	Currency  string       `gorm:"size:16;not null" json:"currency"`
	Amount    money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"amount"`
	Revision  uint         `gorm:"not null;default:0" json:"revision"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (EarningSettlement) TableName() string { return "wallet_earning_settlements" }

// OrderFunding preserves the origin of wallet money through payment switches
// and refunds. Online payments never acquire withdrawal eligibility.
type OrderFunding struct {
	OrderID              uint         `gorm:"primarykey"`
	UserID               uint         `gorm:"not null;index"`
	Amount               money.Amount `gorm:"type:decimal(20,2);not null;default:0"`
	WithdrawableAmount   money.Amount `gorm:"type:decimal(20,2);not null;default:0"`
	RefundedWithdrawable money.Amount `gorm:"type:decimal(20,2);not null;default:0"`
	UpdatedAt            time.Time
}

func (OrderFunding) TableName() string { return "wallet_order_funding" }

type WithdrawRequest struct {
	ID              uint         `gorm:"primarykey" json:"id"`
	UserID          uint         `gorm:"not null;index;uniqueIndex:idx_wallet_withdraw_request_key" json:"user_id"`
	RequestKey      string       `gorm:"size:80;not null;uniqueIndex:idx_wallet_withdraw_request_key" json:"-"`
	Amount          money.Amount `gorm:"type:decimal(20,2);not null" json:"amount"`
	Currency        string       `gorm:"size:16;not null" json:"currency"`
	Channel         string       `gorm:"size:20;not null" json:"channel"`
	Account         string       `gorm:"size:200;not null" json:"account"`
	RealName        string       `gorm:"size:80;not null" json:"real_name"`
	Status          string       `gorm:"size:24;not null;index" json:"status"`
	Remark          string       `gorm:"size:500" json:"remark"`
	ApprovedBy      *uint        `json:"approved_by,omitempty"`
	ApprovedAt      *time.Time   `json:"approved_at,omitempty"`
	ProcessedBy     *uint        `json:"processed_by,omitempty"`
	ProcessedAt     *time.Time   `json:"processed_at,omitempty"`
	PayoutReference string       `gorm:"size:120" json:"payout_reference"`
	CreatedAt       time.Time    `gorm:"index" json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
}

func (WithdrawRequest) TableName() string { return "wallet_withdraw_requests" }

const (
	WithdrawPending  = "pending"
	WithdrawApproved = "approved"
	WithdrawRejected = "rejected"
	WithdrawPaid     = "paid"
)
