// alipay-sandbox uses only virtual funds and never loads the shop database or procurement services.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/alipay"
	"github.com/shopspring/decimal"
)

var orderPattern = regexp.MustCompile(`^ALIPAY-SANDBOX-[A-F0-9]{24}$`)

func loadSandboxConfig(path string) (*alipay.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("cannot read sandbox configuration file")
	}
	var cfg alipay.Config
	if len(data) > 64*1024 {
		return nil, errors.New("sandbox configuration file is too large")
	}
	data = []byte(strings.TrimPrefix(string(data), "\ufeff"))
	if json.Unmarshal(data, &cfg) != nil {
		// The official console's Copy button exports AIPAY_* assignments.
		// Parse them as inert text, never as shell commands or environment code.
		fields := map[string]string{}
		for _, line := range strings.Split(string(data), "\n") {
			name, value, ok := strings.Cut(strings.TrimSpace(line), "=")
			if ok {
				fields[strings.TrimSpace(name)] = strings.Trim(strings.TrimSpace(value), "\"'")
			}
		}
		cfg = alipay.Config{
			AppID: fields["AIPAY_APP_ID"], PrivateKey: fields["AIPAY_PRIVATE_PKCS_KEY"],
			AlipayPublicKey: fields["AIPAY_ALIPAY_PUBLIC_KEY"], GatewayURL: alipay.SandboxGateway, SignType: "RSA2",
		}
		if cfg.PrivateKey == "" {
			cfg.PrivateKey = fields["AIPAY_PRIVATE_KEY"]
		}
	}
	cfg.Normalize()
	if !alipay.IsSandboxGateway(cfg.GatewayURL) {
		return nil, errors.New("only the official sandbox gateway is permitted")
	}
	if err := alipay.ValidateConfig(&cfg, constants.PaymentInteractionPage); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func main() {
	configPath := flag.String("config", "", "private sandbox JSON or official AIPAY_* text file; never pass keys on the command line")
	action := flag.String("action", "", "create or query")
	orderNo := flag.String("order", "", "sandbox order number from create")
	amountText := flag.String("amount", "0.01", "expected sandbox amount in CNY (0.01 to 1.00)")
	flag.Parse()
	if err := run(*configPath, *action, *orderNo, *amountText); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(configPath, action, orderNo, amountText string) error {
	if action != "create" && action != "query" {
		return errors.New("action must be create or query")
	}
	amount, err := decimal.NewFromString(amountText)
	if err != nil || amount.LessThan(decimal.RequireFromString("0.01")) || amount.GreaterThan(decimal.NewFromInt(1)) || !amount.Equal(amount.Round(2)) {
		return errors.New("sandbox test amount must be 0.01 to 1.00 CNY with at most two decimals")
	}
	cfg, err := loadSandboxConfig(configPath)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if action == "create" {
		if orderNo != "" {
			return errors.New("create generates a fresh order; do not supply -order")
		}
		var id [12]byte
		if _, err := rand.Read(id[:]); err != nil {
			return errors.New("cannot generate sandbox order number")
		}
		orderNo = "ALIPAY-SANDBOX-" + strings.ToUpper(hex.EncodeToString(id[:]))
		result, err := alipay.CreatePayment(ctx, cfg, alipay.CreateInput{
			OrderNo: orderNo, Amount: amount.StringFixed(2), Subject: "Alipay sandbox integration test", TimeoutExpress: "15m",
		}, constants.PaymentInteractionPage)
		if err != nil {
			return err
		}
		dir := filepath.Join("work", "alipay-sandbox")
		if err := os.MkdirAll(dir, 0700); err != nil {
			return errors.New("cannot create sandbox output directory")
		}
		path, err := filepath.Abs(filepath.Join(dir, orderNo+".html"))
		if err != nil {
			return errors.New("cannot resolve sandbox output file")
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return errors.New("cannot create sandbox payment page")
		}
		tmpl := template.Must(template.New("payment").Parse(`<!doctype html><meta charset="utf-8"><title>支付宝沙箱测试</title><h1>支付宝沙箱测试</h1><p>订单：{{.Order}}</p><p>金额：{{.Amount}} 元（沙箱虚拟资金）</p><p><a href="{{.URL}}" rel="noreferrer">打开支付宝沙箱收银台</a></p><p>完成后使用 query 命令验证支付结果。生成本页不表示已经付款。</p>`))
		err = tmpl.Execute(file, struct{ Order, Amount, URL string }{orderNo, amount.StringFixed(2), result.PayURL})
		closeErr := file.Close()
		if err != nil || closeErr != nil {
			return errors.New("cannot write sandbox payment page")
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]string{
			"status": "request_prepared", "order_no": orderNo, "amount": amount.StringFixed(2), "payment_page": path,
		})
	}
	if !orderPattern.MatchString(orderNo) {
		return errors.New("use the sandbox order number returned by create")
	}
	result, err := alipay.QueryPaymentByOrderNo(ctx, cfg, orderNo)
	if err != nil {
		return err
	}
	paid := result.TradeStatus == "TRADE_SUCCESS" || result.TradeStatus == "TRADE_FINISHED"
	actual, err := decimal.NewFromString(result.TotalAmount)
	if err != nil || !actual.Equal(amount) {
		return errors.New("verified sandbox amount differs from the expected amount")
	}
	if err := json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
		"order_no": result.OutTradeNo, "trade_status": result.TradeStatus,
		"signature_valid": true, "amount_matched": true, "sandbox_payment_verified": paid,
		"public_notify_tested": false,
	}); err != nil {
		return err
	}
	if !paid {
		return errors.New("sandbox trade is not paid yet")
	}
	return nil
}
