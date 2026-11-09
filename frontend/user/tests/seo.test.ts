import test from 'node:test'
import assert from 'node:assert/strict'
import { seoText, seoTitle } from '../src/utils/seo.ts'

test('catalog metadata supports legacy strings and nonempty locale fallbacks', () => {
  assert.equal(seoText('  商品专属摘要  '), '商品专属摘要')
  assert.equal(seoText({ 'zh-CN': ' ', 'en-US': 'Product summary' }), 'Product summary')
  assert.equal(seoText({ 'zh-CN': '中文摘要', 'en-US': 'English' }, 'en-US'), 'English')
  assert.equal(seoText({ 'zh-CN': {} }), '')
  assert.equal(seoText(null), '')
})

test('page title suffix is the brand, without duplicating a matching name', () => {
  assert.equal(seoTitle('Plus 购买须知', 'π星卡社'), 'Plus 购买须知 - π星卡社')
  assert.equal(seoTitle(' π星卡社 ', 'π星卡社'), 'π星卡社')
  assert.equal(seoTitle('', 'π星卡社'), 'π星卡社')
  assert.equal(seoTitle('Plus 购买须知', ''), 'Plus 购买须知')
})
