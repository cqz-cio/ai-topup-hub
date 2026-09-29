package turnstile

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dujiao-next/internal/modules/captcha/contract"
	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"
)

// Client 通过 Cloudflare Siteverify API 校验 Turnstile 令牌。
type Client struct{}

var _ contract.TurnstileVerifier = Client{}

// New 创建 Turnstile 校验客户端。
func New() Client { return Client{} }

type verifyResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

// Verify 校验令牌。
func (Client) Verify(cfg settingssecurity.CaptchaTurnstileSetting, token, clientIP string) error {
	secret := strings.TrimSpace(cfg.SecretKey)
	verifyURL := strings.TrimSpace(cfg.VerifyURL)
	if secret == "" || verifyURL == "" {
		return contract.ErrConfigInvalid
	}
	timeout := cfg.TimeoutMS
	if timeout < 500 || timeout > 10000 {
		timeout = 2000
	}
	form := url.Values{}
	form.Set("secret", secret)
	form.Set("response", token)
	if clientIP != "" {
		form.Set("remoteip", clientIP)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, verifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("%w: %v", contract.ErrVerifyFailed, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: time.Duration(timeout) * time.Millisecond}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", contract.ErrVerifyFailed, err)
	}
	defer resp.Body.Close()
	var result verifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("%w: %v", contract.ErrVerifyFailed, err)
	}
	if !result.Success {
		return contract.ErrInvalid
	}
	return nil
}
