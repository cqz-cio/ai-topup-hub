export interface RedemptionAccount {
  account_id: string
  current_plan: string
}

export interface RedemptionRecord {
  state: string
  plan: string
  reason?: string
  account?: RedemptionAccount
  confirmation_token?: string
  checked_until?: string
}

const messages: Record<string, string> = {
  recharge_not_found: '未找到可用兑换记录，请核对卡密或联系商城客服。',
  invalid_recharge_request: '核验信息已失效或不完整，请重新核验账号。',
  recharge_busy: '这张卡密正在处理，请稍后查询原兑换进度。',
  recharge_already_started: '该卡密已进入处理流程，请查询原兑换进度。',
  recharge_unavailable: '暂时无法完成核验，请稍后重试或联系商城客服。',
  rate_limited: '操作较频繁，请等待一分钟后重试。',
  network: '连接暂时中断，请稍后查询进度。请勿重复提交充值。',
  unavailable: '兑换服务暂时不可用，请稍后重试。',
}

export class RedemptionError extends Error {
  readonly code: string
  constructor(code: string) {
    super(messages[code] || messages.unavailable)
    this.code = code
    this.name = 'RedemptionError'
  }
}

function parseRecord(value: unknown): RedemptionRecord {
  if (!value || typeof value !== 'object') throw new RedemptionError('unavailable')
  const v = value as Record<string, unknown>
  if (typeof v.state !== 'string' || typeof v.plan !== 'string') throw new RedemptionError('unavailable')
  const record: RedemptionRecord = { state: v.state, plan: v.plan }
  if (typeof v.reason === 'string') record.reason = v.reason
  if (v.account && typeof v.account === 'object') {
    const account = v.account as Record<string, unknown>
    if (typeof account.account_id === 'string' && typeof account.current_plan === 'string') {
      record.account = { account_id: account.account_id, current_plan: account.current_plan }
    }
  }
  if (typeof v.confirmation_token === 'string') record.confirmation_token = v.confirmation_token
  if (typeof v.checked_until === 'string') record.checked_until = v.checked_until
  return record
}

// Separate from the storefront client: no login token, cookie, raw error log,
// redirect, URL credentials or automatic retry for a confirmation request.
export function createRedemptionAPI(base = '', fetcher: typeof fetch = fetch) {
  async function post(path: string, body: Record<string, unknown>, outerSignal?: AbortSignal) {
    const controller = new AbortController()
    const abort = () => controller.abort()
    outerSignal?.addEventListener('abort', abort, { once: true })
    if (outerSignal?.aborted) controller.abort()
    const timer = setTimeout(abort, path === 'account' ? 35000 : 15000)
    try {
      const response = await fetcher(`${base.replace(/\/$/, '')}/api/v1/recharge/redemptions/${path}`, {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body), signal: controller.signal,
        credentials: 'omit', cache: 'no-store', redirect: 'error', referrerPolicy: 'no-referrer',
      })
      if (response.status === 429) throw new RedemptionError('rate_limited')
      let payload: { data?: unknown; code?: unknown }
      try { payload = await response.json() } catch { throw new RedemptionError('unavailable') }
      if (!response.ok) {
        throw new RedemptionError(typeof payload?.code === 'string' && messages[payload.code] ? payload.code : 'unavailable')
      }
      return parseRecord(payload?.data)
    } catch (error) {
      if (error instanceof RedemptionError) throw error
      throw new RedemptionError('network')
    } finally {
      clearTimeout(timer)
      outerSignal?.removeEventListener('abort', abort)
    }
  }
  return {
    lookup: (code: string, signal?: AbortSignal) => post('lookup', { code }, signal),
    account: (code: string, session: string, signal?: AbortSignal) => post('account', { code, session, authorized: true }, signal),
    confirm: (code: string, token: string, signal?: AbortSignal) => post('confirm', { code, confirmation_token: token, confirmed: true }, signal),
  }
}
