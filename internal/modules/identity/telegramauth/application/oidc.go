package telegramauthapp

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"
	"github.com/golang-jwt/jwt/v5"
)

const (
	telegramOIDCIntentLogin = "login"
	telegramOIDCIntentBind  = "bind"

	telegramOIDCIssuer      = "https://oauth.telegram.org"
	telegramOIDCStateTTL    = 600 // seconds
	telegramOIDCStatePrefix = "telegram:oidc:state:"
)

const (
	IntentLogin = telegramOIDCIntentLogin
	IntentBind  = telegramOIDCIntentBind
)

type telegramOIDCState struct {
	CodeVerifier string `json:"v"`
	Intent       string `json:"i"`
	UserID       uint   `json:"u"`
}

func base64URLNoPad(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

func newPKCEPair() (verifier string, challenge string, err error) {
	buf := make([]byte, 48) // 48 bytes -> 64 base64url chars (in [43,128])
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}
	verifier = base64URLNoPad(buf)
	return verifier, s256Challenge(verifier), nil
}

func s256Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64URLNoPad(sum[:])
}

func newOIDCState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64URLNoPad(buf), nil
}

// StartOIDCLogin 生成授权 URL 并把 PKCE/state 存入缓存
func (s *Service) StartOIDCLogin(ctx context.Context, intent string, userID uint) (string, error) {
	if s == nil {
		return "", ErrTelegramAuthConfigInvalid
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cfg, mode := s.currentLoginMode()
	if mode == settingssecurity.TelegramLoginModeDisabled {
		return "", ErrTelegramAuthDisabled
	}
	if mode != settingssecurity.TelegramLoginModeOIDC {
		return "", ErrTelegramAuthConfigInvalid
	}
	clientID := settingssecurity.TelegramBotIDFromToken(cfg.BotToken)
	if clientID == "" {
		return "", ErrTelegramAuthConfigInvalid
	}
	if intent != telegramOIDCIntentLogin && intent != telegramOIDCIntentBind {
		return "", ErrTelegramAuthPayloadInvalid
	}
	if s.oidcStateSet == nil {
		return "", ErrTelegramAuthConfigInvalid
	}

	state, err := newOIDCState()
	if err != nil {
		return "", err
	}
	verifier, challenge, err := newPKCEPair()
	if err != nil {
		return "", err
	}
	rec, err := json.Marshal(telegramOIDCState{CodeVerifier: verifier, Intent: intent, UserID: userID})
	if err != nil {
		return "", err
	}
	ok, err := s.oidcStateSet(ctx, telegramOIDCStatePrefix+state, string(rec), telegramOIDCStateTTL)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", ErrTelegramOIDCStateInvalid
	}

	q := url.Values{}
	q.Set("client_id", clientID)
	q.Set("redirect_uri", cfg.OIDCRedirectURI)
	q.Set("response_type", "code")
	q.Set("scope", "openid profile")
	q.Set("state", state)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	return fmt.Sprintf("%s?%s", s.oidcAuthEndpoint, q.Encode()), nil
}

type telegramJWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func parseTelegramJWKS(raw []byte) (map[string]*rsa.PublicKey, error) {
	var doc struct {
		Keys []telegramJWK `json:"keys"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	out := map[string]*rsa.PublicKey{}
	for _, k := range doc.Keys {
		if strings.ToUpper(k.Kty) != "RSA" {
			continue
		}
		nb, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(k.N, "="))
		if err != nil {
			continue
		}
		eb, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(k.E, "="))
		if err != nil {
			continue
		}
		pub := &rsa.PublicKey{N: new(big.Int).SetBytes(nb), E: int(new(big.Int).SetBytes(eb).Int64())}
		if pub.E == 0 {
			continue
		}
		kid := k.Kid
		if kid == "" {
			kid = "_default"
		}
		out[kid] = pub
	}
	if len(out) == 0 {
		return nil, errors.New("no RSA keys in JWKS")
	}
	return out, nil
}

func (s *Service) jwksKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	if kid == "" {
		kid = "_default"
	}
	s.jwksMu.Lock()
	cached := s.jwksCache[kid]
	stale := time.Since(s.jwksFetchedAt) > 10*time.Minute
	s.jwksMu.Unlock()
	if cached != nil && !stale {
		return cached, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.oidcJWKSEndpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("jwks http %d", resp.StatusCode)
	}
	keys, err := parseTelegramJWKS(body)
	if err != nil {
		return nil, err
	}
	s.jwksMu.Lock()
	s.jwksCache = keys
	s.jwksFetchedAt = time.Now()
	got := s.jwksCache[kid]
	if got == nil && len(keys) == 1 {
		for _, v := range keys {
			got = v
		}
	}
	s.jwksMu.Unlock()
	if got == nil {
		return nil, fmt.Errorf("kid %q not found in JWKS", kid)
	}
	return got, nil
}

type telegramOIDCTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	IDToken     string `json:"id_token"`
}

type telegramOIDCIDClaims struct {
	jwt.RegisteredClaims
	ID                json.Number `json:"id"`
	Name              string      `json:"name"`
	PreferredUsername string      `json:"preferred_username"`
	Picture           string      `json:"picture"`
}

// CompleteOIDCLogin exchanges a code and returns a verified identity.
func (s *Service) CompleteOIDCLogin(ctx context.Context, code, state string) (*IdentityVerified, string, uint, error) {
	if s == nil {
		return nil, "", 0, ErrTelegramAuthConfigInvalid
	}
	if ctx == nil {
		ctx = context.Background()
	}
	code = strings.TrimSpace(code)
	state = strings.TrimSpace(state)
	if code == "" || state == "" {
		return nil, "", 0, ErrTelegramAuthPayloadInvalid
	}
	cfg, mode := s.currentLoginMode()
	if mode == settingssecurity.TelegramLoginModeDisabled {
		return nil, "", 0, ErrTelegramAuthDisabled
	}
	if mode != settingssecurity.TelegramLoginModeOIDC {
		return nil, "", 0, ErrTelegramAuthConfigInvalid
	}
	clientID := settingssecurity.TelegramBotIDFromToken(cfg.BotToken)
	if clientID == "" {
		return nil, "", 0, ErrTelegramAuthConfigInvalid
	}
	if s.oidcStateTake == nil {
		return nil, "", 0, ErrTelegramAuthConfigInvalid
	}

	rawState, ok, err := s.oidcStateTake(ctx, telegramOIDCStatePrefix+state)
	if err != nil {
		return nil, "", 0, err
	}
	if !ok || rawState == "" {
		return nil, "", 0, ErrTelegramOIDCStateInvalid
	}
	var st telegramOIDCState
	if err := json.Unmarshal([]byte(rawState), &st); err != nil || st.CodeVerifier == "" {
		return nil, "", 0, ErrTelegramOIDCStateInvalid
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", cfg.OIDCRedirectURI)
	form.Set("client_id", clientID)
	form.Set("code_verifier", st.CodeVerifier)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.oidcTokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, "", 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(clientID+":"+cfg.ClientSecret)))
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, "", 0, fmt.Errorf("%w: %v", ErrTelegramOIDCTokenExchange, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		return nil, "", 0, fmt.Errorf("%w: http %d", ErrTelegramOIDCTokenExchange, resp.StatusCode)
	}
	var tokResp telegramOIDCTokenResponse
	if err := json.Unmarshal(body, &tokResp); err != nil || strings.TrimSpace(tokResp.IDToken) == "" {
		return nil, "", 0, ErrTelegramOIDCTokenExchange
	}

	var claims telegramOIDCIDClaims
	parser := jwt.NewParser(jwt.WithValidMethods([]string{"RS256"}), jwt.WithIssuer(telegramOIDCIssuer), jwt.WithAudience(clientID), jwt.WithExpirationRequired())
	_, err = parser.ParseWithClaims(tokResp.IDToken, &claims, func(t *jwt.Token) (interface{}, error) {
		kid, _ := t.Header["kid"].(string)
		return s.jwksKey(ctx, kid)
	})
	if err != nil {
		return nil, "", 0, fmt.Errorf("%w: %v", ErrTelegramOIDCIDTokenInvalid, err)
	}

	providerUserID, idNum, err := telegramOIDCProviderUserID(claims)
	if err != nil {
		return nil, "", 0, ErrTelegramOIDCIDTokenInvalid
	}
	subject := strings.TrimSpace(claims.Subject)
	var providerUserIDAliases []string
	if subject != "" && subject != providerUserID {
		providerUserIDAliases = append(providerUserIDAliases, subject)
	}
	firstName, lastName := splitTelegramName(claims.Name)
	authAt := time.Now()
	if claims.IssuedAt != nil {
		authAt = claims.IssuedAt.Time
	}

	fp := sha256.Sum256([]byte(tokResp.IDToken))
	if err := s.markTelegramReplay(ctx, idNum, hex.EncodeToString(fp[:8]), cfg.ReplayTTLSeconds); err != nil {
		return nil, "", 0, err
	}

	return &IdentityVerified{
		Provider:              constants.UserOAuthProviderTelegram,
		ProviderUserID:        providerUserID,
		ProviderUserIDAliases: providerUserIDAliases,
		Username:              strings.TrimSpace(claims.PreferredUsername),
		AvatarURL:             strings.TrimSpace(claims.Picture),
		FirstName:             firstName,
		LastName:              lastName,
		AuthAt:                authAt,
	}, st.Intent, st.UserID, nil
}

// telegramOIDCProviderUserID 从 Telegram OIDC claims 中提取标准 Telegram 数字用户 ID。
func telegramOIDCProviderUserID(claims telegramOIDCIDClaims) (string, int64, error) {
	idNum, err := claims.ID.Int64()
	if err != nil || idNum <= 0 {
		return "", 0, ErrTelegramOIDCIDTokenInvalid
	}
	return strconv.FormatInt(idNum, 10), idNum, nil
}

func splitTelegramName(name string) (string, string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ""
	}
	if i := strings.IndexByte(name, ' '); i > 0 {
		return strings.TrimSpace(name[:i]), strings.TrimSpace(name[i+1:])
	}
	return name, ""
}
