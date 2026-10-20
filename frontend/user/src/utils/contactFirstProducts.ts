const contactFirstSlugs = new Set([
  'gpt-plus-brazil-account-24h',
  'gpt-plus-vietnam-account-30d',
])

/** Explicitly scoped to the two account products; never inferred from stock or category. */
export function isContactFirstProduct(slug: unknown): boolean {
  return typeof slug === 'string' && contactFirstSlugs.has(slug)
}

export function contactFirstCopy(locale: string) {
  if (locale === 'en-US') return {
    title: 'Contact support', description: 'Confirm availability and delivery with support before buying.',
    hint: 'Tell support which product and warranty option you need.',
    wechat: 'WeChat', copy: 'Copy', copied: 'Copied', open: 'Open', close: 'Close',
    failed: 'Could not copy. Please select and copy the contact below.',
    unavailable: 'Contact information is currently unavailable. Please try again later.',
  }
  if (locale === 'zh-TW') return {
    title: '聯絡客服', description: '請先聯絡客服確認庫存和發貨安排，再購買。',
    hint: '告知客服所需商品及質保版本即可。',
    wechat: '微信', copy: '複製', copied: '已複製', open: '開啟', close: '關閉',
    failed: '複製失敗，請選取下方聯絡方式手動複製。',
    unavailable: '暫無聯絡方式，請稍後再試。',
  }
  return {
    title: '联系客服', description: '请先联系客服确认库存和发货安排，再购买。',
    hint: '告知客服所需商品及质保版本即可。',
    wechat: '微信', copy: '复制', copied: '已复制', open: '打开', close: '关闭',
    failed: '复制失败，请选取下方联系方式手动复制。',
    unavailable: '暂无联系方式，请稍后再试。',
  }
}
