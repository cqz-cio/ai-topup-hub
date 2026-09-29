package domain

import "time"

// UserLoginLog 用户登录日志
// 说明：记录用户登录成功或失败行为，用于后台审计与个人安全中心展示。
type UserLoginLog struct {
	ID          uint      `gorm:"primarykey" json:"id"`                       // 主键
	UserID      uint      `gorm:"index" json:"user_id"`                       // 用户ID（失败时可为0）
	Email       string    `gorm:"index;not null" json:"email"`                // 登录尝试邮箱
	Status      string    `gorm:"index;not null" json:"status"`               // 登录结果（success/failed）
	FailReason  string    `gorm:"index" json:"fail_reason"`                   // 失败原因枚举
	ClientIP    string    `gorm:"type:varchar(64);index" json:"client_ip"`    // 客户端IP
	UserAgent   string    `gorm:"type:text" json:"user_agent"`                // 客户端UA
	LoginSource string    `gorm:"type:varchar(32);index" json:"login_source"` // 登录来源（web）
	RequestID   string    `gorm:"type:varchar(64);index" json:"request_id"`   // 请求追踪ID
	CreatedAt   time.Time `gorm:"index" json:"created_at"`                    // 记录时间
}

// TableName 指定表名
func (UserLoginLog) TableName() string {
	return "user_login_logs"
}
