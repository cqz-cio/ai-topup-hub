import test from 'node:test'
import assert from 'node:assert/strict'
import { matchesProductSearch, loadProductSearchPage } from '../src/utils/productSearch.ts'

const products = [
  { id: 1, slug: 'open', title: { 'zh-CN': 'ChatGPT Pro 20新老号(到期)开通 质保一个月' }, price_amount: '1200' },
  { id: 2, slug: 'renew', title: { 'zh-CN': 'ChatGPT Pro 20x 续费绑卡｜自动发码' }, price_amount: '1050' },
  { id: 3, slug: 'fifty', title: 'ChatGPT Pro 50x 开通', price_amount: '5000' },
  { id: 4, slug: 'five', title: 'ChatGPT Pro 5x 开通', price_amount: '500' },
  { id: 5, slug: 'credits', title: 'Codex 25000积分', price_amount: '100' },
]

test('20x aliases match activation and renewal without changing catalog titles', () => {
  for (const query of ['20x', '２０Ｘ', '20 X', '20×', '20倍', 'Pro 20', ' pro 20x ', 'Chat GPT 20x', '20']) {
    assert.deepEqual(products.filter(p => matchesProductSearch(p, query)).map(p => p.slug), ['open', 'renew'], query)
  }
})

test('numeric tiers do not match another tier or credit quantities; compound searches narrow results', () => {
  assert.deepEqual(products.filter(p => matchesProductSearch(p, '5x')).map(p => p.slug), ['five'])
  assert.deepEqual(products.filter(p => matchesProductSearch(p, '50x')).map(p => p.slug), ['fifty'])
  assert.deepEqual(products.filter(p => matchesProductSearch(p, '20x 续费')).map(p => p.slug), ['renew'])
  assert.equal(matchesProductSearch(products[0]!, '20xyz'), false)
})

test('search supports localized titles and category names, empty and missing data', () => {
  const product = { title: { 'zh-CN': '开通', 'en-US': 'ChatGPT Pro 20x activation' }, category: { name: { 'zh-CN': '渠道一' } } }
  assert.equal(matchesProductSearch(product, '渠道一 20x'), true)
  assert.equal(matchesProductSearch(product, '不存在'), false)
  assert.equal(matchesProductSearch({ title: null }, '  '), true)
  assert.equal(matchesProductSearch({ title: null }, '20x'), false)
})

test('search counts all matching pages and deduplicates products before paginating', async () => {
  const requests: number[] = []
  const read = async (page: number) => {
    requests.push(page)
    return { status_code: 0, data: page === 1 ? [products[0]!, products[2]!] : [products[1]!, products[0]!], pagination: { total_page: 2 } }
  }
  const result = await loadProductSearchPage(read, '20x', 2, 1)
  assert.deepEqual(requests, [1, 2])
  assert.equal(result.totalItems, 2)
  assert.equal(result.totalPages, 2)
  assert.deepEqual(result.products.map(p => p.slug), ['renew'])
  const empty = await loadProductSearchPage(read, 'Gemini', 1, 12)
  assert.deepEqual(empty, { products: [], totalItems: 0, totalPages: 0 })
})

test('failed search pages fail the whole result instead of reporting a partial count', async () => {
  await assert.rejects(loadProductSearchPage(async page => {
    if (page === 2) throw new Error('Network unavailable')
    return { status_code: 0, data: [products[0]!], pagination: { total_page: 2 } }
  }, '20x', 1, 12), /Network unavailable/)
})

test('corrected 10x and 25x tiers support aliases and remain distinct from 5x', () => {
  const current = [
    { title: 'ChatGPT Pro 10x 开通｜菲区' },
    { title: 'ChatGPT Pro 10x 续费｜菲区' },
    { title: 'ChatGPT Pro 25x 开通｜菲区' },
    { title: 'ChatGPT Pro 25x 开通｜智利' },
    { title: 'ChatGPT Pro 5x 开通｜智利' },
    { title: 'Codex 25000 点数充值' },
  ]
  for (const query of ['10x', '１０Ｘ', '10 X', '10×', '10倍', 'Pro 10']) {
    assert.equal(current.filter(product => matchesProductSearch(product, query)).length, 2, query)
  }
  for (const query of ['25x', '２５Ｘ', '25 X', '25×', '25倍', 'Pro 25']) {
    assert.equal(current.filter(product => matchesProductSearch(product, query)).length, 2, query)
  }
  assert.equal(current.filter(product => matchesProductSearch(product, '25x 智利')).length, 1)
  assert.equal(current.filter(product => matchesProductSearch(product, '10x 续费')).length, 1)
  assert.equal(current.filter(product => matchesProductSearch(product, '5x')).length, 1)
})
