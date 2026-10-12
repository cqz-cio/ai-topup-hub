package application

import (
	"errors"
	"testing"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	memberlevelapp "github.com/dujiao-next/internal/modules/memberlevel/application"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"
	memberlevelgormstore "github.com/dujiao-next/internal/modules/memberlevel/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

func TestOrderUsesExactUserPriceInsteadOfPromotionWholesaleAndLevelDiscount(t *testing.T) {
	rate, promotion := decimal.NewFromInt(80), decimal.NewFromInt(50)
	f := setupWholesaleOrderFixture(t, "personal-order-price", productdomain.WholesalePriceTiers{{MinQuantity: 1, UnitPrice: money.FromDecimal(decimal.NewFromInt(40))}}, &promotion, &rate)
	if err := f.db.AutoMigrate(&memberleveldomain.UserPrice{}); err != nil {
		t.Fatal(err)
	}
	prices := memberlevelgormstore.NewUserPriceStore(f.db)
	memberService := memberlevelapp.NewService(memberlevelgormstore.NewLevelStore(f.db), memberlevelgormstore.NewPriceStore(f.db), memberlevelgormstore.NewUserStore(f.db), prices)
	f.svc.memberLevelService = memberService
	for _, unit := range []string{"92.33", "140.01"} {
		if err := memberService.ReplaceUserPrices(f.user.ID, []memberleveldomain.UserPrice{{UserID: f.user.ID, ProductID: f.product.ID, PriceAmount: money.FromDecimal(decimal.RequireFromString(unit))}}); err != nil {
			t.Fatal(err)
		}
		for _, quantity := range []int{1, 3} {
			result, err := f.svc.buildOrderResult(orderCreateParams{UserID: f.user.ID, Items: []CreateOrderItem{{ProductID: f.product.ID, SKUID: f.sku.ID, Quantity: quantity}}})
			if err != nil {
				t.Fatal(err)
			}
			want := decimal.RequireFromString(unit).Mul(decimal.NewFromInt(int64(quantity)))
			if !result.TotalAmount.Equal(want) || !result.OriginalAmount.Equal(want) || result.OrderItems[0].UnitPrice.StringFixed(2) != unit {
				t.Fatalf("wrong exact price: %+v", result)
			}
			if !result.MemberDiscountAmount.IsZero() || !result.PromotionDiscountAmount.IsZero() || !result.WholesaleDiscountAmount.IsZero() {
				t.Fatal("fixed price stacked discounts")
			}
			if !result.OrderItems[0].CostPrice.Decimal.Equal(f.sku.CostPriceAmount.Decimal) {
				t.Fatal("customer price changed procurement cost")
			}
		}
	}
	if err := memberService.ReplaceUserPrices(f.user.ID, nil); err != nil {
		t.Fatal(err)
	}
	result, err := f.svc.buildOrderResult(orderCreateParams{UserID: f.user.ID, Items: []CreateOrderItem{{ProductID: f.product.ID, SKUID: f.sku.ID, Quantity: 3}}})
	if err != nil || !result.TotalAmount.Equal(decimal.NewFromInt(96)) {
		t.Fatalf("cleared price failed to restore ordinary pricing: %v, %+v", err, result)
	}
}

type unavailableUserPriceService struct{}

func (unavailableUserPriceService) ResolveMemberPrice(_, _, _ uint, base decimal.Decimal) (decimal.Decimal, decimal.Decimal) {
	return base, decimal.Zero
}
func (unavailableUserPriceService) OnOrderPaid(uint, decimal.Decimal) error { return nil }
func (unavailableUserPriceService) ResolveUserPrice(uint, uint, uint) (*money.Amount, error) {
	return nil, errors.New("price database unavailable")
}

func TestOrderRejectsUnavailableUserPriceInsteadOfChargingRetail(t *testing.T) {
	rate := decimal.NewFromInt(100)
	f := setupWholesaleOrderFixture(t, "personal-order-read-error", nil, nil, &rate)
	f.svc.memberLevelService = unavailableUserPriceService{}
	_, err := f.svc.buildOrderResult(orderCreateParams{UserID: f.user.ID, Items: []CreateOrderItem{{ProductID: f.product.ID, SKUID: f.sku.ID, Quantity: 1}}})
	if err == nil || err.Error() != "price database unavailable" {
		t.Fatalf("should fail closed: %v", err)
	}
}
