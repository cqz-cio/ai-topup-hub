import assert from 'node:assert/strict'
import test from 'node:test'
import { detectDemoIntent, getDemoAnswer, getSupportCopy, supportContacts } from '../src/components/support/supportDemo.ts'

test('demo handles the four displayed topics in each supported locale', () => {
  for (const locale of ['zh-CN', 'zh-TW', 'en-US']) {
    const copy = getSupportCopy(locale)
    copy.questions.forEach((question, index) => assert.equal(getDemoAnswer(question, locale), copy.answers[index]))
  }
})
test('unknown questions return the explicit demo limitation', () => {
  assert.equal(getDemoAnswer('帮我写一首诗', 'zh-CN'), getSupportCopy('zh-CN').unknown)
  assert.equal(detectDemoIntent('   '), 'unknown')
})
test('order status and refunds do not receive a generic redemption answer', () => {
  for (const question of ['GPT 订单没收到', 'GPT 退款', 'GPT order status', '订单状态']) assert.equal(detectDemoIntent(question), 'order')
  assert.match(getDemoAnswer('订单状态', 'zh-CN'), /不能读取或处理/)
})
test('full-width product input and payment intent are recognized', () => {
  assert.equal(detectDemoIntent('ＧＰＴ 怎么用'), 'redeem')
  assert.equal(detectDemoIntent('GPT 怎么付款'), 'payment')
})
test('demo does not disclose redemption domains or fabricate rates', () => {
  for (const locale of ['zh-CN', 'zh-TW', 'en-US']) {
    const content = getSupportCopy(locale).answers.join('\n')
    assert.doesNotMatch(content, /https?:\/\/|\d+(?:\.\d+)?%/)
  }
})
test('approved main-store contacts are consistent', () => {
  assert.equal(supportContacts.wechat, 'Cqz0924ttxk')
  assert.equal(supportContacts.qq, '1669268759')
  assert.equal(supportContacts.telegramUrl, 'https://t.me/czq_piestar')
})
