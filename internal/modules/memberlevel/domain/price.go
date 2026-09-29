package domain

import (
	"time"

	"github.com/dujiao-next/internal/shared/money"
)

// MemberLevelPrice 商品/SKU 等级定价覆盖
type MemberLevelPrice struct {
	ID            uint         `gorm:"primarykey" json:"id"`
	MemberLevelID uint         `gorm:"uniqueIndex:idx_member_level_price;not null" json:"member_level_id"`                // 关联等级
	ProductID     uint         `gorm:"uniqueIndex:idx_member_level_price;not null" json:"product_id"`                     // 关联商品
	SKUID         uint         `gorm:"column:sku_id;uniqueIndex:idx_member_level_price;not null;default:0" json:"sku_id"` // 0=商品级覆盖，>0=SKU级覆盖
	PriceAmount   money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"price_amount"`                         // 等级特价
	CreatedAt     time.Time    `gorm:"index" json:"created_at"`
	UpdatedAt     time.Time    `gorm:"index" json:"updated_at"`
	DeletedAt     *time.Time   `gorm:"index" json:"-"`
}

func (MemberLevelPrice) TableName() string {
	return "member_level_prices"
}
