package app

import (
	"context"
	"github.com/dujiao-next/internal/logger"
	rechargeapp "github.com/dujiao-next/internal/modules/autorecharge/application"
	"time"
)

type autoRechargeWorker struct{ service *rechargeapp.Service }

func (w *autoRechargeWorker) Name() string { return "auto-recharge" }
func (w *autoRechargeWorker) Start(ctx context.Context) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		if err := w.service.RunOnce(ctx); err != nil && ctx.Err() == nil {
			logger.Warnw("auto_recharge_worker_pass_failed")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
func (w *autoRechargeWorker) Stop(context.Context) error { return nil }
