package adminuserhttp

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/gin-gonic/gin"
)

type oauthIdentityUnbinderStub struct {
	googleErr    error
	googleUserID uint
}

func (s *oauthIdentityUnbinderStub) UnbindTelegram(uint) error {
	return nil
}

func (s *oauthIdentityUnbinderStub) UnbindGoogle(userID uint) error {
	s.googleUserID = userID
	return s.googleErr
}

func TestUnbindAdminUserGoogle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name        string
		id          string
		serviceErr  error
		wantCode    int
		wantMessage string
		wantUserID  uint
	}{
		{name: "success", id: "42", wantCode: response.CodeOK, wantMessage: "success", wantUserID: 42},
		{name: "invalid user id", id: "bad", wantCode: response.CodeBadRequest, wantMessage: "Invalid user ID"},
		{name: "user not found", id: "42", serviceErr: ErrNotFound, wantCode: response.CodeNotFound, wantMessage: "User not found", wantUserID: 42},
		{name: "user disabled", id: "42", serviceErr: ErrUserDisabled, wantCode: response.CodeBadRequest, wantMessage: "Account disabled", wantUserID: 42},
		{name: "google not bound", id: "42", serviceErr: ErrUserOAuthNotBound, wantCode: response.CodeBadRequest, wantMessage: "Current account is not bound to Google", wantUserID: 42},
		{name: "would lock account", id: "42", serviceErr: ErrGoogleUnbindLocked, wantCode: response.CodeBadRequest, wantMessage: "Set a local password or bind another usable login method before unbinding Google", wantUserID: 42},
		{name: "unexpected failure", id: "42", serviceErr: errors.New("database unavailable"), wantCode: response.CodeInternal, wantMessage: "Failed to update user profile", wantUserID: 42},
	}

	for _, item := range tests {
		t.Run(item.name, func(t *testing.T) {
			unbinder := &oauthIdentityUnbinderStub{googleErr: item.serviceErr}
			handler := &AdminHandler{oauthUnbinder: unbinder}
			router := gin.New()
			RegisterAdminRoutes(router.Group("/admin"), handler)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodDelete, "/admin/users/"+item.id+"/oauth/google", nil)
			request.Header.Set("Accept-Language", "en-US")

			router.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Fatalf("http status = %d, want %d", recorder.Code, http.StatusOK)
			}
			var body response.Response
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.StatusCode != item.wantCode {
				t.Fatalf("status_code = %d, want %d; body=%s", body.StatusCode, item.wantCode, recorder.Body.String())
			}
			if body.Msg != item.wantMessage {
				t.Fatalf("msg = %q, want %q", body.Msg, item.wantMessage)
			}
			if unbinder.googleUserID != item.wantUserID {
				t.Fatalf("UnbindGoogle userID = %d, want %d", unbinder.googleUserID, item.wantUserID)
			}
		})
	}
}
