import assert from 'node:assert/strict'
import test from 'node:test'
import { contactFirstCopy, isContactFirstProduct } from '../src/utils/contactFirstProducts.ts'

test('contact-first applies only to the two explicitly approved product slugs', () => {
  for (const slug of ['gpt-plus-brazil-account-24h', 'gpt-plus-vietnam-account-30d']) {
    assert.equal(isContactFirstProduct(slug), true)
  }
  for (const slug of [undefined, null, '', 'gpt-ready-accounts', 'qzweb-52', 'qzweb-5',
    'gpt-plus-brazil-account-24h-other', 'GPT-PLUS-BRAZIL-ACCOUNT-24H', { slug: 'gpt-plus-brazil-account-24h' }]) {
    assert.equal(isContactFirstProduct(slug), false)
  }
})

test('contact actions have localized copy and never offer checkout or stock', () => {
  assert.equal(contactFirstCopy('zh-CN').title, '联系客服')
  assert.equal(contactFirstCopy('zh-TW').title, '聯絡客服')
  assert.equal(contactFirstCopy('en-US').title, 'Contact support')
  assert.equal(contactFirstCopy('unknown').title, '联系客服')
})
