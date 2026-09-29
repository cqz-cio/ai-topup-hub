package captchahttp

import (
	"strings"

	captchacontract "github.com/dujiao-next/internal/modules/captcha/contract"
)

// CaptchaPayloadRequest 验证码请求载荷。
type CaptchaPayloadRequest struct {
	CaptchaID      string `json:"captcha_id"`
	CaptchaCode    string `json:"captcha_code"`
	TurnstileToken string `json:"turnstile_token"`
}

// ToCaptchaPayload 转换为验证码模块载荷。
func (r CaptchaPayloadRequest) ToCaptchaPayload() captchacontract.VerifyPayload {
	return captchacontract.VerifyPayload{
		CaptchaID:      strings.TrimSpace(r.CaptchaID),
		CaptchaCode:    strings.TrimSpace(r.CaptchaCode),
		TurnstileToken: strings.TrimSpace(r.TurnstileToken),
	}
}
