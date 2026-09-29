package settingsstore

import (
	"errors"

	"github.com/dujiao-next/internal/shared/jsonmap"
	"gorm.io/gorm"
)

// SettingRecord is the persistence representation of a settings row.
type SettingRecord struct {
	Key       string       `gorm:"primarykey" json:"key"`
	ValueJSON jsonmap.JSON `gorm:"type:json" json:"value"`
}

// TableName preserves the existing database contract.
func (SettingRecord) TableName() string {
	return "settings"
}

// Store persists settings through GORM without exposing GORM records to the
// application layer.
type Store struct {
	db *gorm.DB
}

// New creates a settings store.
func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

// GetByKey reads a setting value.
func (store *Store) GetByKey(key string) (jsonmap.JSON, bool, error) {
	var record SettingRecord
	if err := store.db.Where("key = ?", key).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return record.ValueJSON, true, nil
}

// Upsert creates or updates a setting value.
func (store *Store) Upsert(key string, value jsonmap.JSON) (jsonmap.JSON, error) {
	var record SettingRecord
	err := store.db.Where("key = ?", key).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		record = SettingRecord{Key: key, ValueJSON: value}
		if err := store.db.Create(&record).Error; err != nil {
			return nil, err
		}
		return record.ValueJSON, nil
	}
	if err != nil {
		return nil, err
	}

	record.ValueJSON = value
	if err := store.db.Save(&record).Error; err != nil {
		return nil, err
	}
	return record.ValueJSON, nil
}
