package wechatpayadapter

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/wechatpay"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// wechatpayAdapter 是 wechatpay 网关的 paymentcontract.GatewayProvider/paymentcontract.GatewayCapturer/paymentcontract.GatewayWebhooker 实现。
// 它仅做参数适配和错误映射，网关协议逻辑由同一 Payment 模块下的 wechatpay 基础设施包负责。
type wechatpayAdapter struct{}

// NewWechatpayAdapter 实例化 wechatpay adapter。
func NewWechatpayAdapter() paymentcontract.GatewayProvider { return &wechatpayAdapter{} }

// 编译期断言 wechatpayAdapter 实现了支付和安全诊断 capability interface。
var (
	_ paymentcontract.GatewayProvider       = (*wechatpayAdapter)(nil)
	_ paymentcontract.GatewayCapturer       = (*wechatpayAdapter)(nil)
	_ paymentcontract.GatewayWebhooker      = (*wechatpayAdapter)(nil)
	_ paymentcontract.GatewaySecurityTester = (*wechatpayAdapter)(nil)
)

// Type 返回 provider 标识。
func (a *wechatpayAdapter) Type() string {
	return constants.PaymentProviderOfficial + ":" + constants.PaymentChannelTypeWechat
}

// TestSecurity 使用微信支付官方非交易接口验证商户请求签名及微信支付公钥应答验签。
func (a *wechatpayAdapter) TestSecurity(ctx context.Context, raw jsonmap.JSON) (*paymentcontract.GatewaySecurityTestResult, error) {
	cfg, err := a.parseConfig(raw, "")
	if err != nil {
		return nil, err
	}
	result, err := wechatpay.TestWechatPayPublicKey(ctx, cfg)
	if err != nil {
		return nil, mapWechatpayError(err)
	}
	return &paymentcontract.GatewaySecurityTestResult{
		VerificationMode:         "wechatpay_public_key",
		ResponseSerial:           result.ResponseSerial,
		RequestSignatureAccepted: result.RequestSignatureAccepted,
		ResponseSignatureValid:   result.ResponseSignatureValid,
		EchoMessageMatched:       result.EchoMessageMatched,
	}, nil
}

// parseConfig 解析并验证 wechatpay Config，把 wechatpay.ErrConfigInvalid 等映射为 provider.ErrXxx。
// 4 个公开方法共用，避免每个都重复样板。
// 当 interactionMode 为空字符串时，跳过 wechatpay.ValidateConfig 对 interaction_mode 的校验，
// 仅做 ParseConfig。这用于 QueryPayment/ParseWebhook 等阶段，这些阶段不需要 interaction_mode，
// 但需要能正常解析 Config 以获取认证信息。
func (a *wechatpayAdapter) parseConfig(raw jsonmap.JSON, interactionMode string) (*wechatpay.Config, error) {
	cfg, err := wechatpay.ParseConfig(raw)
	if err != nil {
		return nil, mapWechatpayError(err)
	}
	// interactionMode="" 时跳过 wechatpay.ValidateConfig（它要求 interactionMode 必填，
	// 空字符串会被 IsSupportedInteractionMode 拒绝）。
	// QueryPayment/ParseWebhook 不需要 interactionMode，使用空字符串调用；
	// CreatePayment 和 ValidateConfig 传入具体 mode，触发完整校验。
	if interactionMode == "" {
		return cfg, nil
	}
	if err := wechatpay.ValidateConfig(cfg, interactionMode); err != nil {
		return nil, mapWechatpayError(err)
	}
	return cfg, nil
}

// ValidateConfig 验证 channel.ConfigJSON。
// admin 端 ValidateChannel 调用。service 层 official provider 分支传入 channel.InteractionMode，
// 所以第二参数实际上是 interactionMode（不是 channelType）。
// 如果 interactionMode 为空字符串（调用方未传），使用 QR 作为占位 default
// （QR 模式不要求 h5_redirect_url，对 config 字段完整性校验最宽松，
// IsSupportedInteractionMode 列表内）。
// 实际 interactionMode 在 CreatePayment 阶段从 input.Extra["interaction_mode"] 再次校验。
func (a *wechatpayAdapter) ValidateConfig(raw jsonmap.JSON, interactionMode string) error {
	mode := strings.TrimSpace(interactionMode)
	if mode == "" {
		mode = constants.PaymentInteractionQR
	}
	_, err := a.parseConfig(raw, mode)
	return err
}

// CreatePayment 创建支付。
func (a *wechatpayAdapter) CreatePayment(ctx context.Context, raw jsonmap.JSON, input paymentcontract.GatewayCreateInput) (*paymentcontract.GatewayCreateResult, error) {
	// 从 input.Extra 取 interaction_mode（jsapi/native/h5）
	interactionMode, _ := input.Extra["interaction_mode"].(string)
	cfg, err := a.parseConfig(raw, interactionMode)
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

	// P1.2c Task 3: wechat 回跳使用 cfg.H5RedirectURL，wrapper 没有直接访问该字段的路径，
	// 且 wechat H5 回跳时由微信自行携带 open_id 等参数，marker 附加无意义。
	// input.ReturnURLQuery 在此 adapter 中不使用；wechat 回跳 marker 由 callback handler 设计。
	native := wechatpay.CreateInput{
		OrderNo:     input.OrderNo,
		Amount:      payAmount,
		Currency:    payCurrency,
		Description: input.Subject,
		ClientIP:    input.ClientIP,
		NotifyURL:   input.NotifyURL,
	}
	result, err := wechatpay.CreatePayment(ctx, cfg, native, interactionMode)
	if err != nil {
		return nil, mapWechatpayError(err)
	}

	// wechat CreatePayment 阶段返回 PrepayID，但不是最终的 transaction_id。
	// 最终 transaction_id 在 Query 或 Webhook 时才出现。所以 ProviderRef 设为空，
	// PrepayID 和 PayURL/QRCode 入 Payload 供上游参考。
	payload := jsonmap.JSON{
		"prepay_id": result.PrepayID,
		"raw":       result.Raw,
	}
	if converted {
		payload["exchange_rate"] = strings.TrimSpace(cfg.ExchangeRate)
		payload["original_amount"] = originalAmount
		payload["original_currency"] = originalCurrency
	}

	return &paymentcontract.GatewayCreateResult{
		ProviderRef:  "",
		RedirectURL:  result.PayURL,
		QRCodeURL:    result.QRCode,
		Payload:      payload,
		AmountSent:   payAmount,
		CurrencySent: payCurrency,
	}, nil
}

// QueryPayment 主动查询订单状态(实现 paymentcontract.GatewayCapturer)。
// wechat 的 QueryOrderByOutTradeNo 用商户订单号查询，返回 TransactionID（wechat 的 transaction_id）。
// 调用方传入的 providerRef 实际就是 OrderNo（因为 CreatePayment 阶段没有 transaction_id）。
func (a *wechatpayAdapter) QueryPayment(ctx context.Context, raw jsonmap.JSON, providerRef string) (*paymentcontract.GatewayQueryResult, error) {
	cfg, err := a.parseConfig(raw, "")
	if err != nil {
		return nil, err
	}

	result, err := wechatpay.QueryOrderByOutTradeNo(ctx, cfg, providerRef)
	if err != nil {
		return nil, mapWechatpayError(err)
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
		ProviderRef: result.TransactionID,
		Status:      result.Status,
		Amount:      amount,
		Currency:    strings.ToUpper(strings.TrimSpace(result.Currency)),
		PaidAt:      result.PaidAt,
		Payload:     jsonmap.JSON(result.Raw),
	}, nil
}

// ParseWebhook 验签并解析 webhook(实现 paymentcontract.GatewayWebhooker)。
func (a *wechatpayAdapter) ParseWebhook(ctx context.Context, raw jsonmap.JSON, headers map[string]string, body []byte, _ time.Time) (*paymentcontract.GatewayCallbackResult, error) {
	cfg, err := a.parseConfig(raw, "")
	if err != nil {
		return nil, err
	}

	result, err := wechatpay.VerifyAndDecodeWebhook(ctx, cfg, headers, body)
	if err != nil {
		return nil, mapWechatpayError(err)
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
		ProviderRef: result.TransactionID,
		Status:      result.Status,
		Amount:      amount,
		Currency:    strings.ToUpper(strings.TrimSpace(result.Currency)),
		PaidAt:      result.PaidAt,
		Payload:     jsonmap.JSON(result.Raw),
	}, nil
}

func mapWechatpayError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, wechatpay.ErrConfigInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayConfigInvalid, err)
	case errors.Is(err, wechatpay.ErrRequestFailed):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayRequestFailed, err)
	case errors.Is(err, wechatpay.ErrResponseInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayResponseInvalid, err)
	case errors.Is(err, wechatpay.ErrSignatureInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewaySignatureInvalid, err)
	default:
		return err
	}
}
