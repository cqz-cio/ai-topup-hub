package domain

import (
	"strings"
	"time"

	"github.com/dujiao-next/internal/shared/jsonmap"
)

// PayloadMaxPreviewLines 交付内容截断阈值（API 响应）
const PayloadMaxPreviewLines = 100

// PayloadMaxEmailLines 邮件内嵌交付内容的最大行数，超过则转为附件
const PayloadMaxEmailLines = 20

// ShouldAttachFulfillmentPayload 判断交付内容是否应以邮件附件形式发送
func ShouldAttachFulfillmentPayload(payload string) bool {
	if payload == "" {
		return false
	}
	return len(strings.Split(payload, "\n")) > PayloadMaxEmailLines
}

// Fulfillment 交付记录表
type Fulfillment struct {
	ID               uint         `gorm:"primarykey" json:"id"`                 // 主键
	OrderID          uint         `gorm:"uniqueIndex;not null" json:"order_id"` // 订单ID
	Type             string       `gorm:"not null" json:"type"`                 // 交付类型（auto/manual）
	Status           string       `gorm:"not null" json:"status"`               // 交付状态（pending/delivered）
	Payload          string       `gorm:"type:text" json:"payload"`             // 交付内容
	PayloadLineCount int          `gorm:"-" json:"payload_line_count"`          // 交付内容总行数（非持久化，API 返回时填充）
	LogisticsJSON    jsonmap.JSON `gorm:"type:json" json:"delivery_data"`       // 结构化交付信息
	DeliveredBy      *uint        `gorm:"index" json:"delivered_by,omitempty"`  // 交付管理员ID
	DeliveredAt      *time.Time   `gorm:"index" json:"delivered_at,omitempty"`  // 交付时间
	CreatedAt        time.Time    `gorm:"index" json:"created_at"`              // 创建时间
	UpdatedAt        time.Time    `gorm:"index" json:"updated_at"`              // 更新时间
	DeletedAt        *time.Time   `gorm:"index" json:"-"`                       // 软删除时间
}

// TruncatePayload 计算 payload 行数并截断到 maxLines 行，用于 API 响应防止前端渲染崩溃。
func (f *Fulfillment) TruncatePayload(maxLines int) {
	if f == nil || f.Payload == "" {
		return
	}
	lines := strings.Split(f.Payload, "\n")
	f.PayloadLineCount = len(lines)
	if len(lines) > maxLines {
		f.Payload = strings.Join(lines[:maxLines], "\n")
	}
}

// TableName 指定表名
func (Fulfillment) TableName() string {
	return "fulfillments"
}
