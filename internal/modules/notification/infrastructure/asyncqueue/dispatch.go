package asyncqueue

import (
	"github.com/dujiao-next/internal/modules/notification/contract"
	"github.com/dujiao-next/internal/queue"

	"github.com/hibiken/asynq"
)

type Client struct {
	queue *queue.Client
}

func New(client *queue.Client) *Client {
	return &Client{queue: client}
}

func (c *Client) EnqueueNotificationDispatch(payload queue.NotificationDispatchPayload, maxRetry int) error {
	if c == nil || c.queue == nil {
		return nil
	}
	if maxRetry < 0 {
		maxRetry = 0
	}
	return c.queue.EnqueueNotificationDispatch(payload, asynq.MaxRetry(maxRetry))
}

var _ contract.DispatchQueue = (*Client)(nil)
