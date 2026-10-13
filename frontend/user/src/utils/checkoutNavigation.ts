import type { Router } from 'vue-router'

export const checkoutReturnPath = (items: readonly { slug: string }[]): string => {
  const slugs = new Set(items.map(item => item.slug.trim()))
  const slug = slugs.size === 1 ? slugs.values().next().value : ''
  return slug && slug !== '.' && slug !== '..'
    ? `/products/${encodeURIComponent(slug)}`
    : '/products'
}

export const completedCheckoutReturnPath = (
  targetPath: string,
  historyLocation: string,
  historyState: Record<string, unknown>,
): string | null => {
  // Only consume the history entry being restored. A new checkout must remain usable.
  if (targetPath.split('?')[0] !== '/checkout' || historyLocation !== targetPath) return null
  const returnPath = historyState.checkoutReturnPath
  return typeof returnPath === 'string' && /^\/products(?:\/[^/\\?#]+)?$/.test(returnPath)
    ? returnPath
    : null
}

export const openCheckoutPayment = (router: Router, paymentPath: string, returnPath: string) => {
  const history = router.options.history
  if (history.state.back === returnPath) {
    // Buy now: remove the completed checkout so Back returns to this product directly.
    return router.replace(paymentPath)
  }

  // Cart/direct entry: retain a product destination on this exact checkout entry.
  // Browser history survives payment-page reloads without a global or cross-tab cache.
  history.replace(router.currentRoute.value.fullPath, { checkoutReturnPath: returnPath })
  return router.push(paymentPath)
}
