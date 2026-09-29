package domain

import (
	"time"

	admindomain "github.com/dujiao-next/internal/modules/identity/admin/domain"
	"github.com/dujiao-next/internal/shared/money"
)

// WithdrawRequest 推广返利提现申请
type WithdrawRequest struct {
	ID                 uint         `gorm:"primarykey" json:"id"`                                // 主键
	AffiliateProfileID uint         `gorm:"not null;index" json:"affiliate_profile_id"`          // 推广用户ID
	Amount             money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"amount"` // 申请金额
	Channel            string       `gorm:"type:varchar(50);not null" json:"channel"`            // 提现渠道
	Account            string       `gorm:"type:varchar(255);not null" json:"account"`           // 提现账号
	Status             string       `gorm:"type:varchar(32);not null;index" json:"status"`       // 提现状态
	RejectReason       string       `gorm:"type:varchar(255)" json:"reject_reason"`              // 拒绝原因
	ProcessedBy        *uint        `gorm:"index" json:"processed_by,omitempty"`                 // 审核管理员ID
	ProcessedAt        *time.Time   `gorm:"index" json:"processed_at,omitempty"`                 // 审核时间
	CreatedAt          time.Time    `gorm:"index" json:"created_at"`                             // 创建时间
	UpdatedAt          time.Time    `gorm:"index" json:"updated_at"`                             // 更新时间
	DeletedAt          *time.Time   `gorm:"index" json:"-"`                                      // 软删除时间

	AffiliateProfile Profile            `gorm:"foreignKey:AffiliateProfileID" json:"affiliate_profile,omitempty"` // 推广用户
	Processor        *admindomain.Admin `gorm:"foreignKey:ProcessedBy" json:"processor,omitempty"`                // 审核管理员
}

// TableName 指定表名
func (WithdrawRequest) TableName() string {
	return "affiliate_withdraw_requests"
}
