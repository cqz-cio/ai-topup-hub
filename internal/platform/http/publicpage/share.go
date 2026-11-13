package publicpage

import (
	"net/url"
	"path"
	"strings"
)

// SVG remains available to the product UI and schema, but sharing cards use
// supported raster images. A site logo fallback uses a compact summary card.
func shareImage(origin string, images []string, fallback string) (string, string) {
	raster := func(raw string) string {
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
	for _, raw := range images {
		if image := raster(raw); image != "" {
			return image, "summary_large_image"
		}
	}
	return raster(fallback), "summary"
}
