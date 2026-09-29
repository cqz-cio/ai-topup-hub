package application

import (
	"context"
	"time"

	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/modules/notification/application/format"
	settingsmessaging "github.com/dujiao-next/internal/modules/settings/schema/messaging"
	"github.com/dujiao-next/internal/queue"
)

func (s *Service) dispatchExceptionAlertCheck(ctx context.Context, setting settingsmessaging.NotificationCenterSetting, payload queue.NotificationDispatchPayload) error {
	if s.dashboardSvc == nil || s.settingService == nil {
		return nil
	}

	dashboardSetting, err := s.settingService.GetDashboardSetting()
	if err != nil {
		return err
	}

	var firstErr error
	inventoryAlerts, err := s.dashboardSvc.GetInventoryAlertItems(ctx, dashboardSetting.Alert.LowStockThreshold)
	if err != nil {
		return err
	}
	for _, itemPayload := range format.BuildInventoryAlertDispatchPayloads(setting, dashboardSetting, payload, inventoryAlerts) {
		allowed, intervalErr := acquireInventoryAlertInterval(ctx, setting.InventoryAlertIntervalSeconds, itemPayload)
		if intervalErr != nil {
			logger.Warnw("notification_inventory_alert_interval_failed", "error", intervalErr)
		}
		if intervalErr == nil && !allowed {
			continue
		}
		if err := s.dispatchSingleEvent(ctx, setting, itemPayload); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	paymentOrderAlertNow := time.Now()
	paymentOrderAlertStart := paymentOrderAlertNow.Add(-time.Duration(setting.PaymentOrderAlertCheckSeconds) * time.Second)
	paymentOrderCounts, err := s.dashboardSvc.GetPaymentOrderAlertCounts(ctx, paymentOrderAlertStart, paymentOrderAlertNow)
	if err != nil {
		return err
	}

	for _, itemPayload := range format.BuildPaymentOrderAlertDispatchPayloads(setting, dashboardSetting, payload, paymentOrderCounts) {
		allowed, intervalErr := acquirePaymentOrderAlertInterval(ctx, setting.PaymentOrderAlertIntervalSeconds, itemPayload)
		if intervalErr != nil {
			logger.Warnw("notification_payment_order_alert_interval_failed", "error", intervalErr)
		}
		if intervalErr == nil && !allowed {
			continue
		}
		if err := s.dispatchSingleEvent(ctx, setting, itemPayload); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
