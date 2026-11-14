import titles from '../../../../internal/platform/publiccontent/page-titles.json' with { type: 'json' }
export function categoryPageTitle(name: string, locale = 'zh-CN'): string {
  const copy = titles[locale as keyof typeof titles] || titles['zh-CN']
  return (copy.categoryTitles as Record<string, string>)[name] || name
}
