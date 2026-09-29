package application

import (
	"strings"
	"time"

	"github.com/dujiao-next/internal/modules/auditlog/contract"
	"github.com/dujiao-next/internal/modules/auditlog/domain"
)

// AdminLoginRecord 描述一次管理员认证或 2FA 操作的审计事实。
type AdminLoginRecord struct {
	AdminID    uint
	Username   string
	EventType  string
	Status     string
	FailReason string
	ClientIP   string
	UserAgent  string
	RequestID  string
	OperatorID *uint
}

// AdminLoginService 是管理员登录审计的应用边界，调用方不直接接触持久化端口。
type AdminLoginService struct {
	repo contract.AdminLoginRepository
}

func NewAdminLoginService(repo contract.AdminLoginRepository) *AdminLoginService {
	return &AdminLoginService{repo: repo}
}

func (s *AdminLoginService) Record(input AdminLoginRecord) error {
	if s == nil || s.repo == nil {
		return nil
	}
	return s.repo.Create(&domain.AdminLoginLog{
		AdminID:    input.AdminID,
		Username:   strings.TrimSpace(input.Username),
		EventType:  strings.TrimSpace(input.EventType),
		Status:     strings.TrimSpace(input.Status),
		FailReason: strings.TrimSpace(input.FailReason),
		ClientIP:   strings.TrimSpace(input.ClientIP),
		UserAgent:  strings.TrimSpace(input.UserAgent),
		RequestID:  strings.TrimSpace(input.RequestID),
		OperatorID: input.OperatorID,
		CreatedAt:  time.Now(),
	})
}

func (s *AdminLoginService) List(filter contract.AdminLoginFilter) ([]domain.AdminLoginLog, int64, error) {
	if s == nil || s.repo == nil {
		return []domain.AdminLoginLog{}, 0, nil
	}
	return s.repo.List(filter)
}
