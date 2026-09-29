package contract

import "time"

// ListFilter 定义管理端支付记录查询条件。
type ListFilter struct {
	Page         int
	PageSize     int
	UserID       uint
	OrderID      uint
	ChannelID    uint
	ProviderType string
	ChannelType  string
	Status       string
	CreatedFrom  *time.Time
	CreatedTo    *time.Time
	SkipCount    bool
	Lightweight  bool
}

// ChannelListFilter 定义支付渠道查询条件。
type ChannelListFilter struct {
	Page         int
	PageSize     int
	ProviderType string
	ChannelType  string
	ActiveOnly   bool
}
