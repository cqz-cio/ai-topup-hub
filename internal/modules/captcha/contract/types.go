package contract

// VerifyPayload 是验证码校验输入。
type VerifyPayload struct {
	CaptchaID      string
	CaptchaCode    string
	TurnstileToken string
}

// ImageChallenge 是图片验证码挑战。
type ImageChallenge struct {
	CaptchaID   string
	ImageBase64 string
}
