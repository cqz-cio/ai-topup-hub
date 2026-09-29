package categorycontract

import categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"

// Repository 定义 Category 应用服务和相邻 Catalog 子域共享的持久化契约。
type Repository interface {
	List() ([]categorydomain.Category, error)
	ListActive() ([]categorydomain.Category, error)
	GetByID(id string) (*categorydomain.Category, error)
	Create(category *categorydomain.Category) error
	Update(category *categorydomain.Category) error
	UpdateActive(id string, active bool) error
	Delete(id string) error
	CountBySlug(slug string, excludeID *string) (int64, error)
	CountChildren(categoryID string) (int64, error)
	CountProducts(categoryID string) (int64, error)
	CountActiveProducts(categoryID string) (int64, error)
	GetBySlug(slug string) (*categorydomain.Category, error)
	GetBySlugUnscoped(slug string) (*categorydomain.Category, error)
	Restore(category *categorydomain.Category) error
}
