package queueadapter

import (
	"strings"
	"time"

	ordercontract "github.com/dujiao-next/internal/modules/order/contract"

	"github.com/dujiao-next/internal/queue"
)

// Queue 把平台异步队列适配为订单应用端口。
type Queue struct {
	client *queue.Client
}

var _ ordercontract.Queue = (*Queue)(nil)

func New(client *queue.Client) *Queue {
	return &Queue{client: client}
}

func (q *Queue) Enabled() bool {
	return q != nil && q.client != nil && q.client.Enabled()
}

func (q *Queue) EnqueueTimeoutCancel(orderID uint, delay time.Duration) error {
	if q == nil || q.client == nil {
		return nil
	}
	return q.client.EnqueueOrderTimeoutCancel(queue.OrderTimeoutCancelPayload{OrderID: orderID}, delay)
}

func (q *Queue) EnqueueStatusEmail(orderID uint, status string) error {
	if q == nil || q.client == nil {
		return nil
	}
	return q.client.EnqueueOrderStatusEmail(queue.OrderStatusEmailPayload{
		OrderID: orderID,
		Status:  strings.TrimSpace(status),
	})
}
