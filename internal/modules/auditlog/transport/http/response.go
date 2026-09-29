package auditloghttp

import (
	"time"

	"github.com/dujiao-next/internal/modules/auditlog/domain"
)

// UserLoginResponse 是个人安全中心可见的登录日志视图。
// UserID、FailReason 与 RequestID 属于内部审计数据，不通过用户接口暴露。
type UserLoginResponse struct {
	ID          uint      `json:"id"`
	Email       string    `json:"email"`
	Status      string    `json:"status"`
	ClientIP    string    `json:"client_ip"`
	UserAgent   string    `json:"user_agent"`
	LoginSource string    `json:"login_source"`
	CreatedAt   time.Time `json:"created_at"`
}

func newUserLoginResponse(log *domain.UserLoginLog) UserLoginResponse {
	return UserLoginResponse{
		ID:          log.ID,
		Email:       log.Email,
		Status:      log.Status,
		ClientIP:    log.ClientIP,
		UserAgent:   log.UserAgent,
		LoginSource: log.LoginSource,
		CreatedAt:   log.CreatedAt,
	}
}

func newUserLoginResponseList(logs []domain.UserLoginLog) []UserLoginResponse {
	result := make([]UserLoginResponse, 0, len(logs))
	for i := range logs {
		result = append(result, newUserLoginResponse(&logs[i]))
	}
	return result
}
