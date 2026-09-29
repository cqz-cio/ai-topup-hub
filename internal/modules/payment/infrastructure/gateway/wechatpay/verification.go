package wechatpay

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"strings"

	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/validators"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/downloader"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
)

const (
	verificationModePlatformCertificate = "platform_certificate"
	verificationModeWechatPayPublicKey  = "wechatpay_public_key"
	verificationModeCombined            = "combined"
)

// responseSignatureValidator 把 SDK 的应答验签错误映射到本包的签名错误，
// 让上层可以区分网络失败和不可信应答。
type responseSignatureValidator struct {
	inner auth.Validator
}

func (v *responseSignatureValidator) Validate(ctx context.Context, response *http.Response) error {
	if err := v.inner.Validate(ctx, response); err != nil {
		return fmt.Errorf("%w: verify API response failed: %v", ErrSignatureInvalid, err)
	}
	return nil
}

func (v *responseSignatureValidator) GetAcceptSerial(ctx context.Context) (string, error) {
	return v.inner.GetAcceptSerial(ctx)
}

type responseValidatorOption struct {
	validator auth.Validator
}

func (o responseValidatorOption) Apply(settings *core.DialSettings) error {
	settings.Validator = o.validator
	return nil
}

func withResponseSignatureVerifier(verifier auth.Verifier) core.ClientOption {
	validator := validators.NewWechatPayResponseValidator(verifier)
	return responseValidatorOption{
		validator: &responseSignatureValidator{inner: validator},
	}
}

func createAPIClient(ctx context.Context, cfg *Config) (*core.Client, error) {
	privateKey, err := parsePrivateKey(cfg.MerchantPrivateKey)
	if err != nil {
		return nil, err
	}
	verifier, err := createWechatPayVerifier(ctx, cfg, privateKey)
	if err != nil {
		return nil, err
	}
	return createAPIClientWithVerifier(ctx, cfg, privateKey, verifier)
}

func createAPIClientWithVerifier(
	ctx context.Context,
	cfg *Config,
	privateKey *rsa.PrivateKey,
	verifier auth.Verifier,
	extraOptions ...core.ClientOption,
) (*core.Client, error) {
	options := []core.ClientOption{
		option.WithMerchantCredential(cfg.MerchantID, cfg.MerchantSerialNo, privateKey),
		withResponseSignatureVerifier(verifier),
	}
	options = append(options, extraOptions...)
	client, err := core.NewClient(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("%w: init client failed", ErrConfigInvalid)
	}
	return client, nil
}

type acceptJSONRoundTripper struct {
	base http.RoundTripper
}

func (t acceptJSONRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	cloned := req.Clone(req.Context())
	cloned.Header = req.Header.Clone()
	cloned.Header.Set("Accept", "application/json")
	return base.RoundTrip(cloned)
}

func withAcceptJSONHTTPClient() core.ClientOption {
	return option.WithHTTPClient(&http.Client{
		Timeout: defaultTimeout,
		Transport: acceptJSONRoundTripper{
			base: http.DefaultTransport,
		},
	})
}

func createWechatPayPublicKeyVerifier(cfg *Config) (auth.Verifier, error) {
	publicKey, err := parsePublicKey(cfg.WechatPayPublicKey)
	if err != nil {
		return nil, err
	}
	return verifiers.NewSHA256WithRSAPubkeyVerifier(cfg.WechatPayPublicKeyID, *publicKey), nil
}

func createWechatPayVerifier(
	ctx context.Context,
	cfg *Config,
	privateKey *rsa.PrivateKey,
) (auth.Verifier, error) {
	var certificateVisitor core.CertificateGetter
	if cfg.VerificationMode != verificationModeWechatPayPublicKey {
		visitor, err := getPlatformCertificateVisitor(ctx, cfg, privateKey)
		if err != nil {
			return nil, err
		}
		certificateVisitor = visitor
	}
	return createWechatPayVerifierWithVisitor(cfg, certificateVisitor)
}

// createWechatPayVerifierWithVisitor 把 API 应答和回调统一到同一套模式选择逻辑。
// certificateVisitor 参数单独传入，也便于使用本地测试证书做密码学回归测试。
func createWechatPayVerifierWithVisitor(
	cfg *Config,
	certificateVisitor core.CertificateGetter,
) (auth.Verifier, error) {
	switch cfg.VerificationMode {
	case verificationModePlatformCertificate:
		if certificateVisitor == nil {
			return nil, fmt.Errorf("%w: platform certificate visitor is required", ErrConfigInvalid)
		}
		return verifiers.NewSHA256WithRSAVerifier(certificateVisitor), nil
	case verificationModeWechatPayPublicKey:
		return createWechatPayPublicKeyVerifier(cfg)
	case verificationModeCombined:
		if certificateVisitor == nil {
			return nil, fmt.Errorf("%w: platform certificate visitor is required", ErrConfigInvalid)
		}
		publicKey, err := parsePublicKey(cfg.WechatPayPublicKey)
		if err != nil {
			return nil, err
		}
		return verifiers.NewSHA256WithRSACombinedVerifier(
			certificateVisitor,
			cfg.WechatPayPublicKeyID,
			*publicKey,
		), nil
	default:
		return nil, fmt.Errorf("%w: verification_mode is invalid", ErrConfigInvalid)
	}
}

func getPlatformCertificateVisitor(
	ctx context.Context,
	cfg *Config,
	privateKey *rsa.PrivateKey,
) (core.CertificateVisitor, error) {
	if privateKey == nil {
		return nil, fmt.Errorf("%w: merchant_private_key is required", ErrConfigInvalid)
	}
	mgr := downloader.MgrInstance()
	if !mgr.HasDownloader(ctx, cfg.MerchantID) {
		if err := mgr.RegisterDownloaderWithPrivateKey(
			ctx,
			privateKey,
			cfg.MerchantSerialNo,
			cfg.MerchantID,
			cfg.APIV3Key,
		); err != nil {
			return nil, fmt.Errorf("%w: register certificate downloader failed", ErrRequestFailed)
		}
	}
	return mgr.GetCertificateVisitor(cfg.MerchantID), nil
}

func validateVerificationConfig(cfg *Config) error {
	switch cfg.VerificationMode {
	case verificationModePlatformCertificate:
		return nil
	case verificationModeWechatPayPublicKey, verificationModeCombined:
		if cfg.WechatPayPublicKeyID == "" {
			return fmt.Errorf("%w: wechatpay_public_key_id is required", ErrConfigInvalid)
		}
		if cfg.WechatPayPublicKey == "" {
			return fmt.Errorf("%w: wechatpay_public_key is required", ErrConfigInvalid)
		}
		if _, err := parsePublicKey(cfg.WechatPayPublicKey); err != nil {
			return err
		}
		return nil
	default:
		return fmt.Errorf("%w: verification_mode is invalid", ErrConfigInvalid)
	}
}

func parsePublicKey(raw string) (*rsa.PublicKey, error) {
	normalized := normalizePublicKey(raw)
	if normalized == "" {
		return nil, fmt.Errorf("%w: wechatpay_public_key is empty", ErrConfigInvalid)
	}
	block, _ := pem.Decode([]byte(normalized))
	if block == nil {
		return nil, fmt.Errorf("%w: wechatpay_public_key pem decode failed", ErrConfigInvalid)
	}
	switch block.Type {
	case "PUBLIC KEY":
		parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("%w: parse wechatpay_public_key failed", ErrConfigInvalid)
		}
		publicKey, ok := parsed.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("%w: wechatpay_public_key type is not rsa", ErrConfigInvalid)
		}
		return publicKey, nil
	case "RSA PUBLIC KEY":
		publicKey, err := x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("%w: parse wechatpay_public_key failed", ErrConfigInvalid)
		}
		return publicKey, nil
	default:
		return nil, fmt.Errorf("%w: wechatpay_public_key pem type is invalid", ErrConfigInvalid)
	}
}

func normalizePublicKey(raw string) string {
	normalized := strings.TrimSpace(strings.ReplaceAll(raw, "\\n", "\n"))
	if normalized == "" {
		return ""
	}
	if !strings.Contains(normalized, "BEGIN") {
		return "-----BEGIN PUBLIC KEY-----\n" + normalized + "\n-----END PUBLIC KEY-----"
	}
	return normalized
}

func (c *Config) normalizeVerificationConfig() {
	c.VerificationMode = strings.ToLower(strings.TrimSpace(c.VerificationMode))
	if c.VerificationMode == "" {
		c.VerificationMode = verificationModePlatformCertificate
	}
	c.WechatPayPublicKeyID = strings.TrimSpace(c.WechatPayPublicKeyID)
	c.WechatPayPublicKey = strings.TrimSpace(c.WechatPayPublicKey)
}
