package gormstore

import (
	"errors"
	"strings"
	"time"

	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/dujiao-next/internal/persistence/gormutil"

	"gorm.io/gorm"
)

// SKUStore 是 Catalog Product SKU 端口的 GORM 实现。
type SKUStore struct {
	db *gorm.DB
}

var _ productcontract.SKURepository = (*SKUStore)(nil)

func NewSKUStore(db *gorm.DB) *SKUStore {
	return &SKUStore{db: db}
}

// BindTx 将 Store 绑定到调用方事务，并仅暴露 SKU 端口。
func (r *SKUStore) BindTx(tx *gorm.DB) productcontract.SKURepository {
	if tx == nil {
		return r
	}
	return NewSKUStore(tx)
}

// ListByProduct 根据商品获取 SKU 列表
func (r *SKUStore) ListByProduct(productID uint, onlyActive bool) ([]productdomain.ProductSKU, error) {
	if productID == 0 {
		return nil, errors.New("invalid product id")
	}
	query := r.db.Model(&productdomain.ProductSKU{}).Where("deleted_at IS NULL AND product_id = ?", productID)
	if onlyActive {
		query = query.Where("is_active = ?", true)
	}
	var items []productdomain.ProductSKU
	if err := query.Order("sort_order DESC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// GetByID 根据 ID 获取 SKU
func (r *SKUStore) GetByID(id uint) (*productdomain.ProductSKU, error) {
	if id == 0 {
		return nil, errors.New("invalid sku id")
	}
	var item productdomain.ProductSKU
	if err := r.db.Where("deleted_at IS NULL").First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

// GetByProductAndCode 按商品和编码获取 SKU
func (r *SKUStore) GetByProductAndCode(productID uint, skuCode string) (*productdomain.ProductSKU, error) {
	if productID == 0 {
		return nil, errors.New("invalid product id")
	}
	code := strings.TrimSpace(skuCode)
	if code == "" {
		return nil, errors.New("invalid sku code")
	}

	var item productdomain.ProductSKU
	if err := r.db.Where("deleted_at IS NULL AND product_id = ? AND sku_code = ?", productID, code).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

// ListByIDs 批量获取 SKU
func (r *SKUStore) ListByIDs(ids []uint) ([]productdomain.ProductSKU, error) {
	if len(ids) == 0 {
		return []productdomain.ProductSKU{}, nil
	}
	var items []productdomain.ProductSKU
	if err := r.db.Where("deleted_at IS NULL AND id IN ?", ids).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// Create 创建 SKU
func (r *SKUStore) Create(item *productdomain.ProductSKU) error {
	if item == nil {
		return errors.New("sku is nil")
	}
	return r.db.Create(item).Error
}

// CreateBatch 批量创建 SKU
func (r *SKUStore) CreateBatch(items []productdomain.ProductSKU) error {
	if len(items) == 0 {
		return nil
	}
	return r.db.Create(&items).Error
}

// Update 更新 SKU
func (r *SKUStore) Update(item *productdomain.ProductSKU) error {
	if item == nil {
		return errors.New("sku is nil")
	}
	return r.db.Save(item).Error
}

// Delete 硬删除单个 SKU（绕过软删除，避免唯一索引冲突）
func (r *SKUStore) Delete(id uint) error {
	if id == 0 {
		return errors.New("invalid sku id")
	}
	return r.db.Unscoped().Delete(&productdomain.ProductSKU{}, id).Error
}

// PurgeSoftDeletedByProductAndCode 清理指定商品下同 sku_code 的软删除残留记录
func (r *SKUStore) PurgeSoftDeletedByProductAndCode(productID uint, skuCode string) error {
	return r.db.Unscoped().
		Where("product_id = ? AND sku_code = ? AND deleted_at IS NOT NULL", productID, skuCode).
		Delete(&productdomain.ProductSKU{}).Error
}

// DeleteByProduct 删除指定商品下的 SKU
func (r *SKUStore) DeleteByProduct(productID uint) error {
	if productID == 0 {
		return errors.New("invalid product id")
	}
	return r.db.Model(&productdomain.ProductSKU{}).
		Where("product_id = ? AND deleted_at IS NULL", productID).
		Update("deleted_at", time.Now()).Error
}

// ReserveManualStock 预占手动库存
func (r *SKUStore) ReserveManualStock(skuID uint, quantity int) (int64, error) {
	return gormutil.ReserveManualStock(r.db, &productdomain.ProductSKU{}, skuID, quantity)
}

// ReleaseManualStock 释放手动库存占用
func (r *SKUStore) ReleaseManualStock(skuID uint, quantity int) (int64, error) {
	return gormutil.ReleaseManualStock(r.db, &productdomain.ProductSKU{}, skuID, quantity)
}

// ConsumeManualStock 消耗手动库存（支付成功后占用转已售）
func (r *SKUStore) ConsumeManualStock(skuID uint, quantity int) (int64, error) {
	return gormutil.ConsumeManualStock(r.db, &productdomain.ProductSKU{}, skuID, quantity)
}
