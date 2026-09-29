package application

import (
	"context"
	"strings"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/notification/application/format"
	"github.com/dujiao-next/internal/modules/notification/contract"
	"github.com/dujiao-next/internal/queue"
)

// Service 通知中心服务。
type Service struct {
	settingService contract.SettingsReader
	emailService   contract.EmailSender
	queueClient    contract.DispatchQueue
	dashboardSvc   contract.DashboardAlertReader
	logService     *LogService
	telegramSender contract.TelegramSender
	feishuSender   contract.FeishuSender
}

// NewService 创建通知中心服务。
func NewService(
	settingService contract.SettingsReader,
	emailService contract.EmailSender,
	queueClient contract.DispatchQueue,
	dashboardSvc contract.DashboardAlertReader,
	logService *LogService,
	telegramSender contract.TelegramSender,
	feishuSender contract.FeishuSender,
) *Service {
	return &Service{
		settingService: settingService,
		emailService:   emailService,
		queueClient:    queueClient,
		dashboardSvc:   dashboardSvc,
		logService:     logService,
		telegramSender: telegramSender,
		feishuSender:   feishuSender,
	}
}

// Enqueue 入队通知任务
func (s *Service) Enqueue(input contract.EnqueueInput) error {
	eventType := strings.ToLower(strings.TrimSpace(input.EventType))
	if !isNotificationEventSupported(eventType) {
		return contract.ErrEventInvalid
	}
	if s == nil || s.queueClient == nil {
		return nil
	}

	payload := queue.NotificationDispatchPayload{
		EventType: eventType,
		BizType:   strings.TrimSpace(input.BizType),
		BizID:     input.BizID,
		Locale:    strings.TrimSpace(input.Locale),
		Force:     input.Force,
		Data:      format.JSONToMap(input.Data),
	}
	return s.queueClient.EnqueueNotificationDispatch(payload, 5)
}

// Dispatch 处理通知分发任务
func (s *Service) Dispatch(ctx context.Context, payload queue.NotificationDispatchPayload) error {
	if s == nil {
		return nil
	}
	eventType := strings.ToLower(strings.TrimSpace(payload.EventType))
	if !isNotificationEventSupported(eventType) {
		return contract.ErrEventInvalid
	}

	setting, err := s.settingService.GetNotificationCenterSetting()
	if err != nil {
		return err
	}
	if !setting.Scenes.IsSceneEnabled(eventType) {
		return nil
	}

	if eventType == constants.NotificationEventExceptionAlertCheck {
		return s.dispatchExceptionAlertCheck(ctx, setting, payload)
	}
	return s.dispatchSingleEvent(ctx, setting, payload)
}
