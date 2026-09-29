package domain

import (
	"time"

	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/jsonslice"
	"github.com/dujiao-next/internal/shared/money"
)

// PaymentChannel 支付渠道配置
type PaymentChannel struct {
	ID                 uint              `gorm:"primarykey" json:"id"`                                    // 主键
	Name               string            `gorm:"not null" json:"name"`                                    // 渠道名称
	Icon               string            `gorm:"type:varchar(512);default:''" json:"icon"`                // 渠道图标（可选）
	ProviderType       string            `gorm:"not null" json:"provider_type"`                           // 提供方类型（official/epay）
	ChannelType        string            `gorm:"not null" json:"channel_type"`                            // 渠道类型（wechat/alipay/qqpay/paypal）
	InteractionMode    string            `gorm:"not null" json:"interaction_mode"`                        // 交互方式（qr/redirect）
	FeeRate            money.Amount      `gorm:"type:decimal(6,2);not null;default:0" json:"fee_rate"`    // 手续费比例（百分比）
	FixedFee           money.Amount      `gorm:"type:decimal(6,2);not null;default:0" json:"fixed_fee"`   // 固定手续费
	MinAmount          money.Amount      `gorm:"type:decimal(20,2);not null;default:0" json:"min_amount"` // 最小金额限制（0=不限）
	MaxAmount          money.Amount      `gorm:"type:decimal(20,2);not null;default:0" json:"max_amount"` // 最大金额限制（0=不限）
	HideAmountOutRange bool              `gorm:"not null;default:false" json:"hide_amount_out_range"`     // 不在金额区间不显示
	PaymentRoles       jsonslice.Strings `gorm:"type:json" json:"payment_roles"`                          // 付款角色限制
	MemberLevels       jsonslice.Uints   `gorm:"type:json" json:"member_levels"`                          // 会员等级限制
	PaymentTypes       jsonslice.Strings `gorm:"type:json" json:"payment_types"`                          // 付款类型限制
	ConfigJSON         jsonmap.JSON      `gorm:"type:json" json:"config_json"`                            // 渠道配置
	IsActive           bool              `gorm:"index;not null;default:true" json:"is_active"`            // 是否启用
	SortOrder          int               `gorm:"not null;default:0" json:"sort_order"`                    // 排序
	CreatedAt          time.Time         `gorm:"index" json:"created_at"`                                 // 创建时间
	UpdatedAt          time.Time         `gorm:"index" json:"updated_at"`                                 // 更新时间
	DeletedAt          *time.Time        `gorm:"index" json:"-"`                                          // 软删除时间
}

// TableName 指定表名
func (PaymentChannel) TableName() string {
	return "payment_channels"
}
