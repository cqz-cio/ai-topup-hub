import test from 'node:test'
import assert from 'node:assert/strict'
import { paymentDeadlineExpired, usdtPaymentWindowMessage } from '../src/utils/usdtPaymentWindow.ts'
test('payment window closes at its boundary, including stale order status', () => {
  const end = '2026-10-20T08:15:00.750Z'
  assert.equal(paymentDeadlineExpired([end], Date.parse('2026-10-20T08:14:59.999Z'), true), false)
  assert.equal(paymentDeadlineExpired([end], Date.parse('2026-10-20T08:15:00.000Z'), true), true)
  assert.equal(paymentDeadlineExpired([end], Date.parse('2026-10-20T08:15:00.000Z')), false)
  assert.equal(paymentDeadlineExpired([end, '2026-10-20T08:10:00Z'], Date.parse('2026-10-20T08:11:00Z')), true)
  assert.equal(paymentDeadlineExpired([null, '', 'invalid'], Date.now()), false)
})
test('three languages tell customers to create a new order after expiry', () => {
  for (const locale of ['zh-CN', 'zh-TW', 'en-US']) {
    assert.ok(usdtPaymentWindowMessage(locale).length > 60)
    assert.notEqual(usdtPaymentWindowMessage(locale), usdtPaymentWindowMessage(locale, true))
  }
  assert.match(usdtPaymentWindowMessage('zh-CN'), /超时付款不予处理/)
  assert.match(usdtPaymentWindowMessage('zh-CN', true), /重新下单/)
})
