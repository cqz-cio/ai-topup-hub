package channelclientapp

// ClientDetail 渠道客户端详情（含明文 secret，供后台展示）。
type ClientDetail struct {
	ID            uint   `json:"id"`
	Name          string `json:"name"`
	ChannelType   string `json:"channel_type"`
	ChannelKey    string `json:"channel_key"`
	ChannelSecret string `json:"channel_secret"`
	BotToken      string `json:"bot_token"`
	BotTokenSet   bool   `json:"bot_token_set"`
	CallbackURL   string `json:"callback_url"`
	Description   string `json:"description"`
	Status        int    `json:"status"`
}

// ActiveEndpoint contains the decrypted callback credential needed by an
// integration worker. It deliberately excludes persisted ciphertext.
type ActiveEndpoint struct {
	ClientID      uint
	ChannelKey    string
	CallbackURL   string
	ChannelSecret string
}
