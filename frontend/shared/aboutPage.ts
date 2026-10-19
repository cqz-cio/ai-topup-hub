export const aboutCopy = {
  eyebrow: '关于我们',
  title: '关于 π星卡社',
  subtitle: '让数字商品的选购与使用更简单。',
  introductionTitle: '认识 π星卡社',
  introduction: 'π星卡社专注于 AI 工具相关数字商品，提供 ChatGPT、Gemini 等商品的在线选购与使用指引。我们希望通过清晰的商品说明、便捷的订单查询和及时的沟通，帮助你找到适合自己的套餐。',
  servicesTitle: '从选购到使用，都有清晰指引',
  services: [
    { title: '购买前，确认适用套餐', text: '不确定商品区别、套餐规格或账号是否适用，可以先联系客服咨询。下单前请核对商品详情中的开通条件和使用限制。' },
    { title: '付款后，查看交付进度', text: '标注“自动发货”的商品，系统确认付款后自动处理。卡密与交付状态可在订单中查看；如状态长时间未更新，请提供订单号联系客服核查。' },
    { title: '收到卡密，按指引兑换', text: 'ChatGPT 类商品为兑换码，非成品账号。收到卡密后，请通过订单交付说明中的兑换入口操作，以兑换结果为准；其他商品遵循各自的交付说明。' },
    { title: '遇到异常，协助核查处理', text: '卡密校验失败、兑换报错或充值状态异常时，请提供订单号、问题描述及必要截图。具体质保期限、适用范围和处理方式以对应商品说明为准。' },
  ],
  rechargeTitle: 'ChatGPT 充值与续费说明',
  recharge: [
    { title: '普通续费，请到期后再办理', text: '普通 ChatGPT 续费不支持提前办理。账号到期后，请按商品详情核对开通条件；“10x 续费绑卡”和 iOS 覆盖充值按各自商品说明办理。' },
    { title: 'iOS 续充，请联系客服手动处理', text: 'iOS 订阅到期后，账号可能仍处于宽限期。需要继续通过 iOS 渠道续充，请先联系客服核对账号状态，并由客服协助手动充值。' },
    { title: '充值方式与质保补充', text: 'ChatGPT 订阅采用官方直充方式，正常情况下可持续使用至到期。Go 与 iOS 商品提供一个月质保；质保期内如提前掉订阅，经客服核实后按剩余天数退款。' },
  ],
  rechargeNote: '其他开通条件与售后细则请查看商品详情；Gemini 按所购商品的独立约定执行。',
  cooperationTitle: '代理与长期合作',
  cooperation: '欢迎个人代理与团队咨询代理价格、批量采购及长期合作方式，具体以双方确认的方案为准。',
  cooperationNote: '如需开通分销，可登录后提交申请，审核通过后再配置店铺。',
  contactTitle: '有问题，随时联系我们',
  contactSubtitle: '商品咨询 · 订单售后 · 代理合作',
  contactNote: '微信、QQ 和 Telegram 均支持留言咨询。如未及时回复，请留下订单号和问题描述，我们会在客服在线后尽快处理。',
  contactTip: '咨询订单问题时，请一并提供订单号与必要截图，方便核查。',
  links: [
    { title: '购买须知与代理合作', to: '/blog/purchase-and-agent-guide' },
    { title: '隐私政策', to: '/privacy' },
    { title: '服务条款', to: '/terms' },
  ],
}

export const defaultSupportWechat = 'Cqz0924ttxk'
const defaultCopy = {
  ...aboutCopy,
  wechatLabel: '客服微信',
  copyLabel: '复制微信号',
  copiedLabel: '已复制微信号',
  linksLabel: '购买与服务说明',
  telegramLabel: 'Telegram',
  whatsappLabel: 'WhatsApp',
}
export const aboutLanguages = ['zh-CN', 'zh-TW', 'en-US'] as const
export type AboutLanguage = typeof aboutLanguages[number]
export type LocalizedText = Record<AboutLanguage, string>
export const aboutTextFields = [
  'eyebrow', 'title', 'subtitle', 'introductionTitle', 'introduction', 'servicesTitle',
  'rechargeTitle', 'rechargeNote', 'cooperationTitle', 'cooperation', 'cooperationNote',
  'contactTitle', 'contactSubtitle', 'contactNote', 'contactTip', 'wechatLabel',
  'copyLabel', 'copiedLabel', 'linksLabel', 'telegramLabel', 'whatsappLabel',
] as const
export type AboutTextField = typeof aboutTextFields[number]
export type AboutItem = { title: LocalizedText; text: LocalizedText }
export type AboutLink = { title: LocalizedText; to: string }
export type AboutPageConfig = Record<AboutTextField, LocalizedText> & {
  version: 1
  services: AboutItem[]
  recharge: AboutItem[]
  links: AboutLink[]
  supportWechat: string
}
const object = (raw: unknown): Record<string, unknown> =>
  raw && typeof raw === 'object' && !Array.isArray(raw) ? raw as Record<string, unknown> : {}
export const localizedAboutText = (raw: unknown = {}, chinese = ''): LocalizedText => {
  const value = object(raw)
  return Object.fromEntries(aboutLanguages.map(lang => [
    lang, typeof value[lang] === 'string' ? value[lang] : lang === 'zh-CN' ? chinese : '',
  ])) as LocalizedText
}
const hasText = (value: LocalizedText) => Object.values(value).some(text => text.trim())
export const newAboutItem = (): AboutItem => ({ title: localizedAboutText(), text: localizedAboutText() })
export const isSafeAboutLink = (raw: string): boolean => {
  const value = raw.trim()
  if (!value || /[\u0000-\u0020\\]/.test(value)) return false
  if (value.startsWith('/') && !value.startsWith('//')) return true
  try {
    const url = new URL(value)
    return ['https:', 'http:'].includes(url.protocol) && !!url.hostname && !url.username && !url.password
  } catch { return false }
}
export const isValidSupportWechat = (value: string) => /^[a-zA-Z][a-zA-Z0-9_-]{5,31}$/.test(value.trim())
const textItems = (raw: unknown): AboutItem[] => Array.isArray(raw) ? raw.slice(0, 12).map(item => {
  const entry = object(item)
  return { title: localizedAboutText(entry.title), text: localizedAboutText(entry.text) }
}) : []
export const createAboutPageConfig = (raw?: unknown, legacy?: unknown, contact?: unknown): AboutPageConfig => {
  const input = object(raw)
  const saved = input.version === 1
  const result = {
    version: 1,
    supportWechat: saved && typeof input.supportWechat === 'string' ? input.supportWechat : defaultSupportWechat,
    services: saved ? textItems(input.services) : aboutCopy.services.map(item => ({
      title: localizedAboutText({}, item.title), text: localizedAboutText({}, item.text),
    })),
    recharge: saved ? textItems(input.recharge) : aboutCopy.recharge.map(item => ({
      title: localizedAboutText({}, item.title), text: localizedAboutText({}, item.text),
    })),
    links: saved ? (Array.isArray(input.links) ? input.links.slice(0, 12).map(item => {
      const link = object(item)
      return { title: localizedAboutText(link.title), to: typeof link.to === 'string' ? link.to : '' }
    }) : []) : aboutCopy.links.map(link => ({ title: localizedAboutText({}, link.title), to: link.to })),
  } as AboutPageConfig
  for (const field of aboutTextFields) result[field] = localizedAboutText(input[field], saved ? '' : defaultCopy[field])
  if (saved) return result
  // Preserve the exact old localized overrides when first opening the new editor.
  const old = object(legacy), hero = object(old.hero), services = object(old.services), oldContact = object(old.contact)
  const mappings: Partial<Record<AboutTextField, unknown>> = {
    title: hero.title, subtitle: hero.subtitle, introduction: old.introduction,
    servicesTitle: services.title, contactTitle: oldContact.title, contactSubtitle: oldContact.text,
  }
  for (const [field, value] of Object.entries(mappings)) {
    const localized = localizedAboutText(value)
    if (hasText(localized)) result[field as AboutTextField] = localized
  }
  const items = Array.isArray(services.items) ? services.items.map(item => localizedAboutText(item)).filter(hasText) : []
  if (items.length) result.services = items.slice(0, 12).map(item => {
    const entry = newAboutItem()
    for (const lang of aboutLanguages) {
      const [first = '', ...rest] = item[lang].trim().split('\n')
      entry.title[lang] = rest.length ? first : ''
      entry.text[lang] = rest.length ? rest.join('\n') : first
    }
    return entry
  })
  const wechat = object(contact).wechat
  if (typeof wechat === 'string' && isValidSupportWechat(wechat)) result.supportWechat = wechat.trim()
  return result
}
export const resolveAboutText = (value: LocalizedText, language: string): string => {
  for (const lang of [language, ...aboutLanguages]) {
    const text = value[lang as AboutLanguage]
    if (typeof text === 'string' && text.trim()) return text.trim()
  }
  return ''
}
export const resolveAboutPage = (raw: unknown, legacy: unknown, language: string, contact?: unknown) => {
  const config = createAboutPageConfig(raw, legacy, contact)
  const text = Object.fromEntries(aboutTextFields.map(field => [field, resolveAboutText(config[field], language)])) as Record<AboutTextField, string>
  const resolveItems = (items: AboutItem[]) => items.map(item => {
    // A title/body pair must use one locale, including an intentionally empty title.
    const lang = [language, ...aboutLanguages].find(candidate =>
      item.title[candidate as AboutLanguage]?.trim() || item.text[candidate as AboutLanguage]?.trim(),
    ) as AboutLanguage | undefined
    return { title: lang ? item.title[lang].trim() : '', text: lang ? item.text[lang].trim() : '' }
  }).filter(item => item.title || item.text)
  return {
    ...text,
    services: resolveItems(config.services), recharge: resolveItems(config.recharge),
    links: config.links.map(link => ({ title: resolveAboutText(link.title, language), to: link.to.trim() }))
      .filter(link => link.title && isSafeAboutLink(link.to)),
    supportWechat: isValidSupportWechat(config.supportWechat) ? config.supportWechat.trim() : defaultSupportWechat,
  }
}
