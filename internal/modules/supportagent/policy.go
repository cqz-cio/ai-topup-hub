package supportagent

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"
)

//go:embed policy-data.json
var policyBytes []byte
var policy = func() struct {
	Version  string
	Rules    map[string]string
	Products map[string]string
} {
	var p struct {
		Version  string
		Rules    map[string]string
		Products map[string]string
	}
	if err := json.Unmarshal(policyBytes, &p); err != nil {
		panic("invalid embedded support policy")
	}
	return p
}()

// BusinessState is ephemeral, per browser session and conversation. It never stores
// credentials, images, private orders or a claim that staff performed an action.
type BusinessState struct {
	Case             string
	Order            string
	Product          string
	Page             string
	Pending          string
	Stage            int
	Reopen           bool
	Amount           int64
	Currency         string
	Start, End, Loss time.Time
}

func contains(s string, terms ...string) bool {
	for _, x := range terms {
		if strings.Contains(s, x) {
			return true
		}
	}
	return false
}
func family(p string) string {
	switch {
	case strings.HasPrefix(p, "gemini"):
		return "gemini"
	case strings.HasPrefix(p, "claude"):
		return "claude"
	case strings.HasPrefix(p, "gpt") || p == "overwrite":
		return "gpt"
	}
	return p
}
func detectProduct(message string) string {
	s := strings.ToLower(message)
	// Multiple named families are comparisons, not an unambiguous selection.
	found := []string{}
	if contains(s, "codex", "点数", "點數") {
		found = append(found, "codex")
	}
	if contains(s, "gemini") {
		p := "gemini"
		if contains(s, "无质保", "無質保") {
			p = "gemini_none"
		} else if contains(s, "一个月质保", "一個月質保") {
			p = "gemini_warranty"
		}
		found = append(found, p)
	}
	if contains(s, "claude") {
		p := "claude"
		if contains(s, "max") {
			p = "claude_max"
		} else if contains(s, "pro") {
			p = "claude_pro"
		}
		found = append(found, p)
	}
	if contains(s, "telegram", "tg会员", "tg會員") {
		found = append(found, "telegram")
	}
	if contains(s, "gpt", "chatgpt") && !contains(s, "codex") {
		p := "gpt"
		if contains(s, "可覆盖分类", "可覆盖充值", "可覆蓋儲值") {
			p = "overwrite"
		}
		found = append(found, p)
	}
	if len(found) > 1 {
		return "ambiguous"
	}
	if len(found) == 1 {
		return found[0]
	}
	return ""
}
func policyReply(ids ...string) Reply {
	parts := []string{}
	for _, id := range ids {
		if text := policy.Rules[id]; text != "" {
			parts = append(parts, text)
		}
	}
	return Reply{Text: strings.Join(parts, "\n\n"), Mode: "local", RuleIDs: ids, PolicyVersion: policy.Version}
}
func clarify(text string) Reply {
	return Reply{Text: text, Mode: "local", PolicyVersion: policy.Version}
}
func human(ids ...string) Reply { r := policyReply(ids...); r.Handoff = true; return r }
func unknown(locale string) Reply {
	r := clarify(pick(locale, "这项情况暂时无法按已确认规则判断，请联系人工客服核实。我不会猜测或作承诺。", "This case is not covered by the confirmed rules. Please contact staff for verification; I cannot make a promise.", "這項情況暫時無法按已確認規則判斷，請聯絡人工客服核實，不猜測或作承諾。"))
	r.Handoff = true
	return r
}

func paymentQuestion(message string) bool {
	return contains(message, "付款方式", "支付方式", "支付渠道", "付款渠道", "怎么付款", "如何付款", "怎么支付", "如何支付", "怎么付", "手续费", "手續費", "实付", "實付", "usdt", "trc20", "bep20", "plasma", "币安", "欧易", "支付宝", "payment methods", "payment fee") ||
		(contains(message, "微信", "wechat") && contains(message, "支付", "付款", "扫码", "能", "可以", "支持", "fee"))
}

func paymentGuideLinks() []Link {
	return []Link{{Label: "币安 USDT 付款教程", To: "/blog/binance-usdt-payment-guide"}, {Label: "欧易 USDT 付款教程", To: "/blog/okx-usdt-payment-guide"}}
}

func businessAnswer(in chatInput, state *BusinessState) Reply {
	m := strings.ToLower(strings.TrimSpace(in.Message))
	if contains(m, "取消试算", "取消試算", "重新开始", "重新開始", "cancel") {
		*state = BusinessState{Page: in.Product, Product: policy.Products[in.Product]}
		return clarify("已清空本次试算信息。请说新的问题；这不会取消或修改任何订单。")
	}
	if state.Page != in.Product {
		*state = BusinessState{Page: in.Product, Product: policy.Products[in.Product]}
	}
	named := detectProduct(m)
	if family(state.Product) == "gemini" && named == "" {
		if contains(m, "无质保", "無質保") {
			named = "gemini_none"
		} else if contains(m, "一个月质保", "一個月質保") {
			named = "gemini_warranty"
		}
	}
	if named == "ambiguous" {
		state.Stage = 0
		state.Product = ""
		return clarify("这些商品的规则不同，请先选定一个商品，或发送其商城商品链接中的商品编号。")
	}
	if named != "" && (family(named) != family(state.Product) || named != family(named)) {
		pending := state.Pending
		*state = BusinessState{Page: in.Product, Product: named, Pending: pending}
	}
	for slug, kind := range policy.Products {
		if strings.TrimSpace(m) == slug {
			pending := state.Pending
			*state = BusinessState{Page: in.Product, Product: kind, Pending: pending}
			break
		}
	}
	p := state.Product
	f := family(p)
	if state.Stage > 0 && contains(m, "人工", "客服", "报错", "支付", "未发货", "取消") {
		state.Stage = 0
	}
	if state.Stage > 0 {
		return collectCalculation(m, state)
	}
	if state.Case != "" && orderPattern.MatchString(strings.TrimSpace(in.Message)) {
		state.Order = strings.TrimSpace(in.Message)
		r := human(state.Case)
		r.Text = "已在本次会话记录你提供的订单号：" + state.Order + "。请将此订单号及对应截图发给人工客服。这里只整理材料，未查询订单，也未提交工单、退款或补发。\n\n" + r.Text
		state.Case = ""
		return r
	}
	if contains(m, "人工", "客服时间", "几点", "客服联系方式", "human", "support hours") {
		return human("hours", "boundary")
	}
	if contains(m, "退到", "原路", "退款渠道", "退款方式", "多久退款", "退款多久", "多久到账", "微信退款") || (contains(m, "支付宝") && contains(m, "退")) {
		return human("refund_channel")
	}
	if contains(m, "填错", "选错", "转到其他", "转移账号", "uid错") {
		return human("wrong")
	}
	if contains(m, "未激活", "没激活", "未兑换", "没兑换", "不想要", "买错") && contains(m, "退", "取消") {
		return human("unused", "refund_channel")
	}
	if contains(m, "没发货", "未发货", "尚未发货") {
		return human("manual", "refund_channel")
	}
	if contains(m, "已付款", "已经付款", "支付成功", "已支付", "付了款") && contains(m, "未付款", "没到账", "未支付", "不更新", "显示待支付") {
		return beginCase(state, "payment")
	}
	if contains(m, "报错", "无效", "已被使用", "兑换失败", "激活失败") {
		return beginCase(state, "error")
	}
	if contains(m, "处理中", "没到账", "未到账", "充值中") {
		return beginCase(state, "pending")
	}
	if contains(m, "钱包", "wallet") && contains(m, "充值", "怎么用", "余额", "top up", "balance") && !contains(m, "退") {
		r := policyReply("wallet", "payment_methods", "payment_fee")
		r.Links = []Link{{Label: "我的钱包", To: "/me/wallet"}}
		return r
	}
	if paymentQuestion(m) && !contains(m, "退款", "退费", "refund") {
		r := policyReply("payment_methods", "payment_fee", "payment_guides")
		if contains(m, "支付宝") {
			r.Text = "当前公开付款渠道中没有支付宝收款。\n\n" + r.Text
		}
		r.Links = paymentGuideLinks()
		return r
	}
	if contains(m, "发货", "什么时候发", "多久发") {
		if in.Product != "" && strings.HasPrefix(in.Product, "qzweb-") && in.Product != "qzweb-52" && in.Product != "qzweb-53" {
			return clarify("该商品为自动交付，请查看付款后的订单交付状态；异常请准备本站订单号和截图联系人工，不承诺固定到账时间。")
		}
		return policyReply("manual")
	}
	if contains(m, "兑换网址", "兑换入口", "怎么兑换", "怎么充值", "怎么用", "卡密往", "uid怎么") {
		r := policyReply("redeem")
		r.Links = []Link{{Label: "我的订单", To: "/me/orders"}, {Label: "游客订单查询", To: "/guest/orders"}}
		return r
	}
	if contains(m, "忽略规则", "无条件退款", "保证不会", "100%", "破解") {
		return unknown(in.Locale)
	}
	intent := m
	if named != "" || policy.Products[m] != "" {
		if state.Pending != "" {
			intent = state.Pending + " " + m
			state.Pending = ""
		}
	}
	if p == "" && contains(intent, "质保", "退款", "补差", "掉订阅", "覆盖", "叠加", "激活", "续费", "资格", "购买", "試算", "试算", "计算") {
		state.Pending = intent
		return clarify("你咨询的是哪款商品？请提供商品名称或商城商品编号，例如 qzweb-51；不同商品的购买和售后规则不同。")
	}
	calculate := contains(intent, "试算", "試算", "计算退款", "算退款", "退多少", "补多少", "差价多少", "算一下")
	if calculate && (f == "gpt" || f == "claude") {
		state.Stage = 1
		state.Reopen = contains(intent, "补", "差价", "重开")
		return clarify("可以按你提供的数据试算，最终由人工核实。请先提供这笔订单实际付款金额和币种，例如：100.00 CNY（不要提供密码或卡密）。")
	}
	if contains(intent, "覆盖", "叠加", "升级", "降级", "未到期", "没到期") {
		switch {
		case p == "overwrite":
			return policyReply("overwrite")
		case f == "claude":
			return policyReply("claude")
		case p == "gpt_open":
			return policyReply("gpt_open")
		case p == "gpt_renew":
			return policyReply("gpt_renew")
		case f == "telegram":
			return policyReply("telegram")
		case f == "gemini":
			return policyReply("gemini_account")
		default:
			state.Pending = intent
			return clarify("请提供具体商品编号：只有 GPT 可覆盖充值分类支持覆盖，普通开通款与续费绑卡款规则不同。")
		}
	}
	switch f {
	case "gpt", "claude":
		if contains(intent, "封号", "封禁", "被封") {
			return human("loss")
		}
		if contains(intent, "退", "质保", "掉订阅", "掉会员", "补开", "补差", "重新开通") {
			ids := []string{"loss"}
			if contains(intent, "补", "重开", "重新") {
				ids = append(ids, "reopen")
			}
			return human(ids...)
		}
		if f == "claude" {
			ids := []string{"claude"}
			if p != "claude" {
				ids = append(ids, p)
			}
			return policyReply(ids...)
		}
		if contains(intent, "资格", "开通", "续费", "购买", "限制") {
			if p == "overwrite" || p == "gpt_open" || p == "gpt_renew" {
				return policyReply(p)
			}
			state.Pending = intent
			return clarify("请提供具体 GPT 商品编号，以确认开通、续费绑卡或可覆盖充值条件。")
		}
	case "gemini":
		if contains(intent, "antigravity") {
			return policyReply("gemini_feature")
		}
		if contains(intent, "质保", "退款", "掉", "试算") {
			if p == "gemini" {
				state.Pending = intent
				return clarify("你购买的是 Gemini 无质保款，还是一个月质保款？也可以提供商品编号。")
			}
			return human(p)
		}
		return policyReply("gemini_account")
	case "telegram":
		return policyReply("telegram")
	case "codex":
		return policyReply("codex")
	}
	return unknown(in.Locale)
}

var moneyPattern = regexp.MustCompile(`(?i)^([0-9]{1,9})(?:\.([0-9]{1,2}))?\s*(CNY|USD|USDT|RMB|元|人民币)$`)

func parseMoney(s string) (int64, string, bool) {
	m := moneyPattern.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, "", false
	}
	fraction := m[2]
	for len(fraction) < 2 {
		fraction += "0"
	}
	v, ok := new(big.Int).SetString(m[1]+fraction, 10)
	if !ok || !v.IsInt64() {
		return 0, "", false
	}
	currency := strings.ToUpper(m[3])
	if currency == "RMB" || currency == "元" || currency == "人民币" {
		currency = "CNY"
	}
	return v.Int64(), currency, true
}
func money(v int64) string { return fmt.Sprintf("%d.%02d", v/100, v%100) }
func parseMoment(s string) (time.Time, error) {
	if t, e := time.Parse(time.RFC3339, s); e == nil {
		return t, nil
	}
	return time.ParseInLocation("2006-01-02 15:04", s, time.FixedZone("Asia/Shanghai", 8*3600))
}

// Integer rational arithmetic; half-up only at the final monetary result.
func refundCents(paid int64, start, end, loss time.Time) (int64, error) {
	if paid <= 0 || !end.After(start) || loss.Before(start) || loss.After(end) || end.Year()-start.Year() > 10 {
		return 0, fmt.Errorf("invalid amount or actual times")
	}
	total := end.Unix() - start.Unix()
	remaining := end.Unix() - loss.Unix()
	if total <= 0 {
		return 0, fmt.Errorf("invalid duration")
	}
	numerator := new(big.Int).Mul(big.NewInt(paid), big.NewInt(remaining))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(numerator, big.NewInt(total), remainder)
	if remainder.Mul(remainder, big.NewInt(2)).Cmp(big.NewInt(total)) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if !quotient.IsInt64() {
		return 0, fmt.Errorf("amount overflow")
	}
	return quotient.Int64(), nil
}
func collectCalculation(m string, s *BusinessState) Reply {
	dateHint := "请按 YYYY-MM-DD HH:mm 提供准确时间（北京时间），例如 2026-10-01 12:00；不清楚时请人工核实，不估算。"
	switch s.Stage {
	case 1:
		amount, currency, ok := parseMoney(m)
		if !ok || amount <= 0 {
			return clarify("请提供有效实付金额及币种，例如 100.00 CNY，金额最多两位小数。")
		}
		s.Amount = amount
		s.Currency = currency
		s.Stage = 2
		return clarify("实际订阅成功开始时间是什么？" + dateHint)
	case 2, 3, 4:
		// RFC3339 parsing expects the original uppercase T/Z.
		t, e := parseMoment(strings.ReplaceAll(strings.ReplaceAll(m, "t", "T"), "z", "Z"))
		if e != nil {
			return clarify(dateHint)
		}
		if t.After(time.Now()) && (s.Stage == 2 || s.Stage == 4) {
			return clarify("开始或掉订阅时间不能在未来，请核对后重新提供。")
		}
		switch s.Stage {
		case 2:
			s.Start = t
			s.Stage = 3
			return clarify("原订阅的实际到期时间是什么？" + dateHint)
		case 3:
			if !t.After(s.Start) || t.Year()-s.Start.Year() > 10 {
				return clarify("到期时间须晚于开始时间，请核对真实订阅记录。")
			}
			s.End = t
			s.Stage = 4
			return clarify("实际掉订阅的时间是什么？不是联系售后的时间。" + dateHint)
		case 4:
			if t.Before(s.Start) || t.After(s.End) {
				return clarify("掉订阅时间不在提供的订阅区间内，请核对；区间外情况需人工判断。")
			}
			s.Loss = t
			if s.Reopen {
				s.Stage = 5
				return clarify("重新开通时该商品的售价和币种是多少？请使用与实付金额相同的币种；这只是基于你提供售价的试算。")
			}
		}
	case 5:
		price, currency, ok := parseMoney(m)
		if !ok || price <= 0 || currency != s.Currency {
			return clarify("请提供正数售价与相同币种；不自行换汇，例如 120.00 CNY。")
		}
		refund, err := refundCents(s.Amount, s.Start, s.End, s.Loss)
		if err != nil {
			return human("boundary")
		}
		s.Stage = 0
		if price < refund {
			return human("reopen", "boundary")
		}
		r := human("reopen")
		r.Text = fmt.Sprintf("按你提供的数据，应退 %s %s；以你提供的重开售价 %s %s 计算，应补 %s %s。此为试算，最终由人工核实金额和售价后办理。请准备本站订单号和掉订阅截图。", money(refund), currency, money(price), currency, money(price-refund), currency)
		return r
	}
	refund, e := refundCents(s.Amount, s.Start, s.End, s.Loss)
	if e != nil {
		return human("boundary")
	}
	s.Stage = 0
	r := human("loss")
	r.Text = fmt.Sprintf("试算：%s %s × 剩余 %d 秒 ÷ 总计 %d 秒＝%s %s（四舍五入两位小数）。依据你提供的实际时间，不按固定 30 天。请准备本站订单号和掉订阅截图，最终资格、时间和金额由人工核实；我未提交退款。", money(s.Amount), s.Currency, s.End.Unix()-s.Loss.Unix(), s.End.Unix()-s.Start.Unix(), money(refund), s.Currency)
	return r
}

var orderPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{3,79}$`)

func beginCase(s *BusinessState, id string) Reply {
	s.Case = id
	s.Order = ""
	r := human(id)
	r.Text += "\n\n如果需要我整理给客服的说明，请先提供本站订单号（不要提供支付密码或完整卡密）。"
	return r
}

// Confirmed business policy bypasses the model even if paid natural-language
// assistance is explicitly enabled later. Policy state is never model-controlled.
func requiresPolicy(in chatInput, s BusinessState) bool {
	if s.Stage > 0 || s.Case != "" || s.Pending != "" {
		return true
	}
	m := strings.ToLower(in.Message)
	return paymentQuestion(m) || contains(m, "钱包", "wallet") || contains(m, "退", "质保", "保修", "补差", "补开", "封号", "封禁", "掉订阅", "掉会员", "覆盖", "叠加", "升级", "降级", "激活", "优惠", "试用", "注册", "资格", "填错", "选错", "报错", "失败", "无效", "已被使用", "处理中", "没到账", "未到账", "发货", "客服", "人工", "试算", "billing", "refund", "warranty")
}
func confirmedSources(in chatInput) []Source {
	p := policy.Products[in.Product]
	if named := detectProduct(in.Message); named != "" {
		if family(named) != family(p) || named != family(named) {
			p = named
		}
	}
	ids := []string{"boundary", "hours", "refund_channel", "unused", "error", "payment", "pending", "wrong", "redeem", "manual", "payment_methods", "payment_fee", "payment_guides", "wallet"}
	switch family(p) {
	case "gpt":
		ids = append(ids, "loss", "reopen")
		if p != "gpt" {
			ids = append(ids, p)
		}
	case "claude":
		ids = append(ids, "loss", "reopen", "claude")
		if p != "claude" {
			ids = append(ids, p)
		}
	case "gemini":
		ids = append(ids, "gemini_account", "gemini_feature")
		if p != "gemini" {
			ids = append(ids, p)
		}
	case "codex", "telegram":
		ids = append(ids, p)
	}
	sources := []Source{{ID: "policy:scope", Text: "已确认规则版本：" + policy.Version + "。当前规则组：" + p + "。policy: 资料优先于历史商品文案；未明确适用的规则不能跨商品套用，未知资格与金额请人工核实。"}}
	for _, id := range ids {
		sources = append(sources, Source{ID: "policy:" + id, Text: policy.Rules[id]})
	}
	return sources
}
