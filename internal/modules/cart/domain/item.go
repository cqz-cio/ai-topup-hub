package domain

import (
	"time"
)

// Item 购物车项。
type Item struct {
	ID              uint       `gorm:"primarykey" json:"id"`                                                                 // 主键
	UserID          uint       `gorm:"not null;uniqueIndex:idx_cart_user_product_sku" json:"user_id"`                        // 用户ID
	ProductID       uint       `gorm:"not null;uniqueIndex:idx_cart_user_product_sku" json:"product_id"`                     // 商品ID
	SKUID           uint       `gorm:"column:sku_id;not null;default:0;uniqueIndex:idx_cart_user_product_sku" json:"sku_id"` // SKU ID
	Quantity        int        `gorm:"not null" json:"quantity"`                                                             // 数量
	FulfillmentType string     `gorm:"type:varchar(20);not null;default:'manual'" json:"fulfillment_type"`                   // 交付类型
	CreatedAt       time.Time  `gorm:"index" json:"created_at"`                                                              // 创建时间
	UpdatedAt       time.Time  `gorm:"index" json:"updated_at"`                                                              // 更新时间
	DeletedAt       *time.Time `gorm:"index" json:"-"`                                                                       // 软删除时间
}

// TableName 指定表名
func (Item) TableName() string {
	return "cart_items"
}
