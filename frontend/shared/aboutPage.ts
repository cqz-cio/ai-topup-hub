import aboutDefaults from '../../internal/platform/publiccontent/about-defaults.json' with { type: 'json' }
export const aboutCopy = aboutDefaults

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
