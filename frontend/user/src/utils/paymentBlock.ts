export type PaymentBlock = {
  notice_id?: number
  code: 'card_platform_insufficient_balance' | 'card_platform_balance_unavailable'
  notification_status: 'sent' | 'partial' | 'pending' | 'failed' | 'not_sent'
}

export function readPaymentBlock(value: any): PaymentBlock | null {
  const block = value?.payment_block || value?.data?.payment_block
  if (!['card_platform_insufficient_balance', 'card_platform_balance_unavailable'].includes(block?.code)) return null
  const status = ['sent', 'partial', 'pending', 'failed'].includes(block.notification_status) ? block.notification_status : 'not_sent'
  return { code: block.code, notification_status: status, ...(Number.isSafeInteger(block.notice_id) && block.notice_id > 0 ? { notice_id: block.notice_id } : {}) }
}
