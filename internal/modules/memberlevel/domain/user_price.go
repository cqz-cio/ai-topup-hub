package domain

import (
	"time"

	"github.com/dujiao-next/internal/shared/money"
)

// UserPrice fixes a customer's unit price. SKU 0 applies to every SKU of a product.
type UserPrice struct {
	ID          uint         `gorm:"primarykey" json:"id"`
	UserID      uint         `gorm:"uniqueIndex:idx_user_product_price;not null" json:"user_id"`
	ProductID   uint         `gorm:"uniqueIndex:idx_user_product_price;not null" json:"product_id"`
	SKUID       uint         `gorm:"column:sku_id;uniqueIndex:idx_user_product_price;not null;default:0" json:"sku_id"`
	PriceAmount money.Amount `gorm:"type:decimal(20,2);not null" json:"price_amount"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

func (UserPrice) TableName() string { return "user_product_prices" }
