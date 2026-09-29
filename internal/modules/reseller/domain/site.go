package domain

import (
	"time"

	"github.com/dujiao-next/internal/shared/jsonmap"
)

// Domain 分销商域名绑定。
type Domain struct {
	ID                 uint       `gorm:"primarykey" json:"id"`
	ResellerID         uint       `gorm:"not null;index" json:"reseller_id"`
	Domain             string     `gorm:"type:varchar(255);not null;index" json:"domain"`
	Type               string     `gorm:"type:varchar(24);not null" json:"type"`
	VerificationToken  string     `gorm:"type:varchar(128)" json:"verification_token,omitempty"`
	VerificationStatus string     `gorm:"type:varchar(24);index;not null;default:'pending'" json:"verification_status"`
	Status             string     `gorm:"type:varchar(24);index;not null;default:'pending_review'" json:"status"`
	IsPrimary          bool       `gorm:"not null;default:false" json:"is_primary"`
	VerifiedAt         *time.Time `gorm:"index" json:"verified_at,omitempty"`
	CreatedAt          time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt          time.Time  `gorm:"index" json:"updated_at"`
	DeletedAt          *time.Time `gorm:"index" json:"-"`

	Profile *Profile `gorm:"foreignKey:ResellerID" json:"profile,omitempty"`
}

func (Domain) TableName() string { return "reseller_domains" }

// SiteConfig 分销站点白标配置。
type SiteConfig struct {
	ID               uint         `gorm:"primarykey" json:"id"`
	ResellerID       uint         `gorm:"not null;index" json:"reseller_id"`
	SiteName         string       `gorm:"type:varchar(120)" json:"site_name"`
	Logo             string       `gorm:"type:varchar(500)" json:"logo"`
	Favicon          string       `gorm:"type:varchar(500)" json:"favicon"`
	AnnouncementJSON jsonmap.JSON `gorm:"type:json" json:"announcement_json"`
	SupportJSON      jsonmap.JSON `gorm:"type:json" json:"support_json"`
	SEOJSON          jsonmap.JSON `gorm:"type:json" json:"seo_json"`
	FooterLinksJSON  jsonmap.JSON `gorm:"type:json" json:"footer_links_json"`
	NavConfigJSON    jsonmap.JSON `gorm:"type:json" json:"nav_config_json"`
	ThemeJSON        jsonmap.JSON `gorm:"type:json" json:"theme_json"`
	CreatedAt        time.Time    `gorm:"index" json:"created_at"`
	UpdatedAt        time.Time    `gorm:"index" json:"updated_at"`
	DeletedAt        *time.Time   `gorm:"index" json:"-"`

	Profile *Profile `gorm:"foreignKey:ResellerID;references:ID" json:"profile,omitempty"`
}

func (SiteConfig) TableName() string { return "reseller_site_configs" }
