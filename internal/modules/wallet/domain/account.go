package domain

import (
	"time"

	"github.com/dujiao-next/internal/shared/money"
)

// Account 用户钱包账户。
type Account struct {
	ID      uint         `gorm:"primarykey" json:"id"`
	UserID  uint         `gorm:"uniqueIndex;not null" json:"user_id"`
	Balance money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"balance"`
	// WithdrawableBalance is a subset of Balance. Historical balances and ordinary
	// credits default to zero here; only settled earnings can increase it.
	WithdrawableBalance money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"withdrawable_balance"`
	FrozenBalance       money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"frozen_balance"`
	EarningsDebt        money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"earnings_debt"`
	CreatedAt           time.Time    `gorm:"index" json:"created_at"`
	UpdatedAt           time.Time    `gorm:"index" json:"updated_at"`
	DeletedAt           *time.Time   `gorm:"index" json:"-"`
}

func (Account) TableName() string {
	return "wallet_accounts"
}
