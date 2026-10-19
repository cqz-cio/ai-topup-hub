import type { SupportLink } from './supportConversation'

export type AgentReply = { text: string; links?: SupportLink[]; handoff?: boolean; mode: 'local' | 'model' }
export class SupportError extends Error {
  readonly code: string
  constructor(code: string) { super(code); this.code = code }
}

export async function askSupport(message: string, locale: string, conversationId: string, productSlug: string, signal: AbortSignal): Promise<AgentReply> {
  const timeout = AbortSignal.timeout(48000)
  const combined = AbortSignal.any([signal, timeout])
  // The session is opaque and HttpOnly. No API key, model settings, system prompt,
  // arbitrary history or authenticated store requests are sent by this client.
  const session = await fetch('/api/v1/support/session', { credentials: 'same-origin', signal: combined })
  if (!session.ok) throw new SupportError('unavailable')
  const response = await fetch('/api/v1/support/chat', {
    method: 'POST', credentials: 'same-origin', signal: combined,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ request_id: crypto.randomUUID(), conversation_id: conversationId, message, locale, product_slug: productSlug }),
  })
  const result = await response.json().catch(() => null)
  if (!response.ok) throw new SupportError(typeof result?.error === 'string' ? result.error : 'unavailable')
  if (!result || typeof result.text !== 'string' || !['local', 'model'].includes(result.mode)) throw new SupportError('unavailable')
  // Navigation never trusts generated links: both server and client allow only these store routes.
  const links = Array.isArray(result.links) ? result.links.filter((link: unknown): link is SupportLink => {
    if (!link || typeof link !== 'object') return false
    const value = link as SupportLink
    return typeof value.label === 'string' && typeof value.to === 'string' && /^(?:\/products(?:\/[a-zA-Z0-9][a-zA-Z0-9_-]{0,119})?|\/me\/(?:orders|wallet)|\/guest\/orders|\/blog\/(?:binance|okx)-usdt-payment-guide)$/.test(value.to)
  }).slice(0, 4) : undefined
  return { text: result.text, links, handoff: result.handoff === true, mode: result.mode }
}

export function agentCopy(locale: string) {
  if (locale.startsWith('en')) return { subtitle: 'Support · Offline rules', newQuestion: 'New conversation', footer: 'Rules and estimates · No model calls', intro: 'Offline trial: confirmed rules and refund estimates. Unclear cases go to staff; no AI model is called.', placeholder: 'Tell Pi what you need help with…', unavailable: 'I couldn’t get a verified answer just now. Please try again later, or use Contact support. This has not changed your order or payment.', limit: 'The trial allowance or rate limit has been reached. Please try later, or contact a person.', expired: 'The conversation expired. Send your question again to start a new conversation.' }
  if (/tw|hk|hant/i.test(locale)) return { subtitle: '客服 · 離線規則試用', newQuestion: '開始新會話', footer: '規則解答與試算 · 不呼叫 AI', intro: '目前為離線規則試用，可查詢已確認規則及試算退款；不確定的事項請人工核實，不呼叫 AI。', placeholder: '直接說說遇到了什麼問題…', unavailable: '這次暫時沒能取得可靠回覆，請稍後再試，或點擊「聯絡人工」。你的訂單及付款沒有變動。', limit: '本機試用已達頻率或當日額度限制，請稍後再試，或聯絡人工。', expired: '這段對話已過期，請重新傳送問題，開始新的對話。' }
  return { subtitle: '客服 · 离线规则试用', newQuestion: '开始新会话', topics: ['退款试算', '充值资格', '兑换异常', '联系人工'], hints: ['按实际时间计算', '区分商品规则', '准备核实材料', '07:00—24:00'], questions: ['GPT退款试算', 'Gemini激活资格是什么', '兑换失败怎么办', '人工'], footer: '规则解答与试算 · 不调用 AI', intro: '当前为离线规则试用，可查询已确认规则及试算退款；不确定的事项请人工核实，不调用 AI。', placeholder: '直接说说遇到了什么问题…', unavailable: '这次暂时没能取得可靠回复，请稍后再试，或点击“联系人工”。你的订单及付款没有变动。', limit: '本地试用已达频率或当日额度限制，请稍后再试，或联系人工。', expired: '这段对话已过期，请重新发送问题，开始新的对话。' }
}
export function agentError(error: unknown, locale: string): string {
  const copy = agentCopy(locale)
  if (error instanceof SupportError && ['rate_limit', 'daily_limit', 'busy'].includes(error.code)) return copy.limit
  if (error instanceof SupportError && error.code === 'session_expired') return copy.expired
  return copy.unavailable
}
