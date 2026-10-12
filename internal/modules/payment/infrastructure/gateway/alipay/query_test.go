package alipay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/constants"
)

func TestQueryPaymentAuthenticatesResponseAndOrder(t *testing.T) {
	merchant := buildTestConfig("")
	platform := buildTestConfig("")
	merchant.AlipayPublicKey = platform.AlipayPublicKey
	valid := `{ "trade_status":"TRADE_SUCCESS", "total_amount":"1160.00", "out_trade_no":"DJP-QUERY-1", "code":"10000", "trade_no":"ALI-1", "send_pay_date":"2026-10-12 12:00:00" }`
	for _, tc := range []struct {
		name, payload   string
		badSign, noSign bool
		wantErr         error
	}{
		{name: "signed raw JSON", payload: valid},
		{name: "wrong signing key", payload: valid, badSign: true, wantErr: ErrSignatureInvalid},
		{name: "missing signature", payload: valid, noSign: true, wantErr: ErrSignatureInvalid},
		{name: "different order", payload: strings.ReplaceAll(valid, "DJP-QUERY-1", "DJP-OTHER"), wantErr: ErrResponseInvalid},
		{name: "invalid amount", payload: strings.ReplaceAll(valid, "1160.00", "0.00"), wantErr: ErrResponseInvalid},
		{name: "unknown status", payload: strings.ReplaceAll(valid, "TRADE_SUCCESS", "UNKNOWN"), wantErr: ErrResponseInvalid},
		{name: "business failure", payload: `{"code":"40004","sub_code":"ACQ.TRADE_NOT_EXIST"}`, wantErr: ErrResponseInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseForm(); err != nil {
					t.Error(err)
					return
				}
				if r.Form.Get("method") != "alipay.trade.query" || r.Form.Get("app_id") != merchant.AppID {
					t.Error("wrong query method or app_id")
				}
				var biz map[string]string
				if json.Unmarshal([]byte(r.Form.Get("biz_content")), &biz) != nil || biz["out_trade_no"] != "DJP-QUERY-1" || biz["trade_no"] != "" {
					t.Error("query must use only the merchant order number")
				}
				key := platform.PrivateKey
				if tc.badSign {
					key = merchant.PrivateKey
				}
				sig, err := signContent(tc.payload, key, "RSA2")
				if err != nil {
					t.Error(err)
					return
				}
				if tc.noSign {
					sig = ""
				}
				encodedSign, _ := json.Marshal(sig)
				fmt.Fprintf(w, `{"alipay_trade_query_response":%s,"sign":%s}`, tc.payload, encodedSign)
			}))
			defer server.Close()
			cfg := *merchant
			cfg.GatewayURL = server.URL
			got, err := QueryPaymentByOrderNo(context.Background(), &cfg, "DJP-QUERY-1")
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got != nil {
					t.Fatalf("expected %v, got result=%v err=%v", tc.wantErr, got, err)
				}
				return
			}
			if err != nil || got.TradeNo != "ALI-1" || got.TotalAmount != "1160.00" {
				t.Fatalf("query failed: result=%v err=%v", got, err)
			}
		})
	}
}

func TestSandboxWithoutNotifyDoesNotRelaxProduction(t *testing.T) {
	for _, endpoint := range []string{
		SandboxGateway,
		"https://openapi.alipay.com/gateway.do",
		"https://openapi-sandbox.dl.alipaydev.com.evil.example/gateway.do",
		"http://openapi-sandbox.dl.alipaydev.com/gateway.do",
		"https://openapi-sandbox.dl.alipaydev.com/gateway.do?forward=production",
	} {
		cfg := buildTestConfig(endpoint)
		cfg.NotifyURL = ""
		result, err := CreatePayment(context.Background(), cfg, CreateInput{OrderNo: "SANDBOX-1", Amount: "0.01"}, constants.PaymentInteractionPage)
		if endpoint != SandboxGateway {
			if !errors.Is(err, ErrConfigInvalid) {
				t.Fatalf("missing notify_url accepted for %s", endpoint)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		u, err := url.Parse(result.PayURL)
		if err != nil || u.Query().Get("notify_url") != "" || u.Query().Get("method") != "alipay.trade.page.pay" {
			t.Fatal("wrong sandbox page request")
		}
	}
}
