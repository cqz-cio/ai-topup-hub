package domain

import (
	"time"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	"github.com/dujiao-next/internal/shared/money"
)

// ProductSetting 分销商商品配置。
type ProductSetting struct {
	ID                uint         `gorm:"primarykey" json:"id"`
	ResellerID        uint         `gorm:"not null;index;uniqueIndex:idx_reseller_product_setting_scope,priority:1,where:deleted_at IS NULL" json:"reseller_id"`
	ProductID         uint         `gorm:"not null;index;uniqueIndex:idx_reseller_product_setting_scope,priority:2,where:deleted_at IS NULL" json:"product_id"`
	SKUID             uint         `gorm:"column:sku_id;not null;default:0;index;uniqueIndex:idx_reseller_product_setting_scope,priority:3,where:deleted_at IS NULL" json:"sku_id"`
	IsListed          bool         `gorm:"not null;default:true" json:"is_listed"`
	PricingMode       string       `gorm:"type:varchar(32);not null;default:'inherit'" json:"pricing_mode"`
	MarkupPercent     money.Amount `gorm:"type:decimal(10,2);not null;default:0" json:"markup_percent"`
	FixedMarkupAmount money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"fixed_markup_amount"`
	FixedPriceAmount  money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"fixed_price_amount"`
	SortOrder         int          `gorm:"not null;default:0;index" json:"sort_order"`
	CreatedAt         time.Time    `gorm:"index" json:"created_at"`
	UpdatedAt         time.Time    `gorm:"index" json:"updated_at"`
	DeletedAt         *time.Time   `gorm:"index" json:"-"`

	Product *productdomain.Product `gorm:"foreignKey:ProductID" json:"product,omitempty"`
	Profile *Profile               `gorm:"foreignKey:ResellerID" json:"profile,omitempty"`
}

func (ProductSetting) TableName() string { return "reseller_product_settings" }
