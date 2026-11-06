package supportagent

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var plusNamePattern = regexp.MustCompile(`(?i)\bplus\b`)
var proNamePattern = regexp.MustCompile(`(?i)\bpro\b`)
var naturalPaidPattern = regexp.MustCompile(`(?i)(?:花(?:了)?|付了|实付|實付)\s*([0-9]{1,9}(?:\.[0-9]{1,2})?)(?:\s*(CNY|USDT|USD|RMB|人民币|美元|元|块钱|块))?`)
var shortDatePattern = regexp.MustCompile(`[0-9]{1,2}\s*月\s*[0-9]{1,2}\s*[日号]`)
var naturalDatePattern = regexp.MustCompile(`^([0-9]{4})\s*年\s*([0-9]{1,2})\s*月\s*([0-9]{1,2})\s*[日号]\s*([0-9]{1,2})\s*(?:[:：]([0-9]{1,2})|[点时](?:\s*([0-9]{1,2})\s*分?)?)$`)
var feeBasePattern = regexp.MustCompile(`(?:不是|标价|售价|本金)\s*([0-9]{1,9}(?:\.[0-9]{1,2})?)\s*(?:元|块)`)

// A product family is not a specific catalog selection. Keep Plus as generic
// GPT here; the public catalog still resolves auto-code vs. manual top-up.
func matchesRequestedProduct(query, title string) bool {
	if plusNamePattern.MatchString(query) && !proNamePattern.MatchString(query) && !plusNamePattern.MatchString(title) {
		return false
	}
	if proNamePattern.MatchString(query) && !plusNamePattern.MatchString(query) && !proNamePattern.MatchString(title) {
		return false
	}
	if contains(query, "自动发码", "自動發碼") && !contains(title, "自动发码", "自動發碼") {
		return false
	}
	if contains(query, "人工代充", "人工发货") && !contains(title, "人工", "代充") {
		return false
	}
	return true
}

func asksForPerson(message string) bool {
	return contains(message, "找个人帮", "找个真人", "找人帮我", "找人看看", "找工作人员", "让工作人员", "讓工作人員") &&
		!contains(message, "不要找", "不用找", "不想找", "不用让", "不要让")
}

func handoffPreparation(issue string) Reply {
	ids := []string{"hours", "boundary"}
	if id := issueRule(issue); id != "" {
		ids = append(ids, id)
	}
	r := handoffReply(ids...)
	materials := "本站订单号和报错截图"
	if issue == "payment_pending" {
		materials = "本站订单号和支付成功截图"
	}
	r.Text = "可以，把" + materials + "发给下面的客服，请对方核实。截图请遮住密码、登录 token 和完整卡密。\n\n" + policy.Rules["hours"] + "\n\n这里只帮你整理材料，未提交工单，需要你主动联系。"
	return r
}

func asksAboutWeChatFee(message string) bool {
	return contains(message, "微信", "wechat") && contains(message, "手续费", "手續費", "变成", "變成", "多扣", "多了", "贵了") &&
		!contains(message, "支付方式", "付款方式", "支付渠道", "付款渠道", "usdt", "币安", "欧易")
}

func weChatFeeReply(message string) Reply {
	r := policyReply("payment_fee")
	matches := feeBasePattern.FindAllStringSubmatch(message, -1)
	if len(matches) == 1 {
		principal, _, ok := parseMoney(matches[0][1] + " CNY")
		if ok && principal > 0 {
			fee := (principal*16 + 999) / 1000
			r.Text = fmt.Sprintf("多出来的是微信支付的 1.6%% 手续费，不足一分向上取整。按 %s 元本金计算，手续费 %s 元，合计 %s 元。最终以结算页显示为准。", money(principal), money(fee), money(principal+fee))
		}
	}
	return r
}

func beginNaturalCalculation(message string, state *BusinessState) (Reply, bool) {
	parts := naturalPaidPattern.FindAllStringSubmatch(message, -1)
	// Multiple amounts may refer to different orders. Never pick one silently.
	if len(parts) != 1 {
		return Reply{}, false
	}
	indices := naturalPaidPattern.FindStringSubmatchIndex(message)
	amountEnd := indices[3]
	if amountEnd < len(message) && strings.ContainsRune(".0123456789eE", rune(message[amountEnd])) {
		return Reply{}, false
	}
	amount, _, ok := parseMoney(parts[0][1] + " CNY")
	if !ok || amount <= 0 {
		return Reply{}, false
	}
	state.Amount = amount
	state.NaturalDates = shortDatePattern.MatchString(message)
	state.Currency = currencyReply(parts[0][2])
	if state.Currency == "" {
		return clarify(fmt.Sprintf("可以帮你算。你说的 %s 是这笔订单实际付的人民币金额吗？如果是其他币种，请告诉我币种。", money(amount))), true
	}
	state.Stage = 2
	return calculationStartQuestion(state), true
}

func currencyReply(message string) string {
	switch strings.Trim(strings.ToUpper(strings.TrimSpace(message)), "。.!！") {
	case "CNY", "RMB", "人民币", "人民幣", "元", "块", "块钱", "是人民币", "是人民幣", "是的", "是", "对", "對":
		return "CNY"
	case "USD", "美元", "是美元":
		return "USD"
	case "USDT", "是USDT":
		return "USDT"
	}
	return ""
}

func calculationStartQuestion(state *BusinessState) Reply {
	prefix := "实际订阅成功开始时间是什么？"
	if state.NaturalDates {
		prefix = "你提到的日期还缺年份和具体时刻。请先补充实际开通时间。"
	}
	return clarify(prefix + "例如“2026年2月1日0点”（北京时间）；不清楚时请人工核实，不估算。")
}

func normalizeNaturalDate(value string) (string, bool) {
	parts := naturalDatePattern.FindStringSubmatch(strings.TrimSpace(value))
	if parts == nil {
		return "", false
	}
	values := make([]int, 5)
	for i := 0; i < 4; i++ {
		values[i], _ = strconv.Atoi(parts[i+1])
	}
	minute := parts[5]
	if minute == "" {
		minute = parts[6]
	}
	if minute != "" {
		values[4], _ = strconv.Atoi(minute)
	}
	return fmt.Sprintf("%04d-%02d-%02d %02d:%02d", values[0], values[1], values[2], values[3], values[4]), true
}

// Render the exact duration without rounding the elapsed time used for money.
func readableDuration(seconds int64) string {
	if seconds == 0 {
		return "0天"
	}
	parts := make([]string, 0, 4)
	for _, unit := range []struct {
		seconds int64
		label   string
	}{{86400, "天"}, {3600, "小时"}, {60, "分钟"}, {1, "秒"}} {
		if value := seconds / unit.seconds; value > 0 {
			parts = append(parts, fmt.Sprintf("%d%s", value, unit.label))
			seconds %= unit.seconds
		}
	}
	return strings.Join(parts, "")
}
