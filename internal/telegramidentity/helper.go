package telegramidentity

import (
	"fmt"
	"strings"
)

const (
	placeholderEmailPrefix = "telegram_"
	placeholderEmailDomain = "@login.local"
	defaultDisplayName     = "Telegram User"
)

// BuildPlaceholderEmail 构造 Telegram 虚拟邮箱占位符。
func BuildPlaceholderEmail(providerUserID string) string {
	normalizedID := strings.TrimSpace(providerUserID)
	if normalizedID == "" {
		normalizedID = "unknown"
	}
	return fmt.Sprintf("%s%s%s", placeholderEmailPrefix, normalizedID, placeholderEmailDomain)
}

// IsPlaceholderEmail 判断是否为 Telegram 虚拟邮箱占位符。
func IsPlaceholderEmail(email string) bool {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return false
	}
	return strings.HasPrefix(normalized, placeholderEmailPrefix) &&
		strings.HasSuffix(normalized, placeholderEmailDomain)
}

// ResolveDisplayName 解析 Telegram 身份对应的展示名称。
func ResolveDisplayName(providerUserID, username, firstName, lastName string) string {
	fullName := strings.TrimSpace(strings.TrimSpace(firstName) + " " + strings.TrimSpace(lastName))
	if fullName != "" {
		return fullName
	}
	if strings.TrimSpace(username) != "" {
		return strings.TrimSpace(username)
	}
	if strings.TrimSpace(providerUserID) != "" {
		return fmt.Sprintf("telegram_%s", strings.TrimSpace(providerUserID))
	}
	return defaultDisplayName
}
