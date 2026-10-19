package jobs

import (
	"context"
	"errors"
	"time"

	jobconsumer "github.com/dujiao-next/internal/app/jobs/consumer"
	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/logger"
	settingsintegration "github.com/dujiao-next/internal/modules/settings/schema/integration"
	"github.com/dujiao-next/internal/queue"

	"github.com/hibiken/asynq"
)

// Service 异步队列服务
type Service struct {
	name               string
	server             *asynq.Server
	mux                *asynq.ServeMux
	scheduler          *asynq.Scheduler
	consumer           *jobconsumer.Consumer
	notificationServer *asynq.Server
	notificationMux    *asynq.ServeMux
}

// NewService 创建异步队列服务
func NewService(cfg *config.QueueConfig, consumer *jobconsumer.Consumer) (*Service, error) {
	return newService(cfg, consumer, "worker")
}

// NewCheckoutService only processes payment and local order maintenance jobs.
// It never registers procurement, upstream polling or recurring jobs.
func NewCheckoutService(cfg *config.QueueConfig, consumer *jobconsumer.Consumer) (*Service, error) {
	return newService(cfg, consumer, "checkout")
}

// NewStorefrontService keeps inventory synchronization independent while
// processing customer payments, procurement, delivery and recovery.
func NewStorefrontService(cfg *config.QueueConfig, consumer *jobconsumer.Consumer) (*Service, error) {
	return newService(cfg, consumer, "storefront")
}

func newService(cfg *config.QueueConfig, consumer *jobconsumer.Consumer, profile string) (service *Service, err error) {
	if cfg == nil || !cfg.Enabled {
		return nil, errors.New("queue disabled")
	}
	if consumer == nil {
		return nil, errors.New("consumer is nil")
	}
	defer func() {
		if service != nil && err == nil {
			mailOpt, mailCfg := queue.BuildNotificationServerConfig(cfg)
			service.notificationServer = asynq.NewServer(mailOpt, mailCfg)
			service.notificationMux = asynq.NewServeMux()
			consumer.RegisterNotifications(service.notificationMux)
		}
	}()
	opt, serverCfg := queue.BuildServerConfig(cfg)
	server := asynq.NewServer(opt, serverCfg)
	mux := asynq.NewServeMux()
	if profile == "checkout" {
		consumer.RegisterCheckout(mux)
		return &Service{name: "checkout-worker", server: server, mux: mux, consumer: consumer}, nil
	}
	if profile == "storefront" {
		consumer.RegisterStorefront(mux)
		scheduler := asynq.NewScheduler(opt, nil)
		registerProcurementRecovery(scheduler, consumer)
		registerSettlementTasks(scheduler, consumer)
		return &Service{name: "storefront-worker", server: server, mux: mux, scheduler: scheduler, consumer: consumer}, nil
	}
	consumer.Register(mux)

	scheduler := asynq.NewScheduler(opt, nil)
	registerPeriodicTasks(scheduler, consumer, cfg)
	registerProcurementRecovery(scheduler, consumer)

	return &Service{
		name:      "worker",
		server:    server,
		mux:       mux,
		scheduler: scheduler,
		consumer:  consumer,
	}, nil
}

func registerProcurementRecovery(scheduler *asynq.Scheduler, consumer *jobconsumer.Consumer) {
	if consumer.ProcurementOrderService == nil {
		return
	}
	if _, err := scheduler.Register("@every 1m", asynq.NewTask(queue.TaskProcurementRecover, nil), asynq.Queue(queue.DefaultQueue), asynq.Unique(50*time.Second)); err != nil {
		logger.Warnw("scheduler_register_procurement_recovery_failed", "error", err)
	}
}

// registerPeriodicTasks 注册所有周期性任务
func registerPeriodicTasks(scheduler *asynq.Scheduler, consumer *jobconsumer.Consumer, cfg *config.QueueConfig) {
	if scheduler == nil || consumer == nil {
		return
	}
	registerSettlementTasks(scheduler, consumer)
	registerOperationalTasks(scheduler, consumer, cfg)
}

// Settlement is independent of stock synchronization and does not purchase,
// send broadcasts, or enable optional notification scenes.
func registerSettlementTasks(scheduler *asynq.Scheduler, consumer *jobconsumer.Consumer) {
	if consumer.AffiliateService != nil {
		task := queue.NewAffiliateConfirmCommissionsTask()
		entryID, err := scheduler.Register("@every 1m", task, asynq.Queue(queue.DefaultQueue), asynq.Unique(50*time.Second))
		if err != nil {
			logger.Warnw("scheduler_register_affiliate_confirm_failed", "error", err)
		} else {
			logger.Infow("scheduler_register_affiliate_confirm_ok", "entry_id", entryID)
		}
	}
	if consumer.ResellerAccountingLedger != nil {
		task := queue.NewResellerConfirmLedgerTask()
		entryID, err := scheduler.Register("@every 1m", task, asynq.Queue(queue.DefaultQueue), asynq.Unique(50*time.Second))
		if err != nil {
			logger.Warnw("scheduler_register_reseller_confirm_ledger_failed", "error", err)
		} else {
			logger.Infow("scheduler_register_reseller_confirm_ledger_ok", "entry_id", entryID)
		}
	}
}

func registerOperationalTasks(scheduler *asynq.Scheduler, consumer *jobconsumer.Consumer, cfg *config.QueueConfig) {
	if consumer.ProductMappingService != nil {
		fallbackInterval := "5m"
		if cfg != nil && cfg.UpstreamSyncInterval != "" {
			fallbackInterval = cfg.UpstreamSyncInterval
		}
		// 优先读取后台动态设置；失败则回落到 config.yml 的兜底值
		syncInterval := fallbackInterval
		if consumer.SettingService != nil {
			d, err := consumer.SettingService.GetUpstreamSyncInterval(fallbackInterval)
			if err != nil {
				logger.Warnw("scheduler_load_upstream_sync_interval_failed", "error", err, "fallback", fallbackInterval)
			} else if d > 0 {
				syncInterval = settingsintegration.FormatUpstreamSyncIntervalForScheduler(d)
			}
		}
		task := queue.NewUpstreamSyncStockTask()
		entryID, err := scheduler.Register("@every "+syncInterval, task, asynq.Queue(queue.DefaultQueue))
		if err != nil {
			logger.Warnw("scheduler_register_upstream_sync_stock_failed", "error", err)
		} else {
			logger.Infow("scheduler_register_upstream_sync_stock_ok", "entry_id", entryID, "interval", syncInterval)
		}
	}
	if consumer.NotificationService != nil {
		task, err := queue.NewNotificationInventoryAlertCheckTask()
		if err != nil {
			logger.Warnw("scheduler_register_inventory_alert_check_failed", "error", err)
		} else {
			entryID, registerErr := scheduler.Register("@every 1m", task, asynq.Queue(queue.DefaultQueue))
			if registerErr != nil {
				logger.Warnw("scheduler_register_inventory_alert_check_failed", "error", registerErr)
			} else {
				logger.Infow("scheduler_register_inventory_alert_check_ok", "entry_id", entryID)
			}
		}
	}
	if consumer.ProcurementOrderService != nil {
		task := queue.NewProcurementSyncAcceptedTask()
		entryID, err := scheduler.Register("@every 30m", task, asynq.Queue(queue.DefaultQueue))
		if err != nil {
			logger.Warnw("scheduler_register_procurement_sync_accepted_failed", "error", err)
		} else {
			logger.Infow("scheduler_register_procurement_sync_accepted_ok", "entry_id", entryID)
		}
	}
}

// Name 服务名称
func (s *Service) Name() string {
	if s == nil || s.name == "" {
		return "worker"
	}
	return s.name
}

// Start 启动服务
func (s *Service) Start(ctx context.Context) error {
	if s == nil || s.server == nil || s.mux == nil {
		return errors.New("worker not initialized")
	}
	if s.notificationServer != nil {
		if err := s.notificationServer.Start(s.notificationMux); err != nil {
			return err
		}
		defer s.notificationServer.Shutdown()
	}
	if s.scheduler != nil {
		if err := s.scheduler.Start(); err != nil {
			logger.Warnw("scheduler_start_failed", "error", err)
		}
	}
	return s.server.Run(s.mux)
}

// Stop 停止服务
func (s *Service) Stop(ctx context.Context) error {
	if s == nil || s.server == nil {
		return nil
	}
	_ = ctx
	if s.scheduler != nil {
		s.scheduler.Shutdown()
	}
	s.server.Shutdown()
	if s.notificationServer != nil {
		s.notificationServer.Shutdown()
	}
	return nil
}
