// Package htmltext 提供富文本 HTML 到纯文本的转换工具。
// 用于邮件正文、Telegram 消息等不能渲染 HTML 的场景，保留段落/换行结构并去除标签。
//
// 注意：telegram-bot 子模块（独立 go.mod）中存在同源实现，修改时请同步。
package htmltext

import "strings"

// StripToPlainText 将 HTML 富文本转为纯文本：保留段落/换行/列表结构，折叠连续空行。
// 对常见块级标签替换为换行，剥离所有标签，解码常见实体。
func StripToPlainText(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"<br>", "\n", "<br/>", "\n", "<br />", "\n",
		"</p>", "\n\n", "</div>", "\n", "</li>", "\n",
		"<li>", "• ",
	)
	s = replacer.Replace(s)

	var out strings.Builder
	out.Grow(len(s))
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			out.WriteRune(r)
		}
	}
	text := out.String()

	entityReplacer := strings.NewReplacer(
		"&nbsp;", " ", "&amp;", "&", "&lt;", "<", "&gt;", ">",
		"&quot;", "\"", "&#39;", "'",
	)
	text = entityReplacer.Replace(text)

	lines := strings.Split(text, "\n")
	cleaned := make([]string, 0, len(lines))
	prevBlank := false
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t")
		if trimmed == "" {
			if prevBlank {
				continue
			}
			prevBlank = true
		} else {
			prevBlank = false
		}
		cleaned = append(cleaned, trimmed)
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}
