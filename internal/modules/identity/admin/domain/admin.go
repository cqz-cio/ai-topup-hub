package admindomain

import "time"

// Admin 管理员表
type Admin struct {
	ID                   uint       `gorm:"primarykey" json:"id"`
	Username             string     `gorm:"uniqueIndex;not null" json:"username"`
	PasswordHash         string     `gorm:"not null" json:"-"`
	TokenVersion         uint64     `gorm:"not null;default:0" json:"-"`
	TokenInvalidBefore   *time.Time `gorm:"index" json:"-"`
	IsSuper              bool       `gorm:"not null;default:false;index" json:"is_super"`
	LastLoginAt          *time.Time `json:"last_login_at"`
	TOTPSecret           string     `gorm:"type:varchar(512);default:''" json:"-"`
	TOTPEnabledAt        *time.Time `gorm:"index" json:"totp_enabled_at,omitempty"`
	TOTPPendingSecret    string     `gorm:"type:varchar(512);default:''" json:"-"`
	TOTPPendingExpiresAt *time.Time `json:"-"`
	RecoveryCodes        string     `gorm:"type:text;default:''" json:"-"`
	CreatedAt            time.Time  `gorm:"index" json:"created_at"`
	DeletedAt            *time.Time `gorm:"index" json:"-"`
}

// TableName 指定表名
func (Admin) TableName() string {
	return "admins"
}
