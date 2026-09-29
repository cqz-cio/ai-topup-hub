package telegramauthapp

import (
	"context"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"
)

// LoginPayload is the Telegram Login Widget payload.
type LoginPayload struct {
	ID        int64
	FirstName string
	LastName  string
	Username  string
	PhotoURL  string
	AuthDate  int64
	Hash      string
}

// IdentityVerified is the canonical verified Telegram identity.
type IdentityVerified struct {
	Provider              string
	ProviderUserID        string
	ProviderUserIDAliases []string
	Username              string
	AvatarURL             string
	FirstName             string
	LastName              string
	AuthAt                time.Time
}

type ReplaySetNXFunc func(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error)
type OIDCStateSetFunc func(ctx context.Context, key string, value string, ttlSeconds int) (bool, error)
type OIDCStateTakeFunc func(ctx context.Context, key string) (string, bool, error)

type Option func(*Service)

func WithReplaySetNX(replaySetNX ReplaySetNXFunc) Option {
	return func(service *Service) {
		if replaySetNX != nil {
			service.replaySetNX = replaySetNX
		}
	}
}

func WithOIDCStateStore(set OIDCStateSetFunc, take OIDCStateTakeFunc) Option {
	return func(service *Service) {
		if set != nil && take != nil {
			service.oidcStateSet = set
			service.oidcStateTake = take
		}
	}
}

// Service verifies Telegram authentication payloads.
type Service struct {
	cfg         config.TelegramAuthConfig
	replaySetNX ReplaySetNXFunc

	httpClient        *http.Client
	oidcAuthEndpoint  string
	oidcTokenEndpoint string
	oidcJWKSEndpoint  string
	oidcStateSet      OIDCStateSetFunc
	oidcStateTake     OIDCStateTakeFunc

	jwksMu        sync.Mutex
	jwksCache     map[string]*rsa.PublicKey
	jwksFetchedAt time.Time
}

// NewService creates the Telegram authentication service.
func NewService(cfg config.TelegramAuthConfig, options ...Option) *Service {
	svc := &Service{cfg: normalizeTelegramAuthConfig(cfg)}
	svc.httpClient = &http.Client{Timeout: 10 * time.Second}
	svc.oidcAuthEndpoint = "https://oauth.telegram.org/auth"
	svc.oidcTokenEndpoint = "https://oauth.telegram.org/token"
	svc.oidcJWKSEndpoint = "https://oauth.telegram.org/.well-known/jwks.json"
	svc.jwksCache = map[string]*rsa.PublicKey{}
	for _, option := range options {
		if option != nil {
			option(svc)
		}
	}
	return svc
}

// SetConfig 更新运行时配置
func (s *Service) SetConfig(cfg config.TelegramAuthConfig) {
	if s == nil {
		return
	}
	s.cfg = normalizeTelegramAuthConfig(cfg)
}

// PublicConfig 返回前台可见配置
func (s *Service) PublicConfig() map[string]interface{} {
	if s == nil {
		return map[string]interface{}{
			"enabled":      false,
			"bot_username": "",
			"mini_app_url": "",
			"mode":         "",
		}
	}
	cfg := normalizeTelegramAuthConfig(s.cfg)
	mode := settingssecurity.ResolveTelegramLoginMode(settingssecurity.TelegramAuthSetting{
		Enabled: cfg.Enabled, BotUsername: cfg.BotUsername, BotToken: cfg.BotToken,
		ClientSecret: cfg.ClientSecret, OIDCRedirectURI: cfg.OIDCRedirectURI,
	})
	return map[string]interface{}{
		"enabled":      cfg.Enabled,
		"bot_username": strings.TrimSpace(cfg.BotUsername),
		"mini_app_url": strings.TrimSpace(cfg.MiniAppURL),
		"mode":         string(mode),
	}
}

func (s *Service) currentLoginMode() (config.TelegramAuthConfig, settingssecurity.TelegramLoginMode) {
	cfg := normalizeTelegramAuthConfig(s.cfg)
	mode := settingssecurity.ResolveTelegramLoginMode(settingssecurity.TelegramAuthSetting{
		Enabled: cfg.Enabled, BotUsername: cfg.BotUsername, BotToken: cfg.BotToken,
		ClientSecret: cfg.ClientSecret, OIDCRedirectURI: cfg.OIDCRedirectURI,
	})
	return cfg, mode
}

// VerifyLogin 校验 Telegram 登录载荷
func (s *Service) VerifyLogin(ctx context.Context, payload LoginPayload) (*IdentityVerified, error) {
	if s == nil {
		return nil, ErrTelegramAuthConfigInvalid
	}
	cfg := normalizeTelegramAuthConfig(s.cfg)
	if !cfg.Enabled {
		return nil, ErrTelegramAuthDisabled
	}
	if strings.TrimSpace(cfg.BotToken) == "" {
		return nil, ErrTelegramAuthConfigInvalid
	}
	normalized, err := normalizeTelegramLoginPayload(payload)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	authAt := time.Unix(normalized.AuthDate, 0)
	if err := validateTelegramAuthTime(now, authAt, cfg.LoginExpireSeconds); err != nil {
		return nil, err
	}

	dataCheckString := buildTelegramDataCheckString(normalized)
	expected := buildTelegramHash(cfg.BotToken, dataCheckString)
	if !hmac.Equal([]byte(expected), []byte(normalized.Hash)) {
		return nil, ErrTelegramAuthSignatureInvalid
	}

	if err := s.markTelegramReplay(ctx, normalized.ID, normalized.Hash, cfg.ReplayTTLSeconds); err != nil {
		return nil, err
	}

	return &IdentityVerified{
		Provider:       constants.UserOAuthProviderTelegram,
		ProviderUserID: strconv.FormatInt(normalized.ID, 10),
		Username:       normalized.Username,
		AvatarURL:      normalized.PhotoURL,
		FirstName:      normalized.FirstName,
		LastName:       normalized.LastName,
		AuthAt:         authAt,
	}, nil
}

// VerifyMiniAppInitData 校验 Telegram Mini App initData。
func (s *Service) VerifyMiniAppInitData(ctx context.Context, initData string) (*IdentityVerified, error) {
	if s == nil {
		return nil, ErrTelegramAuthConfigInvalid
	}
	cfg := normalizeTelegramAuthConfig(s.cfg)
	if !cfg.Enabled {
		return nil, ErrTelegramAuthDisabled
	}
	if strings.TrimSpace(cfg.BotToken) == "" {
		return nil, ErrTelegramAuthConfigInvalid
	}

	parsed, err := normalizeTelegramMiniAppInitData(initData)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	authAt := time.Unix(parsed.AuthDate, 0)
	if err := validateTelegramAuthTime(now, authAt, cfg.LoginExpireSeconds); err != nil {
		return nil, err
	}

	expected := buildTelegramMiniAppHash(cfg.BotToken, parsed.DataCheckString)
	if !hmac.Equal([]byte(expected), []byte(parsed.Hash)) {
		return nil, ErrTelegramAuthSignatureInvalid
	}

	if err := s.markTelegramReplay(ctx, parsed.User.ID, parsed.Hash, cfg.ReplayTTLSeconds); err != nil {
		return nil, err
	}

	return &IdentityVerified{
		Provider:       constants.UserOAuthProviderTelegram,
		ProviderUserID: strconv.FormatInt(parsed.User.ID, 10),
		Username:       parsed.User.Username,
		AvatarURL:      parsed.User.PhotoURL,
		FirstName:      parsed.User.FirstName,
		LastName:       parsed.User.LastName,
		AuthAt:         authAt,
	}, nil
}

func normalizeTelegramAuthConfig(cfg config.TelegramAuthConfig) config.TelegramAuthConfig {
	cfg.BotUsername = strings.TrimSpace(cfg.BotUsername)
	cfg.BotToken = strings.TrimSpace(cfg.BotToken)
	cfg.ClientSecret = strings.TrimSpace(cfg.ClientSecret)
	cfg.OIDCRedirectURI = strings.TrimRight(strings.TrimSpace(cfg.OIDCRedirectURI), "/")
	if cfg.LoginExpireSeconds <= 0 {
		cfg.LoginExpireSeconds = 300
	}
	if cfg.ReplayTTLSeconds <= 0 {
		cfg.ReplayTTLSeconds = cfg.LoginExpireSeconds
	}
	if cfg.ReplayTTLSeconds < 60 {
		cfg.ReplayTTLSeconds = 60
	}
	return cfg
}

func normalizeTelegramLoginPayload(payload LoginPayload) (LoginPayload, error) {
	normalized := LoginPayload{
		ID:        payload.ID,
		FirstName: strings.TrimSpace(payload.FirstName),
		LastName:  strings.TrimSpace(payload.LastName),
		Username:  strings.TrimSpace(payload.Username),
		PhotoURL:  strings.TrimSpace(payload.PhotoURL),
		AuthDate:  payload.AuthDate,
		Hash:      strings.ToLower(strings.TrimSpace(payload.Hash)),
	}
	if normalized.ID <= 0 || normalized.AuthDate <= 0 || normalized.Hash == "" {
		return LoginPayload{}, ErrTelegramAuthPayloadInvalid
	}
	return normalized, nil
}

type telegramMiniAppUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	PhotoURL  string `json:"photo_url"`
}

type telegramMiniAppInitDataVerified struct {
	AuthDate        int64
	Hash            string
	User            telegramMiniAppUser
	DataCheckString string
}

func normalizeTelegramMiniAppInitData(raw string) (*telegramMiniAppInitDataVerified, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, ErrTelegramAuthPayloadInvalid
	}

	values, err := url.ParseQuery(trimmed)
	if err != nil {
		return nil, ErrTelegramAuthPayloadInvalid
	}

	hash := strings.ToLower(strings.TrimSpace(values.Get("hash")))
	authDateText := strings.TrimSpace(values.Get("auth_date"))
	userRaw := strings.TrimSpace(values.Get("user"))
	if hash == "" || authDateText == "" || userRaw == "" {
		return nil, ErrTelegramAuthPayloadInvalid
	}

	authDate, err := strconv.ParseInt(authDateText, 10, 64)
	if err != nil || authDate <= 0 {
		return nil, ErrTelegramAuthPayloadInvalid
	}

	var user telegramMiniAppUser
	if err := json.Unmarshal([]byte(userRaw), &user); err != nil {
		return nil, ErrTelegramAuthPayloadInvalid
	}
	user.FirstName = strings.TrimSpace(user.FirstName)
	user.LastName = strings.TrimSpace(user.LastName)
	user.Username = strings.TrimSpace(user.Username)
	user.PhotoURL = strings.TrimSpace(user.PhotoURL)
	if user.ID <= 0 {
		return nil, ErrTelegramAuthPayloadInvalid
	}

	return &telegramMiniAppInitDataVerified{
		AuthDate:        authDate,
		Hash:            hash,
		User:            user,
		DataCheckString: buildTelegramMiniAppDataCheckString(values),
	}, nil
}

func buildTelegramDataCheckString(payload LoginPayload) string {
	values := map[string]string{
		"auth_date": strconv.FormatInt(payload.AuthDate, 10),
		"id":        strconv.FormatInt(payload.ID, 10),
	}
	if payload.FirstName != "" {
		values["first_name"] = payload.FirstName
	}
	if payload.LastName != "" {
		values["last_name"] = payload.LastName
	}
	if payload.Username != "" {
		values["username"] = payload.Username
	}
	if payload.PhotoURL != "" {
		values["photo_url"] = payload.PhotoURL
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, values[key]))
	}
	return strings.Join(parts, "\n")
}

func buildTelegramHash(botToken, dataCheckString string) string {
	secret := sha256.Sum256([]byte(strings.TrimSpace(botToken)))
	mac := hmac.New(sha256.New, secret[:])
	_, _ = mac.Write([]byte(dataCheckString))
	return hex.EncodeToString(mac.Sum(nil))
}

func buildTelegramMiniAppDataCheckString(values url.Values) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		// Mini App 的 data_check_string 仅排除 hash，其他字段按原样参与校验串构造。
		if key == "hash" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, values.Get(key)))
	}
	return strings.Join(parts, "\n")
}

func buildTelegramMiniAppHash(botToken, dataCheckString string) string {
	secretMac := hmac.New(sha256.New, []byte("WebAppData"))
	_, _ = secretMac.Write([]byte(strings.TrimSpace(botToken)))
	secret := secretMac.Sum(nil)

	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(dataCheckString))
	return hex.EncodeToString(mac.Sum(nil))
}

func validateTelegramAuthTime(now, authAt time.Time, expireSeconds int) error {
	if authAt.After(now.Add(time.Minute)) {
		return ErrTelegramAuthPayloadInvalid
	}
	if now.Sub(authAt) > time.Duration(expireSeconds)*time.Second {
		return ErrTelegramAuthExpired
	}
	return nil
}

func (s *Service) markTelegramReplay(ctx context.Context, userID int64, hash string, replayTTLSeconds int) error {
	if s == nil {
		return ErrTelegramAuthConfigInvalid
	}
	if ctx == nil {
		ctx = context.Background()
	}
	setNX := s.replaySetNX
	if setNX == nil {
		return ErrTelegramAuthConfigInvalid
	}
	replayTTL := time.Duration(replayTTLSeconds) * time.Second
	replayKey := fmt.Sprintf("telegram:auth:replay:%d:%s", userID, hash)
	ok, err := setNX(ctx, replayKey, "1", replayTTL)
	if err != nil {
		return err
	}
	if !ok {
		return ErrTelegramAuthReplay
	}
	return nil
}
