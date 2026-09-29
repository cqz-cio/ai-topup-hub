package dujiaopayadapter

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	gatewaycommon "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/common"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/dujiaopay"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// dujiaoPayAdapter 是 DujiaoPay 的 paymentcontract.GatewayProvider + paymentcontract.GatewayWebhooker 实现。
// transaction 模式的 channel_type 使用文档里的 token_id（tron-usdt / base-usdc 等）；
// cashier 模式（延迟分配）channel_type 固定为 dujiaopay，付款人在托管收银台自选链/币。
type dujiaoPayAdapter struct{}

// NewDujiaoPayAdapter 实例化 DujiaoPay adapter。
func NewDujiaoPayAdapter() paymentcontract.GatewayProvider { return &dujiaoPayAdapter{} }

var (
	_ paymentcontract.GatewayProvider  = (*dujiaoPayAdapter)(nil)
	_ paymentcontract.GatewayWebhooker = (*dujiaoPayAdapter)(nil)
)

// Type 返回 provider 标识。DujiaoPay 支持多个 token_id，因此 channelType 部分为空。
func (a *dujiaoPayAdapter) Type() string {
	return constants.PaymentProviderDujiaoPay + ":"
}

func (a *dujiaoPayAdapter) parseConfig(raw jsonmap.JSON, channelType string) (*dujiaopay.Config, error) {
	cfg, err := dujiaopay.ParseConfig(raw)
	if err != nil {
		return nil, mapDujiaoPayError(err)
	}
	channelType = strings.ToLower(strings.TrimSpace(channelType))
	if cfg.TokenID == "" && channelType != "" && channelType != constants.PaymentProviderDujiaoPay {
		cfg.TokenID = channelType
	}
	if cfg.Chain == "" && cfg.TokenID != "" {
		cfg.Chain = dujiaopay.ResolveChain(cfg.TokenID)
	}
	if err := dujiaopay.ValidateConfig(cfg); err != nil {
		return nil, mapDujiaoPayError(err)
	}
	return cfg, nil
}

// checkDujiaoPayChannelTypeForMode 校验 channel_type 与 order_mode 的一致性：
// cashier 渠道的 channel_type 固定为 dujiaopay，transaction 渠道为 token_id。
func checkDujiaoPayChannelTypeForMode(cfg *dujiaopay.Config, channelType string) error {
	if channelType == "" {
		return nil
	}
	if cfg.OrderMode == constants.PaymentDujiaoPayOrderModeCashier {
		if channelType != constants.PaymentProviderDujiaoPay {
			return fmt.Errorf("%w: dujiaopay cashier channel_type %s", paymentcontract.ErrGatewayUnsupportedChannel, channelType)
		}
		return nil
	}
	if channelType == constants.PaymentProviderDujiaoPay {
		return fmt.Errorf("%w: dujiaopay transaction channel_type requires token_id", paymentcontract.ErrGatewayUnsupportedChannel)
	}
	return nil
}

// ValidateConfig 验证 DujiaoPay channel.ConfigJSON。
func (a *dujiaoPayAdapter) ValidateConfig(raw jsonmap.JSON, channelType string) error {
	channelType = strings.ToLower(strings.TrimSpace(channelType))
	// token_id 的合法性由 parseConfig 内的 dujiaopay.ValidateConfig 统一判定：
	// 只要能推导出 chain（或显式配置了 chain）即可，不限定在内置映射表内。
	cfg, err := a.parseConfig(raw, channelType)
	if err != nil {
		return err
	}
	return checkDujiaoPayChannelTypeForMode(cfg, channelType)
}

// CreatePayment 创建 DujiaoPay 收银台订单。
func (a *dujiaoPayAdapter) CreatePayment(ctx context.Context, raw jsonmap.JSON, input paymentcontract.GatewayCreateInput) (*paymentcontract.GatewayCreateResult, error) {
	channelType := strings.ToLower(strings.TrimSpace(input.ChannelType))

	cfg, err := a.parseConfig(raw, channelType)
	if err != nil {
		return nil, err
	}
	if err := checkDujiaoPayChannelTypeForMode(cfg, channelType); err != nil {
		return nil, err
	}
	if cfg.OrderMode == constants.PaymentDujiaoPayOrderModeCashier {
		// 延迟分配订单建单时没有固定收款地址/金额，无法渲染站内二维码，只支持跳转托管收银台。
		if mode, ok := input.Extra["interaction_mode"].(string); ok && strings.ToLower(strings.TrimSpace(mode)) == constants.PaymentInteractionQR {
			return nil, fmt.Errorf("%w: dujiaopay cashier order mode only supports redirect interaction_mode", paymentcontract.ErrGatewayConfigInvalid)
		}
	}
	if rawFiatCurrency(raw) == "" && strings.TrimSpace(input.Currency) != "" {
		cfg.FiatCurrency = strings.ToUpper(strings.TrimSpace(input.Currency))
	}

	successURL := gatewaycommon.PickFirstNonEmpty(input.ReturnURL, cfg.SuccessURL)
	successURL = gatewaycommon.AppendQueryParams(successURL, input.ReturnURLQuery)
	cancelURL := cfg.CancelURL
	if rawCancelURL, ok := input.Extra["cancel_url"].(string); ok && strings.TrimSpace(rawCancelURL) != "" {
		cancelURL = strings.TrimSpace(rawCancelURL)
	}

	metadata := map[string]interface{}{}
	if input.PaymentID > 0 {
		metadata["payment_id"] = input.PaymentID
	}
	if input.OrderID > 0 {
		metadata["order_id"] = input.OrderID
	}
	if input.Subject != "" {
		metadata["subject"] = input.Subject
	}

	result, err := dujiaopay.CreatePayment(ctx, cfg, dujiaopay.CreateInput{
		MerchantOrderID: strings.TrimSpace(input.OrderNo),
		FiatAmount:      input.Amount.Decimal.String(),
		SuccessURL:      successURL,
		CancelURL:       cancelURL,
		Metadata:        metadata,
	})
	if err != nil {
		return nil, mapDujiaoPayError(err)
	}

	qrCodeURL := strings.TrimSpace(result.CheckoutURL)
	if mode, ok := input.Extra["interaction_mode"].(string); ok && strings.ToLower(strings.TrimSpace(mode)) == constants.PaymentInteractionQR {
		qrCodeURL = gatewaycommon.PickFirstNonEmpty(result.PayAddress, result.CheckoutURL)
	}

	payload := jsonmap.JSON{}
	for key, value := range result.Raw {
		payload[key] = value
	}
	setIfNotEmpty := func(key, value string) {
		if strings.TrimSpace(value) != "" {
			payload[key] = strings.TrimSpace(value)
		}
	}
	setIfNotEmpty("order_id", result.OrderID)
	setIfNotEmpty("chain", result.Chain)
	setIfNotEmpty("token_id", result.TokenID)
	setIfNotEmpty("pay_address", result.PayAddress)
	setIfNotEmpty("payable_amount", result.PayableAmount)
	setIfNotEmpty("checkout_url", result.CheckoutURL)
	payload[paymentcontract.GatewayPayloadFiatCurrencySent] = strings.ToUpper(strings.TrimSpace(cfg.FiatCurrency))

	return &paymentcontract.GatewayCreateResult{
		ProviderRef:  result.OrderID,
		RedirectURL:  result.CheckoutURL,
		QRCodeURL:    qrCodeURL,
		Payload:      payload,
		AmountSent:   input.Amount.Decimal.String(),
		CurrencySent: strings.ToUpper(strings.TrimSpace(cfg.FiatCurrency)),
	}, nil
}

// ParseWebhook 验签并解析 DujiaoPay webhook。
func (a *dujiaoPayAdapter) ParseWebhook(_ context.Context, raw jsonmap.JSON, headers map[string]string, body []byte, now time.Time) (*paymentcontract.GatewayCallbackResult, error) {
	cfg, err := dujiaopay.ParseConfig(raw)
	if err != nil {
		return nil, mapDujiaoPayError(err)
	}
	event, err := dujiaopay.ParseWebhook(cfg, headers, body, now)
	if err != nil {
		return nil, mapDujiaoPayError(err)
	}

	payload := jsonmap.JSON{}
	for key, value := range event.Raw {
		payload[key] = value
	}
	if event.TransactionID != "" {
		payload["tx_id"] = event.TransactionID
		// 保留旧的归一化键，避免已存在的通知/审计消费者因字段升级丢失交易标识。
		payload["tx_hash"] = event.TransactionID
	}

	amount := money.Amount{}
	if rawAmount := strings.TrimSpace(event.FiatAmount); rawAmount != "" {
		parsed, parseErr := decimal.NewFromString(rawAmount)
		if parseErr != nil {
			return nil, fmt.Errorf("%w: invalid fiat_amount", paymentcontract.ErrGatewayResponseInvalid)
		}
		amount = money.FromDecimal(parsed)
	}

	return &paymentcontract.GatewayCallbackResult{
		OrderNo:     event.MerchantOrderID,
		ProviderRef: event.OrderID,
		Status:      event.Status,
		Amount:      amount,
		Currency:    strings.ToUpper(strings.TrimSpace(event.FiatCurrency)),
		PaidAt:      event.PaidAt,
		Payload:     payload,
	}, nil
}

func rawFiatCurrency(raw jsonmap.JSON) string {
	if raw == nil {
		return ""
	}
	if value, ok := raw["fiat_currency"].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func mapDujiaoPayError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, dujiaopay.ErrUnsupportedToken):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayUnsupportedChannel, err)
	case errors.Is(err, dujiaopay.ErrConfigInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayConfigInvalid, err)
	case errors.Is(err, dujiaopay.ErrRequestFailed):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayRequestFailed, err)
	case errors.Is(err, dujiaopay.ErrResponseInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewayResponseInvalid, err)
	case errors.Is(err, dujiaopay.ErrSignatureInvalid):
		return fmt.Errorf("%w: %v", paymentcontract.ErrGatewaySignatureInvalid, err)
	default:
		return err
	}
}
