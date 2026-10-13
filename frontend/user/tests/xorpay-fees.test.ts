import test from 'node:test'
import assert from 'node:assert/strict'
import { amountToCents, calculateChannelFeeCents } from '../src/utils/money.ts'

test('XorPay preview charges 1.6% rounded up to the cent', () => {
 const channel = { fee_policy: 'customer_surcharge', fee_rate: '1.60', fixed_fee: '0.00', fee_rounding: 'ceil' }
 for (const [base, expected] of [['99.00',159], ['100.00',160], ['1160.00',1856], ['0.01',1], ['12.30',20]] as const) {
   assert.equal(calculateChannelFeeCents(amountToCents(base)!, channel), expected)
 }
 assert.equal(calculateChannelFeeCents(0,channel),0)
 assert.equal(calculateChannelFeeCents(9900,{...channel,fee_policy:'merchant_absorbed'}),0)
 assert.equal(calculateChannelFeeCents(9900,{...channel,fee_rounding:'round'}),158)
 assert.equal(calculateChannelFeeCents(9900-4900,channel),80)
})
