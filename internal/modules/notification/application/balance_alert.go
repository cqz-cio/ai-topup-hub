package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/cache"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/notification/contract"
	settingsmessaging "github.com/dujiao-next/internal/modules/settings/schema/messaging"
	"github.com/dujiao-next/internal/queue"
	"github.com/google/uuid"
)

type balanceAlertState interface {
	Ready() bool
	Get(context.Context, string) (string, error)
	Claim(context.Context, string, string, time.Duration) (bool, error)
	Put(context.Context, string, string, time.Duration) error
}
type redisBalanceAlertState struct{}

func (redisBalanceAlertState) Ready() bool { return cache.Enabled() }
func (redisBalanceAlertState) Get(ctx context.Context, key string) (string, error) {
	return cache.GetString(ctx, key)
}
func (redisBalanceAlertState) Claim(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	return cache.SetNX(ctx, key, value, ttl)
}
func (redisBalanceAlertState) Put(ctx context.Context, key, value string, ttl time.Duration) error {
	return cache.SetString(ctx, key, value, ttl)
}
func (s *Service) balanceAlertStore() balanceAlertState {
	if s.balanceState != nil {
		return s.balanceState
	}
	return redisBalanceAlertState{}
}

const balanceAlertPrefix = "supplier_balance_"

func isBalanceAlert(payload queue.NotificationDispatchPayload) bool {
	return payload.EventType == constants.NotificationEventExceptionAlert &&
		strings.HasPrefix(payload.BizType, balanceAlertPrefix) && payload.BizID > 0
}

func balanceAlertKey(connectionID uint) string {
	return fmt.Sprintf("notification:balance:%d", connectionID)
}

// BalanceAlertStatus reads the result written after SMTP accepts or rejects a
// message. Queue acceptance alone never becomes "sent".
func (s *Service) BalanceAlertStatus(ctx context.Context, connectionID uint) string {
	if s == nil || !s.balanceAlertStore().Ready() {
		return "not_sent"
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	status, err := s.balanceAlertStore().Get(ctx, balanceAlertKey(connectionID))
	if err != nil {
		return "not_sent"
	}
	switch status {
	case "pending", "sent", "partial", "failed":
		return status
	}
	return "not_sent"
}

// NotifyBalanceShortage queues one email incident per supplier, independent of
// which shopper/channel triggered it. Successful incidents cool down for 30
// minutes; failures for one minute. Redis failure never permits unbounded mail.
func (s *Service) NotifyBalanceShortage(ctx context.Context, connectionID uint, required, available, currency string) string {
	if s == nil || s.settingService == nil || s.queueClient == nil || !s.balanceAlertStore().Ready() {
		return "not_sent"
	}
	if ready, ok := s.queueClient.(interface{ Ready() bool }); ok && !ready.Ready() {
		return "not_sent"
	}
	setting, err := s.settingService.GetNotificationCenterSetting()
	if err != nil || !setting.Channels.Email.Enabled || len(setting.Channels.Email.Recipients) == 0 {
		return "not_sent"
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	key := balanceAlertKey(connectionID)
	claimed, err := s.balanceAlertStore().Claim(ctx, key, "pending", 30*time.Minute)
	if err != nil {
		return "not_sent"
	}
	if !claimed {
		return s.BalanceAlertStatus(ctx, connectionID)
	}
	payload := queue.NotificationDispatchPayload{
		EventType: constants.NotificationEventExceptionAlert,
		BizType:   balanceAlertPrefix + uuid.NewString(), BizID: connectionID,
		Locale: constants.LocaleZhCN,
		Data: map[string]interface{}{
			"required": required, "available": available, "currency": currency,
			"occurred_at": time.Now().In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05 -0700"),
		},
	}
	if err := s.queueClient.EnqueueNotificationDispatch(payload, 3); err != nil {
		_ = s.balanceAlertStore().Put(ctx, key, "failed", time.Minute)
		return "failed"
	}
	return s.BalanceAlertStatus(ctx, connectionID)
}

func (s *Service) dispatchBalanceAlert(ctx context.Context, setting settingsmessaging.NotificationCenterSetting, payload queue.NotificationDispatchPayload) error {
	status, ttl := "not_sent", time.Minute
	defer func() {
		writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_ = s.balanceAlertStore().Put(writeCtx, balanceAlertKey(payload.BizID), status, ttl)
	}()
	if !setting.Channels.Email.Enabled || len(setting.Channels.Email.Recipients) == 0 {
		return nil
	}
	title := "卡台余额不足，请及时充值"
	body := fmt.Sprintf("客户付款已暂停，请补充卡台余额后协助客户重试。\n\n上游连接：%d\n本次采购成本合计：%v %v\n查询余额：%v %v\n北京时间：%v\n\n本提醒未创建采购、扣款或自动充值。",
		payload.BizID, payload.Data["required"], payload.Data["currency"], payload.Data["available"], payload.Data["currency"], payload.Data["occurred_at"])
	succeeded := 0
	for _, recipient := range setting.Channels.Email.Recipients {
		attempt := notificationSendAttempt{eventType: payload.EventType, bizType: payload.BizType, bizID: payload.BizID,
			channel: constants.NotificationChannelEmail, recipient: recipient, locale: constants.LocaleZhCN,
			title: title, body: body, variables: payload.Data}
		err := s.sendNotification(ctx, payload, attempt, func() error {
			if s.emailService == nil {
				return contract.ErrSendFailed
			}
			return s.emailService.SendCustomEmail(recipient, title, body)
		})
		if err == nil {
			succeeded++
		}
	}
	switch {
	case succeeded == len(setting.Channels.Email.Recipients):
		status, ttl = "sent", 30*time.Minute
	case succeeded > 0:
		status, ttl = "partial", 30*time.Minute
	default:
		status = "failed"
	}
	// Retry the same incident promptly, skipping recipients already accepted.
	if succeeded < len(setting.Channels.Email.Recipients) {
		return contract.ErrSendFailed
	}
	return nil
}
