package xorpayadapter

import (
	"context"
	"crypto/md5" // Required by the XorPay protocol.
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

// Native accepts CNY WeChat QR payments. XorPay's query API has no signed
// amount/order tuple, so only verified callbacks can settle payments.
type adapter struct {
	client  *http.Client
	baseURL string
}

func NewXorpayAdapter() paymentcontract.GatewayProvider {
	return &adapter{client: &http.Client{
		Timeout:       12 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}, baseURL: "https://xorpay.com"}
}

var _ paymentcontract.GatewayCallbackVerifier = (*adapter)(nil)

func (*adapter) Type() string { return constants.PaymentProviderXorpay + ":" }

var aidPattern = regexp.MustCompile(`^[1-9][0-9]{0,19}$`)
var amountPattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]{1,2})?$`)

type config struct{ aid, secret, notifyURL string }

func parseConfig(raw jsonmap.JSON) (config, error) {
	c := config{}
	c.aid, _ = raw["aid"].(string)
	c.secret, _ = raw["app_secret"].(string)
	c.notifyURL, _ = raw["notify_url"].(string)
	c.aid, c.secret, c.notifyURL = strings.TrimSpace(c.aid), strings.TrimSpace(c.secret), strings.TrimSpace(c.notifyURL)
	u, err := url.Parse(c.notifyURL)
	if !aidPattern.MatchString(c.aid) || c.secret == "" || c.secret == "••••••••" || err != nil || u.Host == "" || u.User != nil || u.Fragment != "" ||
		(u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost" || u.Hostname() == "::1"))) {
		return c, paymentcontract.ErrGatewayConfigInvalid
	}
	return c, nil
}

func (*adapter) ValidateConfig(raw jsonmap.JSON, channelType string) error {
	if channelType != constants.PaymentChannelTypeWechat {
		return paymentcontract.ErrGatewayUnsupportedChannel
	}
	_, err := parseConfig(raw)
	return err
}

func digest(parts ...string) string {
	sum := md5.Sum([]byte(strings.Join(parts, "")))
	return hex.EncodeToString(sum[:])
}

func (a *adapter) CreatePayment(ctx context.Context, raw jsonmap.JSON, in paymentcontract.GatewayCreateInput) (*paymentcontract.GatewayCreateResult, error) {
	if err := a.ValidateConfig(raw, in.ChannelType); err != nil {
		return nil, err
	}
	c, _ := parseConfig(raw)
	if !strings.EqualFold(in.Currency, "CNY") || !in.Amount.Decimal.IsPositive() || !in.Amount.Decimal.Equal(in.Amount.Decimal.Round(2)) ||
		strings.TrimSpace(in.OrderNo) == "" || strings.TrimSpace(in.Subject) == "" {
		return nil, paymentcontract.ErrGatewayConfigInvalid
	}
	if mode, _ := in.Extra["interaction_mode"].(string); mode != "" && mode != constants.PaymentInteractionQR {
		return nil, paymentcontract.ErrGatewayUnsupportedChannel
	}
	started := time.Now()
	expire := 900
	if deadline, ok := in.Extra["expires_at"].(time.Time); ok {
		expire = int(deadline.Sub(started).Seconds())
		if expire <= 0 {
			return nil, paymentcontract.ErrGatewayConfigInvalid
		}
		if expire > 7200 {
			expire = 7200
		}
	}
	price := in.Amount.Decimal.StringFixed(2)
	form := url.Values{
		"name": {in.Subject}, "pay_type": {"native"}, "price": {price},
		"order_id": {in.OrderNo}, "notify_url": {c.notifyURL}, "expire": {strconv.Itoa(expire)},
		"sign": {digest(in.Subject, "native", price, in.OrderNo, c.notifyURL, c.secret)},
	}
	// Customer emails and credentials are not sent to the provider.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/api/pay/"+c.aid, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, paymentcontract.ErrGatewayRequestFailed
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, paymentcontract.ErrGatewayRequestFailed
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, paymentcontract.ErrGatewayRequestFailed
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 65537))
	if err != nil || len(body) > 65536 {
		return nil, paymentcontract.ErrGatewayResponseInvalid
	}
	var result struct {
		Status    string      `json:"status"`
		AOID      string      `json:"aoid"`
		ExpireIn  json.Number `json:"expire_in"`
		ExpiresIn json.Number `json:"expires_in"`
		Info      struct {
			QR string `json:"qr"`
		} `json:"info"`
	}
	if json.Unmarshal(body, &result) != nil {
		return nil, paymentcontract.ErrGatewayResponseInvalid
	}
	if result.Status != "ok" {
		// Upstream raw responses can echo private request data; never log them.
		if result.Status == "fee_error" {
			return nil, fmt.Errorf("%w: xorpay fee_error (service balance insufficient)", paymentcontract.ErrGatewayRequestFailed)
		}
		return nil, paymentcontract.ErrGatewayRequestFailed
	}
	qr, err := url.Parse(result.Info.QR)
	if err != nil || qr.Host == "" || qr.User != nil || (qr.Scheme != "weixin" && qr.Scheme != "https") || strings.TrimSpace(result.AOID) == "" {
		return nil, paymentcontract.ErrGatewayResponseInvalid
	}
	// Official examples use both spellings. Never extend the store's deadline.
	for _, value := range []json.Number{result.ExpireIn, result.ExpiresIn} {
		if value == "" {
			continue
		}
		seconds, err := value.Int64()
		if err != nil || seconds <= 0 {
			return nil, paymentcontract.ErrGatewayResponseInvalid
		}
		if seconds < int64(expire) {
			expire = int(seconds)
		}
	}
	expires := started.Add(time.Duration(expire) * time.Second)
	return &paymentcontract.GatewayCreateResult{
		ProviderRef: result.AOID, QRCodeURL: result.Info.QR, ExpiresAt: &expires,
		AmountSent: price, CurrencySent: "CNY",
		Payload: jsonmap.JSON{"aoid": result.AOID, "pay_type": "native"},
	}, nil
}

func (*adapter) VerifyCallback(raw jsonmap.JSON, form map[string][]string, _ []byte) (*paymentcontract.GatewayCallbackResult, error) {
	c, err := parseConfig(raw)
	if err != nil {
		return nil, err
	}
	fields := make(map[string]string)
	for _, key := range []string{"aoid", "order_id", "pay_price", "pay_time", "sign"} {
		values := form[key]
		if len(values) != 1 || values[0] == "" {
			return nil, paymentcontract.ErrGatewaySignatureInvalid
		}
		fields[key] = values[0]
	}
	want := digest(fields["aoid"], fields["order_id"], fields["pay_price"], fields["pay_time"], c.secret)
	if subtle.ConstantTimeCompare([]byte(strings.ToLower(fields["sign"])), []byte(want)) != 1 {
		return nil, paymentcontract.ErrGatewaySignatureInvalid
	}
	amount, err := decimal.NewFromString(fields["pay_price"])
	if err != nil || !amountPattern.MatchString(fields["pay_price"]) || !amount.IsPositive() {
		return nil, paymentcontract.ErrGatewayResponseInvalid
	}
	paidAt, err := time.ParseInLocation("2006-01-02 15:04:05", fields["pay_time"], time.FixedZone("Asia/Shanghai", 8*3600))
	if err != nil {
		return nil, paymentcontract.ErrGatewayResponseInvalid
	}
	return &paymentcontract.GatewayCallbackResult{
		OrderNo: fields["order_id"], ProviderRef: fields["aoid"], Status: constants.PaymentStatusSuccess,
		Amount: money.FromDecimal(amount), Currency: "CNY", PaidAt: &paidAt,
		// detail/more are unsigned and must not affect identity or fulfillment.
		Payload: jsonmap.JSON{"aoid": fields["aoid"], "order_id": fields["order_id"], "pay_price": fields["pay_price"], "pay_time": fields["pay_time"]},
	}, nil
}
