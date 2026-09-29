package domain

import (
	"time"
)

// RelatedAccount 分销商关联账号，用于自买风控。
type RelatedAccount struct {
	ID           uint       `gorm:"primarykey" json:"id"`
	ResellerID   uint       `gorm:"not null;index" json:"reseller_id"`
	UserID       uint       `gorm:"not null;index" json:"user_id"`
	RelationType string     `gorm:"type:varchar(32);not null;index" json:"relation_type"`
	Source       string     `gorm:"type:varchar(64);not null" json:"source"`
	Status       string     `gorm:"type:varchar(32);not null;index;default:'active'" json:"status"`
	Remark       string     `gorm:"type:text" json:"remark,omitempty"`
	CreatedBy    *uint      `gorm:"index" json:"created_by,omitempty"`
	CreatedAt    time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"index" json:"updated_at"`
	DeletedAt    *time.Time `gorm:"index" json:"-"`
}

func (RelatedAccount) TableName() string { return "reseller_related_accounts" }
