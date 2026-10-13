// Recharge APIs return a flattened payment payload; older integrations may
// return a nested payment object. Preserve the exact crypto amount as text.
export function normalizeRechargePayment(payload: any) {
  if (payload?.payment) return payload.payment
  if (payload?.payment_id == null) return undefined
  return {
    id: payload.payment_id,
    provider_type: payload.provider_type,
    channel_type: payload.channel_type,
    interaction_mode: payload.interaction_mode,
    pay_url: payload.pay_url,
    qr_code: payload.qr_code,
    expires_at: payload.expires_at,
    status: payload.status,
    fee_policy: payload.fee_policy,
    wallet_address: payload.wallet_address,
    chain_amount: payload.chain_amount,
    chain: payload.chain,
    token_id: payload.token_id,
  }
}
