import { amountToCents, centsToAmount } from './money.ts'

export function hasUserPriceForSku(product: any, skuId: unknown): boolean {
  const id = Number(skuId || 0)
  const sku = Array.isArray(product?.skus) ? product.skus.find((row: any) => Number(row?.id) === id) : null
  const price = amountToCents(id > 0 ? sku?.user_price_amount : product?.user_price_amount)
  return price !== null && price > 0
}

// Same precedence as checkout: fixed SKU price, fixed product price, public price.
export function resolveMemberPriceAmount(
  product: any, skuId: unknown, basePrice: unknown,
  memberLevelId: unknown, _legacyDiscountRate?: unknown,
): string | null {
  if (hasUserPriceForSku(product, skuId)) return null
  const base = amountToCents(basePrice)
  const levelId = Number(memberLevelId || 0)
  if (base === null || base <= 0 || !Number.isSafeInteger(levelId) || levelId <= 0) return null
  const prices = Array.isArray(product?.member_prices) ? product.member_prices : []
  for (const scope of [Number(skuId || 0), 0]) {
    const row = prices.find((p: any) => Number(p?.member_level_id) === levelId && Number(p?.sku_id || 0) === scope)
    const override = amountToCents(row?.price_amount)
    if (override !== null && override > 0) return override < base ? centsToAmount(override) : null
  }
  return null
}

export function resolveCatalogSkuPrice(product: any, sku: any): string | null {
  const base = amountToCents(sku?.price_amount)
  if (base === null || base <= 0) return null
  const effective = amountToCents(sku?.effective_price_amount)
  if (effective !== null && effective >= 0 && effective <= base) return centsToAmount(effective)
  const promotion = amountToCents(sku?.promotion_price_amount)
  const discounted = promotion !== null && promotion >= 0 && promotion < base ? promotion : base
  return resolveMemberPriceAmount(product, sku?.id, centsToAmount(discounted), product?.pricing_member_level_id, product?.member_discount_rate)
    ?? centsToAmount(discounted)
}
