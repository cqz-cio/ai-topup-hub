package gormstore

import (
	"errors"
	"time"

	memberlevelcontract "github.com/dujiao-next/internal/modules/memberlevel/contract"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PriceStore struct {
	db *gorm.DB
}

func NewPriceStore(db *gorm.DB) *PriceStore {
	return &PriceStore{db: db}
}

func (r *PriceStore) GetByID(id uint) (*memberleveldomain.MemberLevelPrice, error) {
	var price memberleveldomain.MemberLevelPrice
	if err := r.db.Where("deleted_at IS NULL").First(&price, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &price, nil
}

func (r *PriceStore) GetByLevelAndProductAndSKU(levelID, productID, skuID uint) (*memberleveldomain.MemberLevelPrice, error) {
	var price memberleveldomain.MemberLevelPrice
	if err := r.db.Where("deleted_at IS NULL AND member_level_id = ? AND product_id = ? AND sku_id = ?", levelID, productID, skuID).First(&price).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &price, nil
}

// ListByProduct 获取商品的所有等级价
func (r *PriceStore) ListByProduct(productID uint) ([]memberleveldomain.MemberLevelPrice, error) {
	var prices []memberleveldomain.MemberLevelPrice
	if err := r.db.Where("deleted_at IS NULL AND product_id = ?", productID).Order("member_level_id asc, sku_id asc").Find(&prices).Error; err != nil {
		return nil, err
	}
	return prices, nil
}

func (r *PriceStore) ListByLevel(levelID uint) ([]memberleveldomain.MemberLevelPrice, error) {
	prices := make([]memberleveldomain.MemberLevelPrice, 0)
	err := r.db.Where("deleted_at IS NULL AND member_level_id = ?", levelID).
		Where("product_id IN (?)", r.db.Table("products").Select("id").Where("deleted_at IS NULL")).
		Where("sku_id = 0 OR sku_id IN (?)", r.db.Table("product_skus").Select("id").Where("deleted_at IS NULL")).
		Order("product_id, sku_id").Find(&prices).Error
	return prices, err
}

func (r *PriceStore) ReplaceByLevel(levelID uint, prices []memberleveldomain.MemberLevelPrice) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Table("member_levels").Where("id = ? AND deleted_at IS NULL", levelID).Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return memberlevelcontract.ErrNotFound
		}
		checked := make(map[uint]bool)
		for _, price := range prices {
			if price.MemberLevelID != levelID {
				return memberlevelcontract.ErrPriceInvalid
			}
			if !checked[price.ProductID] {
				if err := tx.Table("products").Where("id = ? AND deleted_at IS NULL", price.ProductID).Count(&count).Error; err != nil {
					return err
				}
				if count != 1 {
					return memberlevelcontract.ErrPriceInvalid
				}
				checked[price.ProductID] = true
			}
			if price.SKUID > 0 {
				if err := tx.Table("product_skus").Where("id = ? AND product_id = ? AND deleted_at IS NULL", price.SKUID, price.ProductID).Count(&count).Error; err != nil {
					return err
				}
				if count != 1 {
					return memberlevelcontract.ErrPriceInvalid
				}
			}
		}
		if err := tx.Model(&memberleveldomain.MemberLevelPrice{}).Where("member_level_id = ? AND deleted_at IS NULL", levelID).Update("deleted_at", time.Now()).Error; err != nil {
			return err
		}
		return upsertPrices(tx, prices)
	})
}

// ListByLevelAndProducts 获取指定等级和商品集合的等级价
func (r *PriceStore) ListByLevelAndProducts(levelID uint, productIDs []uint) ([]memberleveldomain.MemberLevelPrice, error) {
	if len(productIDs) == 0 {
		return nil, nil
	}
	var prices []memberleveldomain.MemberLevelPrice
	if err := r.db.Where("deleted_at IS NULL AND member_level_id = ? AND product_id IN ?", levelID, productIDs).Find(&prices).Error; err != nil {
		return nil, err
	}
	return prices, nil
}

// BatchUpsert 批量创建/更新等级价
func (r *PriceStore) BatchUpsert(prices []memberleveldomain.MemberLevelPrice) error {
	if len(prices) == 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		return upsertPrices(tx, prices)
	})
}

func upsertPrices(tx *gorm.DB, prices []memberleveldomain.MemberLevelPrice) error {
	if len(prices) == 0 {
		return nil
	}
	rows := append([]memberleveldomain.MemberLevelPrice(nil), prices...)
	for i := range rows {
		rows[i].ID = 0
		rows[i].DeletedAt = nil
		rows[i].UpdatedAt = time.Now()
	}
	// Restore a previously cleared price instead of colliding with its unique key.
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "member_level_id"}, {Name: "product_id"}, {Name: "sku_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"price_amount", "updated_at", "deleted_at"}),
	}).CreateInBatches(rows, 100).Error
}

func (r *PriceStore) ReplaceByProduct(productID uint, prices []memberleveldomain.MemberLevelPrice) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Table("products").Where("id = ? AND deleted_at IS NULL", productID).Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return memberlevelcontract.ErrPriceInvalid
		}
		for _, price := range prices {
			if price.ProductID != productID {
				return memberlevelcontract.ErrPriceInvalid
			}
			if price.SKUID == 0 {
				continue
			}
			if err := tx.Table("product_skus").Where("id = ? AND product_id = ? AND deleted_at IS NULL", price.SKUID, productID).Count(&count).Error; err != nil {
				return err
			}
			if count != 1 {
				return memberlevelcontract.ErrPriceInvalid
			}
		}
		if err := NewPriceStore(tx).DeleteByProduct(productID); err != nil {
			return err
		}
		return upsertPrices(tx, prices)
	})
}

func (r *PriceStore) Delete(id uint) error {
	return r.db.Model(&memberleveldomain.MemberLevelPrice{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", time.Now()).Error
}

func (r *PriceStore) DeleteByProduct(productID uint) error {
	return r.db.Model(&memberleveldomain.MemberLevelPrice{}).
		Where("product_id = ? AND deleted_at IS NULL", productID).
		Update("deleted_at", time.Now()).Error
}

// DeleteByProductInTx is the adapter contract consumed by product deletion.
func (r *PriceStore) DeleteByProductInTx(tx *gorm.DB, productID uint) error {
	if tx == nil {
		return r.DeleteByProduct(productID)
	}
	return NewPriceStore(tx).DeleteByProduct(productID)
}

var _ memberlevelcontract.PriceRepository = (*PriceStore)(nil)
