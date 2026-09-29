package contract

// SendOptions 是 Telegram 消息发送参数。
type SendOptions struct {
	ChatID                string
	Message               string
	ParseMode             string
	DisableWebPagePreview bool
	AttachmentURL         string
	AttachmentDisplayName string
}
