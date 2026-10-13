import test from 'node:test'
import assert from 'node:assert/strict'
import { normalizeRechargePayment } from '../src/utils/rechargePayment.ts'

test('flattened recharge payload preserves chain, exact amount, address and expiry', () => {
  for (const chain of ['bsc', 'plasma']) {
    const payload = { payment_id: 7, chain, chain_amount: '15.025681', wallet_address: '0x123', token_id: chain === 'plasma' ? 'USDT0' : 'USDT', expires_at: '2026-10-13T04:00:00Z', qr_code: '0x123', interaction_mode: 'qr', status: 'pending' }
    const payment = normalizeRechargePayment(payload)
    assert.equal(payment.id, 7)
    assert.equal(payment.chain_amount, payload.chain_amount)
    assert.equal(payment.wallet_address, payload.wallet_address)
    assert.equal(payment.chain, chain)
    assert.equal(payment.token_id, payload.token_id)
    assert.equal(payment.expires_at, payload.expires_at)
    assert.equal(payment.qr_code, payload.wallet_address)
  }
})

test('nested and non-crypto recharge responses remain supported', () => {
  const nested = { id: 9, pay_url: 'https://payment.example.test' }
  assert.equal(normalizeRechargePayment({ payment: nested }), nested)
  assert.equal(normalizeRechargePayment({payment_id:10, pay_url:nested.pay_url}).pay_url,nested.pay_url)
  assert.equal(normalizeRechargePayment({recharge:{status:'pending'}}),undefined)
})
