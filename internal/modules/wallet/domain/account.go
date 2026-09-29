package domain

import (
	"time"

	"github.com/dujiao-next/internal/shared/money"
)

// Account 用户钱包账户。
type Account struct {
	ID        uint         `gorm:"primarykey" json:"id"`
	UserID    uint         `gorm:"uniqueIndex;not null" json:"user_id"`
	Balance   money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"balance"`
	CreatedAt time.Time    `gorm:"index" json:"created_at"`
	UpdatedAt time.Time    `gorm:"index" json:"updated_at"`
	DeletedAt *time.Time   `gorm:"index" json:"-"`
}

func (Account) TableName() string {
	return "wallet_accounts"
}
