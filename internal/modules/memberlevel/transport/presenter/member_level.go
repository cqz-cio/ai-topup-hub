package presenter

import "github.com/dujiao-next/internal/shared/jsonmap"

// MemberLevel 是公开会员等级响应。
type MemberLevel struct {
	ID                uint         `json:"id"`
	Name              jsonmap.JSON `json:"name"`
	Slug              string       `json:"slug"`
	Icon              string       `json:"icon"`
	DiscountRate      float64      `json:"discount_rate"`
	RechargeThreshold float64      `json:"recharge_threshold"`
	SpendThreshold    float64      `json:"spend_threshold"`
	IsDefault         bool         `json:"is_default"`
	SortOrder         int          `json:"sort_order"`
}
