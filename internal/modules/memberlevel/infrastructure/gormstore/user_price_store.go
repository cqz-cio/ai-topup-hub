package gormstore

import (
	"errors"

	memberlevelcontract "github.com/dujiao-next/internal/modules/memberlevel/contract"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"
	"gorm.io/gorm"
)

type UserPriceStore struct{ db *gorm.DB }

func NewUserPriceStore(db *gorm.DB) *UserPriceStore { return &UserPriceStore{db: db} }

func (r *UserPriceStore) ListByUser(userID uint) ([]memberleveldomain.UserPrice, error) {
	var prices []memberleveldomain.UserPrice
	err := r.db.Where("user_id = ?", userID).
		Where("product_id IN (?)", r.db.Table("products").Select("id").Where("deleted_at IS NULL")).
		Where("sku_id = 0 OR sku_id IN (?)", r.db.Table("product_skus").Select("id").Where("deleted_at IS NULL")).
		Order("product_id, sku_id").Find(&prices).Error
	return prices, err
}

func (r *UserPriceStore) GetByUserProductSKU(userID, productID, skuID uint) (*memberleveldomain.UserPrice, error) {
	var price memberleveldomain.UserPrice
	err := r.db.Where("user_id = ? AND product_id = ? AND sku_id IN ?", userID, productID, []uint{skuID, 0}).Order("sku_id DESC").First(&price).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &price, nil
}

func (r *UserPriceStore) ReplaceByUser(userID uint, prices []memberleveldomain.UserPrice) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Table("users").Where("id = ? AND deleted_at IS NULL", userID).Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return memberlevelcontract.ErrUserNotFound
		}
		checked := make(map[uint]bool)
		for _, price := range prices {
			if price.UserID != userID {
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
		if err := tx.Where("user_id = ?", userID).Delete(&memberleveldomain.UserPrice{}).Error; err != nil {
			return err
		}
		if len(prices) == 0 {
			return nil
		}
		rows := append([]memberleveldomain.UserPrice(nil), prices...)
		for i := range rows {
			rows[i].ID = 0
		}
		return tx.CreateInBatches(rows, 100).Error
	})
}

var _ memberlevelcontract.UserPriceRepository = (*UserPriceStore)(nil)
