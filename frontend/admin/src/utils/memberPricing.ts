export interface MemberPriceRow {
  member_level_id: number
  sku_id: number
  price: string
}

export function createMemberPriceRows(
  levels: Array<{ id: number }>, skus: Array<{ id: number }>,
  prices: Array<{ member_level_id: number; sku_id: number; price_amount: number | string }>,
): MemberPriceRow[] {
  return levels.flatMap((level) => [0, ...skus.map((sku) => sku.id)].map((skuId) => {
    const price = prices.find((p) => p.member_level_id === level.id && p.sku_id === skuId)
    return {
      member_level_id: level.id,
      sku_id: skuId,
      price: price ? String(price.price_amount) : '',
    }
  }))
}

// An empty input clears the override. Reject invalid amounts before any request.
export function buildMemberPricePayload(productId: number, rows: MemberPriceRow[]) {
  if (!Number.isSafeInteger(productId) || productId <= 0) throw new Error('invalid_price')
  const seen = new Set<string>()
  const prices = []
  for (const row of rows) {
    const text = row.price.trim()
    if (!text) continue
    const key = `${row.member_level_id}:${row.sku_id}`
    const amount = Number(text)
    if (!/^\d+(\.\d{1,2})?$/.test(text) || amount <= 0 || !Number.isSafeInteger(Math.round(amount * 100)) ||
      !Number.isSafeInteger(row.member_level_id) || row.member_level_id <= 0 ||
      !Number.isSafeInteger(row.sku_id) || row.sku_id < 0 || seen.has(key)) throw new Error('invalid_price')
    seen.add(key)
    prices.push({ product_id: productId, member_level_id: row.member_level_id, sku_id: row.sku_id, price_amount: amount })
  }
  return { product_id: productId, prices }
}
