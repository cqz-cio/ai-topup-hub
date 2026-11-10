package publiccontent

import (
	_ "embed"
	"encoding/json"
	"html"
	"net/url"
	"regexp"
	"strings"
)

// These copy files are also consumed by the storefront. Public documents do
// not invent a company, legal address, author biography or official affiliation.
//
//go:embed about-defaults.json
var aboutDefaultsJSON []byte

//go:embed home-copy.json
var homeCopyJSON []byte

//go:embed site-identity.json
var identityJSON []byte

func IdentityStatement() string {
	var value struct {
		Statement string `json:"statement"`
	}
	if err := json.Unmarshal(identityJSON, &value); err != nil {
		panic(err)
	}
	return value.Statement
}

func object(value any) map[string]any {
	result, _ := value.(map[string]any)
	return result
}
func text(value any) string { s, _ := value.(string); return strings.TrimSpace(s) }
func language(value any) string {
	for _, key := range []string{"zh-CN", "zh-TW", "en-US"} {
		if s := text(object(value)[key]); s != "" {
			return s
		}
	}
	return ""
}
func paragraph(s string) string {
	if s == "" {
		return ""
	}
	return "<p>" + strings.ReplaceAll(html.EscapeString(s), "\n", "<br>") + "</p>"
}
func heading(s string) string {
	if s == "" {
		return ""
	}
	return "<h2>" + html.EscapeString(s) + "</h2>"
}

func HomeCopy() (title, summary, body string) {
	var copy map[string]any
	if err := json.Unmarshal(homeCopyJSON, &copy); err != nil {
		panic(err)
	}
	title = text(copy["heroTitle"]) + text(copy["heroAccent"])
	summary = text(copy["heroDescription"])
	body = paragraph(text(copy["heroEyebrow"])) + heading(text(copy["flowTitle"])) + "<ol>"
	for _, key := range []string{"cart", "checkout", "payment", "delivery"} {
		body += "<li>" + html.EscapeString(text(object(copy["steps"])[key])) + "：" + html.EscapeString(text(object(copy["stepHints"])[key])) + "</li>"
	}
	body += "</ol>" + paragraph(text(copy["flowDescription"])) + paragraph(text(copy["deliveryNotice"])) + paragraph(text(copy["catalogHint"]))
	body += heading(text(copy["helpTitle"])) + paragraph(text(copy["helpDescription"]))
	body += heading(text(copy["lookupTitle"])) + paragraph(text(copy["lookupHint"]))
	body += `<p><a href="/me/orders">我的订单</a> · <a href="/guest/orders">游客查单</a></p>`
	return
}

// AboutCopy mirrors the first-visit Chinese resolution of frontend/shared/aboutPage.ts.
// Saved blank fields stay blank; a title/body pair always uses the same language.
func AboutCopy(raw, legacy, contact map[string]any, main bool) (title, summary, body string) {
	if !main {
		title, summary = language(object(legacy["hero"])["title"]), language(object(legacy["hero"])["subtitle"])
		body = paragraph(language(legacy["introduction"])) + heading(language(object(legacy["services"])["title"]))
		if items, ok := object(legacy["services"])["items"].([]any); ok {
			for _, item := range items {
				body += paragraph(language(item))
			}
		}
		body += heading(language(object(legacy["contact"])["title"])) + paragraph(language(object(legacy["contact"])["text"]))
		return title, summary, body + ContactHTML(contact, false)
	}
	var defaults map[string]any
	if err := json.Unmarshal(aboutDefaultsJSON, &defaults); err != nil {
		panic(err)
	}
	saved := raw["version"] == float64(1) || raw["version"] == 1
	field := func(key string) string {
		if saved {
			return language(raw[key])
		}
		if s := language(raw[key]); s != "" {
			return s
		}
		var old any
		switch key {
		case "title", "subtitle":
			old = object(legacy["hero"])[key]
		case "introduction":
			old = legacy[key]
		case "servicesTitle":
			old = object(legacy["services"])["title"]
		case "contactTitle":
			old = object(legacy["contact"])["title"]
		case "contactSubtitle":
			old = object(legacy["contact"])["text"]
		}
		if s := language(old); s != "" {
			return s
		}
		return text(defaults[key])
	}
	title, summary = field("title"), field("subtitle")
	body = paragraph(field("eyebrow")) + heading(field("introductionTitle")) + paragraph(field("introduction")) + paragraph(IdentityStatement())
	for _, key := range []string{"services", "recharge"} {
		body += heading(field(key + "Title"))
		items, _ := defaults[key].([]any)
		if saved {
			items, _ = raw[key].([]any)
		}
		if !saved && key == "services" {
			if old, ok := object(legacy["services"])["items"].([]any); ok && len(old) > 0 {
				var resolved []any
				for _, item := range old {
					if s := language(item); s != "" {
						parts := strings.SplitN(s, "\n", 2)
						entry := map[string]any{"title": "", "text": parts[0]}
						if len(parts) == 2 {
							entry["title"], entry["text"] = parts[0], parts[1]
						}
						resolved = append(resolved, entry)
					}
				}
				if len(resolved) > 0 {
					items = resolved
				}
			}
		}
		for i, item := range items {
			if i >= 12 {
				break
			}
			entry := object(item)
			t, p := text(entry["title"]), text(entry["text"])
			if saved {
				for _, lang := range []string{"zh-CN", "zh-TW", "en-US"} {
					t, p = text(object(entry["title"])[lang]), text(object(entry["text"])[lang])
					if t != "" || p != "" {
						break
					}
				}
			}
			if t != "" {
				body += "<h3>" + html.EscapeString(t) + "</h3>"
			}
			body += paragraph(p)
		}
	}
	body += paragraph(field("rechargeNote")) + heading(field("cooperationTitle")) + paragraph(field("cooperation")) + paragraph(field("cooperationNote"))
	body += heading(field("contactTitle")) + paragraph(field("contactSubtitle"))
	wechat := text(raw["supportWechat"])
	if !saved {
		wechat = text(contact["wechat"])
	}
	if !regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{5,31}$`).MatchString(wechat) {
		wechat = "Cqz0924ttxk"
	}
	label := field("wechatLabel")
	if !saved && label == "" {
		label = "客服微信"
	}
	body += paragraph(label+"："+wechat) + paragraph(field("contactNote")) + paragraph(field("contactTip")) + ContactHTML(contact, true)
	links, _ := defaults["links"].([]any)
	if saved {
		links, _ = raw["links"].([]any)
	}
	for i, entry := range links {
		if i >= 12 {
			break
		}
		link := object(entry)
		label := text(link["title"])
		if saved {
			label = language(link["title"])
		}
		body += safeLink(label, text(link["to"]))
	}
	return
}

func safeLink(label, path string) string {
	if label == "" || strings.ContainsAny(path, "\\\r\n\t ") {
		return ""
	}
	u, err := url.Parse(path)
	if err != nil || u.User != nil || !(strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "//") || (u.Scheme == "https" || u.Scheme == "http") && u.Hostname() != "") {
		return ""
	}
	return `<p><a href="` + html.EscapeString(path) + `">` + html.EscapeString(label) + `</a></p>`
}

func ContactHTML(contact map[string]any, main bool) string {
	qq, telegram := text(contact["qq"]), text(contact["telegram"])
	if main {
		if qq == "" {
			qq = "1669268759"
		}
		if telegram == "" {
			telegram = "https://t.me/czq_piestar"
		}
	}
	body := ""
	if regexp.MustCompile(`^[1-9][0-9]{4,14}$`).MatchString(qq) {
		body += paragraph("QQ：" + qq)
	}
	if regexp.MustCompile(`^@?[a-zA-Z0-9_]{5,32}$`).MatchString(telegram) {
		telegram = "https://t.me/" + strings.TrimPrefix(telegram, "@")
	}
	if u, err := url.Parse(telegram); err == nil && u.Scheme == "https" && (u.Hostname() == "t.me" || u.Hostname() == "telegram.me") && regexp.MustCompile(`^/[a-zA-Z0-9_]{5,32}/?$`).MatchString(u.Path) {
		body += safeLink("Telegram：@"+strings.Trim(u.Path, "/"), telegram)
	}
	body += safeLink("WhatsApp", text(contact["whatsapp"]))
	return body
}
