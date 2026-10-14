import assert from 'node:assert/strict'
import test from 'node:test'
import { aboutCopy, createAboutPageConfig, resolveAboutPage, isSafeAboutLink, isValidSupportWechat } from '../../shared/aboutPage.ts'

test('first load preserves all approved text and layout data', () => {
  const page = resolveAboutPage(undefined, undefined, 'zh-CN')
  for (const [key, value] of Object.entries(aboutCopy)) assert.deepEqual(page[key as keyof typeof page], value, key)
  assert.equal(page.supportWechat, 'Cqz0924ttxk')
  assert.deepEqual(resolveAboutPage(undefined, undefined, 'en-US').services, aboutCopy.services)
})
test('legacy multilingual overrides and newline service headings are retained', () => {
  const legacy = {
    hero: { title: { 'en-US': 'Existing title' } },
    services: { items: [{ 'zh-CN': '标题\n保留正文\n第二行', 'en-US': 'Body only' }] },
  }
  const config = createAboutPageConfig(undefined, legacy)
  assert.equal(resolveAboutPage(config, {}, 'zh-CN').title, 'Existing title')
  assert.deepEqual(resolveAboutPage(config, {}, 'zh-CN').services, [{ title: '标题', text: '保留正文\n第二行' }])
  assert.deepEqual(resolveAboutPage(config, {}, 'en-US').services, [{ title: '', text: 'Body only' }])
})
test('saved edits, blank paragraphs and deleted lists survive a JSON roundtrip', () => {
  const config = createAboutPageConfig()
  config.recharge[1]!.text['zh-CN'] = '更新的 iOS 说明'
  config.cooperationNote['zh-CN'] = ''
  config.services = []
  config.links = []
  const page = resolveAboutPage(JSON.parse(JSON.stringify(config)), {}, 'zh-CN')
  assert.equal(page.recharge[1]!.text, '更新的 iOS 说明')
  assert.equal(page.cooperationNote, '')
  assert.deepEqual(page.services, [])
  assert.deepEqual(page.links, [])
})
test('locale-specific changes preserve other translations', () => {
  const config = createAboutPageConfig()
  config.rechargeNote['en-US'] = 'Product-specific terms remain applicable.'
  const reloaded = createAboutPageConfig(JSON.parse(JSON.stringify(config)))
  assert.equal(resolveAboutPage(reloaded, {}, 'zh-CN').rechargeNote, aboutCopy.rechargeNote)
  assert.equal(resolveAboutPage(reloaded, {}, 'en-US').rechargeNote, config.rechargeNote['en-US'])
})
test('links exclude executable schemes, protocol-relative URLs and ambiguous slashes', () => {
  for (const url of ['/privacy', '/blog/purchase-and-agent-guide?from=about', 'https://example.com/terms']) assert.equal(isSafeAboutLink(url), true)
  for (const url of ['javascript:alert(1)', 'data:text/html,test', '//example.com', '/\\example.com', 'https://a b.com', 'https://user:pass@example.com', '']) assert.equal(isSafeAboutLink(url), false)
})
test('support contact rejects a phone number and retains the current default', () => {
  assert.equal(isValidSupportWechat('Cqz0924ttxk'), true)
  assert.equal(isValidSupportWechat('13800000000'), false)
  const config = createAboutPageConfig()
  config.supportWechat = '13800000000'
  assert.equal(resolveAboutPage(config, {}, 'zh-CN').supportWechat, 'Cqz0924ttxk')
})
