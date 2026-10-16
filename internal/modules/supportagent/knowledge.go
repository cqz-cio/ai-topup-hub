package supportagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type Link struct {
	Label string `json:"label"`
	To    string `json:"to"`
}
type Source struct {
	ID    string `json:"id"`
	Text  string `json:"text"`
	Links []Link `json:"-"`
}
type Knowledge interface {
	Sources(context.Context, string, string, string) ([]Source, error)
}
type publicProduct struct {
	Slug        string            `json:"slug"`
	Title       map[string]string `json:"title"`
	Description map[string]string `json:"description"`
	Content     map[string]string `json:"content"`
	Price       string            `json:"price_amount"`
	SoldOut     bool              `json:"is_sold_out"`
}
type Catalog struct {
	mu       sync.Mutex
	products []publicProduct
	expires  time.Time
	client   *http.Client
	base     string
}

func NewCatalog() *Catalog {
	return &Catalog{base: "http://127.0.0.1:8080", client: &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

var slugPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,119}$`)
var scriptPattern = regexp.MustCompile(`(?is)<(script|style)\b[^>]*>.*?</(?:script|style)>`)
var tagPattern = regexp.MustCompile(`<[^>]+>`)
var linkPattern = regexp.MustCompile(`(?i)(?:https?://|www\.)[^\s<>"'，。；、）)]+|\b[a-z0-9][a-z0-9.-]*\.(?:com|cn|net|org|bar|cloud|io|ai|top|xyz|shop|me)(?:/[^\s<>"']*)?`)
var secretPattern = regexp.MustCompile(`(?i)\bsk-[a-z0-9_-]{12,}|\beyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+|\b[A-Za-z0-9_-]{28,}\b`)
var emailPattern = regexp.MustCompile(`(?i)[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}`)
var pricePattern = regexp.MustCompile(`^\d{1,6}\.\d{2}$`)

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…（资料节选）"
	}
	return s
}
func plain(s string, n int) string {
	s = html.UnescapeString(s)
	s = scriptPattern.ReplaceAllString(s, " ")
	s = tagPattern.ReplaceAllString(s, " ")
	s = linkPattern.ReplaceAllString(s, "[具体入口请查看订单交付说明]")
	s = secretPattern.ReplaceAllString(s, "[已隐藏]")
	s = emailPattern.ReplaceAllString(s, "[已隐藏邮箱]")
	return truncate(strings.Join(strings.Fields(s), " "), n)
}
func language(m map[string]string, locale string) string {
	if m[locale] != "" {
		return m[locale]
	}
	return m["zh-CN"]
}
func (c *Catalog) load(ctx context.Context) ([]publicProduct, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Now().Before(c.expires) {
		return c.products, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	all := []publicProduct{}
	for page := 1; page <= 5; page++ {
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/v1/public/products?page=%d&page_size=100", c.base, page), nil)
		// Never forward store credentials, cookies or client-selected URLs.
		response, err := c.client.Do(request)
		if err != nil {
			return nil, errors.New("public catalog unavailable")
		}
		b, readErr := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024+1))
		response.Body.Close()
		if readErr != nil || len(b) > 2*1024*1024 || response.StatusCode != 200 {
			return nil, errors.New("public catalog unavailable")
		}
		var data struct {
			Code int             `json:"status_code"`
			Data []publicProduct `json:"data"`
		}
		if json.Unmarshal(b, &data) != nil || data.Code != 0 {
			return nil, errors.New("public catalog unavailable")
		}
		all = append(all, data.Data...)
		if len(data.Data) < 100 {
			c.products = all
			c.expires = time.Now().Add(time.Minute)
			return all, nil
		}
	}
	return nil, errors.New("public catalog too large")
}
func baseSources(locale string) []Source {
	return []Source{
		{"guide:purchase", "本站提供 GPT 订阅类、Codex 点数类及 Gemini 等商品；必须区分商品和用途，不把 Codex 点数当 Plus/Pro 订阅。先了解用户用途、已有订阅及商品条件再推荐；不强制用户回复序号。购买前核对当前商品详情、账号条件、质保与库存，结账显示最终售价。不同渠道和套餐不能互相套用条件。", []Link{{pick(locale, "查看商品", "Browse products", "查看商品"), "/products"}}},
		{"guide:redeem", "购买并付款后，在该笔订单交付说明领取卡密或链接；从该订单的兑换入口操作，按页面要求完成资格预检和提交，再在兑换页确认结果。售前和本客服都不提供具体兑换网址。订单付款、商城发货、兑换成功是不同阶段，不能互相推断。iOS 订阅充值可能覆盖当前套餐且不叠加剩余时长，必须核对该商品明确条件。Gemini、DEMO 不套用 GPT 的兑换地址；Codex 点数不延长 Plus/Pro 订阅。订单已付款但没有交付、兑换失败或结果不变时先询问用户看到的提示（隐藏敏感信息），无法确认就联系人工。不要要求用户在聊天中提供账号密码、登录 token、完整卡密或支付信息。", []Link{{pick(locale, "我的订单", "My orders", "我的訂單"), "/me/orders"}, {pick(locale, "游客查单", "Guest orders", "訪客查單"), "/guest/orders"}}},
		{"guide:wallet", "商城钱包余额用于本站消费，与 ChatGPT/Codex/Gemini 账号内的权益或点数不同。商城钱包在个人中心的我的钱包进行充值，是否到账以钱包流水为准。本客服无法读取钱包余额或流水。", []Link{{pick(locale, "我的钱包", "My wallet", "我的錢包"), "/me/wallet"}}},
		{"guide:payment", "付款页列出该订单当前可用渠道、实付金额及手续费。已有微信、USDT 支付的操作指引，但不能承诺每笔订单都有所有渠道。USDT 必须核对网络、地址和精确到账金额，提现费用另付；付款页有币安/欧易教程。以付款页为准，不在聊天生成收款地址、二维码或支付链接，不诱导重复付款。无法核对是否到账；异常需人工核查。", nil},
		{"guide:partners", "推广返利在个人中心查看当前规则与记录；没有提供返利比例、结算周期、可提现金额等具体数据，不允许猜测或承诺。开分店可在个人中心提交分销申请，仍需管理员审核；开通费用、域名和合作细则询问人工。", nil},
		{"guide:human", "人工客服微信 Cqz0924ttxk、QQ 1669268759、Telegram @czq_piestar。小窗的联系人工按钮可查看。退款、重发、异常订单、资格不确定等交由人工；不承诺一定退款、永久质保或已经完成操作。", nil},
	}
}
func (c *Catalog) Sources(ctx context.Context, locale, query, pageSlug string) ([]Source, error) {
	sources := baseSources(locale)
	products, err := c.load(ctx)
	if err != nil {
		return sources, err
	}
	type scored struct {
		p     publicProduct
		title string
		score int
	}
	scores := []scored{}
	q := strings.ToLower(query)
	seen := map[string]bool{}
	for _, p := range products {
		title := plain(language(p.Title, locale), 120)
		if !slugPattern.MatchString(p.Slug) || title == "" || seen[p.Slug] {
			continue
		}
		seen[p.Slug] = true
		if p.Slug == pageSlug {
			sources = append(sources, Source{"context:page", "用户当前浏览的商品详情页是：" + title + "。当用户说‘这个商品’或‘这个怎么用’且没有明确切换商品时，指的就是这件商品。浏览不代表购买或符合资格。", []Link{{title, "/products/" + p.Slug}}})
		}
		score := 0
		if p.Slug == pageSlug {
			score += 100
		}
		if strings.Contains(q, strings.ToLower(title)) || strings.Contains(q, p.Slug) {
			score += 50
		}
		for _, term := range []string{"gpt", "plus", "pro", "codex", "gemini", "ios", "5x", "20x", "续费", "續費", "开通", "開通"} {
			if strings.Contains(q, term) && strings.Contains(strings.ToLower(title), term) {
				score += 2
			}
		}
		scores = append(scores, scored{p, title, score})
	}
	sort.SliceStable(scores, func(i, j int) bool { return scores[i].score > scores[j].score })
	for i, item := range scores {
		if i >= 40 {
			break
		}
		detail := plain(language(item.p.Description, locale), 200)
		if i < 3 && item.score > 0 {
			detail = plain(language(item.p.Content, locale), 1000)
		}
		text := item.title + "。" + detail
		if pricePattern.MatchString(item.p.Price) {
			text += "；公开参考价 " + item.p.Price + " 元（结账为准）。"
		}
		if item.p.SoldOut {
			text += "当前公开目录显示缺货，勿引导立即购买。"
		}
		sources = append(sources, Source{"product:" + item.p.Slug, text, []Link{{item.title, "/products/" + item.p.Slug}}})
	}
	return sources, nil
}

func pick(locale, zh, en, tw string) string {
	if locale == "en-US" {
		return en
	}
	if locale == "zh-TW" {
		return tw
	}
	return zh
}
