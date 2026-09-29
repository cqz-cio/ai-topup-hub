package bepusdtadapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	gatewaycommon "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/common"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/bepusdt"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// bepusdtAdapter 是 bepusdt 网关的 paymentcontract.GatewayProvider + paymentcontract.GatewayCallbackVerifier 实现。
// 与 epusdt 相似，但需要兼容旧 channel type（usdt-trc20 / usdc-trc20 / trx 等）；
// transaction 模式的交易类型由 trade_type 配置决定，缺省为 usdt.trc20，cashier 模式走 BEpusdt 收银台。
// callback 是同步 JSON POST（不是 form），所以**不**实现 paymentcontract.GatewayCapturer 和 paymentcontract.GatewayWebhooker。
type bepusdtAdapter struct{}

// NewBepusdtAdapter 实例化 bepusdt adapter。
func NewBepusdtAdapter() paymentcontract.GatewayProvider { return &bepusdtAdapter{} }

// 编译期断言 bepusdtAdapter 实现了 paymentcontract.GatewayProvider 和 paymentcontract.GatewayCallbackVerifier。
var (
	_ paymentcontract.GatewayProvider         = (*bepusdtAdapter)(nil)
	_ paymentcontract.GatewayCallbackVerifier = (*bepusdtAdapter)(nil)
)

// Type 返回 provider 标识。bepusdt 是多 channel type provider，返回值中 channelType 部分为空。
func (a *bepusdtAdapter) Type() string {
	return constants.PaymentProviderBepusdt + ":"
}

// parseConfig 解析并验证 bepusdt Config。
// transaction 模式未配置 trade_type 时，由 Config.Normalize 保持旧行为并使用 usdt.trc20。
func (a *bepusdtAdapter) parseConfig(raw jsonmap.JSON) (*bepusdt.Config, error) {
	cfg, err := bepusdt.ParseConfig(raw)
	if err != nil {
		return nil, mapBepusdtError(err)
	}
	if err := bepusdt.ValidateConfig(cfg); err != nil {
		return nil, mapBepusdtError(err)
	}
	return cfg, nil
}

// ValidateConfig 验证 channel.ConfigJSON。
// 新格式 channel_type 固定为 bepusdt；旧数据继续允许 usdt-trc20 / usdc-trc20 / trx 等 legacy channel_type。
func (a *bepusdtAdapter) ValidateConfig(raw jsonmap.JSON, channelType string) error {
	normalizedChannelType := strings.ToLower(strings.TrimSpace(channelType))
	if normalizedChannelType != "" && normalizedChannelType != constants.PaymentProviderBepusdt && !isLegacyBepusdtChannelType(normalizedChannelType) {
		return fmt.Errorf("%w: bepusdt channel_type %s", paymentcontract.ErrGatewayUnsupportedChannel, channelType)
	}
	cfg, err := a.parseConfig(raw)
	if err != nil {
		return err
	}
	if normalizedChannelType == "" {
		return nil
	}
	if cfg.OrderMode == constants.PaymentBepusdtOrderModeCashier {
		if normalizedChannelType != constants.PaymentProviderBepusdt {
			return fmt.Errorf("%w: bepusdt cashier channel_type %s", paymentcontract.ErrGatewayUnsupportedChannel, channelType)
		}
		return nil
	}
	if normalizedChannelType == constants.PaymentProviderBepusdt {
		if strings.TrimSpace(cfg.TradeType) == "" {
			return fmt.Errorf("%w: bepusdt trade_type is required", paymentcontract.ErrGatewayConfigInvalid)
		}
		return nil
	}
	if !isLegacyBepusdtChannelType(normalizedChannelType) {
		return fmt.Errorf("%w: bepusdt channel_type %s", paymentcontract.ErrGatewayUnsupportedChannel, channelType)
	}
	return nil
}

// CreatePayment 创建支付。bepusdt 多 channel type，需要先校验 channelType。
func (a *bepusdtAdapter) CreatePayment(ctx context.Context, raw jsonmap.JSON, input paymentcontract.GatewayCreateInput) (*paymentcontract.GatewayCreateResult, error) {
	cfg, err := a.parseConfig(raw)
	if err != nil {
		return nil, err
	}
	channelType := strings.ToLower(strings.TrimSpace(input.ChannelType))
	if cfg.OrderMode == constants.PaymentBepusdtOrderModeCashier {
		if channelType != "" && channelType != constants.PaymentProviderBepusdt {
			return nil, fmt.Errorf("%w: bepusdt cashier channel_type %s", paymentcontract.ErrGatewayUnsupportedChannel, input.ChannelType)
		}
	} else if channelType != "" {
		if channelType == constants.PaymentProviderBepusdt {
			if strings.TrimSpace(cfg.TradeType) == "" {
				return nil, fmt.Errorf("%w: bepusdt trade_type is required", paymentcontract.ErrGatewayConfigInvalid)
			}
		} else if !isLegacyBepusdtChannelType(channelType) {
			return nil, fmt.Errorf("%w: bepusdt channel_type %s", paymentcontract.ErrGatewayUnsupportedChannel, input.ChannelType)
		}
	}

	returnURL := strings.TrimSpace(input.ReturnURL)
	if returnURL == "" {
		returnURL = strings.TrimSpace(cfg.ReturnURL)
	}
	returnURL = gatewaycommon.AppendQueryParams(returnURL, input.ReturnURLQuery)

	native := bepusdt.CreateInput{
		OrderNo:   input.OrderNo,
		Amount:    input.Amount.Decimal.String(),
		Name:      input.Subject,
		NotifyURL: input.NotifyURL,
		ReturnURL: returnURL,
	}
	mode, _ := input.Extra["interaction_mode"].(string)
	mode = strings.ToLower(strings.TrimSpace(mode))

	var result *bepusdt.CreateResult
	if cfg.OrderMode == constants.PaymentBepusdtOrderModeCashier {
		if mode == constants.PaymentInteractionQR {
			return nil, fmt.Errorf("%w: bepusdt cashier order mode only supports redirect interaction_mode", paymentcontract.ErrGatewayConfigInvalid)
		}
		result, err = bepusdt.CreateCashierOrder(ctx, cfg, native)
	} else {
		result, err = bepusdt.CreatePayment(ctx, cfg, native)
	}
	if err != nil {
		return nil, mapBepusdtError(err)
	}

	redirectURL := strings.TrimSpace(result.PaymentURL)
	qrCodeURL := redirectURL
	switch mode {
	case constants.PaymentInteractionQR:
		qrCodeURL = strings.TrimSpace(result.Token)
		redirectURL = ""
		if qrCodeURL == "" {
			return nil, fmt.Errorf("%w: bepusdt token is empty", paymentcontract.ErrGatewayResponseInvalid)
		}
	case "", constants.PaymentInteractionRedirect:
	default:
		return nil, fmt.Errorf("%w: bepusdt interaction_mode %s", paymentcontract.ErrGatewayConfigInvalid, mode)
	}

	return &paymentcontract.GatewayCreateResult{
		ProviderRef:        result.TradeID,
		RedirectURL:        redirectURL,
		QRCodeURL:          qrCodeURL,
		Payload:            buildBepusdtCreatePayload(result, cfg.TradeType, cfg.OrderMode),
		DisplayChannelType: bepusdtDisplayChannelType(cfg),
	}, nil
}

// bepusdtDisplayChannelType 返回 BEpusdt 支付记录的展示用渠道类型。
// 新格式下 payment.channel_type 固定保存为 bepusdt；交易模式需要展示真实 trade_type，
// 收银台模式没有固定币种，保持空值并回退展示 bepusdt。
func bepusdtDisplayChannelType(cfg *bepusdt.Config) string {
	if cfg == nil || cfg.OrderMode == constants.PaymentBepusdtOrderModeCashier {
		return ""
	}
	return strings.TrimSpace(cfg.TradeType)
}

func buildBepusdtCreatePayload(result *bepusdt.CreateResult, tradeType string, orderMode string) jsonmap.JSON {
	payload := jsonmap.JSON{}
	if result == nil {
		return payload
	}
	if result.Raw != nil {
		for key, value := range result.Raw {
			payload[key] = value
		}
	}

	data := ensureBepusdtPayloadData(payload)
	setBepusdtPayloadString(data, "order_mode", orderMode)
	setBepusdtPayloadString(data, "trade_type", tradeType)
	setBepusdtPayloadString(data, "token", result.Token)
	setBepusdtPayloadString(data, "actual_amount", result.ActualAmount)
	setBepusdtPayloadString(data, "payment_url", result.PaymentURL)

	chain, tokenID := resolveBepusdtTradeLabels(tradeType)
	setBepusdtPayloadString(data, "chain", chain)
	setBepusdtPayloadString(data, "token_id", tokenID)
	return payload
}

func isLegacyBepusdtChannelType(channelType string) bool {
	return bepusdt.ResolveTradeType(channelType) != ""
}

func ensureBepusdtPayloadData(payload jsonmap.JSON) map[string]interface{} {
	if raw, ok := payload["data"].(map[string]interface{}); ok {
		return raw
	}
	if raw, ok := payload["data"].(jsonmap.JSON); ok {
		data := map[string]interface{}(raw)
		payload["data"] = data
		return data
	}
	data := map[string]interface{}{}
	payload["data"] = data
	return data
}

func setBepusdtPayloadString(payload map[string]interface{}, key string, value string) {
	value = strings.TrimSpace(value)
	if value != "" {
		payload[key] = value
	}
}

func resolveBepusdtTradeLabels(tradeType string) (chain string, tokenID string) {
	normalized := strings.ToLower(strings.TrimSpace(tradeType))
	parts := strings.Split(normalized, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", ""
	}

	// BEpusdt 的稳定币采用 token.network，原生币采用 network.token。
	// 根据官方 trade type 合同识别稳定币前缀，其余类型按原生币格式解析，
	// 从而让未来的原生代币支持比如 solana.sol、aptos.apt 等类型无需逐个维护。
	if isBepusdtTokenPrefix(parts[0]) {
		token := parts[0]
		network := normalizeBepusdtNetwork(parts[1])
		return network, network + "-" + token
	}

	network := normalizeBepusdtNetwork(parts[0])
	token := parts[1]
	return network, network + "-" + token
}

func isBepusdtTokenPrefix(value string) bool {
	switch value {
	case "usdt", "usdc":
		return true
	default:
		return false
	}
}

func normalizeBepusdtNetwork(network string) string {
	switch network {
	case "trc20":
		return "tron"
	case "erc20", "eth":
		return "ethereum"
	case "bep20":
		return "bsc"
	default:
		return network
	}
}

// VerifyCallback 实现 paymentcontract.GatewayCallbackVerifier。bepusdt 用 JSON POST body，form 参数忽略。
// 注意：callback 阶段不调 ValidateConfig——配置错误由签名校验兜底，
// 与 alipay/epay/epusdt adapter 行为一致。
func (a *bepusdtAdapter) VerifyCallback(raw jsonmap.JSON, _ map[string][]string, body []byte) (*paymentcontract.GatewayCallbackResult, error) {
	cfg, err := bepusdt.ParseConfig(raw)
	if err != nil {
		return nil, mapBepusdtError(err)
	}

	data, err := bepusdt.ParseCallback(body)
	if err != nil {
		return nil, mapBepusdtError(err)
	}

	if err := bepusdt.VerifyCallback(cfg, data); err != nil {
		return nil, mapBepusdtError(err)
	}

	// bepusdt 用 status int → PaymentStatusXxx string 映射
	status := bepusdt.ToPaymentStatus(data.Status)

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

	// bepusdt callback 不带 currency，从 cfg.Fiat 取（默认 CNY）
	currency := strings.ToUpper(strings.TrimSpace(cfg.Fiat))
	if currency == "" {
		currency = "CNY"
	}

	return &paymentcontract.GatewayCallbackResult{
		OrderNo:     data.OrderID,
		ProviderRef: data.TradeID,
		Status:      status,
		Amount:      amount,
		Currency:    currency,
		PaidAt:      nil, // bepusdt callback 不带付款时间
		Payload:     payload,
	}, nil
}

// mapBepusdtError 把 bepusdt 包的 sentinel error 映射为 provider 统一错误。
// 比 epusdt 多一个 ErrTradeTypeNotSupport → paymentcontract.ErrGatewayUnsupportedChannel 映射。
func mapBepusdtError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, bepusdt.ErrConfigInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayConfigInvalid, err)
	case errors.Is(err, bepusdt.ErrTradeTypeNotSupport):
		// P1.2a Task 1 加的 paymentcontract.ErrGatewayUnsupportedChannel 就是给这种场景用的
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayUnsupportedChannel, err)
	case errors.Is(err, bepusdt.ErrRequestFailed):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayRequestFailed, err)
	case errors.Is(err, bepusdt.ErrResponseInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayResponseInvalid, err)
	case errors.Is(err, bepusdt.ErrSignatureInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewaySignatureInvalid, err)
	default:
		return err
	}
}
