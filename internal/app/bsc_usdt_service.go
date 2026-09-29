package app

import (
	"context"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/modules/payment/application/bscusdt"
	"time"
)

type bscUSDTWorker struct{ service *bscusdt.Service }

func (w *bscUSDTWorker) Name() string { return "bsc-usdt-listener" }
func (w *bscUSDTWorker) Start(ctx context.Context) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		if err := w.service.RunOnce(ctx); err != nil && ctx.Err() == nil {
			logger.Warnw("bsc_usdt_listener_pass_failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
func (w *bscUSDTWorker) Stop(context.Context) error { return nil }
