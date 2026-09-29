package domain

import (
	"strings"
	"time"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
)

// NormalizeCode 归一化外部输入的推广码，并限制为持久化字段允许的长度。
func NormalizeCode(raw string) string {
	code := strings.TrimSpace(raw)
	if code == "" {
		return ""
	}
	if len(code) > 32 {
		return code[:32]
	}
	return code
}

// Profile 推广返利用户档案
type Profile struct {
	ID            uint       `gorm:"primarykey" json:"id"`                              // 主键
	UserID        uint       `gorm:"not null;uniqueIndex" json:"user_id"`               // 用户ID
	AffiliateCode string     `gorm:"type:varchar(32);not null;uniqueIndex" json:"code"` // 联盟短ID
	Status        string     `gorm:"type:varchar(20);not null;index" json:"status"`     // 状态
	CreatedAt     time.Time  `gorm:"index" json:"created_at"`                           // 创建时间
	UpdatedAt     time.Time  `gorm:"index" json:"updated_at"`                           // 更新时间
	DeletedAt     *time.Time `gorm:"index" json:"-"`                                    // 软删除时间

	User userdomain.User `gorm:"foreignKey:UserID" json:"user,omitempty"` // 用户信息
}

// TableName 指定表名
func (Profile) TableName() string {
	return "affiliate_profiles"
}
