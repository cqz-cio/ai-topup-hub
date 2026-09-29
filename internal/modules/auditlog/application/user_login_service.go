package application

import (
	"net/mail"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/auditlog/contract"
	"github.com/dujiao-next/internal/modules/auditlog/domain"
)

// UserLoginService 用户登录日志服务
type UserLoginService struct {
	repo contract.UserLoginRepository
}

// NewUserLoginService 创建用户登录日志服务
func NewUserLoginService(repo contract.UserLoginRepository) *UserLoginService {
	return &UserLoginService{repo: repo}
}

// UserLoginRecord 登录日志记录输入
type UserLoginRecord struct {
	UserID      uint
	Email       string
	Status      string
	FailReason  string
	ClientIP    string
	UserAgent   string
	LoginSource string
	RequestID   string
}

// Record 记录登录行为
func (s *UserLoginService) Record(input UserLoginRecord) error {
	if s == nil || s.repo == nil {
		return nil
	}

	email := strings.TrimSpace(input.Email)
	if normalized, err := normalizeEmail(email); err == nil {
		email = normalized
	}

	status := strings.ToLower(strings.TrimSpace(input.Status))
	if status != constants.LoginLogStatusSuccess {
		status = constants.LoginLogStatusFailed
	}

	failReason := strings.ToLower(strings.TrimSpace(input.FailReason))
	if status == constants.LoginLogStatusSuccess {
		failReason = ""
	} else if failReason == "" {
		failReason = constants.LoginLogFailReasonInternalError
	}

	source := strings.ToLower(strings.TrimSpace(input.LoginSource))
	if source == "" {
		source = constants.LoginLogSourceWeb
	}

	now := time.Now()
	return s.repo.Create(&domain.UserLoginLog{
		UserID:      input.UserID,
		Email:       email,
		Status:      status,
		FailReason:  failReason,
		ClientIP:    strings.TrimSpace(input.ClientIP),
		UserAgent:   strings.TrimSpace(input.UserAgent),
		LoginSource: source,
		RequestID:   strings.TrimSpace(input.RequestID),
		CreatedAt:   now,
	})
}

// ListForAdmin 管理端查询登录日志
func (s *UserLoginService) ListForAdmin(filter contract.UserLoginFilter) ([]domain.UserLoginLog, int64, error) {
	if s == nil || s.repo == nil {
		return []domain.UserLoginLog{}, 0, nil
	}
	return s.repo.ListAdmin(filter)
}

// ListByUser 用户侧查询自己的登录日志
func (s *UserLoginService) ListByUser(userID uint, page, pageSize int) ([]domain.UserLoginLog, int64, error) {
	if s == nil || s.repo == nil || userID == 0 {
		return []domain.UserLoginLog{}, 0, nil
	}
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return s.repo.ListByUser(userID, page, pageSize)
}

func normalizeEmail(email string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	_, err := mail.ParseAddress(normalized)
	if normalized == "" || err != nil {
		return "", err
	}
	return normalized, nil
}
