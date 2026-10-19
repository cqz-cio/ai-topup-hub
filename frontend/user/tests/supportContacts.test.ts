import assert from 'node:assert/strict'
import test from 'node:test'
import { resolveSupportContacts } from '../../shared/supportContacts.ts'

test('store contacts expose the approved accounts and a canonical Telegram link', () => {
  assert.deepEqual(resolveSupportContacts({}), { wechat: 'Cqz0924ttxk', qq: '1669268759', telegram: '@czq_piestar', telegramUrl: 'https://t.me/czq_piestar' })
  assert.equal(resolveSupportContacts({ telegram: 'https://t.me/czq_piestar/' }).telegramUrl, 'https://t.me/czq_piestar')
  assert.equal(resolveSupportContacts({ telegram: '@czq_piestar' }).telegram, '@czq_piestar')
})

test('reseller stores inherit none of the primary store contacts', () => {
  assert.deepEqual(resolveSupportContacts({}, true), { wechat: '', qq: '', telegram: '', telegramUrl: '' })
  assert.equal(resolveSupportContacts({ qq: '1234567', telegram: '@other_store' }, true).qq, '1234567')
})

test('links reject executable URLs, credentials and unrelated hosts', () => {
  for (const telegram of ['javascript:alert(1)', 'https://evil.example/czq_piestar', 'https://user:pass@t.me/czq_piestar', 'https://t.me/czq_piestar/extra']) {
    assert.equal(resolveSupportContacts({ telegram }).telegramUrl, '')
  }
  assert.equal(resolveSupportContacts({ wechat: '13800000000', qq: '<script>' }).wechat, '')
  assert.equal(resolveSupportContacts({ wechat: '13800000000', qq: '<script>' }).qq, '')
})
