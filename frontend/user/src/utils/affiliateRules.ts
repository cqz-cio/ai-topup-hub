export type AffiliateRules = {
  enabled: boolean
  ratePercent: number
  rateText: string
  exampleCommission: string
  confirmDays: number
}

export const AFFILIATE_EXAMPLE_PRINCIPAL = 100

// Missing configuration must not turn into a zero rate or immediate settlement.
export function parseAffiliateRules(raw: unknown): AffiliateRules | null {
  if (!raw || typeof raw !== 'object') return null
  const value = raw as Record<string, unknown>
  if (typeof value.enabled !== 'boolean') return null
  if (!value.enabled) return { enabled: false, ratePercent: 0, rateText: '0', exampleCommission: '0.00', confirmDays: 0 }
  const rate = value.commission_rate
  const days = value.confirm_days
  if (typeof rate !== 'number' || !Number.isFinite(rate) || rate < 0 || rate > 100) return null
  if (typeof days !== 'number' || !Number.isInteger(days) || days < 0 || days > 3650) return null
  const ratePercent = Math.round((rate + Number.EPSILON) * 100) / 100
  return {
    enabled: true,
    ratePercent,
    rateText: String(ratePercent),
    exampleCommission: (AFFILIATE_EXAMPLE_PRINCIPAL * ratePercent / 100).toFixed(2),
    confirmDays: days,
  }
}
