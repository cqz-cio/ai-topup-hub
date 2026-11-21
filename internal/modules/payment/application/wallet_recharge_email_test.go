package application

import (
	"strings"
	"testing"

	"github.com/dujiao-next/internal/constants"
	notificationformat "github.com/dujiao-next/internal/modules/notification/application/format"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	settingsmessaging "github.com/dujiao-next/internal/modules/settings/schema/messaging"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	"github.com/dujiao-next/internal/queue"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

func TestWalletRechargeEmailAmounts(t *testing.T) {
	m := func(s string) money.Amount { return money.FromDecimal(decimal.RequireFromString(s)) }
	localTemplate := settingsmessaging.NotificationCenterDefaultSetting().Templates.WalletRechargeSuccess.ZHCN.Body
	cases := []struct {
		name, provider, want string
		payment              *paymentdomain.Payment
	}{
		{"customer surcharge", "xorpay", "660.40 CNY", &paymentdomain.Payment{Amount: m("660.40"), Currency: "cny"}},
		{"merchant absorbs fee", "xorpay", "650.00 CNY", &paymentdomain.Payment{Amount: m("650"), Currency: "CNY"}},
		{"gateway currency", "official", "97.50 USD", &paymentdomain.Payment{Amount: m("97.50"), Currency: "USD"}},
		{"recharge snapshot", "xorpay", "661.11 CNY", nil},
		{"exact token quantity", "plasmausdt", "97.746800 USDT0", &paymentdomain.Payment{Amount: m("650"), Currency: "CNY", ProviderPayload: jsonmap.JSON{"chain_amount": "97.746800", "token_id": "USDT0"}}},
		{"missing token quantity", "tronusdt", "—", &paymentdomain.Payment{Amount: m("650"), Currency: "CNY"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recharge := &walletdomain.RechargeOrder{Amount: m("650"), PayableAmount: m("661.11"), Currency: "CNY", ProviderType: tc.provider}
			vars := (&PaymentService{}).buildWalletRechargeNotificationPayload(recharge, tc.payment)
			body := notificationformat.RenderTemplate(localTemplate, vars)
			if !strings.Contains(body, "充值金额：650.00 CNY\n用户实付金额："+tc.want+"\n") {
				t.Fatalf("unexpected recharge amounts in %q", body)
			}
		})
	}
}

func TestWalletRechargeEmailSavedTemplatesAndOlderJobs(t *testing.T) {
	defaults := settingsmessaging.NotificationCenterDefaultSetting()
	oldBody := "用户：{{customer_label}}\n邮箱：{{customer_email}}\n充值单号：{{recharge_no}}\n充值金额：{{amount}} {{currency}}\n支付渠道：{{payment_channel}}"
	custom := "Our receipt {{recharge_no}}"
	raw := jsonmap.JSON{"templates": map[string]interface{}{"wallet_recharge_success": map[string]interface{}{
		"zh-CN": map[string]interface{}{"title": "自定义标题", "body": strings.ReplaceAll(oldBody, "\n", "\r\n")},
		"en-US": map[string]interface{}{"body": custom},
	}}}
	saved := settingsmessaging.DecodeNotificationCenterSetting(raw, defaults).Templates.WalletRechargeSuccess
	if saved.ZHCN.Title != "自定义标题" || saved.ZHCN.Body != defaults.Templates.WalletRechargeSuccess.ZHCN.Body || saved.ENUS.Body != custom {
		t.Fatal("saved default body should upgrade while custom text is preserved")
	}
	legacy := notificationformat.BuildTemplateVariables(queue.NotificationDispatchPayload{EventType: constants.NotificationEventWalletRechargeSuccess})
	if legacy["paid_amount"] != "—" {
		t.Fatalf("legacy task without a recorded payment amount: %v", legacy["paid_amount"])
	}
	for _, locale := range []string{constants.LocaleZhCN, constants.LocaleZhTW, constants.LocaleEnUS} {
		vars := notificationformat.BuildTestVariables(constants.NotificationEventWalletRechargeSuccess, locale)
		body := notificationformat.RenderTemplate(defaults.Templates.WalletRechargeSuccess.ResolveLocaleTemplate(locale).Body, vars)
		if !strings.Contains(body, "100.00 USD") || !strings.Contains(body, "101.60 USD") {
			t.Fatalf("%s preview omitted an amount: %s", locale, body)
		}
		partial := map[string]interface{}{"recharge_no": "WR-EXAMPLE", "amount": "650.00"}
		notificationformat.ApplyTestVariables(partial, vars)
		if partial["paid_amount"] != "—" {
			t.Fatalf("preview mixed an example payment into a real recharge: %v", partial["paid_amount"])
		}
	}
}
