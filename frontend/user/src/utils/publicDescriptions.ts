import descriptions from '../../../../internal/platform/publiccontent/page-descriptions.json' with {type: 'json'}
export function pageDescription(key: string): string {
  return (descriptions as Record<string, string>)[key] || ''
}
export function catalogDescription(name: string, names: string[]): string {
  if (!names.length) return ''
  return name + '分类提供「' + Array.from(names[0] || '').slice(0, 36).join('') + '」等当前在售商品。请按页面核对具体套餐、交付方式与适用条件，售价和库存以商品详情为准。使用期限与质保范围请分别确认，购买前阅读兑换及售后说明。'
}
export function needsDescription(description: string, title: string): boolean {
  return Array.from(description.trim()).length < 60 || description.trim() === title.trim()
}
export function blogDescription(names: string[]): string {
  const intro = names.length ? '浏览本站文章列表，阅读「' + Array.from(names[0] || '').slice(0, 32).join('') + '」等已发布内容。' : '浏览本站已发布的文章与购买指引。'
  return intro + '可通过文章了解商品选购、使用与订单相关说明，具体内容以各篇正文为准。下单前请同时核对商品页面的适用条件、交付方式和售后范围。'
}
