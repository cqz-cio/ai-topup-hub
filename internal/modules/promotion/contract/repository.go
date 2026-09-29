package contract

import (
	"time"

	promotiondomain "github.com/dujiao-next/internal/modules/promotion/domain"
)

// ListFilter 定义管理端活动价列表的查询条件。
type ListFilter struct {
	ID         uint
	Name       string
	ScopeRefID uint
	IsActive   *bool
	Page       int
	PageSize   int
}

// Repository 定义 Promotion 领域所需的持久化能力。
type Repository interface {
	GetByID(id uint) (*promotiondomain.Promotion, error)
	GetActiveByProduct(productID uint, now time.Time) (*promotiondomain.Promotion, error)
	GetAllActiveByProduct(productID uint, now time.Time) ([]promotiondomain.Promotion, error)
	Create(promotion *promotiondomain.Promotion) error
	Update(promotion *promotiondomain.Promotion) error
	Delete(id uint) error
	List(filter ListFilter) ([]promotiondomain.Promotion, int64, error)
}
