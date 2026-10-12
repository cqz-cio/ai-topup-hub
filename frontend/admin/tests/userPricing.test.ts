import test from 'node:test'
import assert from 'node:assert/strict'
import { buildUserPricePayload } from '../src/utils/userPricing.ts'

test('visible edits keep prices outside the current product list and clear only empty fields', () => {
  const saved = [{ product_id: 1, sku_id: 0, price_amount: '35.00' }, { product_id: 2, sku_id: 3, price_amount: '40.00' }]
  assert.deepEqual(buildUserPricePayload(saved, [
    { product_id: 1, sku_id: 0, price: '' }, { product_id: 4, sku_id: 0, price: ' 39.9 ' },
  ]), { prices: [{ product_id: 2, sku_id: 3, price_amount: '40.00' }, { product_id: 4, sku_id: 0, price_amount: '39.90' }] })
  assert.equal(saved[0]?.price_amount, '35.00')
})

test('prices use exact two-decimal strings and allow a price above retail', () => {
  assert.deepEqual(buildUserPricePayload([], [{ product_id: 1, sku_id: 2, price: '240.01' }]).prices,
    [{ product_id: 1, sku_id: 2, price_amount: '240.01' }])
})

test('invalid amounts and repeated scopes cannot be saved', () => {
  for (const price of ['0', '-1', '1.001', '1e2', 'NaN', 'Infinity', '0x20', '10000000000', '.5']) {
    assert.throws(() => buildUserPricePayload([], [{ product_id: 1, sku_id: 0, price }]))
  }
  assert.throws(() => buildUserPricePayload([], [{ product_id: 1, sku_id: 0, price: '1' }, { product_id: 1, sku_id: 0, price: '2' }]))
})
