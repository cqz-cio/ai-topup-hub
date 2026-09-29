package app

import (
	"os"
	"time"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/logger"

	"go.uber.org/zap"
)

const (
	ModeAll    = "all"
	ModeAPI    = "api"
	ModeWorker = "worker"
)

// Options 应用启动选项
type Options struct {
	Config          *config.Config
	Logger          *zap.SugaredLogger
	Signals         []os.Signal
	ShutdownTimeout time.Duration
	Mode            string
}

// normalizeOptions 补齐默认参数
func normalizeOptions(opts Options) Options {
	if opts.Logger == nil {
		opts.Logger = logger.S()
	}
	if opts.ShutdownTimeout <= 0 {
		opts.ShutdownTimeout = 10 * time.Second
	}
	if opts.Mode == "" {
		opts.Mode = ModeAll
	}
	return opts
}
