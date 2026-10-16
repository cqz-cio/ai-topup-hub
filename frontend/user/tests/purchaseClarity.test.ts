import test from 'node:test'
import assert from 'node:assert/strict'
import { productPurchaseGuide } from '../src/utils/productPurchaseGuide.ts'
import { purchaseClarityCopy } from '../src/i18n/purchaseClarity.ts'
import { checkoutPaymentSummary } from '../src/utils/checkoutPaymentSummary.ts'
import { calculateChannelFeeCents } from '../src/utils/money.ts'

test('activation, renewal and credits have different eligibility rules', () => {
  assert.deepEqual(productPurchaseGuide('qzweb-5')?.rules, ['activation', 'subscriptionWarranty'])
  assert.deepEqual(productPurchaseGuide('qzweb-7')?.rules, ['renewal', 'subscriptionWarranty'])
  for (let id = 73; id <= 78; id++) assert.deepEqual(productPurchaseGuide(`qzweb-${id}`)?.rules, ['credits'])
  assert.equal(productPurchaseGuide('new-unreviewed-product'), null)
})

test('Gemini retains distinct age and warranty rules and never gets the ChatGPT redemption URL', () => {
  assert.deepEqual(productPurchaseGuide('qzweb-50')?.rules, ['jio', 'accountAge', 'activationOnly', 'antigravity'])
  assert.deepEqual(productPurchaseGuide('qzweb-51')?.rules, ['jio', 'geminiWarranty', 'antigravity'])
  assert.ok(productPurchaseGuide('qzweb-52')?.rules.includes('presale'))
  for (let id = 50; id <= 53; id++) assert.equal(productPurchaseGuide(`qzweb-${id}`)?.kind, 'link')
})


const wechat = { fee_policy: 'customer_surcharge', fee_rate: '1.60', fixed_fee: '0', fee_rounding: 'ceil' }
test('130 CNY WeChat order displays 132.08 including its 2.08 surcharge', () => {
  const fee = calculateChannelFeeCents(13000, wechat)
  assert.deepEqual(checkoutPaymentSummary(13000, 0, 13000, fee), { subtotal: 13000, fee: 208, total: 13208, wallet: 0, online: 13208 })
})
test('partial wallet payment is included in the total and only the online remainder is surcharged', () => {
  const fee = calculateChannelFeeCents(10000, wechat)
  assert.deepEqual(checkoutPaymentSummary(13000, 3000, 10000, fee), { subtotal: 13000, fee: 160, total: 13160, wallet: 3000, online: 10160 })
})
test('full wallet coverage has a nonzero total, zero online due and no payment fee', () => {
  const fee = calculateChannelFeeCents(0, wechat)
  assert.deepEqual(checkoutPaymentSummary(13000, 13000, 0, fee), { subtotal: 13000, fee: 0, total: 13000, wallet: 13000, online: 0 })
})
test('channel changes remove old fees and discounted amounts use the existing cent rounding', () => {
  const total = 8088
  assert.equal(checkoutPaymentSummary(total, 0, total, calculateChannelFeeCents(total, wechat)).total, 8218)
  assert.equal(checkoutPaymentSummary(total, 0, total, calculateChannelFeeCents(total, { fee_policy: 'merchant_absorb' })).total, 8088)
})

test('presales guides contain eligibility and delivery directions without a redemption address', () => {
  for (const slug of ['qzweb-5', 'qzweb-7', 'qzweb-73', 'qzweb-50']) {
    assert.ok(productPurchaseGuide(slug)?.rules.length)
    assert.equal('redemptionUrl' in productPurchaseGuide(slug)!, false)
  }
  assert.doesNotMatch(JSON.stringify(purchaseClarityCopy), /cdk\.bar|cdkai\.kdns\.fr/)
})
