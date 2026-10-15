import test from 'node:test'
import assert from 'node:assert/strict'
import { readPaymentBlock } from '../src/utils/paymentBlock.ts'

test('balance failures retain actual notification state across checkout and retry responses', () => {
  for (const notification_status of ['pending', 'sent', 'partial', 'failed', 'not_sent']) {
    const payment_block = { code: 'card_platform_insufficient_balance', notification_status }
    assert.deepEqual(readPaymentBlock({ payment_block }), payment_block)
    assert.deepEqual(readPaymentBlock(Object.assign(new Error('blocked'), { data: { payment_block } })), payment_block)
  }
})

test('unknown mail state never becomes a successful email notice', () => {
  assert.equal(readPaymentBlock({ payment_block: { code: 'card_platform_insufficient_balance', notification_status: 'queued_ok' } })?.notification_status, 'not_sent')
  assert.equal(readPaymentBlock({ payment_block: { code: 'unrelated_error', notification_status: 'sent' } }), null)
})
