package paypaladapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	gatewaycommon "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/common"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/paypal"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// paypalAdapter 是 paypal 网关的 paymentcontract.GatewayProvider/paymentcontract.GatewayCapturer/paymentcontract.GatewayWebhooker 实现。
// 仅做参数适配和错误映射，网关协议逻辑由同一 Payment 模块下的 paypal 基础设施包负责。
type paypalAdapter struct{}

// NewPaypalAdapter 实例化 paypal adapter。
func NewPaypalAdapter() paymentcontract.GatewayProvider { return &paypalAdapter{} }

// 编译期断言 paypalAdapter 实现了三个 capability interface。
var (
	_ paymentcontract.GatewayProvider  = (*paypalAdapter)(nil)
	_ paymentcontract.GatewayCapturer  = (*paypalAdapter)(nil)
	_ paymentcontract.GatewayWebhooker = (*paypalAdapter)(nil)
)

// Type 返回 provider 标识。
func (a *paypalAdapter) Type() string {
	return constants.PaymentProviderOfficial + ":" + constants.PaymentChannelTypePaypal
}

// parseConfig 解析并验证 paypal Config，把 paypal.ErrConfigInvalid 等映射为 provider.ErrXxx。
// 4 个公开方法共用，避免每个都重复 6 行样板。
func (a *paypalAdapter) parseConfig(raw jsonmap.JSON) (*paypal.Config, error) {
	cfg, err := paypal.ParseConfig(raw)
	if err != nil {
		return nil, mapPaypalError(err)
	}
	if err := paypal.ValidateConfig(cfg); err != nil {
		return nil, mapPaypalError(err)
	}
	return cfg, nil
}

// ValidateConfig 验证 channel.ConfigJSON。
func (a *paypalAdapter) ValidateConfig(raw jsonmap.JSON, _ string) error {
	_, err := a.parseConfig(raw)
	return err
}

// CreatePayment 创建支付。
func (a *paypalAdapter) CreatePayment(ctx context.Context, raw jsonmap.JSON, input paymentcontract.GatewayCreateInput) (*paymentcontract.GatewayCreateResult, error) {
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
		convAmount, convCurrency, convErr := cfg.ConvertAmount(payAmount, payCurrency)
		if convErr != nil {
			return nil, fmt.Errorf("%w: %v", paymentcontract.ErrGatewayConfigInvalid, convErr)
		}
		payAmount = convAmount
		payCurrency = convCurrency
		converted = true
	}

	// P1.2c Task 3: 先 fallback 到 cfg.ReturnURL，再 append tracking marker。
	returnURL := strings.TrimSpace(input.ReturnURL)
	if returnURL == "" {
		returnURL = strings.TrimSpace(cfg.ReturnURL)
	}
	returnURL = gatewaycommon.AppendQueryParams(returnURL, input.ReturnURLQuery)

	cancelURL, _ := input.Extra["cancel_url"].(string)
	native := paypal.CreateInput{
		OrderNo:     input.OrderNo,
		Amount:      payAmount,
		Currency:    payCurrency,
		Description: input.Subject,
		ReturnURL:   returnURL,
		CancelURL:   cancelURL,
	}
	result, err := paypal.CreateOrder(ctx, cfg, native)
	if err != nil {
		return nil, mapPaypalError(err)
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
		ProviderRef:  result.OrderID,
		RedirectURL:  result.ApprovalURL,
		Payload:      payload,
		AmountSent:   payAmount,
		CurrencySent: payCurrency,
	}, nil
}

// QueryPayment 调用 paypal.CaptureOrder 完成捕获并返回状态（实现 paymentcontract.GatewayCapturer）。
func (a *paypalAdapter) QueryPayment(ctx context.Context, raw jsonmap.JSON, providerRef string) (*paymentcontract.GatewayQueryResult, error) {
	cfg, err := a.parseConfig(raw)
	if err != nil {
		return nil, err
	}

	result, err := paypal.CaptureOrder(ctx, cfg, providerRef)
	if err != nil {
		return nil, mapPaypalError(err)
	}

	// amount 解析失败时返回零值：wrapper 仅做适配，金额异常的语义边界（对账失败 / 网关返回脏数据）
	// 留给上游业务层判定，wrapper 不擅自报错。
	amount := money.Amount{}
	if s := strings.TrimSpace(result.Amount); s != "" {
		if parsed, parseErr := decimal.NewFromString(s); parseErr == nil {
			amount = money.FromDecimal(parsed)
		}
	}

	return &paymentcontract.GatewayQueryResult{
		ProviderRef: gatewaycommon.PickFirstNonEmpty(result.OrderID, providerRef),
		Status:      result.Status,
		Amount:      amount,
		Currency:    strings.ToUpper(strings.TrimSpace(result.Currency)),
		PaidAt:      result.PaidAt,
		Payload:     jsonmap.JSON(result.Raw),
	}, nil
}

// ParseWebhook 合并 paypal 的 VerifyWebhookSignature + ParseWebhookEvent 两步（stripe 是一步）。
func (a *paypalAdapter) ParseWebhook(ctx context.Context, raw jsonmap.JSON, headers map[string]string, body []byte, _ time.Time) (*paymentcontract.GatewayCallbackResult, error) {
	cfg, err := a.parseConfig(raw)
	if err != nil {
		return nil, err
	}

	var event map[string]interface{}
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, fmt.Errorf("%w: webhook body not valid JSON: %v", paymentcontract.ErrGatewayResponseInvalid, err)
	}

	httpHeaders := http.Header{}
	for k, v := range headers {
		httpHeaders.Set(k, v)
	}

	if err := paypal.VerifyWebhookSignature(ctx, cfg, httpHeaders, body); err != nil {
		return nil, mapPaypalError(err)
	}

	parsed, err := paypal.ParseWebhookEvent(body)
	if err != nil {
		return nil, mapPaypalError(err)
	}

	status, _ := paypal.ToPaymentStatus(parsed.EventType, parsed.ResourceStatus())

	// amount 解析失败时返回零值：wrapper 仅做适配，金额异常的语义边界（对账失败 / 网关返回脏数据）
	// 留给上游业务层判定，wrapper 不擅自报错。
	amount := money.Amount{}
	rawAmount, currency := parsed.CaptureAmount()
	if s := strings.TrimSpace(rawAmount); s != "" {
		if d, parseErr := decimal.NewFromString(s); parseErr == nil {
			amount = money.FromDecimal(d)
		}
	}

	return &paymentcontract.GatewayCallbackResult{
		OrderNo:     parsed.RelatedInvoiceID(),
		ProviderRef: parsed.RelatedOrderID(),
		Status:      status,
		Amount:      amount,
		Currency:    strings.ToUpper(strings.TrimSpace(currency)),
		PaidAt:      parsed.PaidAt(),
		Payload:     jsonmap.JSON(map[string]interface{}{"event": event}),
	}, nil
}

func mapPaypalError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, paypal.ErrConfigInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayConfigInvalid, err)
	case errors.Is(err, paypal.ErrAuthFailed):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayAuthFailed, err)
	case errors.Is(err, paypal.ErrRequestFailed):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayRequestFailed, err)
	case errors.Is(err, paypal.ErrResponseInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayResponseInvalid, err)
	case errors.Is(err, paypal.ErrWebhookVerifyFailed):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewaySignatureInvalid, err)
	default:
		return err
	}
}
