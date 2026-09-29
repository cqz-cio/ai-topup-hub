package application

import (
	"strings"
	"time"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	promotioncontract "github.com/dujiao-next/internal/modules/promotion/contract"
	promotiondomain "github.com/dujiao-next/internal/modules/promotion/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// Service 活动价服务。
type Service struct {
	promotionRepo promotioncontract.Repository
}

// NewService 创建活动价服务。
func NewService(promotionRepo promotioncontract.Repository) *Service {
	return &Service{
		promotionRepo: promotionRepo,
	}
}

// GetProductPromotions 获取商品所有有效活动规则（用于前端展示）
func (s *Service) GetProductPromotions(productID uint) ([]promotiondomain.Promotion, error) {
	return s.promotionRepo.GetAllActiveByProduct(productID, time.Now())
}

// ApplyPromotion 应用活动价规则（支持阶梯匹配）
func (s *Service) ApplyPromotion(product *productdomain.Product, quantity int) (*promotiondomain.Promotion, money.Amount, error) {
	if product == nil || quantity <= 0 {
		return nil, money.Amount{}, promotioncontract.ErrInvalid
	}

	now := time.Now()
	promotions, err := s.promotionRepo.GetAllActiveByProduct(product.ID, now)
	if err != nil {
		return nil, money.Amount{}, err
	}
	if len(promotions) == 0 {
		return nil, product.PriceAmount, nil
	}

	subtotal := product.PriceAmount.Decimal.Mul(decimal.NewFromInt(int64(quantity)))

	// 从高到低遍历 MinAmount，取第一个满足 MinAmount <= subtotal 的规则
	var matched *promotiondomain.Promotion
	for i := len(promotions) - 1; i >= 0; i-- {
		p := &promotions[i]
		if strings.ToLower(strings.TrimSpace(p.ScopeType)) != constants.ScopeTypeProduct {
			continue
		}
		if p.MinAmount.Decimal.LessThanOrEqual(decimal.Zero) || subtotal.Cmp(p.MinAmount.Decimal) >= 0 {
			matched = p
			break
		}
	}

	if matched == nil {
		return nil, product.PriceAmount, nil
	}

	unitPrice, err := s.calculateUnitPrice(product.PriceAmount, matched)
	if err != nil {
		return nil, money.Amount{}, err
	}

	return matched, unitPrice, nil
}

func (s *Service) calculateUnitPrice(base money.Amount, promotion *promotiondomain.Promotion) (money.Amount, error) {
	value := promotion.Value.Decimal
	if value.LessThanOrEqual(decimal.Zero) {
		return money.Amount{}, promotioncontract.ErrInvalid
	}

	switch strings.ToLower(strings.TrimSpace(promotion.Type)) {
	case constants.PromotionTypeFixed:
		discounted := base.Decimal.Sub(value)
		if discounted.LessThan(decimal.Zero) {
			discounted = decimal.Zero
		}
		return money.FromDecimal(discounted), nil
	case constants.PromotionTypePercent:
		percent := decimal.NewFromInt(100).Sub(value)
		if percent.LessThan(decimal.Zero) {
			percent = decimal.Zero
		}
		discounted := base.Decimal.Mul(percent).Div(decimal.NewFromInt(100))
		return money.FromDecimal(discounted), nil
	case constants.PromotionTypeSpecialPrice:
		return money.FromDecimal(value), nil
	default:
		return money.Amount{}, promotioncontract.ErrInvalid
	}
}
