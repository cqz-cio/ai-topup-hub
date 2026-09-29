package catalogreader

import (
	categorycontract "github.com/dujiao-next/internal/modules/catalog/category/contract"
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	"github.com/dujiao-next/internal/modules/sitemap/contract"
)

// Reader 将 Catalog 的仓储契约收窄为 Sitemap 投影。
type Reader struct {
	products   productcontract.Repository
	categories categorycontract.Repository
}

var _ contract.CatalogReader = (*Reader)(nil)

// New 创建 Catalog 读取适配器。
func New(products productcontract.Repository, categories categorycontract.Repository) *Reader {
	if products == nil || categories == nil {
		panic("sitemap catalog reader: required repository is nil")
	}
	return &Reader{products: products, categories: categories}
}

// ListActiveCategories 返回可索引分类投影。
func (r *Reader) ListActiveCategories() ([]contract.Category, error) {
	rows, err := r.categories.ListActive()
	if err != nil {
		return nil, err
	}
	result := make([]contract.Category, 0, len(rows))
	for _, row := range rows {
		result = append(result, contract.Category{Slug: row.Slug, CreatedAt: row.CreatedAt})
	}
	return result, nil
}

// ListActiveProducts 返回可索引商品投影。
func (r *Reader) ListActiveProducts(limit int) ([]contract.Product, error) {
	rows, _, err := r.products.List(productcontract.ListFilter{
		Page:       1,
		PageSize:   limit,
		OnlyActive: true,
	})
	if err != nil {
		return nil, err
	}
	result := make([]contract.Product, 0, len(rows))
	for _, row := range rows {
		result = append(result, contract.Product{Slug: row.Slug, UpdatedAt: row.UpdatedAt})
	}
	return result, nil
}
