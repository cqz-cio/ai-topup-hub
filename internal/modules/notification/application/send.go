package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/modules/notification/application/format"
	"github.com/dujiao-next/internal/modules/notification/contract"
	settingsmessaging "github.com/dujiao-next/internal/modules/settings/schema/messaging"
	"github.com/dujiao-next/internal/queue"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/outboundctx"
)

func detachOutboundRequestContext(parent context.Context) (context.Context, context.CancelFunc) {
	return outboundctx.Detach(parent, outboundctx.DefaultTimeout)
}

// SendTest 测试发送通知
func (s *Service) SendTest(ctx context.Context, input contract.TestSendInput) error {
	if s == nil {
		return contract.ErrSendFailed
	}
	channel := strings.ToLower(strings.TrimSpace(input.Channel))
	target := strings.TrimSpace(input.Target)
	if channel == "" || target == "" {
		return contract.ErrConfigInvalid
	}

	setting, err := s.settingService.GetNotificationCenterSetting()
	if err != nil {
		return err
	}
	scene := strings.ToLower(strings.TrimSpace(input.Scene))
	if scene == "" {
		scene = constants.NotificationEventExceptionAlert
	}
	template := setting.Templates.TemplateByEvent(scene).ResolveLocaleTemplate(format.ResolveLocale(input.Locale, setting.DefaultLocale))
	variables := format.CloneVariables(input.Variables)
	if variables == nil {
		variables = map[string]interface{}{}
	}
	locale := format.ResolveLocale(input.Locale, setting.DefaultLocale)
	format.ApplyTestVariables(variables, format.BuildTestVariables(scene, locale))
	variables["event_type"] = scene
	variables["message"] = format.PickMessage(variables["message"], "test message")
	title := format.RenderTemplate(template.Title, variables)
	body := format.RenderTemplate(template.Body, variables)
	if strings.TrimSpace(body) == "" {
		body = title
	}
	if strings.TrimSpace(title) == "" {
		title = "Notification Test"
	}

	switch channel {
	case constants.NotificationChannelEmail:
		sendErr := contract.ErrSendFailed
		if s.emailService != nil {
			sendErr = s.emailService.SendCustomEmail(target, title, body)
		}
		s.recordSendAttempt(notificationSendAttempt{
			eventType: scene,
			channel:   channel,
			recipient: target,
			locale:    locale,
			title:     title,
			body:      body,
			variables: variables,
			isTest:    true,
			sendErr:   sendErr,
		})
		return sendErr
	case constants.NotificationChannelTelegram:
		sendErr := contract.ErrSendFailed
		gatewayCtx, cancel := detachOutboundRequestContext(ctx)
		defer cancel()
		if s.telegramSender != nil {
			sendErr = s.telegramSender.SendMessage(gatewayCtx, target, format.ComposePlainTextMessage(title, body))
		}
		s.recordSendAttempt(notificationSendAttempt{
			eventType: scene,
			channel:   channel,
			recipient: target,
			locale:    locale,
			title:     title,
			body:      body,
			variables: variables,
			isTest:    true,
			sendErr:   sendErr,
		})
		return sendErr
	case constants.NotificationChannelFeishu:
		sendErr := contract.ErrSendFailed
		gatewayCtx, cancel := detachOutboundRequestContext(ctx)
		defer cancel()
		if s.feishuSender != nil {
			feishu := setting.Channels.Feishu
			sendErr = s.feishuSender.SendMessage(
				gatewayCtx,
				feishu.AppID,
				feishu.AppSecret,
				feishu.ReceiveIDType,
				target,
				format.ComposePlainTextMessage(title, body),
			)
		}
		s.recordSendAttempt(notificationSendAttempt{
			eventType: scene,
			channel:   channel,
			recipient: target,
			locale:    locale,
			title:     title,
			body:      body,
			variables: variables,
			isTest:    true,
			sendErr:   sendErr,
		})
		return sendErr
	default:
		return contract.ErrConfigInvalid
	}
}

func (s *Service) dispatchSingleEvent(ctx context.Context, setting settingsmessaging.NotificationCenterSetting, payload queue.NotificationDispatchPayload) error {
	if !payload.Force {
		ok, err := acquireNotificationDedupe(ctx, setting.DedupeTTLSeconds, payload)
		if err != nil {
			logger.Warnw("notification_dedupe_failed", "event_type", payload.EventType, "error", err)
		}
		if err == nil && !ok {
			return nil
		}
	}

	locale := format.ResolveLocale(payload.Locale, setting.DefaultLocale)
	template := setting.Templates.TemplateByEvent(payload.EventType).ResolveLocaleTemplate(locale)
	variables := format.BuildTemplateVariables(payload)
	title := format.RenderTemplate(template.Title, variables)
	body := format.RenderTemplate(template.Body, variables)
	if strings.TrimSpace(body) == "" {
		body = title
	}
	if strings.TrimSpace(title) == "" {
		title = "Notification"
	}

	var firstErr error
	if setting.Channels.Email.Enabled && len(setting.Channels.Email.Recipients) > 0 {
		for _, recipient := range setting.Channels.Email.Recipients {
			var sendErr error
			if s.emailService == nil {
				sendErr = contract.ErrSendFailed
			} else {
				sendErr = s.emailService.SendCustomEmail(recipient, title, body)
			}
			s.recordSendAttempt(notificationSendAttempt{
				eventType: payload.EventType,
				bizType:   payload.BizType,
				bizID:     payload.BizID,
				channel:   constants.NotificationChannelEmail,
				recipient: recipient,
				locale:    locale,
				title:     title,
				body:      body,
				variables: variables,
				sendErr:   sendErr,
			})
			if sendErr != nil {
				logger.Warnw("notification_email_send_failed",
					"event_type", payload.EventType,
					"biz_type", payload.BizType,
					"biz_id", payload.BizID,
					"recipient", recipient,
					"error", sendErr,
				)
				if firstErr == nil {
					firstErr = sendErr
				}
			}
		}
	}
	if setting.Channels.Telegram.Enabled && len(setting.Channels.Telegram.Recipients) > 0 {
		message := format.ComposePlainTextMessage(title, body)
		for _, recipient := range setting.Channels.Telegram.Recipients {
			var sendErr error
			if s.telegramSender == nil {
				sendErr = contract.ErrSendFailed
			} else {
				sendErr = s.telegramSender.SendMessage(ctx, recipient, message)
			}
			s.recordSendAttempt(notificationSendAttempt{
				eventType: payload.EventType,
				bizType:   payload.BizType,
				bizID:     payload.BizID,
				channel:   constants.NotificationChannelTelegram,
				recipient: recipient,
				locale:    locale,
				title:     title,
				body:      body,
				variables: variables,
				sendErr:   sendErr,
			})
			if sendErr != nil {
				logger.Warnw("notification_telegram_send_failed",
					"event_type", payload.EventType,
					"biz_type", payload.BizType,
					"biz_id", payload.BizID,
					"recipient", recipient,
					"error", sendErr,
				)
				if firstErr == nil {
					firstErr = sendErr
				}
			}
		}
	}
	if setting.Channels.Feishu.Enabled && len(setting.Channels.Feishu.Recipients) > 0 {
		message := format.ComposePlainTextMessage(title, body)
		for _, recipient := range setting.Channels.Feishu.Recipients {
			var sendErr error
			if s.feishuSender == nil {
				sendErr = contract.ErrSendFailed
			} else {
				sendErr = s.feishuSender.SendMessage(
					ctx,
					setting.Channels.Feishu.AppID,
					setting.Channels.Feishu.AppSecret,
					setting.Channels.Feishu.ReceiveIDType,
					recipient,
					message,
				)
			}
			s.recordSendAttempt(notificationSendAttempt{
				eventType: payload.EventType,
				bizType:   payload.BizType,
				bizID:     payload.BizID,
				channel:   constants.NotificationChannelFeishu,
				recipient: recipient,
				locale:    locale,
				title:     title,
				body:      body,
				variables: variables,
				sendErr:   sendErr,
			})
			if sendErr != nil {
				logger.Warnw("notification_feishu_send_failed",
					"event_type", payload.EventType,
					"biz_type", payload.BizType,
					"biz_id", payload.BizID,
					"recipient", recipient,
					"error", sendErr,
				)
				if firstErr == nil {
					firstErr = sendErr
				}
			}
		}
	}
	if firstErr != nil {
		return fmt.Errorf("%w: %v", contract.ErrSendFailed, firstErr)
	}
	return nil
}

type notificationSendAttempt struct {
	eventType string
	bizType   string
	bizID     uint
	channel   string
	recipient string
	locale    string
	title     string
	body      string
	variables map[string]interface{}
	isTest    bool
	sendErr   error
}

func (s *Service) recordSendAttempt(attempt notificationSendAttempt) {
	if s == nil || s.logService == nil {
		return
	}
	status := notificationLogStatusSuccess
	errMessage := ""
	if attempt.sendErr != nil {
		status = notificationLogStatusFailed
		errMessage = attempt.sendErr.Error()
	}
	if err := s.logService.Record(LogRecordInput{
		EventType:    attempt.eventType,
		BizType:      attempt.bizType,
		BizID:        attempt.bizID,
		Channel:      attempt.channel,
		Recipient:    attempt.recipient,
		Locale:       attempt.locale,
		Title:        attempt.title,
		Body:         attempt.body,
		Status:       status,
		ErrorMessage: errMessage,
		IsTest:       attempt.isTest,
		Variables:    notificationVariablesToJSON(attempt.variables),
	}); err != nil {
		logger.Warnw("notification_log_record_failed",
			"event_type", attempt.eventType,
			"biz_type", attempt.bizType,
			"biz_id", attempt.bizID,
			"channel", attempt.channel,
			"recipient", attempt.recipient,
			"error", err,
		)
	}
}

func notificationVariablesToJSON(data map[string]interface{}) jsonmap.JSON {
	if len(data) == 0 {
		return jsonmap.JSON{}
	}
	result := make(jsonmap.JSON, len(data))
	for key, value := range data {
		result[key] = value
	}
	return result
}
