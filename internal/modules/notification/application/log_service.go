package application

import (
	"strings"
	"time"

	"github.com/dujiao-next/internal/modules/notification/contract"
	"github.com/dujiao-next/internal/modules/notification/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

const (
	notificationLogStatusSuccess = "success"
	notificationLogStatusFailed  = "failed"
)

// LogRecordInput 通知日志记录输入。
type LogRecordInput struct {
	EventType    string
	BizType      string
	BizID        uint
	Channel      string
	Recipient    string
	Locale       string
	Title        string
	Body         string
	Status       string
	ErrorMessage string
	IsTest       bool
	Variables    jsonmap.JSON
}

// LogService 通知日志服务。
type LogService struct {
	repo contract.LogRepository
}

// NewLogService 创建通知日志服务。
func NewLogService(repo contract.LogRepository) *LogService {
	return &LogService{repo: repo}
}

// Record 记录通知发送日志
func (s *LogService) Record(input LogRecordInput) error {
	if s == nil || s.repo == nil {
		return nil
	}

	channel := strings.ToLower(strings.TrimSpace(input.Channel))
	recipient := strings.TrimSpace(input.Recipient)
	if channel == "" || recipient == "" {
		return nil
	}

	status := strings.ToLower(strings.TrimSpace(input.Status))
	if status != notificationLogStatusSuccess {
		status = notificationLogStatusFailed
	}

	item := &domain.NotificationLog{
		EventType:     strings.ToLower(strings.TrimSpace(input.EventType)),
		BizType:       strings.ToLower(strings.TrimSpace(input.BizType)),
		BizID:         input.BizID,
		Channel:       channel,
		Recipient:     recipient,
		Locale:        strings.TrimSpace(input.Locale),
		Title:         strings.TrimSpace(input.Title),
		Body:          strings.TrimSpace(input.Body),
		Status:        status,
		ErrorMessage:  strings.TrimSpace(input.ErrorMessage),
		IsTest:        input.IsTest,
		VariablesJSON: cloneNotificationLogJSON(input.Variables),
		CreatedAt:     time.Now(),
	}
	return s.repo.Create(item)
}

// ListForAdmin 管理端查询通知日志
func (s *LogService) ListForAdmin(filter contract.LogListFilter) ([]domain.NotificationLog, int64, error) {
	if s == nil || s.repo == nil {
		return []domain.NotificationLog{}, 0, nil
	}
	return s.repo.ListAdmin(filter)
}

func cloneNotificationLogJSON(data jsonmap.JSON) jsonmap.JSON {
	if len(data) == 0 {
		return jsonmap.JSON{}
	}
	result := make(jsonmap.JSON, len(data))
	for key, value := range data {
		result[key] = value
	}
	return result
}
