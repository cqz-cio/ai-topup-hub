package application

import (
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	promotioncontract "github.com/dujiao-next/internal/modules/promotion/contract"
	promotiondomain "github.com/dujiao-next/internal/modules/promotion/domain"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// AdminService 活动价管理服务。
type AdminService struct {
	repo promotioncontract.Repository
}

// NewAdminService 创建活动价管理服务。
func NewAdminService(repo promotioncontract.Repository) *AdminService {
	return &AdminService{repo: repo}
}

// CreatePromotionInput 创建活动价输入
type CreatePromotionInput struct {
	Name       string
	Type       string
	ScopeRefID uint
	Value      money.Amount
	MinAmount  money.Amount
	StartsAt   *time.Time
	EndsAt     *time.Time
	IsActive   *bool
}

// UpdatePromotionInput 更新活动价输入
type UpdatePromotionInput struct {
	Name       string
	Type       string
	ScopeRefID uint
	Value      money.Amount
	MinAmount  money.Amount
	StartsAt   *time.Time
	EndsAt     *time.Time
	IsActive   *bool
}

// Create 创建活动价
func (s *AdminService) Create(input CreatePromotionInput) (*promotiondomain.Promotion, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, promotioncontract.ErrInvalid
	}
	if input.ScopeRefID == 0 {
		return nil, promotioncontract.ErrInvalid
	}
	promotionType := strings.ToLower(strings.TrimSpace(input.Type))
	if promotionType != constants.PromotionTypeFixed && promotionType != constants.PromotionTypePercent && promotionType != constants.PromotionTypeSpecialPrice {
		return nil, promotioncontract.ErrInvalid
	}
	if input.Value.Decimal.LessThanOrEqual(decimal.Zero) {
		return nil, promotioncontract.ErrInvalid
	}
	if promotionType == constants.PromotionTypePercent && input.Value.Decimal.GreaterThan(decimal.NewFromInt(100)) {
		return nil, promotioncontract.ErrInvalid
	}
	if input.StartsAt != nil && input.EndsAt != nil && input.EndsAt.Before(*input.StartsAt) {
		return nil, promotioncontract.ErrInvalid
	}

	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	promotion := &promotiondomain.Promotion{
		Name:       name,
		ScopeType:  constants.ScopeTypeProduct,
		ScopeRefID: input.ScopeRefID,
		Type:       promotionType,
		Value:      input.Value,
		MinAmount:  input.MinAmount,
		StartsAt:   input.StartsAt,
		EndsAt:     input.EndsAt,
		IsActive:   isActive,
	}

	if err := s.repo.Create(promotion); err != nil {
		return nil, err
	}
	return promotion, nil
}

// Update 更新活动价
func (s *AdminService) Update(id uint, input UpdatePromotionInput) (*promotiondomain.Promotion, error) {
	if id == 0 {
		return nil, promotioncontract.ErrInvalid
	}
	existing, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, promotioncontract.ErrNotFound
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, promotioncontract.ErrInvalid
	}
	if input.ScopeRefID == 0 {
		return nil, promotioncontract.ErrInvalid
	}
	promotionType := strings.ToLower(strings.TrimSpace(input.Type))
	if promotionType != constants.PromotionTypeFixed && promotionType != constants.PromotionTypePercent && promotionType != constants.PromotionTypeSpecialPrice {
		return nil, promotioncontract.ErrInvalid
	}
	if input.Value.Decimal.LessThanOrEqual(decimal.Zero) {
		return nil, promotioncontract.ErrInvalid
	}
	if promotionType == constants.PromotionTypePercent && input.Value.Decimal.GreaterThan(decimal.NewFromInt(100)) {
		return nil, promotioncontract.ErrInvalid
	}
	if input.StartsAt != nil && input.EndsAt != nil && input.EndsAt.Before(*input.StartsAt) {
		return nil, promotioncontract.ErrInvalid
	}

	isActive := existing.IsActive
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	existing.Name = name
	existing.ScopeType = constants.ScopeTypeProduct
	existing.ScopeRefID = input.ScopeRefID
	existing.Type = promotionType
	existing.Value = input.Value
	existing.MinAmount = input.MinAmount
	existing.StartsAt = input.StartsAt
	existing.EndsAt = input.EndsAt
	existing.IsActive = isActive

	if err := s.repo.Update(existing); err != nil {
		return nil, promotioncontract.ErrUpdateFailed
	}
	return existing, nil
}

// Delete 删除活动价
func (s *AdminService) Delete(id uint) error {
	if id == 0 {
		return promotioncontract.ErrInvalid
	}
	existing, err := s.repo.GetByID(id)
	if err != nil {
		return err
	}
	if existing == nil {
		return promotioncontract.ErrNotFound
	}
	if err := s.repo.Delete(id); err != nil {
		return promotioncontract.ErrDeleteFailed
	}
	return nil
}

// List 获取活动价列表
func (s *AdminService) List(filter promotioncontract.ListFilter) ([]promotiondomain.Promotion, int64, error) {
	return s.repo.List(filter)
}
