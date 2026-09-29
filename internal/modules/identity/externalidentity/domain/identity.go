package externalidentitydomain

import "time"

// UserOAuthIdentity 用户第三方身份映射
// 说明：用于保存第三方账号与站内用户的绑定关系。
type Identity struct {
	ID             uint       `gorm:"primarykey" json:"id"`                                                              // 主键
	UserID         uint       `gorm:"index;not null" json:"user_id"`                                                     // 绑定用户ID
	Provider       string     `gorm:"type:varchar(32);index:idx_provider_user,unique;not null" json:"provider"`          // 提供方
	ProviderUserID string     `gorm:"type:varchar(255);index:idx_provider_user,unique;not null" json:"provider_user_id"` // 提供方用户ID
	Username       string     `gorm:"type:varchar(128)" json:"username"`                                                 // 提供方用户名
	AvatarURL      string     `gorm:"type:text" json:"avatar_url"`                                                       // 头像地址
	AuthAt         *time.Time `json:"auth_at"`                                                                           // 最近认证时间
	CreatedAt      time.Time  `gorm:"index" json:"created_at"`                                                           // 创建时间
	UpdatedAt      time.Time  `gorm:"index" json:"updated_at"`                                                           // 更新时间
}

// TableName 指定表名
func (Identity) TableName() string {
	return "user_oauth_identities"
}
