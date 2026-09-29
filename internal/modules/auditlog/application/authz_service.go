package application

import (
	"strings"
	"time"

	"github.com/dujiao-next/internal/modules/auditlog/contract"
	"github.com/dujiao-next/internal/modules/auditlog/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

// AuthzRecord 权限审计记录输入
type AuthzRecord struct {
	OperatorAdminID  uint
	OperatorUsername string
	TargetAdminID    *uint
	TargetUsername   string
	Action           string
	Role             string
	Object           string
	Method           string
	RequestID        string
	Detail           jsonmap.JSON
}

// AuthzService 权限审计服务
type AuthzService struct {
	repo contract.AuthzRepository
}

// NewAuthzService 创建权限审计服务
func NewAuthzService(repo contract.AuthzRepository) *AuthzService {
	return &AuthzService{repo: repo}
}

// Record 记录权限审计日志
func (s *AuthzService) Record(input AuthzRecord) error {
	if s == nil || s.repo == nil {
		return nil
	}
	if input.OperatorAdminID == 0 {
		return nil
	}
	if strings.TrimSpace(input.Action) == "" {
		return nil
	}

	item := &domain.AuthzAuditLog{
		OperatorAdminID:  input.OperatorAdminID,
		OperatorUsername: strings.TrimSpace(input.OperatorUsername),
		TargetAdminID:    input.TargetAdminID,
		TargetUsername:   strings.TrimSpace(input.TargetUsername),
		Action:           strings.TrimSpace(input.Action),
		Role:             strings.TrimSpace(input.Role),
		Object:           strings.TrimSpace(input.Object),
		Method:           strings.ToUpper(strings.TrimSpace(input.Method)),
		RequestID:        strings.TrimSpace(input.RequestID),
		DetailJSON:       input.Detail,
		CreatedAt:        time.Now(),
	}
	return s.repo.Create(item)
}

// ListForAdmin 管理端查询权限审计日志
func (s *AuthzService) ListForAdmin(filter contract.AuthzFilter) ([]domain.AuthzAuditLog, int64, error) {
	if s == nil || s.repo == nil {
		return []domain.AuthzAuditLog{}, 0, nil
	}
	return s.repo.ListAdmin(filter)
}
