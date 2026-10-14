import test from 'node:test'
import assert from 'node:assert/strict'
import { directUSDTPaymentInfo, isBep20PaymentChannel } from '../src/utils/paymentChannelPresentation.ts'

test('TRON checkout identifies TRC20 USDT and exact net receipt amount', () => {
  const info = directUSDTPaymentInfo('tronusdt')
  assert.ok(info)
  assert.equal(info.token, 'USDT')
  assert.match(info.title, /TRON.*TRC20/)
  assert.match(info.tip, /请选择 TRON（TRC20）/)
  assert.match(info.tip, /订单到期前/)
  assert.match(info.tip, /扣除平台手续费后的到账金额一致/)
  assert.doesNotMatch(info.tip, /Plasma|BEP|BNB|USDT0/)
  assert.equal(isBep20PaymentChannel({ provider_type: 'tronusdt', channel_type: 'usdt-trc20' }), false)
})

test('Plasma checkout instructs the correct network and exact transfer amount', () => {
  const info = directUSDTPaymentInfo('plasmausdt')
  assert.ok(info)
  assert.equal(info.token, 'USDT0')
  assert.match(info.title, /Plasma/)
  assert.match(info.tip, /请选择 Plasma/)
  assert.match(info.tip, /不要省略小数尾数/)
  assert.match(info.tip, /扣除平台手续费后的到账金额一致/)
  assert.doesNotMatch(info.tip, /BEP|BNB/)
  assert.equal(isBep20PaymentChannel({ provider_type: 'plasmausdt', channel_type: 'usdt-plasma' }), false)
})

test('BEP20 instructions and other payment providers stay distinct', () => {
  const info = directUSDTPaymentInfo('bscusdt')
  assert.ok(info)
  assert.equal(info.token, 'USDT')
  assert.match(info.tip, /BNB Smart Chain（BEP-20）/)
  assert.doesNotMatch(info.tip, /Plasma|USDT0/)
  assert.equal(directUSDTPaymentInfo('bepusdt'), null)
  assert.equal(directUSDTPaymentInfo(), null)
})
