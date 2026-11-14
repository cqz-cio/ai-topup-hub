package publicpage

import (
	"net/url"
	"path"
	"strings"

	"github.com/dujiao-next/internal/platform/publiccontent"
)

type ShareMetadata struct {
	Image, Card, Alt string
	Width, Height    int
}

var generatedShares = publiccontent.GeneratedShareImages()

// RasterShareURL rejects SVG, credentials and non-HTTP schemes for social cards.
func RasterShareURL(origin, raw string) string {
	image := absoluteImage(origin, raw)
	u, err := url.Parse(image)
	if err != nil || image == "" {
		return ""
	}
	switch strings.ToLower(path.Ext(u.Path)) {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif":
		return image
	}
	return ""
}

// Dimensions are declared only for a verified export or explicit metadata.
// No image processing or remote image fetch happens on the document path.
func shareImage(origin string, images []string, fallback, alt string) ShareMetadata {
	result := ShareMetadata{Card: "summary_large_image", Alt: alt}
	for _, raw := range append(append([]string{}, images...), fallback) {
		image := RasterShareURL(origin, raw)
		if image == "" {
			continue
		}
		u, _ := url.Parse(image)
		base, _ := url.Parse(origin)
		assets := publiccontent.PublicSEOAssets()
		if u.Scheme == base.Scheme && u.Host == base.Host {
			if exported := generatedShares[u.Path]; exported != "" {
				image = absoluteImage(origin, exported)
				result.Width, result.Height = 1200, 630
			} else if u.Path == assets.ShareImage {
				result.Width, result.Height = assets.ShareImageWidth, assets.ShareImageHeight
			}
		}
		result.Image = image
		return result
	}
	return result
}
