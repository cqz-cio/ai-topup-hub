package publiccontent

import (
	_ "embed"
	"encoding/json"
	"slices"
)

//go:embed grok-products.json
var grokProductsJSON []byte

//go:embed product-clusters.json
var productClustersJSON []byte

type ProductCluster struct {
	Key   string   `json:"key"`
	Title string   `json:"title"`
	Slugs []string `json:"slugs"`
	Extra []string `json:"extra"`
}

func (c ProductCluster) Contains(slug string, aliases []string) bool {
	for _, key := range append(append([]string{}, c.Slugs...), c.Extra...) {
		if key == slug || slices.Contains(aliases, key) {
			return true
		}
	}
	return false
}

func RelatedProductCluster(slug string, aliases []string) ProductCluster {
	if IsGrokProduct(slug, aliases) {
		var keys []string
		_ = json.Unmarshal(grokProductsJSON, &keys)
		return ProductCluster{Key: "grok", Title: "其他 Grok 档位", Slugs: keys}
	}
	var clusters []ProductCluster
	if err := json.Unmarshal(productClustersJSON, &clusters); err != nil {
		panic(err)
	}
	for _, cluster := range clusters {
		if cluster.Contains(slug, aliases) {
			return cluster
		}
	}
	return ProductCluster{}
}

// IsGrokProduct recognizes the original catalog identities, including an alias
// after a later rename. Callers always build links from the current public slug.
func IsGrokProduct(slug string, aliases []string) bool {
	var keys []string
	if err := json.Unmarshal(grokProductsJSON, &keys); err != nil {
		panic(err)
	}
	for _, key := range keys {
		if key == slug || slices.Contains(aliases, key) {
			return true
		}
	}
	return false
}
