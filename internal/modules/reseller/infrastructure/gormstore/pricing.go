package gormstore

import (
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"
)

// ListProductSettingsForPricing 批量获取分销定价所需的商品级与 SKU 级配置。
func (r *Store) ListProductSettingsForPricing(resellerID uint, productIDs []uint, skuIDs []uint) ([]resellerdomain.ProductSetting, error) {
	if resellerID == 0 || len(productIDs) == 0 {
		return []resellerdomain.ProductSetting{}, nil
	}
	productIDs = uniqueUintSlice(productIDs)
	skuIDs = uniqueUintSlice(skuIDs)

	query := r.db.Where("reseller_id = ? AND product_id IN ? AND deleted_at IS NULL", resellerID, productIDs)
	if len(skuIDs) > 0 {
		query = query.Where("(sku_id = 0 OR sku_id IN ?)", skuIDs)
	} else {
		query = query.Where("sku_id = 0")
	}

	var rows []resellerdomain.ProductSetting
	if err := query.Order("product_id ASC, sku_id ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListHiddenProductIDs 返回分销前台列表应在查询层排除的商品 ID。
func (r *Store) ListHiddenProductIDs(resellerID uint) ([]uint, error) {
	if resellerID == 0 {
		return []uint{}, nil
	}

	hidden := map[uint]struct{}{}
	var productHidden []uint
	if err := r.db.Model(&resellerdomain.ProductSetting{}).
		Where("reseller_id = ? AND sku_id = 0 AND is_listed = ? AND deleted_at IS NULL", resellerID, false).
		Pluck("product_id", &productHidden).Error; err != nil {
		return nil, err
	}
	for _, id := range productHidden {
		if id != 0 {
			hidden[id] = struct{}{}
		}
	}

	var skuHidden []uint
	if err := r.db.Model(&productdomain.ProductSKU{}).
		Select("product_skus.product_id").
		Joins(
			"JOIN reseller_product_settings rps ON rps.product_id = product_skus.product_id AND rps.sku_id = product_skus.id AND rps.reseller_id = ? AND rps.is_listed = ? AND rps.deleted_at IS NULL",
			resellerID,
			false,
		).
		Where("product_skus.is_active = ? AND product_skus.deleted_at IS NULL", true).
		Group("product_skus.product_id").
		Having("COUNT(product_skus.id) = (SELECT COUNT(1) FROM product_skus ps2 WHERE ps2.product_id = product_skus.product_id AND ps2.is_active = ? AND ps2.deleted_at IS NULL)", true).
		Pluck("product_skus.product_id", &skuHidden).Error; err != nil {
		return nil, err
	}
	for _, id := range skuHidden {
		if id != 0 {
			hidden[id] = struct{}{}
		}
	}

	ids := make([]uint, 0, len(hidden))
	for id := range hidden {
		ids = append(ids, id)
	}
	return ids, nil
}

func uniqueUintSlice(values []uint) []uint {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[uint]struct{}, len(values))
	result := make([]uint, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
