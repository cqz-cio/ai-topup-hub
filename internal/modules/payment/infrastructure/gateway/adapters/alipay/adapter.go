package alipayadapter

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	gatewaycommon "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/common"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/alipay"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// alipayLocation 是 alipay 网关 gmt_payment 字段使用的固定时区(GMT+8)。
// alipay 回调的 gmt_payment 字段没有 timezone marker，必须用 ParseInLocation
// 显式指定；否则 time.Parse 默认 UTC，PaidAt 会偏 8 小时，导致下游对账、
// 订单完成时间标记、webhook payload 全错。
var alipayLocation = time.FixedZone("CST", 8*3600)

// alipayAdapter 是 alipay 网关的 paymentcontract.GatewayProvider + paymentcontract.GatewayCallbackVerifier 实现。
// alipay 没有主动查询 API，callback 是同步 form POST（不是 JSON webhook），
// 所以**不**实现 paymentcontract.GatewayCapturer 和 paymentcontract.GatewayWebhooker。
type alipayAdapter struct{}

// NewAlipayAdapter 实例化 alipay adapter。
func NewAlipayAdapter() paymentcontract.GatewayProvider { return &alipayAdapter{} }

// 编译期断言 alipayAdapter 实现了 paymentcontract.GatewayProvider 和 paymentcontract.GatewayCallbackVerifier。
var (
	_ paymentcontract.GatewayProvider         = (*alipayAdapter)(nil)
	_ paymentcontract.GatewayCallbackVerifier = (*alipayAdapter)(nil)
)

// Type 返回 provider 标识。
func (a *alipayAdapter) Type() string {
	return constants.PaymentProviderOfficial + ":" + constants.PaymentChannelTypeAlipay
}

// parseConfig 解析并验证 alipay Config。interactionMode 影响 ValidateConfig
// 是否要求 return_url（jump 模式必填）。
func (a *alipayAdapter) parseConfig(raw jsonmap.JSON, interactionMode string) (*alipay.Config, error) {
	cfg, err := alipay.ParseConfig(raw)
	if err != nil {
		return nil, mapAlipayError(err)
	}
	if err := alipay.ValidateConfig(cfg, interactionMode); err != nil {
		return nil, mapAlipayError(err)
	}
	return cfg, nil
}

// ValidateConfig 验证 channel.ConfigJSON。
// admin 端 ValidateChannel 调用。service 层 official provider 分支传入 channel.InteractionMode，
// 所以第二参数实际上是 interactionMode（不是 channelType）。
// 如果 interactionMode 为空字符串（调用方未传），使用 QR 作为占位 default
// （QR 模式不要求 return_url，对 config 字段完整性校验最宽松，
// IsSupportedInteractionMode 列表内）。
// 实际 interactionMode 在 CreatePayment 阶段从 input.Extra["interaction_mode"] 再次校验。
func (a *alipayAdapter) ValidateConfig(raw jsonmap.JSON, interactionMode string) error {
	mode := strings.TrimSpace(interactionMode)
	if mode == "" {
		mode = constants.PaymentInteractionQR
	}
	_, err := a.parseConfig(raw, mode)
	return err
}

// CreatePayment 创建支付。
func (a *alipayAdapter) CreatePayment(ctx context.Context, raw jsonmap.JSON, input paymentcontract.GatewayCreateInput) (*paymentcontract.GatewayCreateResult, error) {
	// 从 input.Extra 取 interaction_mode（jump / qr）。
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

	// P1.2c Task 3: 先 fallback 到 cfg.ReturnURL，再 append tracking marker。
	returnURL := strings.TrimSpace(input.ReturnURL)
	if returnURL == "" {
		returnURL = strings.TrimSpace(cfg.ReturnURL)
	}
	returnURL = gatewaycommon.AppendQueryParams(returnURL, input.ReturnURLQuery)

	native := alipay.CreateInput{
		OrderNo:   input.OrderNo,
		Amount:    payAmount,
		Subject:   input.Subject,
		NotifyURL: input.NotifyURL,
		ReturnURL: returnURL,
	}
	result, err := alipay.CreatePayment(ctx, cfg, native, interactionMode)
	if err != nil {
		return nil, mapAlipayError(err)
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
		ProviderRef:  result.TradeNo,
		RedirectURL:  result.PayURL,
		QRCodeURL:    result.QRCode,
		Payload:      payload,
		AmountSent:   payAmount,
		CurrencySent: payCurrency,
	}, nil
}

// VerifyCallback 实现 paymentcontract.GatewayCallbackVerifier。alipay 用 form POST，body 参数忽略。
// 依次执行：
//  1. alipay.VerifyCallback   — 签名验证（防篡改）
//  2. alipay.VerifyCallbackOwnership — 归属校验（防跨商户注入：app_id 必须与配置一致）
func (a *alipayAdapter) VerifyCallback(raw jsonmap.JSON, form map[string][]string, _ []byte) (*paymentcontract.GatewayCallbackResult, error) {
	cfg, err := alipay.ParseConfig(raw)
	if err != nil {
		return nil, mapAlipayError(err)
	}
	// 注意：这里不调 alipay.ValidateConfig（因为没有 interactionMode 上下文），
	// 直接走 VerifyCallback，失败由 alipay 包内部抛 paymentcontract.ErrGatewaySignatureInvalid。

	if err := alipay.VerifyCallback(cfg, form); err != nil {
		return nil, mapAlipayError(err)
	}
	if err := alipay.VerifyCallbackOwnership(cfg, form); err != nil {
		return nil, mapAlipayError(err)
	}

	orderNo := gatewaycommon.FormValue(form, "out_trade_no")
	providerRef := gatewaycommon.FormValue(form, "trade_no")
	tradeStatus := gatewaycommon.FormValue(form, "trade_status")
	amountStr := gatewaycommon.FormValue(form, "total_amount")
	paidAtRaw := gatewaycommon.FormValue(form, "gmt_payment")

	status := constants.PaymentStatusPending
	if tradeStatus == "TRADE_SUCCESS" || tradeStatus == "TRADE_FINISHED" {
		status = constants.PaymentStatusSuccess
	}

	// amount 解析失败时返回零值：wrapper 仅做适配，金额异常的语义边界由业务层判定。
	amount := money.Amount{}
	if s := strings.TrimSpace(amountStr); s != "" {
		if d, parseErr := decimal.NewFromString(s); parseErr == nil {
			amount = money.FromDecimal(d)
		}
	}

	var paidAt *time.Time
	if t, parseErr := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(paidAtRaw), alipayLocation); parseErr == nil {
		paidAt = &t
	}

	return &paymentcontract.GatewayCallbackResult{
		OrderNo:     orderNo,
		ProviderRef: providerRef,
		Status:      status,
		Amount:      amount,
		Currency:    "CNY",
		PaidAt:      paidAt,
		Payload:     gatewaycommon.FormToJSON(form),
	}, nil
}

// mapAlipayError 把 alipay 包的 sentinel error 映射为 provider 统一错误。
func mapAlipayError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, alipay.ErrConfigInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayConfigInvalid, err)
	case errors.Is(err, alipay.ErrSignGenerate):
		// 签名生成失败 ≈ 配置错误（private_key 不可用）。
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayConfigInvalid, err)
	case errors.Is(err, alipay.ErrRequestFailed):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayRequestFailed, err)
	case errors.Is(err, alipay.ErrResponseInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayResponseInvalid, err)
	case errors.Is(err, alipay.ErrSignatureInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewaySignatureInvalid, err)
	default:
		return err
	}
}
