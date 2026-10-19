package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	notificationformat "github.com/dujiao-next/internal/modules/notification/application/format"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	settingsmessaging "github.com/dujiao-next/internal/modules/settings/schema/messaging"
	"github.com/dujiao-next/internal/queue"
	"github.com/hibiken/asynq"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type repairOrder struct {
	ID        uint   `json:"id"`
	OrderNo   string `json:"order_no"`
	PaymentID uint   `json:"payment_id"`
	Amount    string `json:"amount"`
}
type repairPlan struct {
	Incident string        `json:"incident"`
	UserID   uint          `json:"user_id"`
	Before   string        `json:"before"`
	After    string        `json:"after"`
	Orders   []repairOrder `json:"orders"`
}
type notificationQueue interface {
	EnqueueNotificationDispatch(queue.NotificationDispatchPayload, ...asynq.Option) error
}
type emailResult struct {
	OrderID uint   `json:"order_id"`
	State   string `json:"state"`
}
type repairReport struct {
	Apply   bool          `json:"apply"`
	UserID  uint          `json:"user_id"`
	Current string        `json:"current_total_spent"`
	Target  string        `json:"target_total_spent"`
	Updated bool          `json:"statistic_updated"`
	Emails  []emailResult `json:"emails"`
}

func readSetting(db *gorm.DB, key string, target interface{}) error {
	var row struct{ ValueJSON string }
	if err := db.Table("settings").Select("value_json").Where("key = ?", key).Take(&row).Error; err != nil {
		return err
	}
	return json.Unmarshal([]byte(row.ValueJSON), target)
}

func repair(db *gorm.DB, notifier notificationQueue, plan repairPlan, apply bool, purchaseFlag string) (repairReport, error) {
	r := repairReport{Apply: apply, UserID: plan.UserID, Target: plan.After}
	before, e1 := decimal.NewFromString(plan.Before)
	after, e2 := decimal.NewFromString(plan.After)
	if plan.Incident != "2026-10-16-paid-order-deadlock" || plan.UserID == 0 || len(plan.Orders) != 2 || e1 != nil || e2 != nil || !before.IsZero() || !after.GreaterThan(before) {
		return r, errors.New("plan must describe the reviewed two-order, zero-statistic incident")
	}
	if apply && (purchaseFlag != "false" || notifier == nil) {
		return r, errors.New("explicit purchase shutdown and notification-only queue are required")
	}
	var payloads []queue.NotificationDispatchPayload
	err := db.Transaction(func(tx *gorm.DB) error {
		var user userdomain.User
		if err := tx.First(&user, "id = ? AND deleted_at IS NULL", plan.UserID).Error; err != nil {
			return err
		}
		r.Current = user.TotalSpent.String()
		if !user.TotalSpent.Decimal.Equal(before) && !user.TotalSpent.Decimal.Equal(after) {
			return errors.New("statistic changed; refusing to overwrite")
		}
		var levels int64
		if err := tx.Table("member_levels").Count(&levels).Error; err != nil {
			return err
		}
		if levels != 0 {
			return errors.New("membership tiers require separate review")
		}
		var setting settingsmessaging.NotificationCenterSetting
		if err := readSetting(tx, "notification_center_config", &setting); err != nil {
			return err
		}
		var smtp struct {
			Enabled bool `json:"enabled"`
		}
		if err := readSetting(tx, "smtp_config", &smtp); err != nil {
			return err
		}
		if !smtp.Enabled || !setting.Scenes.OrderPaidSuccess || !setting.Channels.Email.Enabled || len(setting.Channels.Email.Recipients) != 1 || strings.TrimSpace(setting.Channels.Email.Recipients[0]) == "" || setting.Channels.Telegram.Enabled || setting.Channels.Feishu.Enabled {
			return errors.New("expected one existing merchant email destination and no other notification channels")
		}
		var paid []orderdomain.Order
		if err := tx.Where("user_id = ? AND parent_id IS NULL AND paid_at IS NOT NULL AND deleted_at IS NULL", plan.UserID).Find(&paid).Error; err != nil {
			return err
		}
		if len(paid) != len(plan.Orders) {
			return errors.New("paid order set changed")
		}
		seen := map[uint]bool{}
		total := decimal.Zero
		for _, expected := range plan.Orders {
			if expected.ID == 0 || expected.OrderNo == "" || expected.PaymentID == 0 || seen[expected.ID] {
				return errors.New("invalid or duplicate order identity")
			}
			seen[expected.ID] = true
			var order orderdomain.Order
			if err := tx.Preload("Items").Preload("Children.Items").First(&order, expected.ID).Error; err != nil {
				return err
			}
			amount, err := decimal.NewFromString(expected.Amount)
			if err != nil || !amount.IsPositive() || order.OrderNo != expected.OrderNo || order.UserID != plan.UserID || order.ParentID != nil || order.PaidAt == nil || order.DeletedAt != nil || order.Status != "delivered" || order.Currency != "CNY" || !order.RefundedAmount.Decimal.IsZero() || !order.TotalAmount.Decimal.Equal(amount) {
				return errors.New("order no longer matches reviewed paid and delivered state")
			}
			var payment paymentdomain.Payment
			if err := tx.First(&payment, "id = ? AND order_id = ? AND status = ? AND deleted_at IS NULL", expected.PaymentID, order.ID, "success").Error; err != nil {
				return errors.New("successful original payment required")
			}
			leaves := []orderdomain.Order{order}
			if len(order.Children) > 0 {
				leaves = order.Children
			}
			for _, leaf := range leaves {
				if leaf.Status != "delivered" || leaf.DeletedAt != nil {
					return errors.New("delivery no longer complete")
				}
				for _, item := range leaf.Items {
					if item.FulfillmentType != constants.FulfillmentTypeUpstream {
						continue
					}
					var count int64
					if err := tx.Table("procurement_orders").Where("local_order_id = ? AND status = 'fulfilled' AND upstream_order_no <> '' AND deleted_at IS NULL", leaf.ID).Count(&count).Error; err != nil {
						return err
					}
					if count != 1 {
						return errors.New("exactly one completed original purchase required")
					}
				}
			}
			total = total.Add(amount)
			var delivered int64
			if err := tx.Table("notification_logs").Where("event_type = ? AND biz_type = 'order' AND biz_id = ? AND channel = 'email' AND recipient = ? AND status = 'success' AND is_test = ?", constants.NotificationEventOrderPaidSuccess, order.ID, strings.TrimSpace(setting.Channels.Email.Recipients[0]), false).Count(&delivered).Error; err != nil {
				return err
			}
			state := "missing"
			if delivered > 0 {
				state = "already_sent"
			} else {
				payloads = append(payloads, payload(&order, &payment, &user, settingsmessaging.NormalizeNotificationLocale(setting.DefaultLocale)))
			}
			r.Emails = append(r.Emails, emailResult{OrderID: order.ID, State: state})
		}
		if !total.Equal(after) {
			return errors.New("reviewed orders do not sum to target statistic")
		}
		if apply && user.TotalSpent.Decimal.Equal(before) {
			update := tx.Table("users").Where("id = ? AND total_spent = ? AND deleted_at IS NULL", plan.UserID, before.String()).UpdateColumn("total_spent", after.String())
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected != 1 {
				return errors.New("concurrent statistic change")
			}
			r.Updated = true
		}
		return nil
	})
	if err != nil || !apply {
		return r, err
	}
	// Commit the compare-and-set before enqueueing. A queue failure is safe to
	// retry: the statistic is already the target and each task has a stable ID.
	for _, p := range payloads {
		err := notifier.EnqueueNotificationDispatch(p, asynq.TaskID(fmt.Sprintf("paid-order-repair-20261016-%d", p.BizID)), asynq.MaxRetry(3), asynq.Retention(30*24*time.Hour))
		state := "queued"
		if errors.Is(err, asynq.ErrTaskIDConflict) {
			state = "already_queued"
		} else if err != nil {
			return r, errors.New("notification enqueue failed; repeat the same plan safely")
		}
		for i := range r.Emails {
			if r.Emails[i].OrderID == p.BizID {
				r.Emails[i].State = state
			}
		}
	}
	return r, nil
}

func payload(order *orderdomain.Order, payment *paymentdomain.Payment, user *userdomain.User, locale string) queue.NotificationDispatchPayload {
	if len(order.Items) == 0 {
		for _, child := range order.Children {
			order.Items = append(order.Items, child.Items...)
		}
	}
	items, fulfillment, counts := notificationformat.BuildOrderItemSummaries(order.Items, locale)
	label := strings.TrimSpace(user.Email)
	if user.DisplayName != "" {
		label = user.DisplayName + " <" + label + ">"
	}
	data := map[string]interface{}{
		"order_id": fmt.Sprint(order.ID), "order_no": order.OrderNo, "user_id": fmt.Sprint(user.ID), "guest_email": order.GuestEmail,
		"amount": order.TotalAmount.String(), "currency": order.Currency, "order_status": order.Status,
		"customer_email": user.Email, "customer_label": label, "customer_type": "registered",
		"items_summary": items, "fulfillment_items_summary": fulfillment, "delivery_summary": notificationformat.BuildDeliverySummary(locale, counts),
		"item_count": fmt.Sprint(counts.Total), "auto_item_count": fmt.Sprint(counts.Auto), "manual_item_count": fmt.Sprint(counts.Manual), "upstream_item_count": fmt.Sprint(counts.Upstream),
		"payment_id": fmt.Sprint(payment.ID), "provider_type": payment.ProviderType, "channel_type": payment.ChannelType, "payment_channel": payment.ProviderType + "/" + payment.ChannelType,
	}
	return queue.NotificationDispatchPayload{EventType: constants.NotificationEventOrderPaidSuccess, BizType: "order", BizID: order.ID, Locale: locale, Force: false, Data: data}
}
