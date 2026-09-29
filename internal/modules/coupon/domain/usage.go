package domain

import (
	"time"

	"github.com/dujiao-next/internal/shared/money"
)

// CouponUsage 优惠券使用记录
type CouponUsage struct {
	ID             uint         `gorm:"primarykey" json:"id"`                                         // 主键
	CouponID       uint         `gorm:"index;not null" json:"coupon_id"`                              // 优惠券ID
	UserID         uint         `gorm:"index;not null" json:"user_id"`                                // 用户ID
	OrderID        uint         `gorm:"index;not null" json:"order_id"`                               // 订单ID
	DiscountAmount money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"discount_amount"` // 优惠金额
	CreatedAt      time.Time    `gorm:"index" json:"created_at"`                                      // 创建时间
	DeletedAt      *time.Time   `gorm:"index" json:"-"`                                               // 软删除时间
}

// TableName 指定表名
func (CouponUsage) TableName() string {
	return "coupon_usages"
}
