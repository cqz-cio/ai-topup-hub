package integrationtest

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dujiao-next/internal/constants"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	settingstransport "github.com/dujiao-next/internal/modules/settings/transport/http"
	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/gin-gonic/gin"
)

type adminSettingRepository struct {
	store map[string]jsonmap.JSON
}

func newAdminSettingRepository() *adminSettingRepository {
	return &adminSettingRepository{store: make(map[string]jsonmap.JSON)}
}

func (repository *adminSettingRepository) GetByKey(key string) (jsonmap.JSON, bool, error) {
	value, exists := repository.store[key]
	if !exists {
		return nil, false, nil
	}
	return value, true, nil
}

func (repository *adminSettingRepository) Upsert(key string, value jsonmap.JSON) (jsonmap.JSON, error) {
	repository.store[key] = value
	return value, nil
}

func TestUpdateSettingsInvalidatesCallbackRoutesFromRegistryEffect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repository := newAdminSettingRepository()
	settingService := settingsapp.NewService(repository)
	settingService.InvalidateCallbackRoutesCache()
	t.Cleanup(settingService.InvalidateCallbackRoutesCache)

	if _, err := settingService.Update(constants.SettingKeyCallbackRoutesConfig, map[string]interface{}{
		constants.SettingFieldPaymentCallback: "/api/old/callback",
	}); err != nil {
		t.Fatalf("seed callback routes: %v", err)
	}
	if cached := settingService.GetCallbackRoutesCached(); cached == nil || cached.PaymentCallback != "/api/old/callback" {
		t.Fatalf("seed callback route cache mismatch: %#v", cached)
	}

	handler := settingstransport.NewAdminHandler(settingService)
	router := gin.New()
	settingstransport.RegisterAdminRoutes(router, handler)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/settings", bytes.NewBufferString(`{
		"key":"callback_routes_config",
		"value":{"payment_callback":"/api/new/callback"}
	}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("update settings HTTP status want 200 got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if cached := settingService.GetCallbackRoutesCached(); cached == nil || cached.PaymentCallback != "/api/new/callback" {
		t.Fatalf("callback cache was not refreshed after update: %#v", cached)
	}
}
