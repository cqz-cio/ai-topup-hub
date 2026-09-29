package domain

import "time"

// AdminLoginLog 后台管理员登录与 2FA 操作审计日志
type AdminLoginLog struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	AdminID    uint      `gorm:"index" json:"admin_id"`                     // 失败时若用户名不存在为 0
	Username   string    `gorm:"index;not null;default:''" json:"username"` // 登录尝试用户名
	EventType  string    `gorm:"type:varchar(40);index;not null;default:''" json:"event_type"`
	Status     string    `gorm:"index;not null" json:"status"` // success / failed
	FailReason string    `gorm:"index;not null;default:''" json:"fail_reason"`
	ClientIP   string    `gorm:"type:varchar(64);index" json:"client_ip"`
	UserAgent  string    `gorm:"type:text" json:"user_agent"`
	RequestID  string    `gorm:"type:varchar(64);index" json:"request_id"`
	OperatorID *uint     `gorm:"index" json:"operator_id,omitempty"` // 仅 2fa_reset_by_admin 场景，记录超管 ID
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}

// TableName 指定表名
func (AdminLoginLog) TableName() string {
	return "admin_login_logs"
}
