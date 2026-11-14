import DOMPurify from 'dompurify'

/** Metadata accepts the localized catalog format and older plain strings. */
export function seoText(value: unknown, locale = 'zh-CN'): string {
  if (typeof value === 'string') return value.trim()
  if (!value || typeof value !== 'object') return ''
  const values = value as Record<string, unknown>
  for (const language of [locale, 'zh-CN', 'en-US']) {
    const text = values[language]
    if (typeof text === 'string' && text.trim()) return text.trim()
  }
  return ''
}

export function seoTitle(title: string, brand: string): string {
  const page = title.trim()
  const site = brand.trim()
  return page && site && !page.includes(site) ? `${page} - ${site}` : page || site
}

export function seoPlainText(value: string): string {
  const text = document.createElement('template')
  text.innerHTML = DOMPurify.sanitize(value, { ALLOWED_TAGS: [], ALLOWED_ATTR: [] })
  return (text.content.textContent || '').replace(/\s+/g, ' ').trim()
}
