package application

import (
	"errors"

	memberlevelcontract "github.com/dujiao-next/internal/modules/memberlevel/contract"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

func (s *Service) GetUserPrices(userID uint) ([]memberleveldomain.UserPrice, error) {
	if s.userPriceRepo == nil || userID == 0 {
		return nil, nil
	}
	return s.userPriceRepo.ListByUser(userID)
}

// ReplaceUserPrices saves the entire customer's price list in one transaction.
func (s *Service) ReplaceUserPrices(userID uint, prices []memberleveldomain.UserPrice) error {
	if s.userPriceRepo == nil {
		return errors.New("user pricing repository unavailable")
	}
	if userID == 0 || len(prices) > 5000 {
		return memberlevelcontract.ErrPriceInvalid
	}
	seen := make(map[[2]uint]bool, len(prices))
	for _, price := range prices {
		key := [2]uint{price.ProductID, price.SKUID}
		amount := price.PriceAmount.Decimal
		if price.UserID != userID || price.ProductID == 0 || seen[key] ||
			!amount.GreaterThan(decimal.Zero) || amount.GreaterThanOrEqual(decimal.NewFromInt(10000000000)) ||
			!amount.Equal(amount.Round(2)) {
			return memberlevelcontract.ErrPriceInvalid
		}
		seen[key] = true
	}
	return s.userPriceRepo.ReplaceByUser(userID, prices)
}

// ResolveUserPrice is independent of member level, promotion and quantity pricing.
// Read errors must reach checkout instead of silently charging a fallback price.
func (s *Service) ResolveUserPrice(userID, productID, skuID uint) (*money.Amount, error) {
	if s.userPriceRepo == nil || userID == 0 {
		return nil, nil
	}
	price, err := s.userPriceRepo.GetByUserProductSKU(userID, productID, skuID)
	if err != nil {
		return nil, err
	}
	if price == nil {
		return nil, nil
	}
	amount := price.PriceAmount.Decimal
	if !amount.GreaterThan(decimal.Zero) || !amount.Equal(amount.Round(2)) {
		return nil, memberlevelcontract.ErrPriceInvalid
	}
	return &price.PriceAmount, nil
}
