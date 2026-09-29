package broadcaststore

import (
	"errors"

	broadcastcontract "github.com/dujiao-next/internal/modules/telegram/broadcast/contract"
	broadcastdomain "github.com/dujiao-next/internal/modules/telegram/broadcast/domain"

	"gorm.io/gorm"
)

type Store struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (r *Store) Create(broadcast *broadcastdomain.Broadcast) error {
	if broadcast == nil {
		return nil
	}
	return r.db.Create(broadcast).Error
}

func (r *Store) GetByID(id uint) (*broadcastdomain.Broadcast, error) {
	if id == 0 {
		return nil, nil
	}
	var broadcast broadcastdomain.Broadcast
	if err := r.db.Where("deleted_at IS NULL").First(&broadcast, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &broadcast, nil
}

func (r *Store) List(filter broadcastcontract.ListFilter) ([]broadcastdomain.Broadcast, int64, error) {
	query := r.db.Model(&broadcastdomain.Broadcast{}).Where("deleted_at IS NULL")

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if filter.PageSize > 0 {
		page := filter.Page
		if page < 1 {
			page = 1
		}
		query = query.Offset((page - 1) * filter.PageSize).Limit(filter.PageSize)
	}

	var items []broadcastdomain.Broadcast
	if err := query.Order("created_at DESC").Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *Store) Update(broadcast *broadcastdomain.Broadcast) error {
	if broadcast == nil {
		return nil
	}
	return r.db.Save(broadcast).Error
}
