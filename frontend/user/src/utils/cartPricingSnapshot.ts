import type { CartItem } from '../stores/cart'
import { amountToCents, centsToAmount } from './money.ts'
import { resolveCatalogSkuPrice } from './memberPricing.ts'

// Keep the base price for original totals and the current user's display price separately.
// Order previews still determine the amount charged.
export const resolveCartPricingSnapshot = (product: any, sku: any, userId = 0): Partial<CartItem> => {
  const patch: Partial<CartItem> = {
    displayPriceAmount: resolveCatalogSkuPrice(product, sku) ?? undefined,
    promotionPriceAmount: sku?.promotion_price_amount == null ? undefined : String(sku.promotion_price_amount),
    pricingUserId: userId,
    memberLevelId: Number(product?.pricing_member_level_id || 0),
    memberDiscountRate: String(product?.member_discount_rate || '100'),
    memberPrices: Array.isArray(product?.member_prices) ? product.member_prices : undefined,
    wholesalePrices: Array.isArray(product?.wholesale_prices)
      ? product.wholesale_prices
      : undefined,
  }

  const priceCents = amountToCents(sku?.price_amount)
  if (priceCents !== null && priceCents > 0) {
    patch.priceAmount = centsToAmount(priceCents)
  }

  return patch
}
