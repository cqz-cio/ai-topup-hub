package settingshttp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"
)

type googleAuthAdminStub struct {
	setting settingssecurity.GoogleAuthSetting
	patch   settingssecurity.GoogleAuthSettingPatch
	applied settingssecurity.GoogleAuthSetting
}

func (s *googleAuthAdminStub) GetGoogleAuthSetting() (settingssecurity.GoogleAuthSetting, error) {
	return s.setting, nil
}

func (s *googleAuthAdminStub) PatchGoogleAuthSetting(patch settingssecurity.GoogleAuthSettingPatch) (settingssecurity.GoogleAuthSetting, error) {
	s.patch = patch
	if patch.Enabled != nil {
		s.setting.Enabled = *patch.Enabled
	}
	if patch.ClientID != nil {
		s.setting.ClientID = strings.TrimSpace(*patch.ClientID)
	}
	return s.setting, nil
}

func (s *googleAuthAdminStub) ApplyRuntime(setting settingssecurity.GoogleAuthSetting) {
	s.applied = setting
}

func TestGoogleAuthHandlerUpdateAppliesRuntimeSetting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &googleAuthAdminStub{}
	handler := NewGoogleAuthHandler(stub)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/v1/admin/settings/google-auth",
		strings.NewReader(`{"enabled":true,"client_id":" client.apps.googleusercontent.com "}`),
	)
	context.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateGoogleAuth(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if stub.patch.Enabled == nil || !*stub.patch.Enabled || stub.patch.ClientID == nil {
		t.Fatalf("unexpected patch: %#v", stub.patch)
	}
	if !stub.applied.Enabled || stub.applied.ClientID != "client.apps.googleusercontent.com" {
		t.Fatalf("runtime setting = %#v", stub.applied)
	}
	if !strings.Contains(recorder.Body.String(), `"client_id":"client.apps.googleusercontent.com"`) {
		t.Fatalf("response body = %s", recorder.Body.String())
	}
}
