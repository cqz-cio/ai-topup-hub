package contract

import "github.com/dujiao-next/internal/shared/jsonmap"

// Store is the persistence contract shared by settings use cases and the few
// modules that own reserved settings keys. It exposes values rather than GORM
// records so callers do not depend on persistence models.
type Store interface {
	GetByKey(key string) (value jsonmap.JSON, found bool, err error)
	Upsert(key string, value jsonmap.JSON) (jsonmap.JSON, error)
}
