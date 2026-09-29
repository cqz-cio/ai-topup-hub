package epusdtadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	gatewaycommon "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/common"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/epusdt"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// epusdtAdapter 是 epusdt 网关的 paymentcontract.GatewayProvider + paymentcontract.GatewayCallbackVerifier 实现。
// epusdt 没有主动查询 API，callback 是同步 JSON POST（不是 form），
// 所以**不**实现 paymentcontract.GatewayCapturer 和 paymentcontract.GatewayWebhooker。
type epusdtAdapter struct{}

// NewEpusdtAdapter 实例化 epusdt adapter。
func NewEpusdtAdapter() paymentcontract.GatewayProvider { return &epusdtAdapter{} }

// 编译期断言 epusdtAdapter 实现了 paymentcontract.GatewayProvider 和 paymentcontract.GatewayCallbackVerifier。
var (
	_ paymentcontract.GatewayProvider         = (*epusdtAdapter)(nil)
	_ paymentcontract.GatewayCallbackVerifier = (*epusdtAdapter)(nil)
)

// Type 返回 provider 标识。epusdt 的 channel_type 统一为 epusdt，实际币种和网络保存在配置中；
// 注册表使用空 channelType 通配，以兼容历史 usdt-trc20、trx 等渠道记录。
func (a *epusdtAdapter) Type() string {
	return constants.PaymentProviderEpusdt + ":"
}

// parseConfig 解析并验证 epusdt Config。epusdt 不需要 interactionMode。
func (a *epusdtAdapter) parseConfig(raw jsonmap.JSON) (*epusdt.Config, error) {
	cfg, err := epusdt.ParseConfig(raw)
	if err != nil {
		return nil, mapEpusdtError(err)
	}
	if err := epusdt.ValidateConfig(cfg); err != nil {
		return nil, mapEpusdtError(err)
	}
	return cfg, nil
}

// ValidateConfig 验证 channel.ConfigJSON。
func (a *epusdtAdapter) ValidateConfig(raw jsonmap.JSON, _ string) error {
	_, err := a.parseConfig(raw)
	return err
}

// CreatePayment 创建支付。epusdt 的实际币种和网络由配置决定，不依赖 channel_type。
func (a *epusdtAdapter) CreatePayment(ctx context.Context, raw jsonmap.JSON, input paymentcontract.GatewayCreateInput) (*paymentcontract.GatewayCreateResult, error) {
	cfg, err := a.parseConfig(raw)
	if err != nil {
		return nil, err
	}

	returnURL := strings.TrimSpace(input.ReturnURL)
	if returnURL == "" {
		returnURL = strings.TrimSpace(cfg.ReturnURL)
	}
	returnURL = gatewaycommon.AppendQueryParams(returnURL, input.ReturnURLQuery)

	native := epusdt.CreateInput{
		OrderNo:   input.OrderNo,
		Amount:    input.Amount.Decimal.String(),
		Name:      input.Subject,
		NotifyURL: input.NotifyURL,
		ReturnURL: returnURL,
	}
	result, err := epusdt.CreatePayment(ctx, cfg, native)
	if err != nil {
		return nil, mapEpusdtError(err)
	}

	return &paymentcontract.GatewayCreateResult{
		ProviderRef:        result.TradeID,
		RedirectURL:        result.PaymentURL,
		QRCodeURL:          result.PaymentURL, // epusdt 是 USDT 网关，PaymentURL 同时用于跳转和 QR 展示
		Payload:            jsonmap.JSON(result.Raw),
		DisplayChannelType: epusdtDisplayChannelType(cfg),
	}, nil
}

// epusdtDisplayChannelType 返回 epusdt 支付记录的展示用渠道类型。
// epusdt 是单 provider 多币种/网络配置，payment.channel_type 可能只保留渠道兼容值；
// 这里按 config_json.token/network 推导更准确的展示值，写入 display_channel_type。
func epusdtDisplayChannelType(cfg *epusdt.Config) string {
	if cfg == nil {
		return ""
	}
	if cfg.OrderMode == constants.PaymentEpusdtOrderModeCashier {
		return ""
	}
	token := strings.ToLower(strings.TrimSpace(cfg.Token))
	network := strings.ToLower(strings.TrimSpace(cfg.Network))
	if token == "" || network == "" {
		return ""
	}
	return token + "." + network
}

// VerifyCallback 实现 paymentcontract.GatewayCallbackVerifier。epusdt 用 JSON POST body，form 参数忽略。
func (a *epusdtAdapter) VerifyCallback(raw jsonmap.JSON, _ map[string][]string, body []byte) (*paymentcontract.GatewayCallbackResult, error) {
	cfg, err := epusdt.ParseConfig(raw)
	if err != nil {
		return nil, mapEpusdtError(err)
	}

	data, err := epusdt.ParseCallback(body)
	if err != nil {
		return nil, mapEpusdtError(err)
	}

	if err := epusdt.VerifyCallback(cfg, data); err != nil {
		return nil, mapEpusdtError(err)
	}

	// epusdt 用 status int → PaymentStatusXxx string 映射
	status := epusdt.ToPaymentStatus(data.Status)

	// amount 解析失败时返回零值：wrapper 仅做适配，金额异常由业务层判定。
	amount := money.Amount{}
	if data.Amount != nil {
		amountFloat := data.GetAmount()
		if amountFloat > 0 {
			amount = money.FromDecimal(decimal.NewFromFloat(amountFloat))
		}
	}

	// 把 callback 关键字段塞进 Payload
	payload := jsonmap.JSON{}
	if pb, marshalErr := json.Marshal(data); marshalErr == nil {
		var m map[string]interface{}
		if jsonErr := json.Unmarshal(pb, &m); jsonErr == nil {
			payload = jsonmap.JSON(m)
		}
	}

	return &paymentcontract.GatewayCallbackResult{
		OrderNo:     data.OrderID,
		ProviderRef: data.TradeID,
		Status:      status,
		Amount:      amount,
		Currency:    strings.ToUpper(cfg.Currency),
		PaidAt:      nil, // epusdt callback 不带付款时间
		Payload:     payload,
	}, nil
}

// mapEpusdtError 把 epusdt 包的 sentinel error 映射为 provider 统一错误。
func mapEpusdtError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, epusdt.ErrConfigInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayConfigInvalid, err)
	case errors.Is(err, epusdt.ErrRequestFailed):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayRequestFailed, err)
	case errors.Is(err, epusdt.ErrResponseInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayResponseInvalid, err)
	case errors.Is(err, epusdt.ErrSignatureInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewaySignatureInvalid, err)
	default:
		return err
	}
}
