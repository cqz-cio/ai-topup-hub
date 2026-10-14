export interface UsdtPaymentSource {
  provider_type?: string
  channel_type?: string
  token_id?: string
  chain_amount?: string
}

// The payment's token is authoritative for gateways that support several coins.
// Keep the amount as text: historical payments can have more than two decimals.
export function resolveUsdtTransfer(payment?: UsdtPaymentSource | null) {
  if (!payment) return null
  const token = String(payment.token_id || '').trim().split(/[-_:]/).pop()?.toUpperCase() || ''
  if (token && token !== 'USDT' && token !== 'USDT0') return null
  const provider = String(payment.provider_type || '').trim().toLowerCase()
  const channel = String(payment.channel_type || '').trim().toLowerCase()
  const usdt = token || /^(usdt0?)(?:$|[-_])/.test(channel)
    || ['bscusdt', 'plasmausdt', 'tronusdt', 'epusdt', 'bepusdt'].includes(provider)
  if (!usdt) return null
  return {
    amount: String(payment.chain_amount || '').trim(),
    token: token || (provider === 'plasmausdt' || channel === 'usdt-plasma' ? 'USDT0' : 'USDT'),
  }
}
