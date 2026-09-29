package consumer

import (
	"context"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/dujiao-next/internal/logger"
)

// captureWorkerLogs 把 logger.L 临时替换为 zap/observer 实例,返回观察句柄与还原函数。
// 用于断言 worker recovery 写了哪些结构化字段。
func captureWorkerLogs(t *testing.T) (*observer.ObservedLogs, func()) {
	t.Helper()
	core, observed := observer.New(zap.DebugLevel)
	original := logger.L
	logger.L = zap.New(core)
	return observed, func() { logger.L = original }
}

func TestWithPanicRecovery_ConvertsPanicToError(t *testing.T) {
	wrapped := withPanicRecovery("test.task", func(ctx context.Context, t *asynq.Task) error {
		panic("boom")
	})
	err := wrapped(context.Background(), asynq.NewTask("test.task", nil))
	if err == nil {
		t.Fatalf("expected error from recovered panic")
	}
}

func TestWithPanicRecovery_PassThroughNormalReturn(t *testing.T) {
	want := errors.New("normal error")
	wrapped := withPanicRecovery("test.task", func(ctx context.Context, t *asynq.Task) error {
		return want
	})
	err := wrapped(context.Background(), asynq.NewTask("test.task", nil))
	if !errors.Is(err, want) {
		t.Fatalf("expected pass-through error, got %v", err)
	}
}

func TestWithPanicRecovery_NilReturnStaysNil(t *testing.T) {
	wrapped := withPanicRecovery("test.task", func(ctx context.Context, t *asynq.Task) error {
		return nil
	})
	if err := wrapped(context.Background(), asynq.NewTask("test.task", nil)); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// TestWithPanicRecovery_LogsStructuredFields 验证 panic 触发时,
// zap 日志包含全部结构化字段:task_type / task_id / queue / retry_count / max_retry / error / stack。
// asynq 的 ctx-getter 在 context.Background 上返回 zero value,但字段名必须出现,
// 这样后续重构如果不慎 drop 任何一个字段都会被这个测试拦下来。
func TestWithPanicRecovery_LogsStructuredFields(t *testing.T) {
	observed, restore := captureWorkerLogs(t)
	defer restore()

	wrapped := withPanicRecovery("test.task", func(ctx context.Context, t *asynq.Task) error {
		panic("boom")
	})
	_ = wrapped(context.Background(), asynq.NewTask("test.task", nil))

	entries := observed.FilterMessage("worker_panic_recovered").All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 worker_panic_recovered entry, got %d", len(entries))
	}
	fields := entries[0].ContextMap()
	for _, key := range []string{"task_type", "task_id", "queue", "retry_count", "max_retry", "error", "stack"} {
		if _, ok := fields[key]; !ok {
			t.Errorf("field %q missing from worker_panic_recovered log", key)
		}
	}
	if got, _ := fields["task_type"].(string); got != "test.task" {
		t.Errorf("task_type = %q, want %q", got, "test.task")
	}
}

// TestWithPanicRecovery_PanicValueTypes 覆盖 panic(string/int/error) 三种值类型,
// 都应被转成 error 返回给 asynq 走重试。
func TestWithPanicRecovery_PanicValueTypes(t *testing.T) {
	cases := []struct {
		name       string
		panicValue interface{}
	}{
		{"string panic", "string boom"},
		{"int panic", 42},
		{"error panic", errors.New("typed error boom")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observed, restore := captureWorkerLogs(t)
			defer restore()

			panicValue := tc.panicValue
			wrapped := withPanicRecovery("test.task", func(ctx context.Context, t *asynq.Task) error {
				panic(panicValue)
			})
			err := wrapped(context.Background(), asynq.NewTask("test.task", nil))
			if err == nil {
				t.Fatalf("expected non-nil error from recovered panic")
			}
			if len(observed.FilterMessage("worker_panic_recovered").All()) != 1 {
				t.Fatalf("expected 1 worker_panic_recovered entry, got %d", len(observed.All()))
			}
		})
	}
}
