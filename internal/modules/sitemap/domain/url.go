package domain

// URL 是 Sitemap 中的单个可索引地址。
type URL struct {
	Location        string
	LastModified    string
	ChangeFrequency string
	Priority        string
}
