export interface StorefrontProduct {
  id: number
  slug: string
  title: unknown
  description?: unknown
  category?: { name?: unknown }
  images?: string[] | { images?: string[] }
  price_amount: unknown
  promotion_price_amount?: unknown
  fulfillment_type?: string
  purchase_type?: string
  is_sold_out?: boolean
  stock_status?: string
  skus?: Array<{
    is_active?: boolean
    is_sold_out?: boolean
    stock_status?: string
  }>
}

export function storefrontText(value: unknown, locale = 'zh-CN'): string {
  if (typeof value === 'string') return value.trim()
  if (!value || typeof value !== 'object') return ''
  const localized = value as Record<string, unknown>
  for (const key of [locale, 'zh-CN', 'en-US']) {
    const text = localized[key]
    if (typeof text === 'string' && text.trim()) return text.trim()
  }
  return ''
}

const parseAmount = (value: unknown): number | null => {
  if (typeof value !== 'number' && typeof value !== 'string') return null
  if (typeof value === 'string' && !/^\d+(\.\d+)?$/.test(value.trim()))
    return null
  const amount = Number(value)
  return Number.isFinite(amount) && amount >= 0 ? amount : null
}

export function storefrontPrice(product: StorefrontProduct): number | null {
  const base = parseAmount(product.price_amount)
  if (base === null) return null
  const promotion = parseAmount(product.promotion_price_amount)
  return promotion !== null && promotion < base ? promotion : base
}

export function canPurchaseStorefrontProduct(
  product: StorefrontProduct,
): boolean {
  if (!product.slug || storefrontPrice(product) === null) return false
  const stocked = (row: { is_sold_out?: boolean; stock_status?: string }) =>
    row.is_sold_out === false &&
    ['in_stock', 'low_stock', 'unlimited'].includes(row.stock_status || '')
  if (!stocked(product)) return false
  if (product.skus?.length) {
    return product.skus.some((sku) => sku.is_active === true && stocked(sku))
  }
  return true
}

export interface StorefrontProductPage {
  status_code: number
  data: StorefrontProduct[]
  pagination?: { total_page: number }
}

// Read every public page so categories/search also include newly added products.
// A failed page rejects the entire refresh, rather than leaving stale buy links.
export async function loadStorefrontProducts(
  readPage: (page: number) => Promise<StorefrontProductPage>,
): Promise<StorefrontProduct[]> {
  const result = new Map<string, StorefrontProduct>()
  let totalPages = 1
  for (let page = 1; page <= totalPages; page++) {
    const response = await readPage(page)
    if (response.status_code !== 0 || !Array.isArray(response.data))
      throw new Error('Invalid public product response')
    if (page === 1 && response.pagination) {
      const pages = response.pagination.total_page
      if (!Number.isSafeInteger(pages) || pages < 0)
        throw new Error('Invalid public product pagination')
      totalPages = Math.max(1, pages)
    }
    for (const product of response.data) {
      if (
        typeof product?.slug === 'string' &&
        product.slug.trim() &&
        Number.isSafeInteger(product.id) &&
        product.id > 0
      ) {
        result.set(product.slug, product)
      }
    }
  }
  return [...result.values()]
}
