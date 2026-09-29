package domain

import (
	"time"

	"github.com/dujiao-next/internal/shared/jsonmap"
)

// NotificationLog 通知发送日志
// 说明：按渠道与收件人记录每次通知发送结果，供后台通知中心直接追踪成功、失败与错误原因。
type NotificationLog struct {
	ID            uint         `gorm:"primarykey" json:"id"`
	EventType     string       `gorm:"type:varchar(100);index;not null;default:''" json:"event_type"`
	BizType       string       `gorm:"type:varchar(100);index;not null;default:''" json:"biz_type"`
	BizID         uint         `gorm:"index;not null;default:0" json:"biz_id"`
	Channel       string       `gorm:"type:varchar(32);index;not null;default:''" json:"channel"`
	Recipient     string       `gorm:"type:varchar(255);index;not null;default:''" json:"recipient"`
	Locale        string       `gorm:"type:varchar(20);index;not null;default:''" json:"locale"`
	Title         string       `gorm:"type:text;not null" json:"title"`
	Body          string       `gorm:"type:text;not null" json:"body"`
	Status        string       `gorm:"type:varchar(32);index;not null;default:''" json:"status"`
	ErrorMessage  string       `gorm:"type:text" json:"error_message"`
	IsTest        bool         `gorm:"index;not null;default:false" json:"is_test"`
	VariablesJSON jsonmap.JSON `gorm:"type:json" json:"variables"`
	CreatedAt     time.Time    `gorm:"index" json:"created_at"`
}

// TableName 指定表名
func (NotificationLog) TableName() string {
	return "notification_logs"
}
