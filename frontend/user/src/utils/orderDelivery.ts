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
