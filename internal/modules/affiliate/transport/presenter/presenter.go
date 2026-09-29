package presenter

import (
	"time"

	affiliatedomain "github.com/dujiao-next/internal/modules/affiliate/domain"

	"github.com/dujiao-next/internal/shared/money"
)

// Profile 推广用户资料响应
type Profile struct {
	ID            uint      `json:"id"`
	AffiliateCode string    `json:"code"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

// NewProfile 从 affiliatedomain.Profile 构造响应
func NewProfile(p *affiliatedomain.Profile) Profile {
	return Profile{
		ID:            p.ID,
		AffiliateCode: p.AffiliateCode,
		Status:        p.Status,
		CreatedAt:     p.CreatedAt,
	}
	// 排除：UserID、User、UpdatedAt
}

// Commission 佣金记录响应
type Commission struct {
	ID               uint         `json:"id"`
	CommissionType   string       `json:"commission_type"`
	CommissionAmount money.Amount `json:"commission_amount"`
	Status           string       `json:"status"`
	ConfirmAt        *time.Time   `json:"confirm_at,omitempty"`
	AvailableAt      *time.Time   `json:"available_at,omitempty"`
	CreatedAt        time.Time    `json:"created_at"`
}

// NewCommission 从 affiliatedomain.Commission 构造响应
func NewCommission(c *affiliatedomain.Commission) Commission {
	return Commission{
		ID:               c.ID,
		CommissionType:   c.CommissionType,
		CommissionAmount: c.CommissionAmount,
		Status:           c.Status,
		ConfirmAt:        c.ConfirmAt,
		AvailableAt:      c.AvailableAt,
		CreatedAt:        c.CreatedAt,
	}
	// 排除：AffiliateProfileID、OrderItemID、BaseAmount、RatePercent、
	// WithdrawRequestID、InvalidReason、UpdatedAt、关联 Order/AffiliateProfile/WithdrawRequest
}

// NewCommissionList 批量转换佣金列表
func NewCommissionList(commissions []affiliatedomain.Commission) []Commission {
	result := make([]Commission, 0, len(commissions))
	for i := range commissions {
		result = append(result, NewCommission(&commissions[i]))
	}
	return result
}

// Withdraw 提现记录响应
type Withdraw struct {
	ID           uint         `json:"id"`
	Amount       money.Amount `json:"amount"`
	Channel      string       `json:"channel"`
	Account      string       `json:"account"`
	Status       string       `json:"status"`
	RejectReason string       `json:"reject_reason,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
}

// NewWithdraw 从 affiliatedomain.WithdrawRequest 构造响应
func NewWithdraw(w *affiliatedomain.WithdrawRequest) Withdraw {
	return Withdraw{
		ID:           w.ID,
		Amount:       w.Amount,
		Channel:      w.Channel,
		Account:      w.Account,
		Status:       w.Status,
		RejectReason: w.RejectReason,
		CreatedAt:    w.CreatedAt,
	}
	// 排除：AffiliateProfileID、ProcessedBy、ProcessedAt、UpdatedAt、关联
}

// NewWithdrawList 批量转换提现列表
func NewWithdrawList(withdraws []affiliatedomain.WithdrawRequest) []Withdraw {
	result := make([]Withdraw, 0, len(withdraws))
	for i := range withdraws {
		result = append(result, NewWithdraw(&withdraws[i]))
	}
	return result
}
