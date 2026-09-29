package domain

import (
	"time"

	"github.com/dujiao-next/internal/shared/money"
)

const (
	GiftCardStatusActive   = "active"
	GiftCardStatusRedeemed = "redeemed"
	GiftCardStatusDisabled = "disabled"
)

// GiftCard 礼品卡
type GiftCard struct {
	ID             uint           `gorm:"primarykey" json:"id"`                                           // 主键
	BatchID        *uint          `gorm:"index" json:"batch_id,omitempty"`                                // 批次ID
	Name           string         `gorm:"type:varchar(120);not null" json:"name"`                         // 礼品卡名称
	Code           string         `gorm:"type:varchar(80);uniqueIndex;not null" json:"code"`              // 卡密
	Amount         money.Amount   `gorm:"type:decimal(20,2);not null" json:"amount"`                      // 面额
	Currency       string         `gorm:"type:varchar(16);not null;default:'CNY'" json:"currency"`        // 币种
	Status         string         `gorm:"type:varchar(24);index;not null;default:'active'" json:"status"` // 状态
	ExpiresAt      *time.Time     `gorm:"index" json:"expires_at"`                                        // 过期时间
	RedeemedAt     *time.Time     `gorm:"index" json:"redeemed_at"`                                       // 兑换时间
	RedeemedUserID *uint          `gorm:"index" json:"redeemed_user_id,omitempty"`                        // 兑换用户ID
	WalletTxnID    *uint          `gorm:"index" json:"wallet_txn_id,omitempty"`                           // 钱包流水ID
	CreatedAt      time.Time      `gorm:"index" json:"created_at"`                                        // 创建时间
	UpdatedAt      time.Time      `gorm:"index" json:"updated_at"`                                        // 更新时间
	DeletedAt      *time.Time     `gorm:"index" json:"-"`                                                 // 软删除时间
	Batch          *GiftCardBatch `gorm:"foreignKey:BatchID" json:"batch,omitempty"`                      // 批次信息
}

// TableName 指定表名
func (GiftCard) TableName() string {
	return "gift_cards"
}
