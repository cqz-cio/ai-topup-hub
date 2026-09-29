package queueadapter

import (
	reconciliationcontract "github.com/dujiao-next/internal/modules/reconciliation/contract"
	"github.com/dujiao-next/internal/queue"

	"github.com/hibiken/asynq"
)

type Client interface {
	EnqueueReconciliationRun(payload queue.ReconciliationRunPayload, opts ...asynq.Option) error
}

type Enqueuer struct {
	client Client
}

var _ reconciliationcontract.Enqueuer = (*Enqueuer)(nil)

func New(client Client) reconciliationcontract.Enqueuer {
	if client == nil {
		return nil
	}
	return &Enqueuer{client: client}
}

func (e *Enqueuer) Enqueue(jobID uint) error {
	return e.client.EnqueueReconciliationRun(queue.ReconciliationRunPayload{JobID: jobID})
}
