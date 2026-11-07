package queue

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/hibiken/asynq"
)

const TaskBalanceRecoveryPoll = "supplier_balance:poll"
const TaskBalanceRecoveryEmail = "supplier_balance:email"

type BalanceRecoveryEmailPayload struct {
	WaitID uint `json:"wait_id"`
}

func (c *Client) EnqueueBalanceRecoveryEmail(waitID uint) error {
	if !c.Enabled() {
		return errors.New("notification queue unavailable")
	}
	data, err := json.Marshal(BalanceRecoveryEmailPayload{WaitID: waitID})
	if err != nil {
		return err
	}
	_, err = c.client.Enqueue(asynq.NewTask(TaskBalanceRecoveryEmail, data), asynq.Queue(NotificationQueue), asynq.Unique(50*time.Second), asynq.Timeout(45*time.Second), asynq.MaxRetry(3))
	if errors.Is(err, asynq.ErrDuplicateTask) {
		return nil
	}
	return err
}
