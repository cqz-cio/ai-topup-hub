import test from 'node:test'
import assert from 'node:assert/strict'
import { appendRechargeLinks, gptRecharge, gptRechargeHtml } from '../src/utils/gptRecharge.ts'

test('GPT links connect existing public URLs without duplicating authored links or self links', () => {
  const original = '<p>原始购买条件</p><a href="/blog/chatgpt-payment-methods">已有说明</a>'
  const result = appendRechargeLinks(original, 'product', 'qzweb-5', 'qzweb-category-4', false)
  assert.ok(result.startsWith(original))
  assert.equal(result.split('href="/blog/chatgpt-payment-methods"').length, 2)
  assert.ok(result.includes(`href="${gptRecharge.path}"`))
  assert.equal(appendRechargeLinks(result, 'product', 'qzweb-5', 'qzweb-category-4', false), result)
  const article = appendRechargeLinks('', 'article', 'chatgpt-payment-methods', '', false)
  assert.ok(article.includes(gptRecharge.path))
  assert.ok(!article.includes('href="/blog/chatgpt-payment-methods"'))
})

test('main-store copy stays out of reseller, unrelated and ready-account pages', () => {
  for (const [kind, slug, category, reseller] of [
    ['product', 'qzweb-5', 'qzweb-category-4', true],
    ['product', 'grok', 'grok', false],
    ['product', 'ellaix-gpt-plus-account-30d', 'gpt-ready-accounts', false],
    ['article', 'purchase-and-agent-guide', '', false],
  ] as const) assert.equal(appendRechargeLinks('original', kind, slug, category, reseller), 'original')
})

test('recharge guide uses existing category and article URLs without price snapshots or unsupported promises', () => {
  assert.equal(gptRecharge.path, '/categories/qzweb-category-4')
  for (const slug of gptRecharge.blogSlugs) assert.ok(gptRechargeHtml.includes(`/blog/${slug}`))
  assert.ok(gptRechargeHtml.includes('/products/qzweb-5'))
  assert.ok(gptRechargeHtml.includes('/products/86gamestore-88'))
  assert.ok(gptRechargeHtml.includes('独立第三方'))
  assert.doesNotMatch(gptRechargeHtml, /¥|￥|[0-9]+元|保证.*排名|官方授权服务|零风险/)
})
