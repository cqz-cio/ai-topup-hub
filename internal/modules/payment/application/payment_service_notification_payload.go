package application

import (
	"fmt"
	"strings"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	orderapp "github.com/dujiao-next/internal/modules/order/application"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"

	"github.com/dujiao-next/internal/constants"
	notificationformat "github.com/dujiao-next/internal/modules/notification/application/format"
	settingsmessaging "github.com/dujiao-next/internal/modules/settings/schema/messaging"
	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/shopspring/decimal"
)

func (s *PaymentService) buildOrderNotificationPayload(order *orderdomain.Order, payment *paymentdomain.Payment) jsonmap.JSON {
	locale := s.notificationTemplateLocale()
	customerEmail, customerLabel, customerType := s.resolveNotificationCustomer(order)
	// 父订单拆单时商品项可能只存在于子订单，通知变量需先补齐聚合商品明细。
	orderapp.FillOrderItemsFromChildren(order)
	itemsSummary, fulfillmentItemsSummary, counts := notificationformat.BuildOrderItemSummaries(order.Items, locale)
	providerType, channelType, paymentChannel := notificationPaymentChannel(order, payment)

	payload := jsonmap.JSON{
		"order_id":                  fmt.Sprintf("%d", order.ID),
		"order_no":                  strings.TrimSpace(order.OrderNo),
		"user_id":                   fmt.Sprintf("%d", order.UserID),
		"guest_email":               strings.TrimSpace(order.GuestEmail),
		"amount":                    order.TotalAmount.String(),
		"currency":                  strings.ToUpper(strings.TrimSpace(order.Currency)),
		"order_status":              strings.TrimSpace(order.Status),
		"customer_email":            customerEmail,
		"customer_label":            customerLabel,
		"customer_type":             customerType,
		"items_summary":             itemsSummary,
		"fulfillment_items_summary": fulfillmentItemsSummary,
		"delivery_summary":          notificationformat.BuildDeliverySummary(locale, counts),
		"item_count":                fmt.Sprintf("%d", counts.Total),
		"auto_item_count":           fmt.Sprintf("%d", counts.Auto),
		"manual_item_count":         fmt.Sprintf("%d", counts.Manual),
		"upstream_item_count":       fmt.Sprintf("%d", counts.Upstream),
		"payment_channel":           paymentChannel,
	}
	if payment != nil {
		payload["payment_id"] = fmt.Sprintf("%d", payment.ID)
	}
	if providerType != "" {
		payload["provider_type"] = providerType
	}
	if channelType != "" {
		payload["channel_type"] = channelType
	}
	return payload
}

func (s *PaymentService) buildWalletRechargeNotificationPayload(recharge *walletdomain.RechargeOrder, payment *paymentdomain.Payment) jsonmap.JSON {
	customerEmail, customerLabel := s.resolveUserNotificationIdentity(recharge.UserID)
	providerType := strings.TrimSpace(recharge.ProviderType)
	channelType := strings.TrimSpace(recharge.ChannelType)
	paymentChannel := providerType
	if paymentChannel != "" && channelType != "" {
		paymentChannel += "/" + channelType
	} else if paymentChannel == "" {
		paymentChannel = channelType
	}

	payload := jsonmap.JSON{
		"user_id":         fmt.Sprintf("%d", recharge.UserID),
		"recharge_id":     fmt.Sprintf("%d", recharge.ID),
		"recharge_no":     strings.TrimSpace(recharge.RechargeNo),
		"amount":          recharge.Amount.String(),
		"currency":        strings.ToUpper(strings.TrimSpace(recharge.Currency)),
		"provider_type":   providerType,
		"channel_type":    channelType,
		"payment_channel": paymentChannel,
		"customer_email":  customerEmail,
		"customer_label":  customerLabel,
	}
	if payment != nil {
		payload["payment_id"] = fmt.Sprintf("%d", payment.ID)
	}
	return payload
}

func (s *PaymentService) buildManualFulfillmentNotificationPayload(order *orderdomain.Order, parent *orderdomain.Order) jsonmap.JSON {
	payload := s.buildOrderNotificationPayload(order, nil)
	if parent != nil {
		payload["parent_order_id"] = fmt.Sprintf("%d", parent.ID)
		payload["parent_order_no"] = strings.TrimSpace(parent.OrderNo)
	}
	return payload
}

func (s *PaymentService) notificationTemplateLocale() string {
	if s == nil || s.settingService == nil {
		return constants.LocaleZhCN
	}
	setting, err := s.settingService.GetNotificationCenterSetting()
	if err != nil {
		return constants.LocaleZhCN
	}
	return settingsmessaging.NormalizeNotificationLocale(setting.DefaultLocale)
}

func (s *PaymentService) resolveNotificationCustomer(order *orderdomain.Order) (string, string, string) {
	if order == nil {
		return "", "", "guest"
	}
	if order.UserID == 0 {
		guestEmail := strings.TrimSpace(order.GuestEmail)
		return guestEmail, guestEmail, "guest"
	}
	email, label := s.resolveUserNotificationIdentity(order.UserID)
	if email == "" {
		email = strings.TrimSpace(order.GuestEmail)
	}
	if label == "" {
		label = email
	}
	if label == "" {
		label = fmt.Sprintf("user#%d", order.UserID)
	}
	return email, label, "registered"
}

func (s *PaymentService) resolveUserNotificationIdentity(userID uint) (string, string) {
	if userID == 0 || s == nil || s.userRepo == nil {
		return "", ""
	}
	user, err := s.userRepo.GetByID(userID)
	if err != nil || user == nil {
		return "", ""
	}
	email := strings.TrimSpace(user.Email)
	displayName := strings.TrimSpace(user.DisplayName)
	switch {
	case displayName != "" && email != "":
		return email, displayName + " <" + email + ">"
	case email != "":
		return email, email
	case displayName != "":
		return "", displayName
	default:
		return "", fmt.Sprintf("user#%d", userID)
	}
}

func notificationPaymentChannel(order *orderdomain.Order, payment *paymentdomain.Payment) (string, string, string) {
	providerType := ""
	channelType := ""
	if payment != nil {
		providerType = strings.TrimSpace(payment.ProviderType)
		channelType = strings.TrimSpace(payment.ChannelType)
		// display_channel_type 是创建支付时由 adapter 写入的通用展示覆盖值。
		// 例如 BEpusdt 交易模式数据库 channel_type 固定为 bepusdt，
		// 但通知里的支付渠道应展示为例如 bepusdt/usdt.arbitrum。
		if displayChannelType := notificationPayloadString(payment.ProviderPayload, "display_channel_type"); displayChannelType != "" {
			channelType = displayChannelType
		}
	}
	if providerType == "" && order != nil && order.WalletPaidAmount.Decimal.GreaterThan(decimal.Zero) {
		providerType = constants.PaymentProviderWallet
		channelType = constants.PaymentChannelTypeBalance
	}
	paymentChannel := providerType
	if paymentChannel != "" && channelType != "" {
		paymentChannel += "/" + channelType
	} else if paymentChannel == "" {
		paymentChannel = channelType
	}
	return providerType, channelType, paymentChannel
}

// notificationPayloadString 从通知相关 payload 中安全读取字符串字段。
// payload 可能来自 JSON 反序列化，值类型不固定，因此统一 fmt.Sprint 后 trim；
// 缺失或 nil 时返回空字符串，避免 fmt.Sprint(nil) 变成 "<nil>"。
func notificationPayloadString(payload map[string]interface{}, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}
