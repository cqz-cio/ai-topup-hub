package publiccontent

import (
	_ "embed"
	"encoding/json"
	"html"
	"strings"
)

//go:embed delivery-policy.json
var deliveryPolicyJSON []byte

type DeliveryPolicy struct{ Path, Title, Prefix, Suffix string }

func DeliveryAndReturns() DeliveryPolicy {
	var policy DeliveryPolicy
	if err := json.Unmarshal(deliveryPolicyJSON, &policy); err != nil {
		panic(err)
	}
	return policy
}

// AppendLink adds a visible policy entry without introducing a new product
// section. Callers exclude reseller sites, which have their own policies.
func (p DeliveryPolicy) AppendLink(content string) string {
	if strings.Contains(content, p.Path) {
		return content
	}
	return content + "<p>" + html.EscapeString(p.Prefix) + `<a href="` + html.EscapeString(p.Path) + `">` + html.EscapeString(p.Title) + "</a>" + html.EscapeString(p.Suffix) + "</p>"
}
