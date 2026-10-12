package alipay

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/shopspring/decimal"
)

// SandboxGateway is the gateway published by the Alipay AI web payment console.
const SandboxGateway = "https://openapi-sandbox.dl.alipaydev.com/gateway.do"

// IsSandboxGateway requires the exact official sandbox endpoint. It is also used
// by the standalone sandbox tool to prevent accidentally contacting production.
func IsSandboxGateway(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && u.Scheme == "https" && u.Hostname() == "openapi-sandbox.dl.alipaydev.com" &&
		(u.Port() == "" || u.Port() == "443") && u.Path == "/gateway.do" &&
		u.User == nil && u.RawQuery == "" && u.Fragment == ""
}

type QueryResult struct {
	OutTradeNo  string
	TradeNo     string
	TradeStatus string
	TotalAmount string
	SendPayDate string
	Raw         map[string]interface{}
}

// QueryPaymentByOrderNo queries the merchant order number stored when payment
// was created. A signed response must match that number before it can be used.
func QueryPaymentByOrderNo(ctx context.Context, cfg *Config, orderNo string) (*QueryResult, error) {
	if err := ValidateConfig(cfg, constants.PaymentInteractionQR); err != nil {
		return nil, err
	}
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" || len(orderNo) > 64 {
		return nil, fmt.Errorf("%w: out_trade_no is invalid", ErrConfigInvalid)
	}
	biz, err := json.Marshal(map[string]string{"out_trade_no": orderNo})
	if err != nil {
		return nil, fmt.Errorf("%w: encode query failed", ErrConfigInvalid)
	}
	params := map[string]string{
		"app_id": cfg.AppID, "method": "alipay.trade.query", "format": alipayReqFormatJSON,
		"charset": alipayReqCharset, "sign_type": cfg.SignType, "version": alipayReqVersion,
		"timestamp":   time.Now().In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05"),
		"biz_content": string(biz),
	}
	if cfg.AppCertSN != "" {
		params["app_cert_sn"] = cfg.AppCertSN
	}
	if cfg.AlipayRootCertSN != "" {
		params["alipay_root_cert_sn"] = cfg.AlipayRootCertSN
	}
	params["sign"], err = signContent(buildSignContent(params), cfg.PrivateKey, cfg.SignType)
	if err != nil {
		return nil, err
	}
	body, err := postGateway(ctx, cfg.GatewayURL, params)
	if err != nil {
		return nil, err
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("%w: decode query response failed", ErrResponseInvalid)
	}
	payload := envelope["alipay_trade_query_response"]
	var response struct {
		Code        string `json:"code"`
		OutTradeNo  string `json:"out_trade_no"`
		TradeNo     string `json:"trade_no"`
		TradeStatus string `json:"trade_status"`
		TotalAmount string `json:"total_amount"`
		SendPayDate string `json:"send_pay_date"`
	}
	if len(payload) == 0 || json.Unmarshal(payload, &response) != nil || response.Code != alipayRespCodeSuccess {
		// Business errors, including a not-yet-created trade, never change payment state.
		return nil, fmt.Errorf("%w: query was not successful", ErrResponseInvalid)
	}
	var signature string
	if json.Unmarshal(envelope["sign"], &signature) != nil || strings.TrimSpace(signature) == "" {
		return nil, fmt.Errorf("%w: query signature missing", ErrSignatureInvalid)
	}
	// Verify the original JSON bytes, not a re-marshalled map with different order.
	if err := verifyResponseSignature(cfg, payload, signature); err != nil {
		return nil, err
	}
	if response.OutTradeNo != orderNo || strings.TrimSpace(response.TradeNo) == "" {
		return nil, fmt.Errorf("%w: query order mismatch or missing trade_no", ErrResponseInvalid)
	}
	amount, err := decimal.NewFromString(response.TotalAmount)
	if err != nil || !amount.IsPositive() || !amount.Equal(amount.Round(2)) {
		return nil, fmt.Errorf("%w: query amount invalid", ErrResponseInvalid)
	}
	switch response.TradeStatus {
	case "WAIT_BUYER_PAY", "TRADE_SUCCESS", "TRADE_FINISHED", "TRADE_CLOSED":
	default:
		return nil, fmt.Errorf("%w: query trade_status invalid", ErrResponseInvalid)
	}
	return &QueryResult{
		OutTradeNo: response.OutTradeNo, TradeNo: response.TradeNo, TradeStatus: response.TradeStatus,
		TotalAmount: response.TotalAmount, SendPayDate: response.SendPayDate,
		// Keep only reconciliation facts; buyer identity is not needed by the shop.
		Raw: map[string]interface{}{
			"out_trade_no": response.OutTradeNo, "trade_no": response.TradeNo,
			"trade_status": response.TradeStatus, "total_amount": response.TotalAmount,
			"send_pay_date": response.SendPayDate,
		},
	}, nil
}

func verifyResponseSignature(cfg *Config, payload []byte, signature string) error {
	key, err := parsePublicKey(cfg.AlipayPublicKey)
	if err != nil {
		return err
	}
	sig, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("%w: query signature encoding invalid", ErrSignatureInvalid)
	}
	hash := crypto.SHA256
	sum := sha256.Sum256(payload)
	digest := sum[:]
	if cfg.SignType == alipaySignTypeRSA {
		hash = crypto.SHA1
		sum := sha1.Sum(payload)
		digest = sum[:]
	}
	if rsa.VerifyPKCS1v15(key, hash, digest, sig) != nil {
		return fmt.Errorf("%w: query signature verification failed", ErrSignatureInvalid)
	}
	return nil
}
