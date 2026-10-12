export const isBep20PaymentChannel = (channel?: {
  provider_type?: string
  channel_type?: string
} | null): boolean => (
  String(channel?.provider_type || '').toLowerCase() === 'bscusdt'
  && String(channel?.channel_type || '').toLowerCase() === 'usdt-bep20'
)

export const directUSDTPaymentInfo = (providerType?: string) => {
  const provider = String(providerType || '').trim().toLowerCase()
  const network = provider === 'bscusdt' ? 'BNB Smart Chain（BEP-20）'
    : provider === 'plasmausdt' ? 'Plasma' : ''
  if (!network) return null
  return {
    token: provider === 'plasmausdt' ? 'USDT0' : 'USDT',
    title: provider === 'plasmausdt' ? '使用 USDT · Plasma 转账' : '使用 USDT · BEP-20 转账',
    tip: `请选择 ${network}，在订单到期前按下方完整 USDT 金额转账，不要省略小数尾数；尾数用于识别订单。请确保扣除平台手续费后的到账金额一致。链上确认后订单会自动更新为已付款，请在订单详情查看发货进度，请勿重复转账。`
      + (provider === 'plasmausdt' ? 'Plasma 链上的代币名称为 USDT0。' : ''),
  }
}
