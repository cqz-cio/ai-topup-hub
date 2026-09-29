package userauthhttp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

type googleBodyLimitService struct {
	loginCalls int
	bindCalls  int
}

type googleRedirectHandlerService struct {
	createState         string
	createErr           error
	createCalls         int
	completeResult      *GoogleRedirectCompletionResult
	completeErr         error
	completeCalls       int
	completeState       string
	completeCredential  string
	exchangeLoginResult *AuthLoginResult
	exchangeLoginErr    error
	exchangeLoginCalls  int
	exchangeBindErr     error
	exchangeBindCalls   int
	popupResult         *AuthLoginResult
	popupErr            error
	popupCalls          int
}

type googleRedirectAuditRecorder struct {
	calls      int
	email      string
	userID     uint
	status     string
	failReason string
	source     string
}

func (r *googleRedirectAuditRecorder) Record(
	email string,
	userID uint,
	status string,
	failReason string,
	source string,
	_ string,
	_ string,
	_ string,
) {
	r.calls++
	r.email = email
	r.userID = userID
	r.status = status
	r.failReason = failReason
	r.source = source
}

func (s *googleRedirectHandlerService) LoginWithGoogle(context.Context, string) (*AuthLoginResult, error) {
	s.popupCalls++
	return s.popupResult, s.popupErr
}

func (*googleRedirectHandlerService) GetGoogleBinding(uint) (*GoogleBindingResult, error) {
	return &GoogleBindingResult{}, nil
}

func (*googleRedirectHandlerService) BindGoogle(context.Context, uint, string) (*GoogleBindingResult, error) {
	return &GoogleBindingResult{}, nil
}

func (*googleRedirectHandlerService) UnbindGoogle(uint) error { return nil }

func (s *googleRedirectHandlerService) CreateGoogleRedirectIntent(
	_ context.Context,
	_ string,
	_ uint,
	_ GoogleRedirectTenant,
) (string, error) {
	s.createCalls++
	return s.createState, s.createErr
}

func (s *googleRedirectHandlerService) CompleteGoogleRedirect(
	_ context.Context,
	state string,
	credential string,
	_ GoogleRedirectTenant,
) (*GoogleRedirectCompletionResult, error) {
	s.completeCalls++
	s.completeState = state
	s.completeCredential = credential
	return s.completeResult, s.completeErr
}

func (s *googleRedirectHandlerService) ExchangeGoogleRedirectLogin(
	context.Context,
	string,
	GoogleRedirectTenant,
) (*AuthLoginResult, error) {
	s.exchangeLoginCalls++
	return s.exchangeLoginResult, s.exchangeLoginErr
}

func (s *googleRedirectHandlerService) ExchangeGoogleRedirectBind(
	context.Context,
	string,
	uint,
	GoogleRedirectTenant,
) (*GoogleBindingResult, error) {
	s.exchangeBindCalls++
	if s.exchangeBindErr != nil {
		return nil, s.exchangeBindErr
	}
	return &GoogleBindingResult{}, nil
}

func (s *googleBodyLimitService) LoginWithGoogle(context.Context, string) (*AuthLoginResult, error) {
	s.loginCalls++
	return nil, nil
}

func (*googleBodyLimitService) GetGoogleBinding(uint) (*GoogleBindingResult, error) {
	return &GoogleBindingResult{}, nil
}

func (s *googleBodyLimitService) BindGoogle(context.Context, uint, string) (*GoogleBindingResult, error) {
	s.bindCalls++
	return &GoogleBindingResult{}, nil
}

func (*googleBodyLimitService) UnbindGoogle(uint) error {
	return nil
}

func (*googleBodyLimitService) CreateGoogleRedirectIntent(
	context.Context,
	string,
	uint,
	GoogleRedirectTenant,
) (string, error) {
	return "", nil
}

func (*googleBodyLimitService) CompleteGoogleRedirect(
	context.Context,
	string,
	string,
	GoogleRedirectTenant,
) (*GoogleRedirectCompletionResult, error) {
	return nil, nil
}

func (*googleBodyLimitService) ExchangeGoogleRedirectLogin(
	context.Context,
	string,
	GoogleRedirectTenant,
) (*AuthLoginResult, error) {
	return nil, nil
}

func (*googleBodyLimitService) ExchangeGoogleRedirectBind(
	context.Context,
	string,
	uint,
	GoogleRedirectTenant,
) (*GoogleBindingResult, error) {
	return nil, nil
}

func TestRespondGoogleLoginError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code int
		msg  string
	}{
		{
			name: "invalid credential",
			err:  ErrGoogleCredentialInvalid,
			code: response.CodeBadRequest,
			msg:  "Google 身份凭证无效，请重试",
		},
		{
			name: "unsafe automatic link",
			err:  ErrGoogleAutoLinkForbidden,
			code: response.CodeForbidden,
			msg:  "请先使用原账号登录，再绑定此 Google 账号",
		},
		{
			name: "upstream unavailable",
			err:  ErrGoogleJWKSUnavailable,
			code: response.CodeInternal,
			msg:  "Google 登录服务暂时不可用，请稍后重试",
		},
		{
			name: "unknown error",
			err:  errors.New("boom"),
			code: response.CodeInternal,
			msg:  "登录失败",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			context, recorder := newTelegramErrorTestContext()
			handler := &UserGoogleHandler{}
			handler.respondGoogleLoginError(context, tt.err)
			assertTelegramErrorResponse(t, recorder, tt.code, tt.msg)
		})
	}
}

func TestRespondGoogleBindError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code int
		msg  string
	}{
		{
			name: "identity belongs to another user",
			err:  ErrUserOAuthIdentityExists,
			code: response.CodeBadRequest,
			msg:  "该 Google 账号已绑定其他用户",
		},
		{
			name: "current user already has another identity",
			err:  ErrUserOAuthAlreadyBound,
			code: response.CodeBadRequest,
			msg:  "当前账号已绑定其他 Google 账号",
		},
		{
			name: "invalid credential",
			err:  ErrGoogleCredentialInvalid,
			code: response.CodeBadRequest,
			msg:  "Google 身份凭证无效，请重试",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			context, recorder := newTelegramErrorTestContext()
			respondGoogleBindError(context, tt.err)
			assertTelegramErrorResponse(t, recorder, tt.code, tt.msg)
		})
	}
}

func TestGoogleCredentialEndpointsRejectOversizedBodiesBeforeService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &googleBodyLimitService{}
	handler := NewUserGoogleHandler(service, nil)
	router := gin.New()
	router.POST("/auth/google/login", handler.UserGoogleLogin)
	router.POST("/me/google", func(c *gin.Context) {
		c.Set("user_id", uint(1))
		handler.BindMyGoogle(c)
	})

	body := `{"credential":"` + strings.Repeat("x", maxGoogleCredentialRequestBodyBytes) + `"}`
	for _, test := range []struct {
		name string
		path string
	}{
		{name: "login", path: "/auth/google/login"},
		{name: "bind", path: "/me/google"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("HTTP status = %d, want %d; body=%s", recorder.Code, http.StatusRequestEntityTooLarge, recorder.Body.String())
			}
			var payload response.Response
			if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if payload.StatusCode != response.CodeBadRequest {
				t.Fatalf("status_code = %d, want %d", payload.StatusCode, response.CodeBadRequest)
			}
		})
	}
	if service.loginCalls != 0 || service.bindCalls != 0 {
		t.Fatalf("service calls = login:%d bind:%d, want zero", service.loginCalls, service.bindCalls)
	}
}

func TestGoogleRedirectIntentReturnsStateAndSecureHostCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	state := testGoogleRedirectHandle('s')
	service := &googleRedirectHandlerService{createState: state}
	handler := NewUserGoogleHandler(service, nil)
	router := gin.New()
	router.POST("/auth/google/redirect/intent", handler.CreateGoogleRedirectLoginIntent)

	request := httptest.NewRequest(http.MethodPost, "/auth/google/redirect/intent", strings.NewReader(`{}`))
	request = request.WithContext(resellercontract.WithTenantContext(
		request.Context(),
		resellercontract.MainTenantContext("shop.example.com"),
	))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var payload response.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, ok := payload.Data.(map[string]interface{})
	if !ok || data["state"] != state || data["expires_in"] != float64(googleRedirectIntentTTLSeconds) {
		t.Fatalf("intent data = %#v", payload.Data)
	}
	cookie := responseCookieByName(t, recorder, googleRedirectIntentCookieName)
	if cookie.Value != state ||
		cookie.Path != "/" ||
		cookie.Domain != "" ||
		!cookie.HttpOnly ||
		!cookie.Secure ||
		cookie.SameSite != http.SameSiteNoneMode ||
		cookie.MaxAge != googleRedirectIntentTTLSeconds {
		t.Fatalf("state cookie = %#v", cookie)
	}
	assertGoogleRedirectSecurityHeaders(t, recorder)
}

func TestGoogleRedirectCallbackSuccessUsesOnlyFixedBindLocationAndSecureHandoff(t *testing.T) {
	gin.SetMode(gin.TestMode)
	state := testGoogleRedirectHandle('s')
	handoff := testGoogleRedirectHandle('h')
	service := &googleRedirectHandlerService{
		completeResult: &GoogleRedirectCompletionResult{
			Flow:          googleRedirectFlowBind,
			HandoffHandle: handoff,
		},
	}
	handler := NewUserGoogleHandler(service, nil)
	router := gin.New()
	router.POST("/auth/google/redirect/callback", handler.CompleteGoogleRedirect)

	form := url.Values{
		"credential":   {"secret-google-credential"},
		"state":        {state},
		"g_csrf_token": {"google-csrf"},
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/auth/google/redirect/callback",
		strings.NewReader(form.Encode()),
	)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(&http.Cookie{Name: "g_csrf_token", Value: "google-csrf"})
	request.AddCookie(&http.Cookie{Name: googleRedirectIntentCookieName, Value: state})
	request = request.WithContext(resellercontract.WithTenantContext(
		request.Context(),
		resellercontract.MainTenantContext("shop.example.com"),
	))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	location := recorder.Header().Get("Location")
	if location != "/auth/google/callback?flow=bind" {
		t.Fatalf("Location = %q", location)
	}
	for _, secret := range []string{state, handoff, "secret-google-credential", "google-csrf"} {
		if strings.Contains(location, secret) {
			t.Fatalf("Location leaked %q: %s", secret, location)
		}
	}
	if service.completeCalls != 1 ||
		service.completeState != state ||
		service.completeCredential != "secret-google-credential" {
		t.Fatalf("complete call = count:%d state:%q credential:%q",
			service.completeCalls,
			service.completeState,
			service.completeCredential,
		)
	}
	stateCookie := responseCookieByName(t, recorder, googleRedirectIntentCookieName)
	if stateCookie.MaxAge >= 0 {
		t.Fatalf("state cookie was not cleared: %#v", stateCookie)
	}
	handoffCookie := responseCookieByName(t, recorder, googleRedirectHandoffCookieName)
	if handoffCookie.Value != handoff ||
		handoffCookie.Path != "/" ||
		handoffCookie.Domain != "" ||
		!handoffCookie.HttpOnly ||
		!handoffCookie.Secure ||
		handoffCookie.SameSite != http.SameSiteLaxMode ||
		handoffCookie.MaxAge != googleRedirectHandoffTTLSeconds {
		t.Fatalf("handoff cookie = %#v", handoffCookie)
	}
	assertGoogleRedirectSecurityHeaders(t, recorder)
}

func TestGoogleRedirectCallbackPreservesTrustedBindFlowOnCredentialFailure(t *testing.T) {
	state := testGoogleRedirectHandle('s')
	service := &googleRedirectHandlerService{
		completeResult: &GoogleRedirectCompletionResult{Flow: googleRedirectFlowBind},
		completeErr:    ErrGoogleCredentialInvalid,
	}
	recorder := serveGoogleRedirectCallback(t, service, state, state, "csrf", "csrf")
	if location := recorder.Header().Get("Location"); location !=
		"/auth/google/callback?flow=bind&error=credential_invalid" {
		t.Fatalf("Location = %q", location)
	}
}

func TestGoogleRedirectCallbackAuditsOnlyTrustedLoginVerificationFailures(t *testing.T) {
	state := testGoogleRedirectHandle('s')
	for _, test := range []struct {
		name       string
		flow       string
		err        error
		wantCalls  int
		wantReason string
	}{
		{
			name:       "login invalid credential",
			flow:       googleRedirectFlowLogin,
			err:        ErrGoogleCredentialInvalid,
			wantCalls:  1,
			wantReason: constants.LoginLogFailReasonGoogleInvalid,
		},
		{
			name:       "login verifier configuration",
			flow:       googleRedirectFlowLogin,
			err:        ErrGoogleJWKSUnavailable,
			wantCalls:  1,
			wantReason: constants.LoginLogFailReasonGoogleConfig,
		},
		{
			name:      "bind verification failure is not a login audit",
			flow:      googleRedirectFlowBind,
			err:       ErrGoogleCredentialInvalid,
			wantCalls: 0,
		},
		{
			name:      "login tenant mismatch is not verifier noise",
			flow:      googleRedirectFlowLogin,
			err:       ErrGoogleRedirectTenantMismatch,
			wantCalls: 0,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &googleRedirectHandlerService{
				completeResult: &GoogleRedirectCompletionResult{Flow: test.flow},
				completeErr:    test.err,
			}
			recorder := &googleRedirectAuditRecorder{}
			serveGoogleRedirectCallbackWithRecorder(
				t,
				service,
				recorder,
				state,
				state,
				"csrf",
				"csrf",
			)
			if recorder.calls != test.wantCalls {
				t.Fatalf("audit calls = %d, want %d", recorder.calls, test.wantCalls)
			}
			if test.wantCalls == 1 {
				if recorder.email != "" ||
					recorder.userID != 0 ||
					recorder.status != constants.LoginLogStatusFailed ||
					recorder.failReason != test.wantReason ||
					recorder.source != constants.LoginLogSourceGoogle {
					t.Fatalf("audit record = %#v", recorder)
				}
			}
		})
	}
}

func TestGoogleRedirectCallbackInvalidGISCSRFPreservesIntentCookie(t *testing.T) {
	state := testGoogleRedirectHandle('s')
	service := &googleRedirectHandlerService{}
	recorder := serveGoogleRedirectCallback(t, service, state, state, "form-csrf", "cookie-csrf")

	if service.completeCalls != 0 {
		t.Fatalf("CompleteGoogleRedirect calls = %d, want 0", service.completeCalls)
	}
	if location := recorder.Header().Get("Location"); location !=
		"/auth/google/callback?flow=login&error=csrf_mismatch" {
		t.Fatalf("Location = %q", location)
	}
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == googleRedirectIntentCookieName {
			t.Fatalf("untrusted CSRF failure cleared intent cookie: %#v", cookie)
		}
	}
}

func TestGoogleRedirectCallbackRejectsMalformedStateBeforeService(t *testing.T) {
	service := &googleRedirectHandlerService{}
	malformed := strings.Repeat("x", 512)
	recorder := serveGoogleRedirectCallback(t, service, malformed, malformed, "csrf", "csrf")

	if service.completeCalls != 0 {
		t.Fatalf("CompleteGoogleRedirect calls = %d, want 0", service.completeCalls)
	}
	if location := recorder.Header().Get("Location"); location !=
		"/auth/google/callback?flow=login&error=session_expired" {
		t.Fatalf("Location = %q", location)
	}
}

func TestGoogleRedirectExchangeClearsHandoffBeforeContextFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handle := testGoogleRedirectHandle('h')
	service := &googleRedirectHandlerService{exchangeLoginErr: ErrGoogleRedirectTenantMismatch}
	handler := NewUserGoogleHandler(service, nil)
	router := gin.New()
	router.POST("/auth/google/redirect/exchange", handler.ExchangeGoogleRedirectLogin)

	request := httptest.NewRequest(http.MethodPost, "/auth/google/redirect/exchange", strings.NewReader(`{}`))
	request.AddCookie(&http.Cookie{Name: googleRedirectHandoffCookieName, Value: handle})
	request = request.WithContext(resellercontract.WithTenantContext(
		request.Context(),
		resellercontract.MainTenantContext("other.example.com"),
	))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if service.exchangeLoginCalls != 1 {
		t.Fatalf("exchange calls = %d, want 1", service.exchangeLoginCalls)
	}
	cookie := responseCookieByName(t, recorder, googleRedirectHandoffCookieName)
	if cookie.MaxAge >= 0 || !cookie.HttpOnly || !cookie.Secure || cookie.Path != "/" {
		t.Fatalf("handoff deletion cookie = %#v", cookie)
	}
}

func TestGoogleRedirectUnavailableDoesNotDisablePopupLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &googleRedirectHandlerService{
		createErr: ErrGoogleRedirectUnavailable,
		popupResult: &AuthLoginResult{
			User:      &userdomain.User{ID: 7, Email: "person@gmail.com"},
			Token:     "jwt",
			ExpiresAt: time.Now().Add(time.Hour),
		},
	}
	handler := NewUserGoogleHandler(service, nil)
	router := gin.New()
	router.POST("/auth/google/login", handler.UserGoogleLogin)
	router.POST("/auth/google/redirect/intent", handler.CreateGoogleRedirectLoginIntent)

	intentRequest := httptest.NewRequest(http.MethodPost, "/auth/google/redirect/intent", strings.NewReader(`{}`))
	intentRequest = intentRequest.WithContext(resellercontract.WithTenantContext(
		intentRequest.Context(),
		resellercontract.MainTenantContext("shop.example.com"),
	))
	intentRecorder := httptest.NewRecorder()
	router.ServeHTTP(intentRecorder, intentRequest)
	if intentRecorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("intent HTTP status = %d, want %d", intentRecorder.Code, http.StatusServiceUnavailable)
	}

	popupRequest := httptest.NewRequest(
		http.MethodPost,
		"/auth/google/login",
		strings.NewReader(`{"credential":"popup-credential"}`),
	)
	popupRequest.Header.Set("Content-Type", "application/json")
	popupRecorder := httptest.NewRecorder()
	router.ServeHTTP(popupRecorder, popupRequest)
	if popupRecorder.Code != http.StatusOK || service.popupCalls != 1 {
		t.Fatalf("popup status/calls = %d/%d; body=%s", popupRecorder.Code, service.popupCalls, popupRecorder.Body.String())
	}
}

func TestGoogleRedirectCallbackAndExchangeDoNotConsumeLoginAttemptRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	state := testGoogleRedirectHandle('s')
	rateLimitCalls := 0
	rateLimit := func(c *gin.Context) {
		rateLimitCalls++
		c.Next()
	}
	service := &googleRedirectHandlerService{
		createState: state,
		popupErr:    ErrGoogleCredentialInvalid,
	}
	handler := NewUserGoogleHandler(service, nil)
	router := gin.New()
	auth := router.Group("/auth")
	RegisterUserGoogleAuthRoutes(auth, handler, rateLimit)

	requests := []*http.Request{
		httptest.NewRequest(
			http.MethodPost,
			"/auth/google/login",
			strings.NewReader(`{"credential":"invalid"}`),
		),
		httptest.NewRequest(
			http.MethodPost,
			"/auth/google/redirect/intent",
			strings.NewReader(`{}`),
		),
		httptest.NewRequest(
			http.MethodPost,
			"/auth/google/redirect/callback",
			strings.NewReader(`not-form`),
		),
		httptest.NewRequest(
			http.MethodPost,
			"/auth/google/redirect/exchange",
			strings.NewReader(`{}`),
		),
	}
	requests[0].Header.Set("Content-Type", "application/json")
	for index, request := range requests {
		request = request.WithContext(resellercontract.WithTenantContext(
			request.Context(),
			resellercontract.MainTenantContext("shop.example.com"),
		))
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code == http.StatusNotFound {
			t.Fatalf("request %d route not registered", index)
		}
	}
	if rateLimitCalls != 2 {
		t.Fatalf("rate limit calls = %d, want only popup login + intent", rateLimitCalls)
	}
}

func serveGoogleRedirectCallback(
	t *testing.T,
	service *googleRedirectHandlerService,
	formState string,
	cookieState string,
	formCSRF string,
	cookieCSRF string,
) *httptest.ResponseRecorder {
	return serveGoogleRedirectCallbackWithRecorder(
		t,
		service,
		nil,
		formState,
		cookieState,
		formCSRF,
		cookieCSRF,
	)
}

func serveGoogleRedirectCallbackWithRecorder(
	t *testing.T,
	service *googleRedirectHandlerService,
	loginRecorder LoginRecorder,
	formState string,
	cookieState string,
	formCSRF string,
	cookieCSRF string,
) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	handler := NewUserGoogleHandler(service, loginRecorder)
	router := gin.New()
	router.POST("/auth/google/redirect/callback", handler.CompleteGoogleRedirect)
	form := url.Values{
		"credential":   {"secret-google-credential"},
		"state":        {formState},
		"g_csrf_token": {formCSRF},
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/auth/google/redirect/callback",
		strings.NewReader(form.Encode()),
	)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(&http.Cookie{Name: "g_csrf_token", Value: cookieCSRF})
	request.AddCookie(&http.Cookie{Name: googleRedirectIntentCookieName, Value: cookieState})
	request = request.WithContext(resellercontract.WithTenantContext(
		request.Context(),
		resellercontract.MainTenantContext("shop.example.com"),
	))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	return recorder
}

func testGoogleRedirectHandle(seed byte) string {
	return base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{seed}, 32))
}

func responseCookieByName(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
	name string,
) *http.Cookie {
	t.Helper()
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("response cookie %s not found; Set-Cookie=%v", name, recorder.Header().Values("Set-Cookie"))
	return nil
}

func assertGoogleRedirectSecurityHeaders(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	for header, expected := range map[string]string{
		"Cache-Control":   "no-store",
		"Pragma":          "no-cache",
		"Referrer-Policy": "no-referrer",
	} {
		if got := recorder.Header().Get(header); got != expected {
			t.Fatalf("%s = %q, want %q", header, got, expected)
		}
	}
}
