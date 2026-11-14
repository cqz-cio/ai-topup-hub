import test from 'node:test'
import assert from 'node:assert/strict'
import { appendRechargeLinks as appendLinks, gptRecharge, gptRechargeHtml, resolveRechargeGuide, rechargeHtml } from '../src/utils/gptRecharge.ts'

const links = Object.fromEntries(Object.keys(gptRecharge.catalogIdentities).map(path => [path, path]))
const appendRechargeLinks = (...args: Parameters<typeof appendLinks>) => appendLinks(...args.slice(0, 5) as [string, 'product' | 'article', string, string, boolean], links)

test('GPT links connect existing public URLs without duplicating authored links or self links', () => {
  const original = '<p>原始购买条件</p><a href="/blog/chatgpt-payment-methods">已有说明</a>'
  const result = appendRechargeLinks(original, 'product', 'qzweb-5', 'gpt-recharge', false)
  assert.ok(result.startsWith(original))
  assert.equal(result.split('href="/blog/chatgpt-payment-methods"').length, 2)
  assert.ok(result.includes(`href="${gptRecharge.path}"`))
  assert.equal(appendRechargeLinks(result, 'product', 'qzweb-5', 'gpt-recharge', false), result)
  const article = appendRechargeLinks('', 'article', 'chatgpt-payment-methods', '', false)
  assert.ok(article.includes(gptRecharge.path))
  assert.ok(!article.includes('href="/blog/chatgpt-payment-methods"'))
})

test('main-store copy stays out of reseller, unrelated and ready-account pages', () => {
  for (const [kind, slug, category, reseller] of [
    ['product', 'qzweb-5', 'gpt-recharge', true],
    ['product', 'grok', 'grok', false],
    ['product', 'ellaix-gpt-plus-account-30d', 'gpt-ready-accounts', false],
    ['article', 'purchase-and-agent-guide', '', false],
  ] as const) assert.equal(appendRechargeLinks('original', kind, slug, category, reseller), 'original')
})

test('recharge guide uses existing category and article URLs without price snapshots or unsupported promises', () => {
  assert.equal(gptRecharge.path, '/categories/gpt-recharge')
  for (const slug of gptRecharge.blogSlugs) assert.ok(gptRechargeHtml.includes(`/blog/${slug}`))
  assert.ok(gptRechargeHtml.includes('/products/chatgpt-plus-code'))
  assert.ok(gptRechargeHtml.includes('/products/chatgpt-plus-manual'))
  assert.ok(gptRechargeHtml.includes('独立第三方'))
  assert.doesNotMatch(gptRechargeHtml, /¥|￥|[0-9]+元|保证.*排名|官方授权服务|零风险/)
})

test('permanent identities keep fixed links and filters on the migrated owner through conflicts and later renames', () => {
  const targets = Object.fromEntries(Object.keys(links).map(path => [path, path + '-2']))
  const resolved = resolveRechargeGuide(targets)
  assert.equal(resolved.path, '/categories/gpt-recharge-2')
  assert.equal(resolved.categorySlug, 'gpt-recharge-2')
  assert.deepEqual(resolved.productCategories, ['gpt-recharge-2', 'gpt-overwrite-topup-2'])
  const html = rechargeHtml(resolved)
  assert.ok(html.includes('href="/products/chatgpt-plus-code-2"'))
  assert.ok(html.includes('href="/products/chatgpt-plus-manual-2"'))
  assert.ok(!html.includes('href="/products/chatgpt-plus-code"'))
  assert.ok(appendLinks('', 'product', 'example', 'gpt-recharge-2', false, targets).includes('/categories/gpt-recharge-2'))
  assert.equal(appendLinks('', 'product', 'other-owner', 'gpt-recharge', false, targets), '')
  targets['/categories/gpt-recharge'] = '/categories/operator-chosen-name'
  assert.equal(resolveRechargeGuide(targets).categorySlug, 'operator-chosen-name')
  assert.equal(resolveRechargeGuide({}).path, '')
  assert.ok(!rechargeHtml(resolveRechargeGuide({})).includes('href="/products/'))
})
