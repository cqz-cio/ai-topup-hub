package domain

import (
	"time"

	"github.com/dujiao-next/internal/shared/money"
)

// Promotion 活动价/折扣规则
type Promotion struct {
	ID         uint         `gorm:"primarykey" json:"id"`                                    // 主键
	Name       string       `gorm:"not null" json:"name"`                                    // 名称
	ScopeType  string       `gorm:"not null" json:"scope_type"`                              // 适用范围（product）
	ScopeRefID uint         `gorm:"index;not null" json:"scope_ref_id"`                      // 关联商品ID
	Type       string       `gorm:"not null" json:"type"`                                    // 类型（fixed/percent/special_price）
	Value      money.Amount `gorm:"type:decimal(20,2);not null" json:"value"`                // 数值（固定金额/百分比/活动价）
	MinAmount  money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"min_amount"` // 使用门槛
	StartsAt   *time.Time   `gorm:"index" json:"starts_at"`                                  // 生效时间
	EndsAt     *time.Time   `gorm:"index" json:"ends_at"`                                    // 失效时间
	IsActive   bool         `gorm:"not null;default:true" json:"is_active"`                  // 是否启用
	CreatedAt  time.Time    `gorm:"index" json:"created_at"`                                 // 创建时间
	UpdatedAt  time.Time    `gorm:"index" json:"updated_at"`                                 // 更新时间
	DeletedAt  *time.Time   `gorm:"index" json:"-"`                                          // 软删除时间
}

// TableName 指定表名
func (Promotion) TableName() string {
	return "promotions"
}
