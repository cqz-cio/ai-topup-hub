package xorpayadapter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

func testConfig() jsonmap.JSON {
	return jsonmap.JSON{"aid": "12345", "app_secret": "test-secret-only", "notify_url": "https://shop.example/api/v1/payments/callback"}
}
func testInput() paymentcontract.GatewayCreateInput {
	return paymentcontract.GatewayCreateInput{OrderNo: "DJP-test-1", Subject: "测试订单", Currency: "CNY", Amount: money.FromDecimal(decimal.RequireFromString("12.30")), ChannelType: "wechat", Extra: jsonmap.JSON{"interaction_mode": "qr"}}
}

func TestCreateNativePayment(t *testing.T) {
	deadline := time.Now().Add(300 * time.Second)
	for _, expiryKey := range []string{`"expire_in":120`, `"expires_in":120`, `"expire_in":"120"`, `"expires_in":"120"`} {
		t.Run(expiryKey, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.URL.Path != "/api/pay/12345" || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
					t.Error("incorrect protocol request")
				}
				if err := r.ParseForm(); err != nil {
					t.Fatal(err)
				}
				if r.Form.Get("price") != "12.30" || r.Form.Get("pay_type") != "native" || r.Form.Get("order_id") != "DJP-test-1" {
					t.Error("incorrect payment facts")
				}
				if r.Form.Get("sign") != digest("测试订单", "native", "12.30", "DJP-test-1", "https://shop.example/api/v1/payments/callback", "test-secret-only") {
					t.Error("incorrect signature")
				}
				if r.Form.Has("app_secret") || r.Form.Has("order_uid") || strings.Contains(r.Form.Encode(), "test-secret-only") {
					t.Error("private data sent")
				}
				var seconds int
				fmt.Sscan(r.Form.Get("expire"), &seconds)
				if seconds < 290 || seconds > 300 {
					t.Errorf("incorrect expiry: %d", seconds)
				}
				fmt.Fprintf(w, `{"status":"ok","aoid":"aoid-1",%s,"info":{"qr":"weixin://wxpay/bizpayurl?pr=fixture"}}`, expiryKey)
			}))
			defer server.Close()
			a := &adapter{client: server.Client(), baseURL: server.URL}
			in := testInput()
			in.Extra["expires_at"] = deadline
			result, err := a.CreatePayment(context.Background(), testConfig(), in)
			if err != nil {
				t.Fatal(err)
			}
			if result.ProviderRef != "aoid-1" || result.CurrencySent != "CNY" || result.AmountSent != "12.30" || result.QRCodeURL == "" || result.RedirectURL != "" {
				t.Fatalf("invalid payment result: %+v", result)
			}
			if result.ExpiresAt == nil || time.Until(*result.ExpiresAt) > 121*time.Second {
				t.Fatal("provider expiry not honored")
			}
		})
	}
}

func TestNativeRejectsInvalidResponses(t *testing.T) {
	for _, body := range []string{`{"status":"fee_error","info":"sensitive-echo"}`, `{"status":"sign_error"}`, `{"status":"ok","aoid":"aoid-1","info":{"qr":"javascript:alert(1)"}}`, `{"status":"ok","info":{"qr":"weixin://wxpay/a"}}`, `{`, strings.Repeat("x", 65537)} {
		t.Run(fmt.Sprintf("case-%d", len(body)), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer server.Close()
			_, err := (&adapter{client: server.Client(), baseURL: server.URL}).CreatePayment(context.Background(), testConfig(), testInput())
			if err == nil || strings.Contains(err.Error(), "sensitive-echo") {
				t.Fatalf("unsafe result: %v", err)
			}
		})
	}
}

func TestNativeRejectsInvalidInputsBeforeHTTP(t *testing.T) {
	a := NewXorpayAdapter()
	for _, change := range []func(*paymentcontract.GatewayCreateInput){
		func(in *paymentcontract.GatewayCreateInput) { in.Currency = "USD" },
		func(in *paymentcontract.GatewayCreateInput) { in.ChannelType = "alipay" },
		func(in *paymentcontract.GatewayCreateInput) { in.Amount = money.FromDecimal(decimal.NewFromInt(0)) },
		func(in *paymentcontract.GatewayCreateInput) {
			in.Amount = money.FromDecimal(decimal.RequireFromString("1.001"))
		},
		func(in *paymentcontract.GatewayCreateInput) { in.Extra["interaction_mode"] = "redirect" },
		func(in *paymentcontract.GatewayCreateInput) { in.Extra["expires_at"] = time.Now().Add(-time.Second) },
	} {
		in := testInput()
		change(&in)
		if _, err := a.CreatePayment(context.Background(), testConfig(), in); err == nil {
			t.Fatal("invalid input accepted")
		}
	}
	for _, key := range []string{"aid", "app_secret", "notify_url"} {
		c := testConfig()
		c[key] = ""
		if a.ValidateConfig(c, "wechat") == nil {
			t.Fatalf("missing %s accepted", key)
		}
	}
	for _, notify := range []string{"http://shop.example/callback", "https://user:pass@shop.example/callback", "https://shop.example/callback#fragment"} {
		c := testConfig()
		c["notify_url"] = notify
		if a.ValidateConfig(c, "wechat") == nil {
			t.Fatal("invalid callback accepted")
		}
	}
}

func signedForm() url.Values {
	form := url.Values{"aoid": {"aoid-1"}, "order_id": {"DJP-test-1"}, "pay_price": {"12.30"}, "pay_time": {"2026-10-13 10:20:30"}}
	signForm(form)
	return form
}
func signForm(f url.Values) {
	f.Set("sign", strings.ToUpper(digest(f.Get("aoid"), f.Get("order_id"), f.Get("pay_price"), f.Get("pay_time"), "test-secret-only")))
}

func TestCallbackSignedFacts(t *testing.T) {
	a := NewXorpayAdapter().(paymentcontract.GatewayCallbackVerifier)
	f := signedForm()
	f.Set("more", "untrusted")
	f.Set("detail", `{"order_id":"different","transaction_id":"untrusted"}`)
	result, err := a.VerifyCallback(testConfig(), f, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.OrderNo != "DJP-test-1" || result.ProviderRef != "aoid-1" || result.Amount.String() != "12.30" || result.Status != "success" || result.Currency != "CNY" {
		t.Fatalf("bad callback: %+v", result)
	}
	if result.PaidAt.UTC().Hour() != 2 {
		t.Fatal("payment time must use China timezone")
	}
	if _, ok := result.Payload["sign"]; ok {
		t.Fatal("signature retained")
	}
	if _, ok := result.Payload["detail"]; ok {
		t.Fatal("unsigned metadata retained")
	}
	for _, key := range []string{"aoid", "order_id", "pay_price", "pay_time", "sign"} {
		f := signedForm()
		f.Set(key, "tampered")
		if _, err := a.VerifyCallback(testConfig(), f, nil); !errors.Is(err, paymentcontract.ErrGatewaySignatureInvalid) {
			t.Fatalf("tampered %s accepted", key)
		}
		f = signedForm()
		f[key] = append(f[key], f[key][0])
		if _, err := a.VerifyCallback(testConfig(), f, nil); err == nil {
			t.Fatalf("duplicate %s accepted", key)
		}
	}
	for _, amount := range []string{"0", "-1", "1e1", "12.301"} {
		f := signedForm()
		f.Set("pay_price", amount)
		signForm(f)
		if _, err := a.VerifyCallback(testConfig(), f, nil); err == nil {
			t.Fatalf("invalid amount %s accepted", amount)
		}
	}
	f = signedForm()
	f.Set("pay_time", "invalid-date")
	signForm(f)
	if _, err := a.VerifyCallback(testConfig(), f, nil); err == nil {
		t.Fatal("invalid date accepted")
	}
}

func TestClientRejectsRedirects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://example.invalid/steal", http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	a := NewXorpayAdapter().(*adapter)
	a.baseURL = server.URL
	if _, err := a.CreatePayment(context.Background(), testConfig(), testInput()); !errors.Is(err, paymentcontract.ErrGatewayRequestFailed) {
		t.Fatal("redirect accepted")
	}
}
