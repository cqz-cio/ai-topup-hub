import test from 'node:test'
import assert from 'node:assert/strict'
import { buildLevelProductPricePayload } from '../src/utils/levelProductPricing.ts'

test('saving one level preserves hidden products and only clears the edited scope', () => {
  assert.deepEqual(buildLevelProductPricePayload(7, [
    { product_id: 1, sku_id: 0, price_amount: '35.00' },
    { product_id: 2, sku_id: 3, price_amount: '40.00' },
  ], [
    { product_id: 1, sku_id: 0, price: '' },
    { product_id: 4, sku_id: 0, price: '39.90' },
    { product_id: 4, sku_id: 8, price: '72.01' },
  ]), {
    member_level_id: 7,
    prices: [
      { product_id: 2, sku_id: 3, member_level_id: 7, price_amount: 40 },
      { product_id: 4, sku_id: 0, member_level_id: 7, price_amount: 39.9 },
      { product_id: 4, sku_id: 8, member_level_id: 7, price_amount: 72.01 },
    ],
  })
})

test('blank prices clear the complete level without reverting to a percentage', () => {
  assert.deepEqual(buildLevelProductPricePayload(7,
    [{ product_id: 1, sku_id: 0, price_amount: '35.00' }],
    [{ product_id: 1, sku_id: 0, price: ' ' }]), { member_level_id: 7, prices: [] })
})

test('invalid levels, precision, and non-positive prices are rejected', () => {
  for (const level of [0, -1, 1.5, NaN]) assert.throws(() => buildLevelProductPricePayload(level, [], []))
  for (const price of ['0', '-1', '1.001', '1e2', '10000000000']) {
    assert.throws(() => buildLevelProductPricePayload(7, [], [{ product_id: 1, sku_id: 0, price }]))
  }
})
