package supportagent

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var catalogXName = regexp.MustCompile(`(?i)(^|[^a-z0-9])x([^a-z0-9]|$)`)
var catalogSlug = regexp.MustCompile(`(?:ellaix-|qzweb-|86gamestore-|gpt-plus-)[a-z0-9-]+`)
var catalogPriceChoice = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s*元(?:的|款)`)

func catalogFamilies(q string) []string {
	result := []string{}
	for _, f := range []string{"grok", "claude", "gemini", "telegram", "codex", "gpt"} {
		if strings.Contains(q, f) {
			result = append(result, f)
		}
	}
	if catalogXName.MatchString(q) || contains(q, "推特", "twitter") {
		result = append(result, "x")
	}
	if len(result) == 0 && contains(q, "tg会员", "tg會員") {
		result = append(result, "telegram")
	}
	if len(result) == 0 && contains(q, "成品号", "成品號") {
		result = append(result, "gpt")
	}
	return result
}

func catalogCandidates(q string, families []string) []publicProduct {
	result := []publicProduct{}
	for _, p := range syncedCatalog.Products {
		f := snapshotFamily(p)
		if len(families) > 0 {
			found := false
			for _, want := range families {
				if f == want {
					found = true
				}
			}
			if !found {
				continue
			}
		}
		title := strings.ToLower(language(p.Title, "zh-CN"))
		match := true
		for _, term := range []string{"premium", "super", "ultra", "plus", "pro", "max", "月付", "年付", "半价"} {
			if strings.Contains(q, term) && !strings.Contains(title, term) {
				match = false
			}
		}
		for _, term := range []string{"5x", "10x", "20x", "25x"} {
			re := regexp.MustCompile(`(?:^|[^0-9])` + term + `(?:$|[^a-z0-9])`)
			if re.MatchString(q) && !re.MatchString(title) {
				match = false
			}
		}
		if contains(q, "正价", "全价款") && strings.Contains(title, "半价") {
			match = false
		}
		if f == "gpt" && contains(q, "成品号", "成品號") && !strings.Contains(title, "成品号") {
			match = false
		}
		if contains(q, "自动发码", "自動發碼") && !strings.Contains(title, "自动发码") {
			match = false
		}
		if contains(q, "人工代充", "人工发货", "覆盖充值") && !strings.Contains(title, "人工") {
			match = false
		}
		if f == "gemini" {
			if contains(q, "无质保", "無質保", "无持续") && !strings.Contains(title, "无持续") {
				match = false
			}
			if contains(q, "一个月质保", "一個月質保") && !strings.Contains(title, "一个月质保") {
				match = false
			}
		}
		if f == "telegram" {
			for _, term := range []string{"3个月", "6个月", "12个月"} {
				if strings.Contains(strings.ReplaceAll(q, " ", ""), term) && !strings.Contains(strings.ReplaceAll(title, " ", ""), term) {
					match = false
				}
			}
		}
		if parts := catalogPriceChoice.FindStringSubmatch(q); parts != nil {
			want, _, ok := parseMoney(parts[1] + " CNY")
			got, _, _ := parseMoney(p.Price + " CNY")
			if !ok || want != got {
				match = false
			}
		}
		if match {
			result = append(result, p)
		}
	}
	return result
}

func catalogList(products []publicProduct, state *BusinessState, question string) Reply {
	state.Case = ""
	state.CatalogChoices = nil
	state.CatalogQuestion = question
	state.CatalogSlug = ""
	state.Stage = 0
	lines := []string{fmt.Sprintf("同步的正式商城在售目录共 %d 款；本次匹配 %d 款：", syncedCatalog.Count, len(products))}
	for i, p := range products {
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, snapshotPrice(p)))
		state.CatalogChoices = append(state.CatalogChoices, p.Slug)
	}
	lines = append(lines, "请回复具体商品名称、编号（slug）或本次列表序号，我再说明这款的购买条件与售后。", snapshotNotice())
	return Reply{Text: strings.Join(lines, "\n"), Mode: "local", PolicyVersion: policy.Version}
}

func catalogTopic(q string) string {
	switch {
	case contains(q, "未兑换", "未使用", "没兑换", "没用过", "未激活", "没激活", "买错", "不想要"):
		return "unused"
	case contains(q, "退", "质保", "保修", "掉订阅", "掉会员", "封号", "封禁", "补差", "重开", "补开", "售后"):
		return "warranty"
	case contains(q, "发货", "多久发", "交付"):
		return "delivery"
	case contains(q, "怎么用", "如何用", "怎么登录", "怎么领", "卡密在哪", "2fa", "邮箱", "怎么充值", "怎么兑换", "兑换入口"):
		return "redemption"
	case contains(q, "资格", "条件", "kyc", "优惠", "半价", "覆盖", "叠加", "续费", "未到期", "没到期", "验证", "地区", "尼日利亚", "学生", "免费", "试用", "激活", "能用", "可以", "能不能"):
		return "eligibility"
	case contains(q, "多少钱", "价格", "售价", "库存", "有货", "规格", "套餐", "介绍", "详情"):
		return "general"
	default:
		return "general"
	}
}

func catalogReply(p publicProduct, q string) Reply {
	topic := catalogTopic(q)
	rule := snapshotPolicyRule(p.Slug, topic)
	text := rule.Text
	if topic == "general" {
		text = language(p.Description, "zh-CN") + "\n\n购买条件：\n" + syncedCatalog.Policies[p.Slug].Eligibility + "\n\n售后：\n" + syncedCatalog.Policies[p.Slug].AfterSales
		for _, sku := range p.SKUs {
			if sku.Active && len(sku.Specs) > 0 {
				encoded, _ := json.Marshal(sku.Specs)
				text += "\n规格：" + string(encoded) + "；¥" + sku.Price
			}
		}
		if contains(q, "全部详情", "完整详情") {
			text = language(p.Content, "zh-CN")
		}
	}
	if contains(q, "未发货", "没发货", "尚未发货") && contains(q, "退", "取消") {
		text = "该商品未发货退款条件请提供订单号由人工核实，不承诺已退款。"
		for _, section := range p.Sections {
			if section.Heading == "未发货退款" {
				text = section.Text
			}
		}
		topic = "warranty"
	}
	if contains(q, "填错", "选错", "uid错") {
		text = "请提供订单号由人工核实账号错误的处理条件；客服不能修改账号或订单，也不承诺退款。"
		for _, section := range p.Sections {
			if section.Heading == "账号核对" {
				text = section.Text
			}
		}
	}
	if contains(q, "antigravity") {
		for _, s := range p.Sections {
			if strings.Contains(strings.ToLower(s.Heading), "antigravity") {
				text = s.Text
			}
		}
	}
	return Reply{Text: snapshotPrice(p) + "\n\n" + text + "\n\n" + snapshotNotice(), Mode: "local", PolicyVersion: policy.Version, RuleIDs: []string{rule.ID}, Handoff: topic == "warranty" || topic == "unused"}
}

// This path consumes the immutable reviewed public snapshot. It makes no network,
// model, payment, database or supplier calls. General support retains existing
// rules, while product-specific policy can never fall back to another product.
func (c *Catalog) OfflineAnswer(in chatInput, state *BusinessState) (Reply, bool) {
	if c.base != snapshotCatalogBase {
		return Reply{}, false
	}
	q := strings.ToLower(strings.TrimSpace(in.Message))
	if state.CatalogPage != in.Product {
		*state = BusinessState{CatalogPage: in.Product, Page: in.Product}
		if _, ok := snapshotProduct(in.Product); ok {
			state.CatalogSlug = in.Product
		}
	}
	if contains(q, "重新开始", "重新開始", "取消试算", "cancel") {
		*state = BusinessState{CatalogPage: in.Product, Page: in.Product, CatalogSlug: in.Product}
		return clarify("已清空本次问答和试算进度。请说新的问题；没有取消或修改任何订单。"), true
	}
	if contains(q, "人工", "客服时间", "客服几点", "客服联系方式") || asksForPerson(q) {
		state.Stage = 0
		state.CatalogChoices = nil
		state.CatalogQuestion = ""
		return handoffPreparation(""), true
	}
	if contains(q, "已付款", "支付成功", "已支付") && contains(q, "未付款", "未支付", "没到账", "待支付") {
		state.Stage = 0
		return beginCase(state, "payment"), true
	}
	if state.Case != "" && orderPattern.MatchString(in.Message) && !catalogSlug.MatchString(q) && len(catalogFamilies(q)) == 0 {
		return businessAnswer(in, state), true
	}
	if contains(q, "报错", "无效", "已被使用", "兑换失败", "激活失败") {
		state.Stage = 0
		return beginCase(state, "error"), true
	}
	if contains(q, "处理中", "充值中", "未到账", "没到账") {
		state.Stage = 0
		return beginCase(state, "pending"), true
	}
	if paymentQuestion(q) && !contains(q, "退", "refund") || contains(q, "钱包") && !contains(q, "退") {
		state.Stage = 0
		return businessAnswer(in, state), true
	}
	if contains(q, "退款方式", "退款渠道", "原路", "退款多久", "多久到账") {
		state.Stage = 0
		return handoffReply("refund_channel"), true
	}
	if contains(q, "忽略规则", "无条件退款", "100%", "破解") {
		return unknown(in.Locale), true
	}
	if contains(q, "全部商品", "所有商品", "有哪些商品", "卖什么", "all products") {
		return catalogList(syncedCatalog.Products, state, q), true
	}

	selected := ""
	explicit := false
	if matches := catalogSlug.FindAllString(q, -1); len(matches) > 0 {
		if len(matches) != 1 {
			return clarify("请一次指定一个商品编号，避免混用不同商品的规则。"), true
		}
		explicit = true
		selected = matches[0]
		if _, ok := snapshotProduct(selected); !ok {
			*state = BusinessState{CatalogPage: in.Product, Page: in.Product}
			return Reply{Text: "该商品不在本次同步的正式在售目录中，可能已下架或更换。历史订单请联系人工按原订单条款核实；不套用新商品或旧分类的退款规则。", Mode: "local", Handoff: true}, true
		}
	}
	if n, err := strconv.Atoi(q); err == nil && len(state.CatalogChoices) > 0 {
		if n < 1 || n > len(state.CatalogChoices) {
			return clarify("请回复本次列表中的有效序号，或完整商品名称。"), true
		}
		selected = state.CatalogChoices[n-1]
		explicit = true
		q = state.CatalogQuestion
	}
	families := catalogFamilies(q)
	if !explicit && len(families) == 0 && len(state.CatalogChoices) == 0 && contains(q, "max", "premium", "super", "ultra", "5x", "10x", "20x", "25x", "月付", "年付", "半价") && !contains(q, "升级", "叠加", "覆盖") {
		if previous, ok := snapshotProduct(state.CatalogSlug); ok {
			// Keep the selected product for questions about its current variant;
			// only search again when the requested variant excludes it.
			matchesCurrent := false
			for _, candidate := range catalogCandidates(q, []string{snapshotFamily(previous)}) {
				if candidate.Slug == previous.Slug {
					matchesCurrent = true
				}
			}
			if !matchesCurrent {
				families = []string{snapshotFamily(previous)}
			}
		}
	}
	if !explicit && len(families) > 0 {
		matches := catalogCandidates(q, families)
		if len(matches) == 0 {
			state.Stage = 0
			state.CatalogSlug = ""
			return Reply{Text: "本次同步的正式在售目录没有匹配这款商品。可发送“全部商品”查看目录；已下架商品的历史售后请由人工核实。", Mode: "local", Handoff: true}, true
		}
		if len(matches) > 1 {
			return catalogList(matches, state, q), true
		}
		selected = matches[0].Slug
		explicit = true
	} else if !explicit && len(state.CatalogChoices) > 0 {
		// Narrow a prior family selection (e.g. “一个月质保款”, “月付半价”).
		matches := []publicProduct{}
		for _, p := range catalogCandidates(q, nil) {
			for _, slug := range state.CatalogChoices {
				if p.Slug == slug {
					matches = append(matches, p)
				}
			}
		}
		if len(matches) == 1 {
			selected = matches[0].Slug
			explicit = true
			q = state.CatalogQuestion + " " + q
		}
		if len(matches) > 1 {
			return catalogList(matches, state, state.CatalogQuestion), true
		}
	}
	if selected == "" {
		selected = state.CatalogSlug
	}
	if selected == "" && in.Product != "" {
		return Reply{Text: "当前页面商品不在同步的正式在售目录中。请发送正式商品名称或“全部商品”；历史订单售后需人工核实。", Mode: "local", Handoff: true}, true
	}
	p, ok := snapshotProduct(selected)
	if !ok {
		// No generic GPT/Claude fallback: it could promise a policy that the
		// actual ready account or new Claude explicitly does not provide.
		return clarify("请先告诉我具体商品名称或编号，也可以发送“全部商品”查看目录。不同商品的购买条件与售后不同；历史订单请联系人工核实。"), true
	}
	if state.CatalogSlug != selected {
		*state = BusinessState{CatalogPage: in.Product, Page: in.Product, CatalogSlug: selected, Product: policy.Products[selected]}
	}
	state.CatalogChoices = nil
	state.CatalogQuestion = ""
	if state.Stage > 0 && !explicit {
		return collectCalculation(q, state), true
	}
	if explicit {
		state.Stage = 0
	}
	if contains(q, "试算", "試算", "计算退款", "算退款", "退多少", "补多少", "差价多少", "算一下") {
		if syncedCatalog.Policies[selected].RefundMode != "proportional" {
			return catalogReply(p, "退款质保"), true
		}
		state.Product = policy.Products[selected]
		state.Stage = 1
		state.Reopen = contains(q, "补", "差价", "重开")
		if reply, ok := beginNaturalCalculation(in.Message, state); ok {
			return reply, true
		}
		return clarify("仅按本款已确认规则试算，最终由人工核实。请提供这笔订单的实际付款金额和币种，例如 100.00 CNY。"), true
	}
	return catalogReply(p, q), true
}
