package publiccontent

import (
	_ "embed"
	"encoding/json"
)

// Existing lossless assets are reused; no published image bytes are changed.
//
//go:embed seo-assets.json
var seoAssetsJSON []byte

type SEOAssets struct {
	LegacyLogo string `json:"legacyLogo"`
	Logo       string `json:"logo"`
	Icon       string `json:"icon"`
}

func PublicSEOAssets() SEOAssets {
	var assets SEOAssets
	if err := json.Unmarshal(seoAssetsJSON, &assets); err != nil {
		panic(err)
	}
	return assets
}
