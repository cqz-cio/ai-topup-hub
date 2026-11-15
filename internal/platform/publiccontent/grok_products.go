package publiccontent

import (
	_ "embed"
	"encoding/json"
	"slices"
)

//go:embed grok-products.json
var grokProductsJSON []byte

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
