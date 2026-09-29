package contract

import (
	"time"

	"github.com/dujiao-next/internal/modules/auditlog/domain"
)

type UserLoginFilter struct {
	Page        int
	PageSize    int
	UserID      uint
	Email       string
	Status      string
	FailReason  string
	ClientIP    string
	CreatedFrom *time.Time
	CreatedTo   *time.Time
}

type AuthzFilter struct {
	Page            int
	PageSize        int
	OperatorAdminID uint
	TargetAdminID   uint
	Action          string
	Role            string
	Object          string
	Method          string
	CreatedFrom     *time.Time
	CreatedTo       *time.Time
}

type AdminLoginFilter struct {
	Page      int
	PageSize  int
	AdminID   *uint
	Username  string
	EventType string
	Status    string
}

type UserLoginRepository interface {
	Create(log *domain.UserLoginLog) error
	ListAdmin(filter UserLoginFilter) ([]domain.UserLoginLog, int64, error)
	ListByUser(userID uint, page, pageSize int) ([]domain.UserLoginLog, int64, error)
}

type AuthzRepository interface {
	Create(log *domain.AuthzAuditLog) error
	ListAdmin(filter AuthzFilter) ([]domain.AuthzAuditLog, int64, error)
}

type AdminLoginRepository interface {
	Create(log *domain.AdminLoginLog) error
	List(filter AdminLoginFilter) ([]domain.AdminLoginLog, int64, error)
}
