package settingsstorefront

import (
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	settingsvalue "github.com/dujiao-next/internal/modules/settings/schema/value"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

// homeAnnouncementTypes 首页公告允许的类型集合。
var homeAnnouncementTypes = map[string]struct{}{
	"normal":  {},
	"info":    {},
	"warning": {},
}

// NormalizeHomeAnnouncementJSON 是 Registry 使用的原始 JSON 写入策略。
func NormalizeHomeAnnouncementJSON(value jsonmap.JSON) jsonmap.JSON {
	return normalizeHomeAnnouncement(value)
}

// normalizeHomeAnnouncement 归一化首页公告设置，避免非法值入库。
func normalizeHomeAnnouncement(value map[string]interface{}) jsonmap.JSON {
	annType := normalizeAnnouncementText(value["type"])
	if _, ok := homeAnnouncementTypes[annType]; !ok {
		annType = "normal"
	}
	return jsonmap.JSON{
		"enabled":  settingsvalue.ParseBool(value["enabled"]),
		"type":     annType,
		"title":    normalizeAnnouncementLocalizedField(value["title"]),
		"content":  normalizeAnnouncementLocalizedField(value["content"]),
		"start_at": normalizeHomeAnnouncementTime(value["start_at"]),
		"end_at":   normalizeHomeAnnouncementTime(value["end_at"]),
	}
}

func normalizeAnnouncementText(raw interface{}) string {
	text, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func normalizeAnnouncementLocalizedField(raw interface{}) map[string]interface{} {
	fieldResult := make(map[string]interface{}, len(constants.SupportedLocales))
	for _, language := range constants.SupportedLocales {
		fieldResult[language] = ""
	}

	fieldRaw, ok := raw.(map[string]interface{})
	if !ok {
		return fieldResult
	}

	for _, language := range constants.SupportedLocales {
		fieldResult[language] = normalizeAnnouncementText(fieldRaw[language])
	}

	return fieldResult
}

// normalizeHomeAnnouncementTime 校验 RFC3339 时间字符串，非法或为空时返回空串。
func normalizeHomeAnnouncementTime(raw interface{}) string {
	text := normalizeAnnouncementText(raw)
	if text == "" {
		return ""
	}
	if _, err := time.Parse(time.RFC3339, text); err != nil {
		return ""
	}
	return text
}

// homeAnnouncementVersion 基于类型与多语言标题、内容计算 8 位十六进制指纹。
// 内容任意变化都会改变指纹；仅修改排期时间不影响指纹。
func homeAnnouncementVersion(annType string, title, content map[string]interface{}) string {
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(annType))
	for _, lang := range constants.SupportedLocales {
		_, _ = hasher.Write([]byte{0})
		if title != nil {
			if text, ok := title[lang].(string); ok {
				_, _ = hasher.Write([]byte(text))
			}
		}
		_, _ = hasher.Write([]byte{0})
		if content != nil {
			if text, ok := content[lang].(string); ok {
				_, _ = hasher.Write([]byte(text))
			}
		}
	}
	return fmt.Sprintf("%08x", hasher.Sum32())
}

// isHomeAnnouncementInSchedule 判断当前时间是否处于排期内。空时间表示该端不限制。
func isHomeAnnouncementInSchedule(startAt, endAt string, now time.Time) bool {
	if startAt != "" {
		if start, err := time.Parse(time.RFC3339, startAt); err == nil && now.Before(start) {
			return false
		}
	}
	if endAt != "" {
		if end, err := time.Parse(time.RFC3339, endAt); err == nil && now.After(end) {
			return false
		}
	}
	return true
}

// hasHomeAnnouncementContent 判断公告内容是否至少有一个语言非空。
func hasHomeAnnouncementContent(content map[string]interface{}) bool {
	for _, lang := range constants.SupportedLocales {
		if text, ok := content[lang].(string); ok && strings.TrimSpace(text) != "" {
			return true
		}
	}
	return false
}

// ActiveHomeAnnouncement 返回给定设置在指定时间是否应展示。
func ActiveHomeAnnouncement(value jsonmap.JSON, now time.Time) (jsonmap.JSON, bool) {
	announcement := normalizeHomeAnnouncement(value)
	if !settingsvalue.ParseBool(announcement["enabled"]) {
		return nil, false
	}
	startAt, _ := announcement["start_at"].(string)
	endAt, _ := announcement["end_at"].(string)
	if !isHomeAnnouncementInSchedule(startAt, endAt, now) {
		return nil, false
	}
	content, _ := announcement["content"].(map[string]interface{})
	if !hasHomeAnnouncementContent(content) {
		return nil, false
	}
	title, _ := announcement["title"].(map[string]interface{})
	annType, _ := announcement["type"].(string)
	return jsonmap.JSON{
		"type":    annType,
		"title":   title,
		"content": content,
		"version": homeAnnouncementVersion(annType, title, content),
	}, true
}
