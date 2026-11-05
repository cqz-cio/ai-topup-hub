import { api } from './client'

export interface PAIPLUSCard {
  id: string
  code: string
  source_code: string
  reference: string
  order_id: number
  order_no: string
  product_id: number
  sku_id: number
  label: string
  status: string
  local_status: string
  created_at: number
  updated_at: number
  revoked_at: number
  upstream_checked_at: number
  upstream: { status: string; used: boolean; can_resubmit: boolean; plan: string; used_at: string | number } | null
  submitted: boolean
  attempt_state: string | null
}
export interface PAIPLUSDetail extends PAIPLUSCard {
  operations: Array<{ id: string; state: string; actor: string; reason: string; message: string | null; created_at: number; finished_at: number | null }>
  audit: Array<{ action: string; actor: string; at: number }>
}
export interface PAIPLUSList {
  cards: PAIPLUSCard[]
  total: number
  page: number
  page_size: number
  counts: Record<string, number>
}
const options = { cache: 'no-store', timeout: 35000 }
export const paiplusAPI = {
  async list(input: { query: string; status: string; page: number; page_size: number }) {
    const response = await api.post('/admin/card-secrets/paiplus/query', input, options)
    return response.data.data as PAIPLUSList
  },
  async reveal(card_id: string) {
    const response = await api.post('/admin/card-secrets/paiplus/reveal', { card_id }, options)
    return response.data.data as PAIPLUSDetail
  },
  async check(card_id: string) {
    const response = await api.post('/admin/card-secrets/paiplus/check', { card_id }, options)
    return response.data.data as PAIPLUSCard
  },
  async revoke(card_id: string, reason: string, request_id: string) {
    const response = await api.post('/admin/card-secrets/paiplus/revoke', { card_id, reason, request_id, confirmed: true }, options)
    return response.data.data as { card: PAIPLUSCard; destroyed: boolean; operation_id: string }
  },
}
