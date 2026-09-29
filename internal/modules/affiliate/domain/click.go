package domain

import "time"

// Click 推广返利点击记录
type Click struct {
	ID                 uint      `gorm:"primarykey" json:"id"`                                       // 主键
	AffiliateProfileID uint      `gorm:"not null;index" json:"affiliate_profile_id"`                 // 推广用户ID
	VisitorKey         string    `gorm:"type:varchar(128);index" json:"visitor_key"`                 // 访客标识
	LandingPath        string    `gorm:"type:varchar(512)" json:"landing_path"`                      // 落地页面路径
	Referrer           string    `gorm:"type:varchar(1024)" json:"referrer"`                         // 来源地址
	ClientIP           string    `gorm:"type:varchar(64)" json:"client_ip"`                          // 客户端IP
	UserAgent          string    `gorm:"type:varchar(1024)" json:"user_agent"`                       // 客户端UA
	CreatedAt          time.Time `gorm:"index;not null;default:CURRENT_TIMESTAMP" json:"created_at"` // 创建时间

	AffiliateProfile Profile `gorm:"foreignKey:AffiliateProfileID" json:"affiliate_profile,omitempty"` // 推广用户
}

// TableName 指定表名
func (Click) TableName() string {
	return "affiliate_clicks"
}
