package gormstore

import (
	reconciliationcontract "github.com/dujiao-next/internal/modules/reconciliation/contract"
	reconciliationdomain "github.com/dujiao-next/internal/modules/reconciliation/domain"

	"gorm.io/gorm"
)

type ItemStore struct {
	db *gorm.DB
}

var _ reconciliationcontract.ItemRepository = (*ItemStore)(nil)

func NewItemStore(db *gorm.DB) *ItemStore { return &ItemStore{db: db} }

func (s *ItemStore) BatchCreate(items []reconciliationdomain.Item) error {
	if len(items) == 0 {
		return nil
	}
	return s.db.Create(&items).Error
}

func (s *ItemStore) GetByID(id uint) (*reconciliationdomain.Item, error) {
	var item reconciliationdomain.Item
	if err := s.db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *ItemStore) Update(item *reconciliationdomain.Item) error {
	return s.db.Save(item).Error
}

func (s *ItemStore) ListByJobID(jobID uint, page, pageSize int) ([]reconciliationdomain.Item, int64, error) {
	var items []reconciliationdomain.Item
	var total int64
	query := s.db.Model(&reconciliationdomain.Item{}).Where("job_id = ?", jobID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("id ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
