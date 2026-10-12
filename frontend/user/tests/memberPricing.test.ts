import test from 'node:test'
import assert from 'node:assert/strict'
import { resolveMemberPriceAmount, resolveCatalogSkuPrice } from '../src/utils/memberPricing.ts'
import { storefrontPrice } from '../src/utils/storefront.ts'
import { resolveCartPricingSnapshot } from '../src/utils/cartPricingSnapshot.ts'

const product = {
  id: 1, slug: 'prices', title: 'Prices', price_amount: '100.00',
  pricing_member_level_id: 2, member_discount_rate: '80.00',
  member_prices: [
    { member_level_id: 2, sku_id: 0, price_amount: '75.00' },
    { member_level_id: 2, sku_id: 11, price_amount: '70.00' },
    { member_level_id: 3, sku_id: 11, price_amount: '20.00' },
  ],
}

test('fixed variant and product prices take precedence over the level rate', () => {
  assert.equal(resolveMemberPriceAmount(product, 11, '100.00', 2, 50), '70.00')
  assert.equal(resolveMemberPriceAmount(product, 12, '100.00', 2, 50), '75.00')
  assert.equal(resolveMemberPriceAmount({ member_prices: [] }, 11, '100.00', 2, 80), '80.00')
  assert.equal(resolveMemberPriceAmount(product, 11, '100.00', 0, 80), null)
})

test('fixed overrides do not stack the rate or raise a better promotion price', () => {
  assert.equal(resolveMemberPriceAmount(product, 11, '80.00', 2, 80), '70.00')
  assert.equal(resolveMemberPriceAmount(product, 11, '60.00', 2, 80), null)
  assert.equal(resolveMemberPriceAmount({}, 11, '80.00', 2, 80), '64.00')
  assert.equal(resolveMemberPriceAmount({}, 11, '0.03', 2, '50.00'), '0.02')
})

test('a legacy zero variant override falls through to the product price', () => {
  const legacy = { ...product, member_prices: [...product.member_prices.filter((p) => p.sku_id === 0), { member_level_id: 2, sku_id: 11, price_amount: '0.00' }] }
  assert.equal(resolveMemberPriceAmount(legacy, 11, '100', 2, 80), '75.00')
})

test('homepage and cart use the server current-user price while retaining the base', () => {
  const sku = { id: 11, price_amount: '100.00', effective_price_amount: '70.00' }
  assert.equal(storefrontPrice({ ...product, effective_price_amount: '70.00' }), 70)
  assert.equal(resolveCatalogSkuPrice(product, sku), '70.00')
  const snapshot = resolveCartPricingSnapshot(product, sku, 9)
  assert.equal(snapshot.priceAmount, '100.00')
  assert.equal(snapshot.displayPriceAmount, '70.00')
  assert.equal(snapshot.pricingUserId, 9)
  assert.equal(snapshot.memberLevelId, 2)
  const guest = resolveCartPricingSnapshot({ pricing_member_level_id: 0 }, { ...sku, effective_price_amount: '100.00' })
  assert.equal(guest.displayPriceAmount, '100.00')
  assert.equal(guest.memberLevelId, 0)
  assert.equal(guest.memberPrices, undefined)
})

test('invalid server prices cannot raise or corrupt the public catalog price', () => {
  assert.equal(storefrontPrice({ ...product, effective_price_amount: '120.00' }), 100)
  assert.equal(storefrontPrice({ ...product, effective_price_amount: '-1' }), 100)
  assert.equal(resolveCatalogSkuPrice({}, { price_amount: '100', effective_price_amount: 'bad', promotion_price_amount: '80' }), '80.00')
})

test('personal prices bypass level discounts and keep the retail snapshot for account switching', () => {
  const sku = { id: 11, price_amount: '140.01', effective_price_amount: '140.01', user_price_amount: '140.01', retail_price_amount: '100.00' }
  const personalized = { ...product, price_amount: '140.01', user_price_amount: '140.01', effective_price_amount: '140.01', skus: [sku] }
  assert.equal(resolveMemberPriceAmount(personalized, 11, '140.01', 2, 80), null)
  assert.equal(storefrontPrice(personalized), 140.01)
  assert.equal(resolveCatalogSkuPrice(personalized, sku), '140.01')
  const snapshot = resolveCartPricingSnapshot(personalized, sku, 9)
  assert.equal(snapshot.priceAmount, '100.00')
  assert.equal(snapshot.userPriceAmount, '140.01')
  assert.equal(snapshot.displayPriceAmount, '140.01')
  assert.equal(snapshot.pricingUserId, 9)
  const cleared = resolveCartPricingSnapshot(product, { id: 11, price_amount: '100.00', effective_price_amount: '70.00' }, 9)
  assert.equal(cleared.userPriceAmount, undefined)
  assert.equal(cleared.displayPriceAmount, '70.00')
})
