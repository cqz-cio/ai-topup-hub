package consumer

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"

	"github.com/dujiao-next/internal/logger"
)

type taskHandler func(context.Context, *asynq.Task) error

// withPanicRecovery 把 asynq HandlerFunc 包装成具备 panic 恢复能力的版本。
//
// 注意:asynq processor 自身已经会 recover panic 并将其转为 PanicError 走重试,
// 但 asynq 内部 recover 不输出结构化 task_id / task_type 标签。本 wrapper 在
// asynq 之上增加 zap 结构化日志(defer 后进先出,我们的 recover 先执行,
// asynq 那一层收到的就是 nil panic + 我们返回的 error)。
//
// 行为:
//   - panic 被恢复后转换为 error 返回给 asynq(asynq 会按配置重试)
//   - zap 结构化日志记录 task_type / task_id / retry_count / queue / stack,
//     运维可通过 logs/app.log 的 worker_panic_recovered 日志做告警
func withPanicRecovery(taskType string, fn taskHandler) taskHandler {
	return func(ctx context.Context, t *asynq.Task) (retErr error) {
		defer func() {
			r := recover()
			if r == nil {
				return
			}
			stack := debug.Stack()
			err := fmt.Errorf("worker panic in %s: %v (type=%T)", taskType, r, r)

			taskID, _ := asynq.GetTaskID(ctx)
			retryCount, _ := asynq.GetRetryCount(ctx)
			maxRetry, _ := asynq.GetMaxRetry(ctx)
			queueName, _ := asynq.GetQueueName(ctx)

			logger.Z().Error("worker_panic_recovered",
				zap.String("task_type", taskType),
				zap.String("task_id", taskID),
				zap.String("queue", queueName),
				zap.Int("retry_count", retryCount),
				zap.Int("max_retry", maxRetry),
				zap.Error(err),
				zap.ByteString("stack", stack),
			)
			retErr = err
		}()
		return fn(ctx, t)
	}
}
