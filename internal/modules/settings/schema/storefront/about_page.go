package settingsstorefront

import (
	"net/url"
	"regexp"
	"strings"

	settingsvalue "github.com/dujiao-next/internal/modules/settings/schema/value"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

var aboutWechatPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{5,31}$`)

// Independent content settings keep About saves away from other site settings.
func NormalizeAboutPageJSON(value jsonmap.JSON) jsonmap.JSON {
	result := jsonmap.JSON{"version": 1}
	for _, field := range []string{
		"eyebrow", "title", "subtitle", "introductionTitle", "introduction", "servicesTitle",
		"rechargeTitle", "rechargeNote", "cooperationTitle", "cooperation", "cooperationNote",
		"contactTitle", "contactSubtitle", "contactNote", "contactTip", "wechatLabel",
		"copyLabel", "copiedLabel", "linksLabel", "telegramLabel", "whatsappLabel",
	} {
		result[field] = normalizeAboutLocalized(value[field])
	}
	wechat := settingsvalue.NormalizeTextWithRuneLimit(value["supportWechat"], 32)
	if !aboutWechatPattern.MatchString(wechat) {
		wechat = "Cqz0924ttxk"
	}
	result["supportWechat"] = wechat
	for _, field := range []string{"services", "recharge"} {
		items := make([]interface{}, 0)
		if raw, ok := value[field].([]interface{}); ok {
			for _, entry := range raw {
				item, ok := entry.(map[string]interface{})
				if !ok {
					continue
				}
				items = append(items, map[string]interface{}{
					"title": normalizeAboutLocalized(item["title"]),
					"text":  normalizeAboutLocalized(item["text"]),
				})
				if len(items) == 12 {
					break
				}
			}
		}
		result[field] = items
	}
	links := make([]interface{}, 0)
	if raw, ok := value["links"].([]interface{}); ok {
		for _, entry := range raw {
			item, ok := entry.(map[string]interface{})
			if !ok {
				continue
			}
			link := settingsvalue.NormalizeTextWithRuneLimit(item["to"], 2000)
			if !validAboutLink(link) {
				continue
			}
			links = append(links, map[string]interface{}{"title": normalizeAboutLocalized(item["title"]), "to": link})
			if len(links) == 12 {
				break
			}
		}
	}
	result["links"] = links
	return result
}

func normalizeAboutLocalized(raw interface{}) map[string]interface{} {
	result := normalizeAnnouncementLocalizedField(raw)
	for key, value := range result {
		result[key] = settingsvalue.NormalizeTextWithRuneLimit(value, 8000)
	}
	return result
}

func validAboutLink(value string) bool {
	if value == "" || strings.ContainsAny(value, "\\ \t\r\n") {
		return false
	}
	for _, character := range value {
		if character < 32 {
			return false
		}
	}
	if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") {
		return true
	}
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Hostname() != "" && parsed.User == nil
}
