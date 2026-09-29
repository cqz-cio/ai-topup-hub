package middleware

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"syscall"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/dujiao-next/internal/logger"
)

// captureLogs 把 logger.L 临时替换为 zap/observer 实例,返回观察句柄与还原函数。
// 这样可以断言 RecoveryMiddleware 究竟写了哪些结构化字段,而不是只测响应码。
func captureLogs(t *testing.T) (*observer.ObservedLogs, func()) {
	t.Helper()
	core, observed := observer.New(zap.DebugLevel)
	original := logger.L
	logger.L = zap.New(core)
	return observed, func() { logger.L = original }
}

func TestRecoveryMiddleware_RecoversPanicAndLogsStructured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	observed, restore := captureLogs(t)
	defer restore()

	r := gin.New()
	r.Use(RequestIDMiddleware())
	r.Use(RecoveryMiddleware())
	r.GET("/boom", func(c *gin.Context) {
		panic("simulated panic")
	})

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	req.Header.Set(requestIDHeader, "req-test-001")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if w.Body.Len() == 0 {
		t.Fatalf("expected non-empty 500 body for panic")
	}

	entries := observed.FilterMessage("panic_recovered").All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 panic_recovered entry, got %d", len(entries))
	}
	fields := entries[0].ContextMap()
	if got, _ := fields["request_id"].(string); got != "req-test-001" {
		t.Errorf("request_id field = %q, want %q", got, "req-test-001")
	}
	if got, _ := fields["method"].(string); got != http.MethodGet {
		t.Errorf("method field = %q, want %q", got, http.MethodGet)
	}
	if got, _ := fields["path"].(string); got != "/boom" {
		t.Errorf("path field = %q, want %q", got, "/boom")
	}
	if _, ok := fields["stack"]; !ok {
		t.Errorf("stack field missing")
	}
	if _, ok := fields["client_ip"]; !ok {
		t.Errorf("client_ip field missing")
	}
	if _, ok := fields["error"]; !ok {
		t.Errorf("error field missing")
	}
}

func TestRecoveryMiddleware_PassThroughNormalRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RecoveryMiddleware())
	r.GET("/ok", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// TestRecoveryMiddleware_BrokenPipeNeverWrites500 覆盖 4 种客户端断开 panic:
// http.ErrAbortHandler / syscall.EPIPE / syscall.ECONNRESET / *net.OpError(包了 EPIPE)。
// 这些都必须走 warn 路径,绝不写 500 body——给死 socket 写 500 是浪费,且日志告警会失真。
func TestRecoveryMiddleware_BrokenPipeNeverWrites500(t *testing.T) {
	cases := []struct {
		name       string
		panicValue interface{}
	}{
		{"http.ErrAbortHandler", http.ErrAbortHandler},
		{"syscall.EPIPE", syscall.EPIPE},
		{"syscall.ECONNRESET", syscall.ECONNRESET},
		{"wrapped EPIPE via net.OpError", &net.OpError{Op: "write", Net: "tcp", Err: syscall.EPIPE}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			observed, restore := captureLogs(t)
			defer restore()

			r := gin.New()
			r.Use(RequestIDMiddleware())
			r.Use(RecoveryMiddleware())
			panicValue := tc.panicValue
			r.GET("/abort", func(c *gin.Context) {
				panic(panicValue)
			})

			req := httptest.NewRequest(http.MethodGet, "/abort", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code == http.StatusInternalServerError {
				t.Fatalf("broken-pipe should not return 500, got status=%d body=%q", w.Code, w.Body.String())
			}

			warns := observed.FilterMessage("client_disconnected_panic").All()
			if len(warns) != 1 {
				t.Fatalf("expected 1 client_disconnected_panic entry, got %d", len(warns))
			}
			if errs := observed.FilterMessage("panic_recovered").All(); len(errs) != 0 {
				t.Fatalf("broken-pipe should not log panic_recovered, got %d entries", len(errs))
			}
		})
	}
}

// TestRecoveryMiddleware_PanicValueTypes 覆盖 panic(string/int/error) 三种值类型,
// 验证 normalizePanic 都能转成可日志化的 error,且都走 error 路径。
func TestRecoveryMiddleware_PanicValueTypes(t *testing.T) {
	cases := []struct {
		name       string
		panicValue interface{}
	}{
		{"string panic", "simulated panic"},
		{"int panic", 42},
		{"error panic", errors.New("typed error panic")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			observed, restore := captureLogs(t)
			defer restore()

			r := gin.New()
			r.Use(RequestIDMiddleware())
			r.Use(RecoveryMiddleware())
			panicValue := tc.panicValue
			r.GET("/boom", func(c *gin.Context) {
				panic(panicValue)
			})

			req := httptest.NewRequest(http.MethodGet, "/boom", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusInternalServerError {
				t.Fatalf("expected 500, got %d", w.Code)
			}
			if len(observed.FilterMessage("panic_recovered").All()) != 1 {
				t.Fatalf("expected 1 panic_recovered entry, got %d", len(observed.All()))
			}
		})
	}
}
