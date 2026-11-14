package publiccontent

import (
	"strings"
	"testing"
)

func TestDeliveryPolicyPreservesRulesSectionsAndExistingLinks(t *testing.T) {
	p := DeliveryAndReturns()
	input := "<h2>原有条件</h2><p>原有售后约定</p>"
	result := p.AppendLink(input)
	if !strings.HasPrefix(result, input) || strings.Count(result, "<h2>") != 1 || !strings.Contains(result, `href="/blog/purchase-and-agent-guide"`) {
		t.Fatal(result)
	}
	if p.AppendLink(result) != result {
		t.Fatal("duplicate policy entry")
	}
	gpt := GPTRecharge().AppendRelated(input, "product", "qzweb-5", "gpt-recharge")
	if p.AppendLink(gpt) != gpt {
		t.Fatal("GPT guide already links to the policy")
	}
}
