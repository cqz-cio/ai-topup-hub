export const isBep20PaymentChannel = (channel?: {
  provider_type?: string
  channel_type?: string
} | null): boolean => (
  String(channel?.provider_type || '').toLowerCase() === 'bscusdt'
  && String(channel?.channel_type || '').toLowerCase() === 'usdt-bep20'
)
