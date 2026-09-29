package queueadapter

import (
	"time"

	downstreamcontract "github.com/dujiao-next/internal/modules/downstreamcallback/contract"
	"github.com/dujiao-next/internal/queue"

	"github.com/hibiken/asynq"
)

// Adapter 将领域队列端口映射到全局任务客户端。
type Adapter struct {
	client *queue.Client
}

var _ downstreamcontract.CallbackQueue = (*Adapter)(nil)

func New(client *queue.Client) *Adapter {
	if client == nil {
		panic("downstream callback queue adapter: client is nil")
	}
	return &Adapter{client: client}
}

func (a *Adapter) EnqueueCallback(refID uint, delay time.Duration) error {
	options := make([]asynq.Option, 0, 1)
	if delay > 0 {
		options = append(options, asynq.ProcessIn(delay))
	}
	return a.client.EnqueueDownstreamCallback(queue.DownstreamCallbackPayload{
		DownstreamOrderRefID: refID,
	}, options...)
}
