package contract

import (
	"context"
	"time"

	categoryapp "github.com/dujiao-next/internal/modules/catalog/category/application"
	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"
	settingsintegration "github.com/dujiao-next/internal/modules/settings/schema/integration"
	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
	"github.com/dujiao-next/internal/upstream"
)

// ConnectionProvider 隔离连接读取与上游协议适配器构造。
type ConnectionProvider interface {
	GetByID(id uint) (*siteconnectiondomain.Connection, error)
	GetAdapter(conn *siteconnectiondomain.Connection) (upstream.Adapter, error)
}

// MediaRecorder 记录导入后落盘的上游媒体。
type MediaRecorder interface {
	RecordLocalFile(ctx context.Context, localPath, scene string)
}

// CategoryCreator 自动创建导入商品所需分类。
type CategoryCreator interface {
	Create(input categoryapp.UpsertInput) (*categorydomain.Category, error)
}

// SettingsProvider 提供上游同步动态配置。
type SettingsProvider interface {
	GetUpstreamSyncConfig(fallbackInterval string) (settingsintegration.UpstreamSyncConfig, error)
	GetUpstreamSyncInterval(fallbackInterval string) (time.Duration, error)
}
