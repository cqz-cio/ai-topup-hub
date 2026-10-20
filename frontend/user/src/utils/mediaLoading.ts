type Connection = { saveData?: boolean; effectiveType?: string }

export function canWarmMedia(connection?: Connection): boolean {
  return !connection?.saveData && !['slow-2g', '2g', '3g'].includes(connection?.effectiveType || '')
}

export function hasSpareBandwidth(): boolean {
  return typeof navigator !== 'undefined' &&
    canWarmMedia((navigator as Navigator & { connection?: Connection }).connection)
}

/** Wait for the current document, a visible tab and an idle slot. Always cancellable. */
export function afterPageLoad(task: () => void, delay = 800): () => void {
  let cancelled = false
  let timer: number | undefined
  let idle: number | undefined
  const cleanup = () => {
    window.removeEventListener('load', schedule)
    document.removeEventListener('visibilitychange', schedule)
    window.clearTimeout(timer)
    if (idle !== undefined) window.cancelIdleCallback?.(idle)
  }
  const run = () => {
    timer = undefined
    idle = undefined
    if (cancelled) return
    if (document.visibilityState === 'hidden') return
    cleanup()
    task()
  }
  const schedule = () => {
    if (cancelled || document.readyState !== 'complete' ||
        document.visibilityState === 'hidden' || timer !== undefined || idle !== undefined) return
    timer = window.setTimeout(() => {
      timer = undefined
      if (typeof window.requestIdleCallback === 'function') {
        idle = window.requestIdleCallback(run, { timeout: 2000 })
      } else run()
    }, delay)
  }
  window.addEventListener('load', schedule)
  document.addEventListener('visibilitychange', schedule)
  schedule()
  return () => { cancelled = true; cleanup() }
}

export function catalogWarmupUrls(urls: string[]): string[] {
  // Only public local catalog assets. Never fetch orders, delivery data or remote URLs.
  return [...new Set(urls.filter(url =>
    /^\/(?:catalog-products\/|media\/storefront-20261020\/)/.test(url),
  ))].slice(0, 24)
}

/** One low-priority image at a time, after the first screen has loaded. */
export function warmCatalogImages(urls: string[]): () => void {
  if (!hasSpareBandwidth()) return () => {}
  const queue = catalogWarmupUrls(urls)
  let cancelled = false
  let cancelScheduled = () => {}
  let active: HTMLImageElement | undefined
  let timeout: number | undefined
  const next = () => {
    if (cancelled || !hasSpareBandwidth()) return
    const url = queue.shift()
    if (!url) return
    const img = new Image()
    active = img
    img.decoding = 'async'
    img.fetchPriority = 'low'
    const finish = () => {
      window.clearTimeout(timeout)
      img.onload = img.onerror = null
      active = undefined
      if (!cancelled) cancelScheduled = afterPageLoad(next, 150)
    }
    img.onload = img.onerror = finish
    timeout = window.setTimeout(() => {
      // A stalled download must not grow into several competing requests.
      cancelled = true
      img.onload = img.onerror = null
      active = undefined
    }, 12000)
    img.src = url
  }
  cancelScheduled = afterPageLoad(next, 1200)
  return () => {
    cancelled = true
    cancelScheduled()
    window.clearTimeout(timeout)
    if (active) active.onload = active.onerror = null
  }
}
