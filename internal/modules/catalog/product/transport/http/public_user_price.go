package producthttp

import (
	productpresenter "github.com/dujiao-next/internal/modules/catalog/product/transport/presenter"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/gin-gonic/gin"
)

func (h *PublicHandler) userPricesForRequest(c *gin.Context) (map[[2]uint]money.Amount, bool) {
	if h.memberLevels == nil || c.GetUint("user_id") == 0 || isResellerTenant(tenantFromRequest(c)) {
		return nil, true
	}
	rows, err := h.memberLevels.GetUserPrices(c.GetUint("user_id"))
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.product_fetch_failed", err)
		return nil, false
	}
	prices := make(map[[2]uint]money.Amount, len(rows))
	for _, row := range rows {
		prices[[2]uint{row.ProductID, row.SKUID}] = row.PriceAmount
	}
	return prices, true
}

// Only the authenticated customer's rows are applied, after ordinary pricing.
// The retail amount is kept separately for cart snapshots and account switching.
func applyUserProductPrices(product *productpresenter.Product, prices map[[2]uint]money.Amount, displaySKUID uint) {
	active, personalized := 0, 0
	for i := range product.SKUs {
		sku := &product.SKUs[i]
		if !sku.IsActive {
			continue
		}
		active++
		price, found := prices[[2]uint{product.ID, sku.ID}]
		if !found {
			price, found = prices[[2]uint{product.ID, 0}]
		}
		if !found {
			continue
		}
		personalized++
		retail := sku.PriceAmount
		sku.RetailPriceAmount = &retail
		sku.UserPriceAmount = &price
		sku.PriceAmount = price
		sku.EffectivePriceAmount = &price
		sku.MemberPriceAmount = nil
		sku.PromotionPriceAmount = nil
		if sku.ID == displaySKUID {
			productRetail := product.PriceAmount
			product.RetailPriceAmount = &productRetail
			product.UserPriceAmount = &price
			product.PriceAmount = price
			product.EffectivePriceAmount = &price
			product.PromotionPriceAmount = nil
			product.PromotionID = nil
			product.PromotionName = ""
			product.PromotionType = ""
		}
	}
	if active > 0 && active == personalized {
		product.MemberPrices = nil
		product.PricingMemberLevelID = 0
		product.WholesalePrices = nil
		product.PromotionRules = nil
	}
}
