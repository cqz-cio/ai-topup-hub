package stripeadapter

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	gatewaycommon "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/common"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/stripe"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// stripeAdapter 是 stripe 网关的 paymentcontract.GatewayProvider/paymentcontract.GatewayCapturer/paymentcontract.GatewayWebhooker 实现。
// 它仅做参数适配和错误映射，网关协议逻辑由同一 Payment 模块下的 stripe 基础设施包负责。
type stripeAdapter struct{}

// NewStripeAdapter 实例化 stripe adapter。
func NewStripeAdapter() paymentcontract.GatewayProvider { return &stripeAdapter{} }

// 编译期断言 stripeAdapter 实现了三个 capability interface。
var (
	_ paymentcontract.GatewayProvider  = (*stripeAdapter)(nil)
	_ paymentcontract.GatewayCapturer  = (*stripeAdapter)(nil)
	_ paymentcontract.GatewayWebhooker = (*stripeAdapter)(nil)
)

// Type 返回 provider 标识。
func (a *stripeAdapter) Type() string {
	return constants.PaymentProviderOfficial + ":" + constants.PaymentChannelTypeStripe
}

// parseConfig 解析并验证 stripe Config，把 stripe.ErrConfigInvalid 等映射为 provider.ErrXxx。
// 4 个公开方法共用，避免每个都重复 6 行样板。
func (a *stripeAdapter) parseConfig(raw jsonmap.JSON) (*stripe.Config, error) {
	cfg, err := stripe.ParseConfig(raw)
	if err != nil {
		return nil, mapStripeError(err)
	}
	if err := stripe.ValidateConfig(cfg); err != nil {
		return nil, mapStripeError(err)
	}
	return cfg, nil
}

// ValidateConfig 验证 channel.ConfigJSON。
// 第二参数 interactionMode 由 admin 端 ValidateChannel 传入；stripe 只支持 redirect 模式。
// 若传空字符串（非 admin 端调用），不做 interactionMode 校验，以保持向后兼容。
func (a *stripeAdapter) ValidateConfig(raw jsonmap.JSON, interactionMode string) error {
	if interactionMode != "" && strings.ToLower(strings.TrimSpace(interactionMode)) != constants.PaymentInteractionRedirect {
		return fmt.Errorf("%w: stripe only supports redirect interaction_mode", paymentcontract.ErrGatewayConfigInvalid)
	}
	_, err := a.parseConfig(raw)
	return err
}

// CreatePayment 创建支付。
func (a *stripeAdapter) CreatePayment(ctx context.Context, raw jsonmap.JSON, input paymentcontract.GatewayCreateInput) (*paymentcontract.GatewayCreateResult, error) {
	cfg, err := a.parseConfig(raw)
	if err != nil {
		return nil, err
	}

	// P1.2c: wrapper 内做 currency conversion + audit 字段写入。
	// exchange_rate / original_amount / original_currency 保留到 result.Payload，
	// 供运营/财务跨币种对账追溯实际收费 vs 原始金额。
	// result.AmountSent/CurrencySent 反映实际发给网关的金额/币种，
	// 让 service 层据此更新 payment.Amount/Currency，保持记录与实际收费一致。
	originalAmount := input.Amount.Decimal.String()
	originalCurrency := input.Currency
	payAmount := originalAmount
	payCurrency := originalCurrency
	converted := false
	if cfg.NeedsCurrencyConversion() {
		convAmount, convCurrency, convErr := cfg.ConvertAmount(payAmount, payCurrency, 2)
		if convErr != nil {
			return nil, fmt.Errorf("%w: %v", paymentcontract.ErrGatewayConfigInvalid, convErr)
		}
		payAmount = convAmount
		payCurrency = convCurrency
		converted = true
	}

	// P1.2c Task 3: 先 fallback 到 cfg.SuccessURL，再 append tracking marker。
	// stripe 用 SuccessURL 对应 ReturnURL。CancelURL 不 append marker（取消路径无需识别 biz_type）。
	successURL := strings.TrimSpace(input.ReturnURL)
	if successURL == "" {
		successURL = strings.TrimSpace(cfg.SuccessURL)
	}
	successURL = gatewaycommon.AppendQueryParams(successURL, input.ReturnURLQuery)

	cancelURL, _ := input.Extra["cancel_url"].(string)
	native := stripe.CreateInput{
		OrderNo:     input.OrderNo,
		Amount:      payAmount,
		Currency:    payCurrency,
		Description: input.Subject,
		Email:       strings.TrimSpace(input.Email),
		SuccessURL:  successURL,
		CancelURL:   cancelURL,
	}
	result, err := stripe.CreatePayment(ctx, cfg, native)
	if err != nil {
		return nil, mapStripeError(err)
	}

	payload := jsonmap.JSON{}
	if result.Raw != nil {
		payload = jsonmap.JSON(result.Raw)
	}
	if converted {
		payload["exchange_rate"] = strings.TrimSpace(cfg.ExchangeRate)
		payload["original_amount"] = originalAmount
		payload["original_currency"] = originalCurrency
	}

	return &paymentcontract.GatewayCreateResult{
		ProviderRef:  gatewaycommon.PickFirstNonEmpty(result.SessionID, result.PaymentIntentID),
		RedirectURL:  result.URL,
		Payload:      payload,
		AmountSent:   payAmount,
		CurrencySent: payCurrency,
	}, nil
}

// QueryPayment 主动查询订单状态(实现 paymentcontract.GatewayCapturer)。
func (a *stripeAdapter) QueryPayment(ctx context.Context, raw jsonmap.JSON, providerRef string) (*paymentcontract.GatewayQueryResult, error) {
	cfg, err := a.parseConfig(raw)
	if err != nil {
		return nil, err
	}

	result, err := stripe.QueryPayment(ctx, cfg, providerRef)
	if err != nil {
		return nil, mapStripeError(err)
	}

	// amount 解析失败时返回零值：wrapper 仅做适配，金额异常的语义边界(对账失败 / 网关返回脏数据)
	// 留给上游业务层判定，wrapper 不擅自报错。
	amount := money.Amount{}
	if s := strings.TrimSpace(result.Amount); s != "" {
		if parsed, parseErr := decimal.NewFromString(s); parseErr == nil {
			amount = money.FromDecimal(parsed)
		}
	}

	return &paymentcontract.GatewayQueryResult{
		ProviderRef: gatewaycommon.PickFirstNonEmpty(result.SessionID, result.PaymentIntentID, providerRef),
		Status:      result.Status,
		Amount:      amount,
		Currency:    strings.ToUpper(strings.TrimSpace(result.Currency)),
		PaidAt:      result.PaidAt,
		Payload:     jsonmap.JSON(result.Raw),
	}, nil
}

// ParseWebhook 验签并解析 webhook(实现 paymentcontract.GatewayWebhooker)。
func (a *stripeAdapter) ParseWebhook(_ context.Context, raw jsonmap.JSON, headers map[string]string, body []byte, now time.Time) (*paymentcontract.GatewayCallbackResult, error) {
	cfg, err := a.parseConfig(raw)
	if err != nil {
		return nil, err
	}

	result, err := stripe.VerifyAndParseWebhook(cfg, headers, body, now)
	if err != nil {
		return nil, mapStripeError(err)
	}

	// amount 解析失败时返回零值：wrapper 仅做适配，金额异常的语义边界(对账失败 / 网关返回脏数据)
	// 留给上游业务层判定，wrapper 不擅自报错。
	amount := money.Amount{}
	if s := strings.TrimSpace(result.Amount); s != "" {
		if parsed, parseErr := decimal.NewFromString(s); parseErr == nil {
			amount = money.FromDecimal(parsed)
		}
	}

	return &paymentcontract.GatewayCallbackResult{
		OrderNo:     result.OrderNo,
		ProviderRef: gatewaycommon.PickFirstNonEmpty(result.ProviderRef, result.SessionID, result.PaymentIntentID),
		Status:      result.Status,
		Amount:      amount,
		Currency:    strings.ToUpper(strings.TrimSpace(result.Currency)),
		PaidAt:      result.PaidAt,
		Payload:     jsonmap.JSON(result.Raw),
	}, nil
}

func mapStripeError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, stripe.ErrConfigInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayConfigInvalid, err)
	case errors.Is(err, stripe.ErrRequestFailed):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayRequestFailed, err)
	case errors.Is(err, stripe.ErrResponseInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayResponseInvalid, err)
	case errors.Is(err, stripe.ErrSignatureInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewaySignatureInvalid, err)
	default:
		return err
	}
}
