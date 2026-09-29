package application

import (
	"time"

	"github.com/dujiao-next/internal/shared/money"
)

// GenerateInput 生成礼品卡输入。
type GenerateInput struct {
	Name      string
	Quantity  int
	Amount    money.Amount
	ExpiresAt *time.Time
	CreatedBy *uint
}

// ListInput 礼品卡列表输入。
type ListInput struct {
	Code           string
	Status         string
	BatchNo        string
	RedeemedUserID uint
	CreatedFrom    *time.Time
	CreatedTo      *time.Time
	RedeemedFrom   *time.Time
	RedeemedTo     *time.Time
	ExpiresFrom    *time.Time
	ExpiresTo      *time.Time
	Page           int
	PageSize       int
}

// UpdateInput 礼品卡更新输入。
type UpdateInput struct {
	Name           *string
	Status         *string
	ExpiresAt      *time.Time
	ClearExpiresAt bool
}

// RedeemInput 礼品卡兑换输入。
type RedeemInput struct {
	UserID uint
	Code   string
}
