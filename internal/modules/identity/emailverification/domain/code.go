package emailverificationdomain

import "time"

// Code is a persisted email verification challenge.
type Code struct {
	ID           uint       `gorm:"primarykey" json:"id"`           // 主键
	Email        string     `gorm:"index;not null" json:"email"`    // 邮箱
	UserID       *uint      `gorm:"index" json:"user_id"`           // 关联用户ID
	Purpose      string     `gorm:"index;not null" json:"purpose"`  // 用途（register/reset）
	Code         string     `gorm:"not null" json:"-"`              // 验证码（不返回给前端）
	ExpiresAt    time.Time  `gorm:"index" json:"expires_at"`        // 过期时间
	VerifiedAt   *time.Time `gorm:"index" json:"verified_at"`       // 验证时间
	AttemptCount int        `gorm:"default:0" json:"attempt_count"` // 尝试次数
	SentAt       time.Time  `gorm:"index" json:"sent_at"`           // 发送时间
	CreatedAt    time.Time  `gorm:"index" json:"created_at"`        // 创建时间
	DeletedAt    *time.Time `gorm:"index" json:"-"`                 // 软删除时间
}

// TableName 指定表名
func (Code) TableName() string {
	return "email_verify_codes"
}
