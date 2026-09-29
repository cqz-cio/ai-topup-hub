package settingsapp

import (
	"sync"
	"time"

	"github.com/dujiao-next/internal/constants"
	settingsintegration "github.com/dujiao-next/internal/modules/settings/schema/integration"
)

var callbackRoutesCache struct {
	mu      sync.RWMutex
	routes  *settingsintegration.CallbackRoutesSetting
	loaded  bool
	expires time.Time
}

const callbackRoutesCacheTTL = 5 * time.Minute

// InvalidateCallbackRoutesCache clears the callback route cache after an
// administrator changes its backing setting.
func (s *Service) InvalidateCallbackRoutesCache() {
	callbackRoutesCache.mu.Lock()
	callbackRoutesCache.loaded = false
	callbackRoutesCache.routes = nil
	callbackRoutesCache.expires = time.Time{}
	callbackRoutesCache.mu.Unlock()
}

// GetCallbackRoutesCached returns custom callback routes from the bounded
// settings cache, loading them from the store on a miss.
func (s *Service) GetCallbackRoutesCached() *settingsintegration.CallbackRoutesSetting {
	callbackRoutesCache.mu.RLock()
	if callbackRoutesCache.loaded && time.Now().Before(callbackRoutesCache.expires) {
		routes := callbackRoutesCache.routes
		callbackRoutesCache.mu.RUnlock()
		return routes
	}
	callbackRoutesCache.mu.RUnlock()

	callbackRoutesCache.mu.Lock()
	defer callbackRoutesCache.mu.Unlock()
	if callbackRoutesCache.loaded && time.Now().Before(callbackRoutesCache.expires) {
		return callbackRoutesCache.routes
	}

	routes := s.GetCallbackRoutes()
	callbackRoutesCache.routes = routes
	callbackRoutesCache.loaded = true
	callbackRoutesCache.expires = time.Now().Add(callbackRoutesCacheTTL)
	return routes
}

// GetCallbackRoutes returns configured custom callback routes, or nil when no
// custom route is active.
func (s *Service) GetCallbackRoutes() *settingsintegration.CallbackRoutesSetting {
	if s == nil {
		return nil
	}
	value, err := s.GetByKey(constants.SettingKeyCallbackRoutesConfig)
	if err != nil || value == nil {
		return nil
	}
	setting := settingsintegration.DecodeCallbackRoutesSetting(value)
	if !setting.HasCustomRoutes() {
		return nil
	}
	return &setting
}
