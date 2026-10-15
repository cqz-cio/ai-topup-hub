export interface DeliveryGroup {
  orderNo: string
  items: any[]
  fulfillment: any
  delivered: boolean
}

/** Keep delivery state separate from payment state, especially for split orders. */
export function orderDeliveryGroups(order: any): DeliveryGroup[] {
  if (!order) return []
  const children = Array.isArray(order.children) ? order.children : []
  const sources = children.length ? (order.fulfillment ? [order, ...children] : children) : [order]
  return sources.map((source: any) => ({
    orderNo: String(source.order_no || order.order_no || ''),
    items: Array.isArray(source.items) ? source.items : [],
    fulfillment: source.fulfillment || null,
    delivered: source.fulfillment?.status === 'delivered',
  }))
}

export function orderDeliveryState(order: any): 'unpaid' | 'waiting' | 'partial' | 'ready' | 'closed' {
  const status = String(order?.status || '')
  if (['canceled', 'expired', 'refunded'].includes(status)) return 'closed'
  const groups = orderDeliveryGroups(order)
  if (groups.length && groups.every(group => group.delivered)) return 'ready'
  if (groups.some(group => group.delivered)) return 'partial'
  if (['paid', 'fulfilling', 'partially_delivered', 'delivered', 'completed', 'partially_refunded'].includes(status)) return 'waiting'
  return 'unpaid'
}

/** Only a confirmed supplier balance failure may show the administrator hint. */
export function hasCardPlatformBalanceIssue(order: any): boolean {
  if (!['paid', 'fulfilling', 'partially_delivered', 'partially_refunded'].includes(String(order?.status || ''))) return false
  if (order.fulfillment?.status !== 'delivered' && order.fulfillment_issue_code === 'card_platform_insufficient_balance') return true
  return Array.isArray(order.children) && order.children.some(hasCardPlatformBalanceIssue)
}

/** One toast per order and failure episode; polling must not repeat the alert. */
export function createCardPlatformBalanceNotice(notify: () => void) {
  let notifiedOrder = ''
  return (order: any) => {
    if (!order) return
    if (!hasCardPlatformBalanceIssue(order)) { notifiedOrder = ''; return }
    const orderNo = String(order.order_no || '')
    if (!orderNo || notifiedOrder === orderNo) return
    notifiedOrder = orderNo
    notify()
  }
}
