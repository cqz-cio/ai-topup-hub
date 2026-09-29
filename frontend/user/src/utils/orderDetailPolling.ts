const pendingStatuses = new Set(['pending_payment', 'paid', 'fulfilling', 'partially_delivered'])

export function needsOrderDetailRefresh(order: { status?: string } | null): boolean {
  return pendingStatuses.has(order?.status || '')
}

// Wait for each request to finish before scheduling the next one. Delivery may
// happen after the payment page has already redirected to the order detail.
export function createOrderDetailPolling(refresh: () => Promise<void>, shouldPoll: () => boolean) {
  let timer: ReturnType<typeof setTimeout> | null = null
  let refreshing = false
  let stopped = false

  const schedule = () => {
    if (stopped || refreshing || timer !== null || !shouldPoll()) return
    timer = setTimeout(async () => {
      timer = null
      if (stopped || !shouldPoll()) return
      refreshing = true
      try {
        await refresh()
      } catch {
        // A temporary network failure must not stop waiting for delivery.
      } finally {
        refreshing = false
        schedule()
      }
    }, 5000)
  }

  const stop = () => {
    stopped = true
    if (timer !== null) clearTimeout(timer)
    timer = null
  }

  return { schedule, stop }
}
