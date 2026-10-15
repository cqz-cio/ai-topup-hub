import type { CartItem } from '../stores/cart'

export const BUY_NOW_SESSION_KEY = 'buy_now_item_v1'
export const BUY_NOW_MAX_AGE = 24 * 60 * 60 * 1000
type SessionStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

export function buyNowSessionStorage(): SessionStorage | undefined {
  try { return window.sessionStorage } catch { return undefined }
}

// Only retain the selected product snapshot. Checkout credentials and entered
// manual-form values never belong in this session record.
function selectedItem(value: unknown): CartItem | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  const row = value as Record<string, unknown>
  const positiveInteger = (n: unknown) => typeof n === 'number' && Number.isSafeInteger(n) && n > 0
  if (!positiveInteger(row.productId) || !positiveInteger(row.quantity)) return null
  if (row.skuId !== undefined && !positiveInteger(row.skuId)) return null
  if (typeof row.slug !== 'string' || !row.slug.trim() || row.slug.length > 256) return null
  if (typeof row.priceAmount !== 'string' || !/^\d+(\.\d+)?$/.test(row.priceAmount) || !Number.isFinite(Number(row.priceAmount))) return null
  const title = typeof row.title === 'string' ? row.title
    : row.title && typeof row.title === 'object' && !Array.isArray(row.title)
      ? Object.fromEntries(Object.entries(row.title).filter(([, text]) => typeof text === 'string')) : null
  if (!title) return null
  const item: CartItem = {
    productId: row.productId as number, quantity: row.quantity as number,
    slug: row.slug, priceAmount: row.priceAmount, title,
  }
  const target = item as unknown as Record<string, unknown>
  for (const key of ['skuId', 'skuManualStockTotal', 'skuManualStockLocked', 'skuManualStockSold',
    'skuAutoStockAvailable', 'skuUpstreamStock', 'skuStockRangeMin', 'skuStockRangeMax',
    'pricingUserId', 'memberLevelId', 'minPurchaseQuantity', 'maxPurchaseQuantity']) {
    if (typeof row[key] === 'number' && Number.isFinite(row[key])) target[key] = row[key]
  }
  for (const key of ['skuCode', 'skuStockStatus', 'skuStockDisplayMode', 'skuStockDisplay',
    'skuStockSnapshotAt', 'displayPriceAmount', 'userPriceAmount', 'promotionPriceAmount',
    'memberDiscountRate', 'image', 'purchaseType', 'fulfillmentType']) {
    if (typeof row[key] === 'string') target[key] = row[key]
  }
  for (const key of ['skuStockQuantityHidden', 'skuStockEnforced']) {
    if (typeof row[key] === 'boolean') target[key] = row[key]
  }
  for (const key of ['skuSpecValues', 'manualFormSchema']) {
    if (row[key] && typeof row[key] === 'object' && !Array.isArray(row[key])) target[key] = row[key]
  }
  for (const key of ['memberPrices', 'wholesalePrices']) {
    if (Array.isArray(row[key])) target[key] = row[key]
  }
  if (Array.isArray(row.paymentChannelIds)) item.paymentChannelIds = row.paymentChannelIds.filter(positiveInteger) as number[]
  return item
}

export function clearBuyNowSession(storage = buyNowSessionStorage()) {
  try { storage?.removeItem(BUY_NOW_SESSION_KEY) } catch { /* Storage may be disabled. */ }
}

export function readBuyNowSession(storage = buyNowSessionStorage(), now = Date.now()): CartItem | null {
  try {
    const raw = storage?.getItem(BUY_NOW_SESSION_KEY)
    if (!raw) return null
    if (raw.length > 128_000) throw new Error('Invalid snapshot size')
    const saved = JSON.parse(raw)
    if (saved?.version !== 1 || !Number.isFinite(saved.savedAt)
      || saved.savedAt > now || now - saved.savedAt >= BUY_NOW_MAX_AGE) throw new Error('Expired snapshot')
    const item = selectedItem(saved.item)
    if (!item) throw new Error('Invalid selected product')
    return item
  } catch {
    clearBuyNowSession(storage)
    return null
  }
}

export function saveBuyNowSession(value: CartItem, storage = buyNowSessionStorage(), now = Date.now()) {
  try {
    const item = selectedItem(value)
    if (!item) throw new Error('Invalid selected product')
    const raw = JSON.stringify({ version: 1, savedAt: now, item })
    if (raw.length > 128_000) throw new Error('Invalid snapshot size')
    storage?.setItem(BUY_NOW_SESSION_KEY, raw)
  } catch {
    // A failed replacement must not restore an earlier, different product.
    clearBuyNowSession(storage)
  }
}
