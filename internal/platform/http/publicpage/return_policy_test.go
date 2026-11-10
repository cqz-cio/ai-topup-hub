package publicpage

import (
	"encoding/json"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestProductReferencesTheVisibleMerchantPolicyAndCurrentOffer(t *testing.T) {
	for _, policyPath := range []string{"", "/blog/purchase-and-agent-guide"} {
		s := &sourceStub{returnPolicyPath: policyPath, page: Page{Kind: "product", Path: "/products/item", Title: "数字商品", Price: "145", Availability: "https://schema.org/OutOfStock", Content: `<p>原有商品规则</p><a href="/blog/purchase-and-agent-guide">交付与售后说明</a>`}}
		body := request(t, s, "GET", s.page.Path, "Googlebot").Body.String()
		doc, err := html.Parse(strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		var product, organization map[string]any
		var walk func(*html.Node)
		walk = func(n *html.Node) {
			if n.Type == html.ElementNode && n.Data == "script" {
				for _, a := range n.Attr {
					if a.Key == "type" && a.Val == "application/ld+json" && n.FirstChild != nil {
						var value map[string]any
						if err := json.Unmarshal([]byte(n.FirstChild.Data), &value); err != nil {
							t.Fatal(err)
						}
						if value["@type"] == "Product" {
							product = value
						}
						if graph, ok := value["@graph"].([]any); ok {
							organization = graph[0].(map[string]any)
						}
					}
				}
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
		walk(doc)
		offer := product["offers"].(map[string]any)
		if product["@id"] != "https://www.qzweb.cloud/products/item#product" || offer["price"] != "145" || offer["availability"] != "https://schema.org/OutOfStock" {
			t.Fatal(product)
		}
		if offer["seller"].(map[string]any)["@id"] != organization["@id"] {
			t.Fatal("seller identity mismatch")
		}
		if policyPath == "" {
			if offer["hasMerchantReturnPolicy"] != nil || organization["hasMerchantReturnPolicy"] != nil {
				t.Fatal("unconfigured store inherited policy")
			}
		} else {
			policy := organization["hasMerchantReturnPolicy"].(map[string]any)
			ref := offer["hasMerchantReturnPolicy"].(map[string]any)
			if len(policy) != 3 || len(ref) != 1 || ref["@id"] != policy["@id"] || policy["merchantReturnLink"] != "https://www.qzweb.cloud"+policyPath {
				t.Fatal(policy, ref)
			}
		}
		for _, field := range []string{"aggregateRating", "review", "shippingDetails", "merchantReturnDays", "gtin"} {
			if strings.Contains(body, `"`+field+`"`) {
				t.Fatal("invented field: " + field)
			}
		}
	}
}
