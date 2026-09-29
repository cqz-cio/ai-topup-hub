// Package settingsstore provides an in-memory implementation of the settings
// persistence contract for tests outside the settings bounded context.
package memorysettings

import "github.com/dujiao-next/internal/shared/jsonmap"

// Store keeps settings values in memory.
type Store struct {
	Values map[string]jsonmap.JSON
}

// New creates an empty in-memory settings store.
func New() *Store {
	return &Store{Values: make(map[string]jsonmap.JSON)}
}

// GetByKey implements settings/contract.Store.
func (store *Store) GetByKey(key string) (jsonmap.JSON, bool, error) {
	value, found := store.Values[key]
	return value, found, nil
}

// Upsert implements settings/contract.Store.
func (store *Store) Upsert(key string, value jsonmap.JSON) (jsonmap.JSON, error) {
	store.Values[key] = value
	return value, nil
}
