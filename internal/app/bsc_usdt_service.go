package app

import (
	"context"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/modules/payment/application/bscusdt"
	"time"
)

type bscUSDTWorker struct {
	service *bscusdt.Service
	network string
}

func (w *bscUSDTWorker) Name() string { return w.network + "-usdt-listener" }
func (w *bscUSDTWorker) Start(ctx context.Context) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		if err := w.service.RunOnce(ctx); err != nil && ctx.Err() == nil {
			logger.Warnw("chain_usdt_listener_pass_failed", "network", w.network, "error", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
func (w *bscUSDTWorker) Stop(context.Context) error { return nil }
