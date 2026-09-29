package gatewayconfig_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	alipayadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/alipay"
	bepusdtadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/bepusdt"
	dujiaopayadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/dujiaopay"
	epayadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/epay"
	epusdtadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/epusdt"
	okpayadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/okpay"
	paypaladapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/paypal"
	stripeadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/stripe"
	tokenpayadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/tokenpay"
	wechatpayadapter "github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/adapters/wechatpay"

	paymentapp "github.com/dujiao-next/internal/modules/payment/application"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/provider"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// buildMinimalPaymentServiceWithRegistry 构造一个只注入了 Registry 的 PaymentService，
// 供无需 DB 的 ValidateChannel 测试使用。
func buildMinimalPaymentServiceWithRegistry(t *testing.T) *paymentapp.PaymentService {
	t.Helper()
	reg := provider.NewRegistry()
	reg.Register(constants.PaymentProviderOfficial, constants.PaymentChannelTypeStripe, stripeadapter.NewStripeAdapter())
	reg.Register(constants.PaymentProviderOfficial, constants.PaymentChannelTypePaypal, paypaladapter.NewPaypalAdapter())
	reg.Register(constants.PaymentProviderOfficial, constants.PaymentChannelTypeWechat, wechatpayadapter.NewWechatpayAdapter())
	reg.Register(constants.PaymentProviderOfficial, constants.PaymentChannelTypeAlipay, alipayadapter.NewAlipayAdapter())
	reg.Register(constants.PaymentProviderEpay, "", epayadapter.NewEpayAdapter())
	reg.Register(constants.PaymentProviderEpusdt, "", epusdtadapter.NewEpusdtAdapter())
	reg.Register(constants.PaymentProviderBepusdt, "", bepusdtadapter.NewBepusdtAdapter())
	reg.Register(constants.PaymentProviderDujiaoPay, "", dujiaopayadapter.NewDujiaoPayAdapter())
	reg.Register(constants.PaymentProviderTokenpay, "", tokenpayadapter.NewTokenpayAdapter())
	reg.Register(constants.PaymentProviderOkpay, "", okpayadapter.NewOkpayAdapter())
	return paymentapp.NewPaymentService(paymentapp.PaymentServiceOptions{PaymentProviderRegistry: reg})
}

func TestValidateChannelWechatOfficial(t *testing.T) {
	svc := buildMinimalPaymentServiceWithRegistry(t)
	channel := &paymentdomain.PaymentChannel{
		ProviderType:    constants.PaymentProviderOfficial,
		ChannelType:     constants.PaymentChannelTypeWechat,
		InteractionMode: constants.PaymentInteractionRedirect,
		FeeRate:         money.FromDecimal(decimal.Zero),
		ConfigJSON: jsonmap.JSON{
			"appid":                "wx1234567890",
			"mchid":                "1900000109",
			"merchant_serial_no":   "ABC123456789",
			"merchant_private_key": buildWechatTestPrivateKey(),
			"api_v3_key":           "12345678901234567890123456789012",
			"notify_url":           "https://example.com/api/v1/payments/callback",
			"h5_redirect_url":      "https://example.com/pay",
		},
	}
	if err := svc.ValidateChannel(channel); err != nil {
		t.Fatalf("validate wechat channel failed: %v", err)
	}
}

func TestValidateChannelWechatInvalidInteractionMode(t *testing.T) {
	svc := buildMinimalPaymentServiceWithRegistry(t)
	channel := &paymentdomain.PaymentChannel{
		ProviderType:    constants.PaymentProviderOfficial,
		ChannelType:     constants.PaymentChannelTypeWechat,
		InteractionMode: constants.PaymentInteractionWAP,
		FeeRate:         money.FromDecimal(decimal.Zero),
		ConfigJSON: jsonmap.JSON{
			"appid":                "wx1234567890",
			"mchid":                "1900000109",
			"merchant_serial_no":   "ABC123456789",
			"merchant_private_key": buildWechatTestPrivateKey(),
			"api_v3_key":           "12345678901234567890123456789012",
			"notify_url":           "https://example.com/api/v1/payments/callback",
			"h5_redirect_url":      "https://example.com/pay",
		},
	}
	if err := svc.ValidateChannel(channel); err == nil {
		t.Fatalf("expected invalid interaction mode error")
	}
}

func buildWechatTestPrivateKey() string {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		panic(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER}))
}
