export const SUPPORT_WECHAT = 'Cqz0924ttxk'
export const SUPPORT_QQ = '1669268759'
export const SUPPORT_TELEGRAM = '@czq_piestar'
export const SUPPORT_TELEGRAM_URL = 'https://t.me/czq_piestar'

export function resolveSupportContacts(raw: unknown, reseller = false) {
  const input = raw && typeof raw === 'object' ? raw as Record<string, unknown> : {}
  const wechat = String(input.wechat || '').trim() || (reseller ? '' : SUPPORT_WECHAT)
  const qq = String(input.qq || '').trim() || (reseller ? '' : SUPPORT_QQ)
  const telegram = String(input.telegram || '').trim() || (reseller ? '' : SUPPORT_TELEGRAM)
  let handle = ''
  if (/^@?[a-zA-Z0-9_]{5,32}$/.test(telegram)) handle = telegram.replace(/^@/, '')
  else {
    try {
      const url = new URL(telegram)
      if (url.protocol === 'https:' && ['t.me', 'telegram.me'].includes(url.hostname) && !url.username && !url.password && /^\/[a-zA-Z0-9_]{5,32}\/?$/.test(url.pathname)) {
        handle = url.pathname.replace(/\//g, '')
      }
    } catch { /* Invalid links are omitted. */ }
  }
  return {
    wechat: /^[a-zA-Z][a-zA-Z0-9_-]{5,31}$/.test(wechat) ? wechat : '',
    qq: /^[1-9][0-9]{4,14}$/.test(qq) ? qq : '',
    telegram: handle ? '@' + handle : '',
    telegramUrl: handle ? 'https://t.me/' + handle : '',
  }
}
