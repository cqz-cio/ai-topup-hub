export function paymentDeadlineExpired(values: unknown[], now: number, wholeSeconds = false): boolean {
  const times = values.filter(v => typeof v === 'string' && v.trim()).map(v => Date.parse(v as string)).filter(Number.isFinite)
  if (!times.length) return false
  const deadline = Math.min(...times)
  return now >= (wholeSeconds ? Math.floor(deadline / 1000) * 1000 : deadline)
}

const messages = {
  'zh-CN': {
    active: '请在倒计时结束前完成支付。超时后订单失效，请重新下单，并严格按照新订单显示的金额和地址付款。请勿继续支付已过期订单，超时付款不予处理。',
    expired: '付款时间已结束，请重新下单，并使用新订单显示的金额和地址付款。请勿支付已过期订单，超时付款不予处理。',
  },
  'zh-TW': {
    active: '請在倒數結束前完成付款。逾時後訂單失效，請重新下單，並嚴格按照新訂單顯示的金額和地址付款。請勿繼續支付已過期訂單，逾時付款不予處理。',
    expired: '付款時間已結束，請重新下單，並使用新訂單顯示的金額和地址付款。請勿支付已過期訂單，逾時付款不予處理。',
  },
  'en-US': {
    active: 'Complete payment before the countdown ends. Once the order expires, place a new order and pay the exact amount to the address shown on the new order. Do not pay expired orders. Late payments will not be processed.',
    expired: 'The payment window has ended. Place a new order and use its payment amount and address. Do not pay expired orders. Late payments will not be processed.',
  },
} as const
export function usdtPaymentWindowMessage(locale: string, expired = false): string {
  const copy = messages[locale as keyof typeof messages] || messages['zh-CN']
  return expired ? copy.expired : copy.active
}
