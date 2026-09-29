package domain

// RenderSlot 广告位信息。
type RenderSlot struct {
	Code       string `json:"code"`
	Scene      string `json:"scene"`
	Layout     string `json:"layout"`
	RenderMode string `json:"render_mode"`
	MaxItems   int    `json:"max_items"`
}

// RenderItem 广告渲染项。
type RenderItem struct {
	ID              int64  `json:"id"`
	AdvertiserName  string `json:"advertiser_name"`
	Title           string `json:"title"`
	Subtitle        string `json:"subtitle"`
	CTALabel        string `json:"cta_label"`
	Badge           string `json:"badge"`
	Image           string `json:"image"`
	MobileImage     string `json:"mobile_image"`
	Icon            string `json:"icon"`
	LinkType        string `json:"link_type"`
	OpenInNewTab    bool   `json:"open_in_new_tab"`
	Theme           string `json:"theme"`
	Dismissible     bool   `json:"dismissible"`
	ClickURL        string `json:"click_url"`
	ImpressionToken string `json:"impression_token"`
}

// RenderResponse 广告渲染响应。
type RenderResponse struct {
	Slot  RenderSlot   `json:"slot"`
	Items []RenderItem `json:"items"`
}
