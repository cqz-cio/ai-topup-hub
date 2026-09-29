package domain

import (
	"time"

	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
)

// MemberLevel 会员等级定义
type MemberLevel struct {
	ID                uint         `gorm:"primarykey" json:"id"`
	NameJSON          jsonmap.JSON `gorm:"type:json;not null" json:"name"`                                  // 多语言名称
	Slug              string       `gorm:"uniqueIndex;not null" json:"slug"`                                // 唯一标识（default/silver/gold/diamond）
	Icon              string       `gorm:"default:''" json:"icon"`                                          // 等级图标（emoji 或图片 URL）
	DiscountRate      money.Amount `gorm:"type:decimal(6,2);not null;default:100" json:"discount_rate"`     // 全局折扣率（100=原价, 90=9折, 80=8折）
	RechargeThreshold money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"recharge_threshold"` // 充值累计升级阈值（0=不按此条件）
	SpendThreshold    money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"spend_threshold"`    // 消费累计升级阈值（0=不按此条件）
	IsDefault         bool         `gorm:"not null;default:false" json:"is_default"`                        // 是否默认等级（仅一个）
	SortOrder         int          `gorm:"not null;default:0" json:"sort_order"`                            // 排序权重（越大等级越高）
	IsActive          bool         `gorm:"not null;default:true" json:"is_active"`                          // 是否启用
	CreatedAt         time.Time    `gorm:"index" json:"created_at"`
	UpdatedAt         time.Time    `gorm:"index" json:"updated_at"`
	DeletedAt         *time.Time   `gorm:"index" json:"-"`
}

func (MemberLevel) TableName() string {
	return "member_levels"
}
