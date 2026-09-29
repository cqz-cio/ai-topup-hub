package gormstore

import (
	"errors"
	"time"

	cardsecretcontract "github.com/dujiao-next/internal/modules/cardsecret/contract"
	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"

	"gorm.io/gorm"
)

// BatchStore 是卡密批次端口的 GORM 实现。
type BatchStore struct {
	db *gorm.DB
}

var _ cardsecretcontract.BatchRepository = (*BatchStore)(nil)

func NewBatch(db *gorm.DB) *BatchStore {
	return &BatchStore{db: db}
}

// BindTx 将批次端口绑定到调用方事务，不暴露具体 BatchStore 类型。
func (r *BatchStore) BindTx(tx *gorm.DB) cardsecretcontract.BatchRepository {
	if tx == nil {
		return r
	}
	return &BatchStore{db: tx}
}

// Create 创建批次
func (r *BatchStore) Create(batch *cardsecretdomain.Batch) error {
	if batch == nil {
		return errors.New("batch is nil")
	}
	return r.db.Create(batch).Error
}

// GetByID 获取批次
func (r *BatchStore) GetByID(id uint) (*cardsecretdomain.Batch, error) {
	if id == 0 {
		return nil, errors.New("invalid batch id")
	}
	var batch cardsecretdomain.Batch
	if err := r.db.Where("deleted_at IS NULL").First(&batch, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &batch, nil
}

// ListByProduct 按商品获取批次列表
func (r *BatchStore) ListByProduct(productID, skuID uint, page, pageSize int) ([]cardsecretdomain.Batch, int64, error) {
	if productID == 0 {
		return nil, 0, errors.New("invalid product id")
	}
	query := r.db.Model(&cardsecretdomain.Batch{}).Where("product_id = ? AND deleted_at IS NULL", productID)
	if skuID > 0 {
		query = query.Where("sku_id = ?", skuID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if pageSize > 0 {
		offset := (page - 1) * pageSize
		query = query.Limit(pageSize).Offset(offset)
	}

	var items []cardsecretdomain.Batch
	if err := query.Order("id desc").Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// DeleteByProduct 删除指定商品下的所有卡密批次
func (r *BatchStore) DeleteByProduct(productID uint) error {
	if productID == 0 {
		return errors.New("invalid product id")
	}
	now := time.Now()
	return r.db.Model(&cardsecretdomain.Batch{}).
		Where("product_id = ? AND deleted_at IS NULL", productID).
		Updates(map[string]interface{}{"deleted_at": now, "updated_at": now}).Error
}
