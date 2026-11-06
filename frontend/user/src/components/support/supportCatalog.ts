import { normalizeSupportProducts, type SupportProduct } from './supportConversation'

// Public catalog access only. No order endpoint, customer credential or model call.
export async function loadSupportCatalog(locale: string, signal: AbortSignal): Promise<SupportProduct[]> {
  const controller = new AbortController()
  const abort = () => controller.abort()
  if (signal.aborted) controller.abort()
  signal.addEventListener('abort', abort, { once: true })
  const timer = setTimeout(abort, 8000)
  const products: SupportProduct[] = []
  try {
    let pages = 1
    for (let page = 1; page <= pages; page++) {
      const base = import.meta.env.VITE_API_BASE_URL || ''
      const response = await fetch(`${base}/api/v1/public/products?page=${page}&page_size=100`, {
        method: 'GET', credentials: 'omit', signal: controller.signal, headers: { 'X-Lang': locale },
      })
      if (!response.ok) throw new Error('Public catalog unavailable')
      const data = await response.json()
      if (!Array.isArray(data?.data) || (data.status_code !== undefined && data.status_code !== 0)) throw new Error('Invalid public catalog')
      const totalPages = data.pagination?.total_page ?? 1
      if (!Number.isInteger(totalPages) || totalPages < 0 || totalPages > 5) throw new Error('Unsupported catalog pagination')
      pages = Math.max(1, totalPages)
      products.push(...normalizeSupportProducts(data.data, locale))
    }
    return [...new Map(products.map(product => [product.slug, product])).values()]
  } finally {
    clearTimeout(timer)
    signal.removeEventListener('abort', abort)
  }
}
