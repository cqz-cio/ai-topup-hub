package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/notification/contract"
	"github.com/dujiao-next/internal/modules/notification/domain"
	settingsmessaging "github.com/dujiao-next/internal/modules/settings/schema/messaging"
	settingsstorefront "github.com/dujiao-next/internal/modules/settings/schema/storefront"
	"github.com/dujiao-next/internal/queue"
)

type notificationSettingsStub struct {
	notification settingsmessaging.NotificationCenterSetting
	dashboard    settingsstorefront.DashboardSetting
}

func (s notificationSettingsStub) GetNotificationCenterSetting() (settingsmessaging.NotificationCenterSetting, error) {
	return s.notification, nil
}

func (s notificationSettingsStub) GetDashboardSetting() (settingsstorefront.DashboardSetting, error) {
	return s.dashboard, nil
}

type notificationEmailStub struct{}

func (notificationEmailStub) SendCustomEmail(recipient, _, _ string) error {
	if strings.Contains(recipient, "failure") {
		return errors.New("simulated email failure")
	}
	return nil
}

type notificationFeishuCall struct {
	appID         string
	appSecret     string
	receiveIDType string
	receiveID     string
	message       string
}

type notificationFeishuStub struct {
	calls []notificationFeishuCall
}

func (s *notificationFeishuStub) SendMessage(_ context.Context, appID, appSecret, receiveIDType, receiveID, message string) error {
	s.calls = append(s.calls, notificationFeishuCall{
		appID:         appID,
		appSecret:     appSecret,
		receiveIDType: receiveIDType,
		receiveID:     receiveID,
		message:       message,
	})
	if strings.Contains(receiveID, "failure") {
		return errors.New("simulated feishu failure")
	}
	return nil
}

type notificationLogRepositoryStub struct {
	items []domain.NotificationLog
}

func (r *notificationLogRepositoryStub) Create(item *domain.NotificationLog) error {
	if item != nil {
		copy := *item
		copy.ID = uint(len(r.items) + 1)
		r.items = append(r.items, copy)
	}
	return nil
}

func (r *notificationLogRepositoryStub) ListAdmin(filter contract.LogListFilter) ([]domain.NotificationLog, int64, error) {
	result := make([]domain.NotificationLog, 0, len(r.items))
	for _, item := range r.items {
		if filter.EventType != "" && item.EventType != filter.EventType {
			continue
		}
		if filter.IsTest != nil && item.IsTest != *filter.IsTest {
			continue
		}
		result = append(result, item)
	}
	return result, int64(len(result)), nil
}

func setupLogService(t *testing.T) (*Service, *LogService) {
	t.Helper()
	logService := NewLogService(&notificationLogRepositoryStub{})
	setting := settingsmessaging.NotificationCenterSetting{
		DefaultLocale: constants.LocaleEnUS,
		Scenes: settingsmessaging.NotificationSceneSetting{
			OrderPaidSuccess: true,
			ExceptionAlert:   true,
		},
		Channels: settingsmessaging.NotificationChannelsSetting{
			Email: settingsmessaging.NotificationChannelSetting{
				Enabled:    true,
				Recipients: []string{"success@example.com", "failure@example.com"},
			},
		},
		Templates: settingsmessaging.NotificationTemplatesSetting{
			OrderPaidSuccess: settingsmessaging.NotificationSceneTemplate{
				ENUS: settingsmessaging.NotificationLocalizedTemplate{
					Title: "Order {{order_no}}",
					Body:  "Customer {{customer_email}}",
				},
			},
			ExceptionAlert: settingsmessaging.NotificationSceneTemplate{
				ENUS: settingsmessaging.NotificationLocalizedTemplate{
					Title: "Alert {{message}}",
					Body:  "{{message}}",
				},
			},
		},
	}
	service := NewService(notificationSettingsStub{notification: setting}, notificationEmailStub{}, nil, nil, logService, nil, nil)
	return service, logService
}

func TestServiceSendTestRecordsSuccessLog(t *testing.T) {
	service, logService := setupLogService(t)
	if err := service.SendTest(context.Background(), contract.TestSendInput{
		Channel: "email",
		Target:  "success@example.com",
		Scene:   constants.NotificationEventOrderPaidSuccess,
		Locale:  constants.LocaleEnUS,
	}); err != nil {
		t.Fatalf("SendTest failed: %v", err)
	}

	isTest := true
	items, total, err := logService.ListForAdmin(contract.LogListFilter{
		Page: 1, PageSize: 10, EventType: constants.NotificationEventOrderPaidSuccess, IsTest: &isTest,
	})
	if err != nil {
		t.Fatalf("list notification logs failed: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected 1 test log, total=%d len=%d", total, len(items))
	}
	if items[0].Status != notificationLogStatusSuccess || !items[0].IsTest {
		t.Fatalf("unexpected log: %#v", items[0])
	}
	if !strings.Contains(items[0].Title, "DJ202603230001") {
		t.Fatalf("title should include sample order no, got %s", items[0].Title)
	}
}

func TestServiceDispatchSingleEventRecordsPerRecipientResult(t *testing.T) {
	service, logService := setupLogService(t)
	setting, err := service.settingService.GetNotificationCenterSetting()
	if err != nil {
		t.Fatalf("get notification center setting failed: %v", err)
	}
	dispatchErr := service.dispatchSingleEvent(context.Background(), setting, queue.NotificationDispatchPayload{
		EventType: constants.NotificationEventOrderPaidSuccess,
		BizType:   constants.NotificationBizTypeOrder,
		BizID:     88,
		Locale:    constants.LocaleEnUS,
		Force:     true,
		Data: map[string]interface{}{
			"order_no":       "DJ-LOG-88",
			"customer_email": "member@example.com",
		},
	})
	if !errors.Is(dispatchErr, contract.ErrSendFailed) {
		t.Fatalf("expected send failure, got %v", dispatchErr)
	}

	items, total, err := logService.ListForAdmin(contract.LogListFilter{Page: 1, PageSize: 10, EventType: constants.NotificationEventOrderPaidSuccess})
	if err != nil {
		t.Fatalf("list notification logs failed: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("expected 2 recipient logs, total=%d len=%d", total, len(items))
	}
	statuses := map[string]string{}
	for _, item := range items {
		statuses[item.Recipient] = item.Status
	}
	if statuses["success@example.com"] != notificationLogStatusSuccess {
		t.Fatalf("success recipient status mismatch: %v", statuses)
	}
	if statuses["failure@example.com"] != notificationLogStatusFailed {
		t.Fatalf("failure recipient status mismatch: %v", statuses)
	}
}

func TestServiceDispatchSingleEventSendsFeishuAndRecordsEachRecipient(t *testing.T) {
	logService := NewLogService(&notificationLogRepositoryStub{})
	feishuSender := &notificationFeishuStub{}
	setting := settingsmessaging.NotificationCenterDefaultSetting()
	setting.DefaultLocale = constants.LocaleZhCN
	setting.Channels.Feishu = settingsmessaging.FeishuNotificationChannelSetting{
		Enabled:       true,
		AppID:         "cli_demo",
		AppSecret:     "secret",
		ReceiveIDType: settingsmessaging.FeishuReceiveIDTypeChatID,
		Recipients:    []string{"oc_success", "oc_failure"},
	}
	service := NewService(notificationSettingsStub{notification: setting}, nil, nil, nil, logService, nil, feishuSender)

	err := service.dispatchSingleEvent(context.Background(), setting, queue.NotificationDispatchPayload{
		EventType: constants.NotificationEventOrderPaidSuccess,
		BizType:   constants.NotificationBizTypeOrder,
		BizID:     99,
		Locale:    constants.LocaleZhCN,
		Force:     true,
		Data: map[string]interface{}{
			"order_no":       "DJ-FEISHU-99",
			"customer_email": "member@example.com",
		},
	})
	if !errors.Is(err, contract.ErrSendFailed) {
		t.Fatalf("expected one feishu failure, got %v", err)
	}
	if len(feishuSender.calls) != 2 {
		t.Fatalf("expected two feishu calls, got %d", len(feishuSender.calls))
	}
	firstCall := feishuSender.calls[0]
	if firstCall.appID != "cli_demo" || firstCall.appSecret != "secret" || firstCall.receiveIDType != settingsmessaging.FeishuReceiveIDTypeChatID {
		t.Fatalf("unexpected feishu credentials or id type: %#v", firstCall)
	}
	if !strings.Contains(firstCall.message, "DJ-FEISHU-99") {
		t.Fatalf("rendered feishu message missing order number: %q", firstCall.message)
	}

	items, total, listErr := logService.ListForAdmin(contract.LogListFilter{
		Page: 1, PageSize: 10, Channel: constants.NotificationChannelFeishu,
	})
	if listErr != nil {
		t.Fatalf("list feishu notification logs: %v", listErr)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("expected two feishu logs, total=%d len=%d", total, len(items))
	}
	statuses := map[string]string{}
	for _, item := range items {
		statuses[item.Recipient] = item.Status
	}
	if statuses["oc_success"] != notificationLogStatusSuccess || statuses["oc_failure"] != notificationLogStatusFailed {
		t.Fatalf("unexpected feishu log statuses: %#v", statuses)
	}
}

type saleSummaryStub struct {
	err     error
	calls   int
	summary string
}

func (s *saleSummaryStub) Build(context.Context, uint, string, bool) (map[string]interface{}, error) {
	s.calls++
	return map[string]interface{}{"upstream_summary": s.summary}, s.err
}

type saleEmailStub struct {
	calls  map[string]int
	bodies []string
	fail   string
}

func (s *saleEmailStub) SendCustomEmail(recipient, _, body string) error {
	s.calls[recipient]++
	s.bodies = append(s.bodies, body)
	if recipient == s.fail {
		return errors.New("temporary SMTP failure")
	}
	return nil
}

type saleLogStub struct{ notificationLogRepositoryStub }

func (s *saleLogStub) HasSuccessfulDelivery(event, biz string, id uint, channel, recipient string) (bool, error) {
	for _, item := range s.items {
		if item.EventType == event && item.BizType == biz && item.BizID == id && item.Channel == channel && item.Recipient == recipient && item.Status == "success" && !item.IsTest {
			return true, nil
		}
	}
	return false, nil
}

func TestSaleNotificationCombinesBalanceAndRetriesOnlyFailedRecipient(t *testing.T) {
	setting := settingsmessaging.NotificationCenterDefaultSetting()
	setting.Channels.Email = settingsmessaging.NotificationChannelSetting{Enabled: true, Recipients: []string{"owner@example.com", "backup@example.com"}}
	setting.Templates.OrderPaidSuccess.ZHCN.Body = "订单 {{order_no}}，实收 {{amount}} CNY"
	email := &saleEmailStub{calls: map[string]int{}, fail: "backup@example.com"}
	logs := &saleLogStub{}
	s := NewService(notificationSettingsStub{notification: setting}, email, nil, nil, NewLogService(logs), nil, nil)
	reader := &saleSummaryStub{err: contract.ErrProcurementPending, summary: "采购 33.00 CNY；剩余余额 67.00 CNY"}
	s.SetSaleSummaryReader(reader)
	payload := queue.NotificationDispatchPayload{EventType: constants.NotificationEventOrderPaidSuccess, BizType: constants.NotificationBizTypeOrder, BizID: 7, Data: map[string]interface{}{"order_no": "SALE-7", "amount": "50.00"}}
	if err := s.Dispatch(context.Background(), payload); !errors.Is(err, contract.ErrProcurementPending) {
		t.Fatal(err)
	}
	if len(email.calls) != 0 {
		t.Fatal("email sent before procurement was ready")
	}
	reader.err = nil
	if err := s.Dispatch(context.Background(), payload); err == nil {
		t.Fatal("SMTP failure must remain retryable")
	}
	if !strings.Contains(email.bodies[0], "SALE-7") || !strings.Contains(email.bodies[0], "67.00 CNY") {
		t.Fatal(email.bodies)
	}
	email.fail = ""
	reader.summary = "采购 33.00 CNY；剩余余额 60.00 CNY"
	if err := s.Dispatch(context.Background(), payload); err != nil {
		t.Fatal(err)
	}
	if email.calls["owner@example.com"] != 1 || email.calls["backup@example.com"] != 2 {
		t.Fatal(email.calls)
	}
	if err := s.Dispatch(context.Background(), payload); err != nil {
		t.Fatal(err)
	}
	if email.calls["owner@example.com"] != 1 || email.calls["backup@example.com"] != 2 {
		t.Fatal("duplicate task resent sale email")
	}
	if _, ok := payload.Data["upstream_summary"]; ok {
		t.Fatal("dispatch mutated original task payload")
	}
}

func TestSaleSummaryPlaceholderDoesNotAppendTwice(t *testing.T) {
	setting := settingsmessaging.NotificationCenterDefaultSetting()
	setting.Channels.Email = settingsmessaging.NotificationChannelSetting{Enabled: true, Recipients: []string{"owner@example.com"}}
	setting.Templates.OrderPaidSuccess.ZHCN.Body = "{{order_no}}\n{{ upstream_summary }}"
	email := &saleEmailStub{calls: map[string]int{}}
	s := NewService(notificationSettingsStub{notification: setting}, email, nil, nil, nil, nil, nil)
	s.SetSaleSummaryReader(&saleSummaryStub{summary: "余额 67.00 CNY"})
	if err := s.Dispatch(context.Background(), queue.NotificationDispatchPayload{EventType: constants.NotificationEventOrderPaidSuccess, BizType: constants.NotificationBizTypeOrder, BizID: 8}); err != nil {
		t.Fatal(err)
	}
	if strings.Count(email.bodies[0], "67.00") != 1 {
		t.Fatal(email.bodies)
	}
}

func TestDisabledSaleNotificationsDoNotQuerySupplier(t *testing.T) {
	setting := settingsmessaging.NotificationCenterDefaultSetting()
	reader := &saleSummaryStub{err: contract.ErrProcurementPending}
	s := NewService(notificationSettingsStub{notification: setting}, nil, nil, nil, nil, nil, nil)
	s.SetSaleSummaryReader(reader)
	if err := s.Dispatch(context.Background(), queue.NotificationDispatchPayload{EventType: constants.NotificationEventOrderPaidSuccess, BizType: constants.NotificationBizTypeOrder, BizID: 8}); err != nil {
		t.Fatal(err)
	}
	if reader.calls != 0 {
		t.Fatal("disabled channel queried supplier")
	}
}
