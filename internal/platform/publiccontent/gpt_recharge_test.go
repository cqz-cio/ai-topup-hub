package publiccontent

import (
	"strings"
	"testing"
)

func TestRelatedLinksPreserveContentAndAvoidDuplicates(t *testing.T) {
	g := GPTRecharge()
	input := `<p>保留购买条件</p><a href="/blog/chatgpt-payment-methods">已有教程</a>`
	result := g.AppendRelated(input, "product", "qzweb-5", "gpt-recharge")
	if !strings.HasPrefix(result, input) || !strings.Contains(result, g.Path) || strings.Count(result, `href="/blog/chatgpt-payment-methods"`) != 1 {
		t.Fatal(result)
	}
	if g.AppendRelated(result, "product", "qzweb-5", "gpt-recharge") != result {
		t.Fatal("not idempotent")
	}
	article := g.AppendRelated("", "article", "chatgpt-payment-methods", "")
	if !strings.Contains(article, g.Path) || strings.Contains(article, `href="/blog/chatgpt-payment-methods"`) {
		t.Fatal(article)
	}
	if g.AppendRelated(input, "product", "account", "gpt-ready-accounts") != input {
		t.Fatal("account is not topup")
	}
	if g.AppendRelated(input, "notice", "chatgpt-payment-methods", "") != input {
		t.Fatal("notice changed")
	}
}
