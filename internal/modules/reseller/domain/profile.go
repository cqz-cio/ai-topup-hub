package domain

import (
	"time"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	"github.com/dujiao-next/internal/shared/money"
)

// Profile 分销商资料。
type Profile struct {
	ID                   uint         `gorm:"primarykey" json:"id"`
	UserID               uint         `gorm:"not null;uniqueIndex" json:"user_id"`
	Status               string       `gorm:"type:varchar(32);index;not null;default:'pending_review'" json:"status"`
	ApplyReason          string       `gorm:"type:text" json:"apply_reason,omitempty"`
	RejectReason         string       `gorm:"type:text" json:"reject_reason,omitempty"`
	DefaultMarkupPercent money.Amount `gorm:"type:decimal(10,2);not null;default:0" json:"default_markup_percent"`
	MaxMarkupPercent     money.Amount `gorm:"type:decimal(10,2);not null;default:0" json:"max_markup_percent"`
	SettlementStatus     string       `gorm:"type:varchar(32);index;not null;default:'normal'" json:"settlement_status"`
	ReviewedBy           *uint        `gorm:"index" json:"reviewed_by,omitempty"`
	ReviewedAt           *time.Time   `gorm:"index" json:"reviewed_at,omitempty"`
	CreatedAt            time.Time    `gorm:"index" json:"created_at"`
	UpdatedAt            time.Time    `gorm:"index" json:"updated_at"`
	DeletedAt            *time.Time   `gorm:"index" json:"-"`

	User *userdomain.User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (Profile) TableName() string { return "reseller_profiles" }
