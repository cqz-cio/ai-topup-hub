package contract

import (
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"
	"github.com/shopspring/decimal"
)

type ListFilter struct {
	IsActive *bool
	Page     int
	PageSize int
}

type LevelRepository interface {
	GetByID(id uint) (*memberleveldomain.MemberLevel, error)
	GetBySlug(slug string) (*memberleveldomain.MemberLevel, error)
	GetDefault() (*memberleveldomain.MemberLevel, error)
	GetActiveBySortOrder(sortOrder int, excludeID uint) (*memberleveldomain.MemberLevel, error)
	ListAllActive() ([]memberleveldomain.MemberLevel, error)
	Create(level *memberleveldomain.MemberLevel) error
	Update(level *memberleveldomain.MemberLevel) error
	Delete(id uint) error
	List(filter ListFilter) ([]memberleveldomain.MemberLevel, int64, error)
	ClearDefault(excludeID uint) error
}

type PriceRepository interface {
	GetByID(id uint) (*memberleveldomain.MemberLevelPrice, error)
	GetByLevelAndProductAndSKU(levelID, productID, skuID uint) (*memberleveldomain.MemberLevelPrice, error)
	ListByProduct(productID uint) ([]memberleveldomain.MemberLevelPrice, error)
	ListByLevelAndProducts(levelID uint, productIDs []uint) ([]memberleveldomain.MemberLevelPrice, error)
	BatchUpsert(prices []memberleveldomain.MemberLevelPrice) error
	Delete(id uint) error
	DeleteByProduct(productID uint) error
}

// UserRepository is the member-level consumer's minimal user persistence port.
type UserRepository interface {
	GetByID(id uint) (*userdomain.User, error)
	Update(user *userdomain.User) error
	IncrementTotalRecharged(userID uint, amount decimal.Decimal) error
	IncrementTotalSpent(userID uint, amount decimal.Decimal) error
	UpdateMemberLevelIfCurrent(userID, currentLevelID, nextLevelID uint) (int64, error)
	AssignDefaultMemberLevel(defaultLevelID uint) (int64, error)
}
