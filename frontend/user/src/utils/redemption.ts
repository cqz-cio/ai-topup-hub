export function normalizeRedemptionCode(input: string): string | null {
  if (!/^[A-Za-z2-9-]+$/.test(input.trim())) return null
  const value = input.trim().toUpperCase()
  if (!/^(?:[A-HJ-NP-Z2-9]{25}|[A-HJ-NP-Z2-9]{5}(?:-[A-HJ-NP-Z2-9]{5}){4})$/.test(value)) return null
  const raw = value.replace(/-/g, '')
  if (!/[A-Z]/.test(raw) || !/[2-9]/.test(raw)) return null
  return raw.match(/.{5}/g)!.join('-')
}

export function isRedemptionPath(path: string): boolean {
  return /(?:^|\/)redeem\/?$/i.test(path)
}

export function validSession(input: string): boolean {
  if (new TextEncoder().encode(input).length > 65536) return false
  try {
    const v = JSON.parse(input)
    return typeof v?.accessToken === 'string' && !!v.accessToken.trim()
      && typeof v?.sessionToken === 'string' && !!v.sessionToken.trim()
  } catch { return false }
}

export function planLabel(plan: string): string {
  return ({ chatgptplusplan: 'ChatGPT Plus', chatgptprolite: 'ChatGPT Pro 5x', chatgptpro: 'ChatGPT Pro 20x' } as Record<string, string>)[plan] || '订阅套餐'
}

export function subscriptionLabel(plan: string): string {
  return ({ free: '免费版', plus: 'ChatGPT Plus', pro: 'ChatGPT Pro',
    chatgptplusplan: 'ChatGPT Plus', chatgptprolite: 'ChatGPT Pro 5x', chatgptpro: 'ChatGPT Pro 20x',
    team: 'ChatGPT Team', business: 'ChatGPT Business', enterprise: 'ChatGPT Enterprise' } as Record<string, string>)[plan] || '其他订阅（类型待核实）'
}

export function needsAccountCheck(state: string): boolean {
  return state === 'issued' || state === 'awaiting_confirmation'
}

const processing = new Set(['issuing', 'waiting_card', 'waiting_configuration', 'waiting_session', 'submitting', 'queued', 'running', 'stopping', 'action_required', 'unknown', 'confirming_subscription', 'succeeded'])
export function shouldPoll(state: string): boolean { return processing.has(state) }

export function statusPresentation(state: string): { title: string; detail: string; tone: string } {
  switch (state) {
    case 'completed': return { title: '充值成功', detail: '订阅已确认到账，本次卡密已完成兑换。你可以回到 ChatGPT 查看订阅。', tone: 'success' }
    case 'failed': return { title: '本次充值未完成', detail: '请联系商城客服核实原兑换记录。卡密不会自动重新扣款，请勿再次购买或重复提交。', tone: 'warning' }
    case 'manual_review': return { title: '需要客服协助', detail: '本次兑换需要进一步核实，请保管好卡密并联系商城客服。系统不会自动再次扣款。', tone: 'warning' }
    case 'canceled': return { title: '兑换已停止', detail: '本次兑换已停止处理，请联系商城客服了解后续处理方式。', tone: 'warning' }
    case 'unknown': return { title: '正在核实充值结果', detail: '暂时无法确认最终结果。我们会继续核实原记录，请勿重复充值。', tone: 'warning' }
    case 'action_required': return { title: '等待商家完成验证', detail: '商家正在处理付款验证，你无需提供银行卡资料。完成后将继续更新进度。', tone: 'pending' }
    case 'waiting_card': case 'waiting_configuration': case 'waiting_session':
      return { title: '充值申请已确认', detail: '订单正在等待商家处理。你可以离开页面，稍后凭卡密查询进度。', tone: 'pending' }
    case 'confirming_subscription': case 'succeeded':
      return { title: '正在确认订阅到账', detail: '付款结果已收到，正在确认订阅状态。无需再次提交充值。', tone: 'pending' }
    case 'issued': case 'awaiting_confirmation':
      return { title: '尚未开始充值', detail: '请重新核验账号，并确认目标账号后再提交。', tone: 'pending' }
    case 'issuing': return { title: '正在准备兑换记录', detail: '请稍后查询当前卡密的状态。', tone: 'pending' }
    case 'submitting': case 'queued': case 'running': case 'stopping':
      return { title: '正在为你充值', detail: '充值处理中，结果会自动更新。请勿重复提交。', tone: 'pending' }
    default: return { title: '状态需要进一步核实', detail: '暂时无法识别当前结果，请联系商城客服核实原兑换记录。', tone: 'warning' }
  }
}
