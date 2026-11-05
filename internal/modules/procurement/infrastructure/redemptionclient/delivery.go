package redemptionclient

import (
	"errors"
	"html"
	"net/url"
	"regexp"
	"strings"

	"github.com/dujiao-next/internal/shared/jsonmap"
)

var cardPattern = regexp.MustCompile(`^PAIPLUS-[A-HJ-NP-Z2-9]{10}-[A-HJ-NP-Z2-9]{10}$`)
var seedPattern = regexp.MustCompile(`^[A-Z2-7]{1,128}$`)
var sourceCodePattern = regexp.MustCompile(`(?i)ELL[A-Z0-9]*-[A-Z0-9_-]+`)

type deliveryResponse struct {
	Format string `json:"format"`
	Count  int    `json:"count"`
	Cards  []struct {
		Code    string `json:"code"`
		Account *struct {
			Email        string `json:"email"`
			Password     string `json:"password"`
			TwoFactor    string `json:"two_factor"`
			TwoFactorURL string `json:"two_factor_url"`
		} `json:"account"`
	} `json:"cards"`
}

// Build customer content ourselves from the versioned, validated response. In
// particular, never forward the issuer's free-form payload or supplier prose.
func (d deliveryResponse) customerPayload(quantity int, origin string) (string, error) {
	invalid := errors.New("incomplete or unsafe redemption delivery response")
	if d.Format != "paiplus-v1" || d.Count != quantity || len(d.Cards) != quantity {
		return "", invalid
	}
	seen := map[string]bool{}
	blocks := make([]string, 0, quantity)
	for _, card := range d.Cards {
		if !cardPattern.MatchString(card.Code) || seen[card.Code] {
			return "", invalid
		}
		seen[card.Code] = true
		text := "卡密：" + card.Code
		if a := card.Account; a != nil {
			for _, value := range []string{a.Email, a.Password, a.TwoFactor, a.TwoFactorURL} {
				if value == "" || len(value) > 512 || strings.ContainsAny(value, "\r\n\t ") || sourceCodePattern.MatchString(value) {
					return "", invalid
				}
			}
			u, err := url.Parse(a.TwoFactorURL)
			if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" ||
				strings.EqualFold(u.Hostname(), "cdk.bar") || strings.HasSuffix(strings.ToLower(u.Hostname()), ".cdk.bar") ||
				!strings.Contains(a.Email, "@") || !seedPattern.MatchString(strings.ToUpper(a.TwoFactor)) {
				return "", invalid
			}
			text = "邮箱号：" + a.Email + "\n邮箱密码：" + a.Password + "\n密钥2fa：" + a.TwoFactor + "\n密钥2fa在线获取：" + a.TwoFactorURL + "\n兑换卡密：" + card.Code
		}
		blocks = append(blocks, text)
	}
	return strings.Join(blocks, "\n\n") + "\n兑换地址：" + origin + "/", nil
}

func customerInstructions(origin string) jsonmap.JSON {
	link := html.EscapeString(origin + "/")
	return jsonmap.JSON{
		"zh-CN": `<p>请使用订单中的 PAIPLUS 卡密，在本站兑换中心核验账号后确认充值。可使用同一卡密查询进度，请勿重复提交。</p><p>兑换地址：<a href="` + link + `">` + link + `</a></p>`,
		"zh-TW": `<p>請使用訂單中的 PAIPLUS 卡密，在本站兌換中心核驗帳號後確認儲值。可使用同一卡密查詢進度，請勿重複提交。</p><p>兌換地址：<a href="` + link + `">` + link + `</a></p>`,
		"en-US": `<p>Use your PAIPLUS code at our redemption center. Verify your account and confirm. Check progress with the same code; do not submit again.</p><p><a href="` + link + `">Redemption center</a></p>`,
	}
}
