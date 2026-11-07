package consumer

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/dujiao-next/internal/queue"
	"github.com/hibiken/asynq"
)

func (c *Consumer) handleBalanceRecoveryPoll(ctx context.Context, _ *asynq.Task) error {
	if c.BalanceRecoveryService == nil {
		return errors.New("balance recovery unavailable")
	}
	return c.BalanceRecoveryService.Poll(ctx)
}

func (c *Consumer) handleBalanceRecoveryEmail(ctx context.Context, task *asynq.Task) error {
	var payload queue.BalanceRecoveryEmailPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return errors.Join(asynq.SkipRetry, err)
	}
	if payload.WaitID == 0 {
		return asynq.SkipRetry
	}
	if c.BalanceRecoveryService == nil {
		return errors.New("balance recovery unavailable")
	}
	return c.BalanceRecoveryService.Send(ctx, payload.WaitID)
}
