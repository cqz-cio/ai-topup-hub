package contract

import (
	"github.com/dujiao-next/internal/modules/cart/domain"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
)

// StoredItem 是购物车持久化端口返回的条目及其商品快照。
type StoredItem struct {
	domain.Item
	Product *productdomain.Product
	SKU     *productdomain.ProductSKU
}

// Repository 是购物车应用层所需的最小持久化端口。
type Repository interface {
	ListByUser(userID uint) ([]StoredItem, error)
	Upsert(item *domain.Item) error
	DeleteByUserProductSKU(userID, productID, skuID uint) error
}

// ProductReader 是购物车服务所需的商品读取端口。
type ProductReader interface {
	GetByID(id string) (*productdomain.Product, error)
}

// SKUReader 是购物车服务所需的 SKU 读取端口。
type SKUReader interface {
	GetByID(id uint) (*productdomain.ProductSKU, error)
	ListByProduct(productID uint, onlyActive bool) ([]productdomain.ProductSKU, error)
}

// CurrencyReader 是购物车服务所需的站点币种端口。
type CurrencyReader interface {
	GetSiteCurrency(defaultCurrency string) (string, error)
}
