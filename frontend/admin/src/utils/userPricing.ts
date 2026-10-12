export interface UserPriceInput {
  product_id: number
  sku_id: number
  price_amount: string | number
}

export interface UserPriceDraft {
  product_id: number
  sku_id: number
  price: string
}

// Merge visible edits with saved prices, including products outside the current list.
export function buildUserPricePayload(saved: UserPriceInput[], drafts: UserPriceDraft[]) {
  const prices = new Map<string, { product_id: number; sku_id: number; price_amount: string }>()
  for (const row of saved) {
    prices.set(`${row.product_id}:${row.sku_id}`, { ...row, price_amount: String(row.price_amount) })
  }
  const seen = new Set<string>()
  for (const row of drafts) {
    const key = `${row.product_id}:${row.sku_id}`
    if (!Number.isSafeInteger(row.product_id) || row.product_id <= 0 ||
      !Number.isSafeInteger(row.sku_id) || row.sku_id < 0 || seen.has(key)) throw new Error('invalid_price')
    seen.add(key)
    const text = row.price.trim()
    if (!text) {
      prices.delete(key)
      continue
    }
    if (!/^\d{1,10}(\.\d{1,2})?$/.test(text) || Number(text) <= 0) throw new Error('invalid_price')
    prices.set(key, { product_id: row.product_id, sku_id: row.sku_id, price_amount: Number(text).toFixed(2) })
  }
  return { prices: [...prices.values()] }
}
