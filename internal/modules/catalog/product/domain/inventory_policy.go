package productdomain

import (
	"strings"

	"github.com/dujiao-next/internal/constants"
)

func hasMultipleActiveSKUs(product *Product) bool {
	if product == nil || len(product.SKUs) == 0 {
		return false
	}
	activeCount := 0
	for i := range product.SKUs {
		if !product.SKUs[i].IsActive {
			continue
		}
		activeCount++
		if activeCount > 1 {
			return true
		}
	}
	return false
}

// ManualSKUAvailable 返回 SKU 当前可用于规则判断的手工库存；无限库存映射为最大 int。
func ManualSKUAvailable(sku *ProductSKU) int {
	if sku == nil {
		return 0
	}
	if sku.ManualStockTotal == constants.ManualStockUnlimited {
		return int(^uint(0) >> 1)
	}
	if sku.ManualStockTotal < 0 {
		return 0
	}
	return sku.ManualStockTotal
}

// ShouldEnforceManualSKUStock 判断当前 SKU 是否需要执行手工库存约束。
func ShouldEnforceManualSKUStock(product *Product, sku *ProductSKU) bool {
	if product == nil || sku == nil {
		return false
	}
	if sku.ManualStockTotal == constants.ManualStockUnlimited {
		return false
	}
	if sku.ManualStockTotal >= 0 {
		return true
	}
	if strings.ToUpper(strings.TrimSpace(sku.SKUCode)) != DefaultSKUCode {
		return true
	}
	return hasMultipleActiveSKUs(product)
}
