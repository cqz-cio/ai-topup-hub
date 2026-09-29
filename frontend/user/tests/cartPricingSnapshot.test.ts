import test from 'node:test'
import assert from 'node:assert/strict'
import { resolveCartPricingSnapshot } from '../src/utils/cartPricingSnapshot.ts'

test('cart pricing snapshot replaces cached sku price and wholesale tiers', () => {
  const patch = resolveCartPricingSnapshot(
    {
      wholesale_prices: [{ sku_id: 12, min_quantity: 3, unit_price: '11.00' }],
    },
    {
      price_amount: '12.345',
    },
  )

  assert.equal(patch.priceAmount, '12.35')
  assert.deepEqual(patch.wholesalePrices, [
    { sku_id: 12, min_quantity: 3, unit_price: '11.00' },
  ])
})

test('cart pricing snapshot preserves cached price when current sku price is invalid', () => {
  for (const priceAmount of [undefined, '', 'invalid', '0', '-1']) {
    const patch = resolveCartPricingSnapshot(
      { wholesale_prices: [] },
      { price_amount: priceAmount },
    )

    assert.equal(Object.hasOwn(patch, 'priceAmount'), false)
    assert.deepEqual(patch.wholesalePrices, [])
  }
})

test('cart pricing snapshot clears cached wholesale tiers when the current product has none', () => {
  const patch = resolveCartPricingSnapshot(
    {},
    { price_amount: '10.00' },
  )

  assert.ok(Object.hasOwn(patch, 'wholesalePrices'))
  assert.equal(patch.wholesalePrices, undefined)
})
