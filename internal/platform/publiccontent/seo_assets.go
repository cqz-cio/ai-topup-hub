package publiccontent

import (
	_ "embed"
	"encoding/json"
	"net/url"
)

// Logo and social images are separate; published assets are never overwritten.
//
//go:embed seo-assets.json
var seoAssetsJSON []byte

type SEOAssets struct {
	LegacyLogo       string `json:"legacyLogo"`
	Logo             string `json:"logo"`
	Icon             string `json:"icon"`
	ShareImage       string `json:"shareImage"`
	ShareImageWidth  int    `json:"shareImageWidth"`
	ShareImageHeight int    `json:"shareImageHeight"`
	ShareImageAlt    string `json:"shareImageAlt"`
}

//go:embed seo-share-images.json
var shareImagesJSON []byte

//go:embed image-dimensions.json
var imageDimensionsJSON []byte

type ImageSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

var publicImageSizes = func() map[string]ImageSize {
	var values map[string]ImageSize
	if err := json.Unmarshal(imageDimensionsJSON, &values); err != nil {
		panic(err)
	}
	return values
}()

func PublicImageSize(raw string) ImageSize {
	u, err := url.Parse(raw)
	if err != nil {
		return ImageSize{}
	}
	if u.Host != "" && u.Hostname() != "www.qzweb.cloud" && u.Hostname() != "qzweb.cloud" {
		return ImageSize{}
	}
	return publicImageSizes[u.Path]
}

// GeneratedShareImages maps existing public raster covers to offline exports.
func GeneratedShareImages() map[string]string {
	var images map[string]string
	if err := json.Unmarshal(shareImagesJSON, &images); err != nil {
		panic(err)
	}
	return images
}

func PublicSEOAssets() SEOAssets {
	var assets SEOAssets
	if err := json.Unmarshal(seoAssetsJSON, &assets); err != nil {
		panic(err)
	}
	return assets
}
