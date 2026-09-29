package captchahttp

import (
	captchaapp "github.com/dujiao-next/internal/modules/captcha/application"
)

// Verifier adapts the captcha module to HTTP transport contracts.
type Verifier struct {
	service *captchaapp.Service
}

func NewVerifier(service *captchaapp.Service) Verifier {
	return Verifier{service: service}
}

func (v Verifier) Verify(scene string, payload CaptchaPayloadRequest, clientIP string) error {
	if v.service == nil {
		return nil
	}
	return v.service.Verify(scene, payload.ToCaptchaPayload(), clientIP)
}
