package domain

import (
	"time"

	"github.com/dujiao-next/internal/shared/money"
)

// GiftCardBatch 礼品卡批次
type GiftCardBatch struct {
	ID        uint         `gorm:"primarykey" json:"id"`                                                  // 主键
	BatchNo   string       `gorm:"type:varchar(48);uniqueIndex;not null" json:"batch_no"`                 // 批次号
	Name      string       `gorm:"type:varchar(120);not null" json:"name"`                                // 批次名称
	Amount    money.Amount `gorm:"type:decimal(20,2);not null" json:"amount"`                             // 面额
	Currency  string       `gorm:"type:varchar(16);not null;default:'CNY'" json:"currency"`               // 币种
	Quantity  int          `gorm:"not null;default:0" json:"quantity"`                                    // 生成数量
	ExpiresAt *time.Time   `gorm:"index" json:"expires_at"`                                               // 过期时间（为空表示永久有效）
	CreatedBy *uint        `gorm:"index" json:"created_by,omitempty"`                                     // 创建管理员ID
	CreatedAt time.Time    `gorm:"index" json:"created_at"`                                               // 创建时间
	UpdatedAt time.Time    `gorm:"index" json:"updated_at"`                                               // 更新时间
	DeletedAt *time.Time   `gorm:"index" json:"-"`                                                        // 软删除时间
	Cards     []GiftCard   `gorm:"foreignKey:BatchID;constraint:OnUpdate:CASCADE" json:"cards,omitempty"` // 批次卡片
}

// TableName 指定表名
func (GiftCardBatch) TableName() string {
	return "gift_card_batches"
}
