package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	settingsintegration "github.com/dujiao-next/internal/modules/settings/schema/integration"
	"github.com/gin-gonic/gin"
)

func TestBodyLimitRejectsKnownAndChunkedOversizeBeforeHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, unknown := range []bool{false, true} {
		called := false
		router := gin.New()
		router.POST("/auth", BodyLimit(16), func(c *gin.Context) { called = true; c.Status(200) })
		req := httptest.NewRequest("POST", "/auth", strings.NewReader(strings.Repeat("x", 17)))
		if unknown {
			req.ContentLength = -1
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != 413 || called {
			t.Fatalf("unknown=%v status=%d called=%v", unknown, rec.Code, called)
		}
	}
}

func TestAccountLimitSurvivesIPRotationAndIPLimitSurvivesEmailRotation(t *testing.T) {
	for _, byAccount := range []bool{true, false} {
		router := gin.New()
		key := KeyByIP
		if byAccount {
			key = KeyByJSONField("email")
		}
		router.POST("/login", RateLimitMiddleware(nil, RateLimitRule{WindowSeconds: 60, MaxRequests: 1}, key), func(c *gin.Context) { c.Status(200) })
		for i := 0; i < 2; i++ {
			email, ip := "buyer@example.com", "192.0.2.1:1"
			if i == 1 {
				if byAccount {
					ip = "192.0.2.2:1"
					email = " BUYER@EXAMPLE.COM "
				} else {
					email = "other@example.com"
				}
			}
			req := httptest.NewRequest("POST", "/login", strings.NewReader(`{"email":"`+email+`"}`))
			req.RemoteAddr = ip
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			want := 200
			if i == 1 {
				want = 429
			}
			if rec.Code != want {
				t.Fatalf("byAccount=%v status=%d want=%d", byAccount, rec.Code, want)
			}
			if i == 1 && rec.Header().Get("Retry-After") == "" {
				t.Fatal("missing Retry-After")
			}
		}
	}
}

func TestJSONKeyExtractionIsBoundedEvenWithoutBodyMiddleware(t *testing.T) {
	router := gin.New()
	router.POST("/login", RateLimitMiddleware(nil, RateLimitRule{WindowSeconds: 60, MaxRequests: 1}, KeyByJSONField("email")), func(c *gin.Context) { c.Status(200) })
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest("POST", "/login", strings.NewReader(strings.Repeat("x", (64<<10)+1))))
	if rec.Code != 413 {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestConcurrencyRejectionAndSlotRelease(t *testing.T) {
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	router := gin.New()
	router.GET("/slow", ConcurrencyLimit(1), func(c *gin.Context) { close(entered); <-release; c.Status(200) })
	go func() {
		defer close(done)
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/slow", nil))
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest("GET", "/slow", nil))
	close(release)
	<-done
	if rec.Code != 503 || rec.Header().Get("Retry-After") != "1" {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestUserMutationQuotaDoesNotThrottleReads(t *testing.T) {
	router := gin.New()
	router.Use(MutationsOnly(RateLimitMiddleware(nil, RateLimitRule{WindowSeconds: 60, MaxRequests: 1}, KeyByUserID)))
	router.Any("/me", func(c *gin.Context) { c.Status(http.StatusOK) })
	for _, tc := range []struct {
		method string
		status int
	}{{"POST", 200}, {"POST", 429}, {"GET", 200}, {"GET", 200}} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(tc.method, "/me", nil))
		if rec.Code != tc.status {
			t.Fatalf("%s: %d", tc.method, rec.Code)
		}
	}
}

type callbackSettingsStub struct{}

func (callbackSettingsStub) GetCallbackRoutesCached() *settingsintegration.CallbackRoutesSetting {
	return &settingsintegration.CallbackRoutesSetting{PaymentCallback: "/api/custom-payment"}
}

func TestCustomCallbackCannotBypassBodyAndRateGuards(t *testing.T) {
	router := gin.New()
	router.Use(CallbackRouteMiddleware(callbackSettingsStub{}, nil, nil, nil,
		RateLimitMiddleware(nil, RateLimitRule{WindowSeconds: 60, MaxRequests: 1}, KeyByIP)))
	// These guarded requests must be rejected before the nil payment handler.
	oversized := httptest.NewRecorder()
	router.ServeHTTP(oversized, httptest.NewRequest("POST", "/api/custom-payment", strings.NewReader(strings.Repeat("x", (4<<20)+1))))
	if oversized.Code != 413 {
		t.Fatalf("custom callback body status=%d", oversized.Code)
	}
	limited := httptest.NewRecorder()
	router.ServeHTTP(limited, httptest.NewRequest("POST", "/api/custom-payment", nil))
	if limited.Code != 429 {
		t.Fatalf("custom callback rate status=%d", limited.Code)
	}
	other := httptest.NewRecorder()
	router.GET("/api/read", func(c *gin.Context) { c.Status(200) })
	router.ServeHTTP(other, httptest.NewRequest("GET", "/api/read", nil))
	if other.Code != 200 {
		t.Fatalf("callback quota leaked into unrelated API: %d", other.Code)
	}
}
