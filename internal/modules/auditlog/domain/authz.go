package domain

import (
	"time"

	"github.com/dujiao-next/internal/shared/jsonmap"
)

// AuthzAuditLog 权限策略审计日志
// 说明：用于记录后台权限相关的变更操作，支持按管理员与时间范围检索。
type AuthzAuditLog struct {
	ID               uint         `gorm:"primarykey" json:"id"`
	OperatorAdminID  uint         `gorm:"index;not null" json:"operator_admin_id"`
	OperatorUsername string       `gorm:"type:varchar(100);index;not null;default:''" json:"operator_username"`
	TargetAdminID    *uint        `gorm:"index" json:"target_admin_id,omitempty"`
	TargetUsername   string       `gorm:"type:varchar(100);index;not null;default:''" json:"target_username"`
	Action           string       `gorm:"type:varchar(100);index;not null" json:"action"`
	Role             string       `gorm:"type:varchar(120);index;not null;default:''" json:"role"`
	Object           string       `gorm:"type:varchar(255);index;not null;default:''" json:"object"`
	Method           string       `gorm:"type:varchar(20);index;not null;default:''" json:"method"`
	RequestID        string       `gorm:"type:varchar(64);index;not null;default:''" json:"request_id"`
	DetailJSON       jsonmap.JSON `gorm:"type:json" json:"detail"`
	CreatedAt        time.Time    `gorm:"index" json:"created_at"`
}

// TableName 指定表名
func (AuthzAuditLog) TableName() string {
	return "authz_audit_logs"
}
