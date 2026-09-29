package userauthhttp

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

func TestRespondTelegramBindError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		err  error
		code int
		msg  string
	}{
		{
			name: "auth disabled",
			err:  ErrTelegramAuthDisabled,
			code: response.CodeBadRequest,
			msg:  "Telegram 登录未启用",
		},
		{
			name: "identity conflict",
			err:  ErrUserOAuthIdentityExists,
			code: response.CodeBadRequest,
			msg:  "该 Telegram 账号已绑定其他用户",
		},
		{
			name: "unknown error",
			err:  errors.New("boom"),
			code: response.CodeInternal,
			msg:  "更新用户资料失败",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, recorder := newTelegramErrorTestContext()
			respondTelegramBindError(c, tt.err)
			assertTelegramErrorResponse(t, recorder, tt.code, tt.msg)
		})
	}
}

func TestRespondTelegramLoginError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		err  error
		code int
		msg  string
	}{
		{
			name: "expired auth payload",
			err:  ErrTelegramAuthExpired,
			code: response.CodeBadRequest,
			msg:  "Telegram 登录已过期，请重试",
		},
		{
			name: "disabled user",
			err:  ErrUserDisabled,
			code: response.CodeUnauthorized,
			msg:  "账号已禁用",
		},
		{
			name: "registration disabled",
			err:  ErrRegistrationDisabled,
			code: response.CodeForbidden,
			msg:  "注册功能已关闭",
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
			c, recorder := newTelegramErrorTestContext()
			h := &UserTelegramHandler{}
			h.respondTelegramLoginError(c, tt.err)
			assertTelegramErrorResponse(t, recorder, tt.code, tt.msg)
		})
	}
}

func newTelegramErrorTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	return c, recorder
}

func assertTelegramErrorResponse(t *testing.T, recorder *httptest.ResponseRecorder, wantCode int, wantMsg string) {
	t.Helper()
	if recorder.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var body response.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.StatusCode != wantCode {
		t.Fatalf("status_code = %d, want %d; body=%s", body.StatusCode, wantCode, recorder.Body.String())
	}
	if body.Msg != wantMsg {
		t.Fatalf("msg = %q, want %q; body=%s", body.Msg, wantMsg, recorder.Body.String())
	}
}
