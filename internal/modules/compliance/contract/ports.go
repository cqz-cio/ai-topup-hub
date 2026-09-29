package contract

import "github.com/dujiao-next/internal/shared/jsonmap"

// SettingsStore 是合规应用层读写保留设置键所需的最小端口。
type SettingsStore interface {
	GetByKey(key string) (value jsonmap.JSON, found bool, err error)
	Upsert(key string, value jsonmap.JSON) (jsonmap.JSON, error)
}
