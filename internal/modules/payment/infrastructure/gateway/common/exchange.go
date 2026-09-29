package common

import (
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

// ExchangeRateConfig 通用汇率配置，可嵌入到各支付渠道的 Config 中。
type ExchangeRateConfig struct {
	TargetCurrency string `json:"target_currency"`
	ExchangeRate   string `json:"exchange_rate"`
}

// NormalizeExchangeRate 归一化汇率配置字段。
func (c *ExchangeRateConfig) NormalizeExchangeRate() {
	c.TargetCurrency = strings.ToUpper(strings.TrimSpace(c.TargetCurrency))
	c.ExchangeRate = strings.TrimSpace(c.ExchangeRate)
}

// NeedsCurrencyConversion 是否需要货币转换。
func (c *ExchangeRateConfig) NeedsCurrencyConversion() bool {
	return c.TargetCurrency != "" && c.ExchangeRate != ""
}

// ConvertAmount 将原始金额按汇率转换为目标货币金额。
// precision 指定小数位数（法币一般为 2）。
// 返回转换后的金额字符串和目标货币。
func (c *ExchangeRateConfig) ConvertAmount(amount, currency string, precision int32) (string, string, error) {
	if !c.NeedsCurrencyConversion() {
		return amount, currency, nil
	}
	amountDec, err := decimal.NewFromString(strings.TrimSpace(amount))
	if err != nil {
		return "", "", fmt.Errorf("invalid amount %q", amount)
	}
	rate, err := decimal.NewFromString(c.ExchangeRate)
	if err != nil || rate.LessThanOrEqual(decimal.Zero) {
		return "", "", fmt.Errorf("invalid exchange_rate %q", c.ExchangeRate)
	}
	converted := amountDec.Mul(rate).Round(precision)
	return converted.String(), c.TargetCurrency, nil
}
