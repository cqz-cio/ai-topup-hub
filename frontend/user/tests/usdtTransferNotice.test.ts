import test from 'node:test'
import assert from 'node:assert/strict'
import { resolveUsdtTransfer } from '../src/utils/usdtTransferNotice.ts'

test('native USDT channels preserve the exact amount, including old payment precision', () => {
  assert.deepEqual(resolveUsdtTransfer({ provider_type: 'bscusdt', chain_amount: '20.00' }), { amount: '20.00', token: 'USDT' })
  assert.deepEqual(resolveUsdtTransfer({ provider_type: 'plasmausdt', chain_amount: '14.890123' }), { amount: '14.890123', token: 'USDT0' })
  assert.deepEqual(resolveUsdtTransfer({ provider_type: 'tronusdt', chain_amount: '0.010001' }), { amount: '0.010001', token: 'USDT' })
})

test('gateway tokens and channel metadata identify U without guessing from the channel name', () => {
  for (const token_id of ['USDT', 'tron-usdt', 'bsc-USDT', 'plasma-USDT0']) {
    assert.ok(resolveUsdtTransfer({ provider_type: 'tokenpay', token_id, chain_amount: '10.00' }))
  }
  assert.ok(resolveUsdtTransfer({ channel_type: 'USDT-TRC20' }))
  assert.ok(resolveUsdtTransfer({ provider_type: 'epusdt' }))
  assert.deepEqual(resolveUsdtTransfer({ channel_type: 'usdt-plasma' }), { amount: '', token: 'USDT0' })
})

test('non-U currencies, WeChat, and wallet balance do not receive a USDT notice', () => {
  for (const payment of [null, {}, { provider_type: 'xorpay', channel_type: 'wechat' }, { channel_type: 'balance' }, { provider_type: 'bepusdt', token_id: 'bsc-BNB' }, { provider_type: 'tokenpay', token_id: 'ethereum-USDC' }]) {
    assert.equal(resolveUsdtTransfer(payment), null)
  }
})
