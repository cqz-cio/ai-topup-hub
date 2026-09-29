package domain

import "fmt"

// ItemKey 返回商品与 SKU 的稳定组合键。
func ItemKey(productID, skuID uint) string {
	return fmt.Sprintf("%d:%d", productID, skuID)
}
