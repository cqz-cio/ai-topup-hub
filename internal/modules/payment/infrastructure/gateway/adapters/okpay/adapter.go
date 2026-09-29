package okpayadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	gatewaycommon "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/common"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/okpay"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// okpayAdapter 是 okpay 网关的 paymentcontract.GatewayProvider + paymentcontract.GatewayCallbackVerifier 实现。
// 与 bepusdt 类似，支持多 channel type（usdt / trx），
// 需要根据 channelType 动态设置 cfg.Coin。
// callback 是同步 JSON POST（不是 form），所以**不**实现 paymentcontract.GatewayCapturer 和 paymentcontract.GatewayWebhooker。
type okpayAdapter struct{}

// NewOkpayAdapter 实例化 okpay adapter。
func NewOkpayAdapter() paymentcontract.GatewayProvider { return &okpayAdapter{} }

// 编译期断言 okpayAdapter 实现了 paymentcontract.GatewayProvider 和 paymentcontract.GatewayCallbackVerifier。
var (
	_ paymentcontract.GatewayProvider         = (*okpayAdapter)(nil)
	_ paymentcontract.GatewayCallbackVerifier = (*okpayAdapter)(nil)
)

// Type 返回 provider 标识。okpay 是多 channel type provider，返回值中 channelType 部分为空。
func (a *okpayAdapter) Type() string {
	return constants.PaymentProviderOkpay + ":"
}

// parseConfig 解析并验证 okpay Config。
// 关键：如果 cfg.Coin 未显式配置且 channelType 非空，
// 则从 channelType 自动 resolve coin（沿用 payment_service_provider.go 的逻辑）。
func (a *okpayAdapter) parseConfig(raw jsonmap.JSON, channelType string) (*okpay.Config, error) {
	cfg, err := okpay.ParseConfig(raw)
	if err != nil {
		return nil, mapOkpayError(err)
	}
	// 如果配置中没有指定 coin，根据 channel_type 自动设置
	if strings.TrimSpace(cfg.Coin) == "" && channelType != "" {
		cfg.Coin = okpay.ResolveCoin(channelType)
	}
	if err := okpay.ValidateConfig(cfg); err != nil {
		return nil, mapOkpayError(err)
	}
	return cfg, nil
}

// ValidateConfig 验证 channel.ConfigJSON。
// 入口先校验 channelType（如果非空）是否被 okpay 支持，
// 然后调用 parseConfig 验证配置完整性。
func (a *okpayAdapter) ValidateConfig(raw jsonmap.JSON, channelType string) error {
	if channelType != "" && !okpay.IsSupportedChannelType(channelType) {
		return fmt.Errorf("%w: okpay channel_type %s", paymentcontract.ErrGatewayUnsupportedChannel, channelType)
	}
	_, err := a.parseConfig(raw, channelType)
	return err
}

// CreatePayment 创建支付。okpay 多 channel type，需要先校验 channelType。
func (a *okpayAdapter) CreatePayment(ctx context.Context, raw jsonmap.JSON, input paymentcontract.GatewayCreateInput) (*paymentcontract.GatewayCreateResult, error) {
	// 先校验 channelType
	if input.ChannelType != "" && !okpay.IsSupportedChannelType(input.ChannelType) {
		return nil, fmt.Errorf("%w: okpay channel_type %s", paymentcontract.ErrGatewayUnsupportedChannel, input.ChannelType)
	}

	cfg, err := a.parseConfig(raw, input.ChannelType)
	if err != nil {
		return nil, err
	}

	// P1.2c C4 fix: okpay native CreatePayment 内部用 cfg.ExchangeRate 做 conversion，
	// 但不在 paymentcontract.GatewayCreateResult 中暴露转换后金额。wrapper 在 native 返回后自行重新计算，
	// 填 AmountSent/CurrencySent 供 service 层更新 payment.Amount/Currency，
	// 保证 callback 时 USDT amount 与 payment.Amount 对齐（避免 currency mismatch）。
	originalAmount := input.Amount.Decimal.String()
	originalCurrency := input.Currency

	returnURL := strings.TrimSpace(input.ReturnURL)
	if returnURL == "" {
		returnURL = strings.TrimSpace(cfg.ReturnURL)
	}
	returnURL = gatewaycommon.AppendQueryParams(returnURL, input.ReturnURLQuery)

	// 注意 okpay paymentcontract.GatewayCreateInput 字段命名独特
	native := okpay.CreateInput{
		UniqueID:    input.OrderNo,
		Name:        input.Subject,
		Amount:      originalAmount,
		ReturnURL:   returnURL,
		CallbackURL: cfg.CallbackURL, // 从 cfg 取（对齐现有 service 逻辑）
		Coin:        cfg.Coin,
		Status:      cfg.Status,
	}
	result, err := okpay.CreatePayment(ctx, cfg, native)
	if err != nil {
		return nil, mapOkpayError(err)
	}

	// 计算转换后金额/币种（与 native 内部使用相同的 ConvertAmountByRate 逻辑）。
	// okpay native 已做 conversion，wrapper 这里只是**重新计算一遍**以填 AmountSent/CurrencySent，
	// 不影响实际发给网关的数字（那由 native 决定）。
	amountSent := originalAmount
	currencySent := strings.ToUpper(strings.TrimSpace(originalCurrency))
	if coin := strings.ToUpper(strings.TrimSpace(cfg.Coin)); coin != "" {
		currencySent = coin
	}
	converted := false
	if rate := strings.TrimSpace(cfg.ExchangeRate); rate != "" && rate != "1" && rate != "1.0" {
		if convertedDec, convErr := okpay.ConvertAmountByRate(originalAmount, cfg.ExchangeRate); convErr == nil {
			amountSent = convertedDec.StringFixed(8)
			converted = true
		}
	}

	// 构造 Payload：先从 native raw 复制，再附加 audit 字段。
	payload := jsonmap.JSON{}
	for k, v := range result.Raw {
		payload[k] = v
	}
	if converted {
		payload["exchange_rate"] = strings.TrimSpace(cfg.ExchangeRate)
		payload["original_amount"] = originalAmount
		payload["original_currency"] = originalCurrency
	}

	// okpay 既支持 redirect 也支持 QR 展示，两者都用同一个 PayURL。
	// 上层通过 QRCode 字段展示二维码，所以 QRCodeURL 与 RedirectURL 保持一致。
	return &paymentcontract.GatewayCreateResult{
		ProviderRef:  result.OrderID,
		RedirectURL:  result.PayURL,
		QRCodeURL:    result.PayURL,
		Payload:      payload,
		AmountSent:   amountSent,
		CurrencySent: currencySent,
	}, nil
}

// VerifyCallback 实现 paymentcontract.GatewayCallbackVerifier。okpay 用 JSON POST body，form 参数忽略。
// 注意：callback 阶段不调 ValidateConfig——配置错误由签名校验兜底，
// 与 alipay/epay/epusdt/bepusdt/tokenpay adapter 行为一致。
func (a *okpayAdapter) VerifyCallback(raw jsonmap.JSON, _ map[string][]string, body []byte) (*paymentcontract.GatewayCallbackResult, error) {
	cfg, err := okpay.ParseConfig(raw)
	if err != nil {
		return nil, mapOkpayError(err)
	}

	data, err := okpay.ParseCallback(body)
	if err != nil {
		return nil, mapOkpayError(err)
	}

	if err := okpay.VerifyCallback(cfg, data); err != nil {
		return nil, mapOkpayError(err)
	}

	// okpay 状态映射：用 RequestStatus + PaymentStatus 两个字段拼接出最终 status
	status := okpay.ToPaymentStatus(data.RequestStatus, data.PaymentStatus)

	// amount 解析：CallbackData.Amount 是 string，直接 decimal.NewFromString
	// amount silent-fallback：wrapper 仅做适配，金额异常由业务层判定。
	amount := money.Amount{}
	if s := strings.TrimSpace(data.Amount); s != "" {
		if d, parseErr := decimal.NewFromString(s); parseErr == nil {
			amount = money.FromDecimal(d)
		}
	}

	// Currency 优先 callback data.Coin，fallback cfg.Coin(uppercase)
	currency := strings.ToUpper(strings.TrimSpace(data.Coin))
	if currency == "" {
		currency = strings.ToUpper(strings.TrimSpace(cfg.Coin))
	}

	// Payload 通过 json.Marshal+Unmarshal CallbackData 序列化
	payload := jsonmap.JSON{}
	if pb, marshalErr := json.Marshal(data); marshalErr == nil {
		var m map[string]interface{}
		if jsonErr := json.Unmarshal(pb, &m); jsonErr == nil {
			payload = jsonmap.JSON(m)
		}
	}

	return &paymentcontract.GatewayCallbackResult{
		OrderNo:     data.UniqueID,
		ProviderRef: data.OrderID,
		Status:      status,
		Amount:      amount,
		Currency:    currency,
		PaidAt:      nil, // okpay callback 不带付款时间
		Payload:     payload,
	}, nil
}

// mapOkpayError 把 okpay 包的 sentinel error 映射为 provider 统一错误。
// okpay 有 4 个 sentinel（同 epusdt，不像 bepusdt 多 ErrTradeTypeNotSupport）。
// unsupported channel 由 wrapper 入口 IsSupportedChannelType 校验拦截。
func mapOkpayError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, okpay.ErrConfigInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayConfigInvalid, err)
	case errors.Is(err, okpay.ErrRequestFailed):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayRequestFailed, err)
	case errors.Is(err, okpay.ErrResponseInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayResponseInvalid, err)
	case errors.Is(err, okpay.ErrSignatureInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewaySignatureInvalid, err)
	default:
		return err
	}
}
