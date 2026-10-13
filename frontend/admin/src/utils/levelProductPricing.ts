import { buildUserPricePayload, type UserPriceDraft, type UserPriceInput } from './userPricing.ts'

// Share amount validation and preserve prices outside the visible catalog.
export function buildLevelProductPricePayload(levelId: number, saved: UserPriceInput[], drafts: UserPriceDraft[]) {
  if (!Number.isSafeInteger(levelId) || levelId <= 0) throw new Error('invalid_price')
  const { prices } = buildUserPricePayload(saved, drafts)
  return {
    member_level_id: levelId,
    prices: prices.map((row) => ({
      product_id: row.product_id, sku_id: row.sku_id,
      member_level_id: levelId, price_amount: Number(row.price_amount),
    })),
  }
}
