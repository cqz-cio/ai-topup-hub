package publicpage

import (
	"encoding/json"
	"io"
	"io/fs"
	"regexp"
	"strings"

	htmlnode "golang.org/x/net/html"
)

var publicCSSPath = regexp.MustCompile(`^/assets/[A-Za-z0-9_/-]+-[A-Za-z0-9_-]+\.css$`)

// WithAssets binds the current static release directory (or embedded FS).
// No startup-only cache: a symlink switch must be visible without a backend restart.
func (h *Handler) WithAssets(assets fs.FS) *Handler { h.assets = assets; return h }

func boundedAsset(assets fs.FS, name string) []byte {
	if assets == nil {
		return nil
	}
	file, err := assets.Open(name)
	if err != nil {
		return nil
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 256*1024+1))
	if err != nil || len(data) > 256*1024 {
		return nil
	}
	return data
}

func (h *Handler) stylesheets() []string {
	valid := func(paths []string) []string {
		var result []string
		seen := map[string]bool{}
		for _, path := range paths {
			if !publicCSSPath.MatchString(path) || strings.Contains(path, "..") {
				return nil
			}
			info, err := fs.Stat(h.assets, strings.TrimPrefix(path, "/"))
			if err != nil || info.IsDir() {
				return nil
			}
			if !seen[path] {
				result = append(result, path)
				seen[path] = true
			}
		}
		return result
	}
	var manifest struct {
		Version int      `json:"version"`
		Styles  []string `json:"styles"`
	}
	if err := json.Unmarshal(boundedAsset(h.assets, "storefront-assets.json"), &manifest); err == nil && manifest.Version == 1 && len(manifest.Styles) <= 16 {
		if paths := valid(manifest.Styles); len(paths) > 0 {
			return paths
		}
	}
	// Older builds have no manifest. Read their actual index, never invent a hash.
	var paths []string
	reader := htmlnode.NewTokenizer(strings.NewReader(string(boundedAsset(h.assets, "index.html"))))
	for reader.Next() != htmlnode.ErrorToken {
		token := reader.Token()
		if token.Data != "link" {
			continue
		}
		attrs := map[string]string{}
		for _, attr := range token.Attr {
			attrs[attr.Key] = attr.Val
		}
		if attrs["rel"] == "stylesheet" {
			paths = append(paths, attrs["href"])
		}
	}
	if styles := valid(paths); len(styles) > 0 {
		return append(styles, "/storefront-entry.css")
	}
	// CSS remains readable without JS; the stable JS entry supplies matching app
	// styles in installations with no local static mount or during a release race.
	return []string{"/storefront-entry.css"}
}
