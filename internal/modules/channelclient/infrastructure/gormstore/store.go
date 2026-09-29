package channelclientstore

import (
	"errors"
	"time"

	channelclientdomain "github.com/dujiao-next/internal/modules/channelclient/domain"

	"gorm.io/gorm"
)

type Store struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (store *Store) Create(client *channelclientdomain.Client) error {
	return store.db.Create(client).Error
}

func (store *Store) FindByID(id uint) (*channelclientdomain.Client, error) {
	return store.first(store.db.Where("id = ?", id))
}

func (store *Store) FindByChannelKey(key string) (*channelclientdomain.Client, error) {
	return store.first(store.db.Where("channel_key = ?", key))
}

func (store *Store) FindActiveByChannelType(channelType string) (*channelclientdomain.Client, error) {
	return store.first(store.db.Where("channel_type = ? AND status = 1", channelType))
}

func (store *Store) FindAll() ([]channelclientdomain.Client, error) {
	var clients []channelclientdomain.Client
	if err := store.db.Where("deleted_at IS NULL").Order("created_at DESC").Find(&clients).Error; err != nil {
		return nil, err
	}
	return clients, nil
}

func (store *Store) Update(client *channelclientdomain.Client) error {
	return store.db.Save(client).Error
}

func (store *Store) UpdateLastUsed(id uint, usedAt time.Time) error {
	return store.db.Model(&channelclientdomain.Client{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("last_used_at", usedAt).Error
}

func (store *Store) Delete(id uint, deletedAt time.Time) error {
	return store.db.Model(&channelclientdomain.Client{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", deletedAt).Error
}

func (store *Store) first(query *gorm.DB) (*channelclientdomain.Client, error) {
	var client channelclientdomain.Client
	if err := query.Where("deleted_at IS NULL").First(&client).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &client, nil
}
