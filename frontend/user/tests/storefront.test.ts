import test from 'node:test'
import assert from 'node:assert/strict'
import {
  canPurchaseStorefrontProduct,
  loadStorefrontProducts,
  storefrontPrice,
  type StorefrontProduct,
} from '../src/utils/storefront.ts'

const product = (
  changes: Partial<StorefrontProduct> = {},
): StorefrontProduct => ({
  id: 1,
  slug: 'zxa-1',
  title: { 'zh-CN': '账号' },
  price_amount: '88.00',
  is_sold_out: false,
  stock_status: 'in_stock',
  ...changes,
})

test('purchase availability requires current, complete public stock and price data', () => {
  assert.equal(canPurchaseStorefrontProduct(product()), true)
  assert.equal(
    canPurchaseStorefrontProduct(product({ is_sold_out: true })),
    false,
  )
  assert.equal(
    canPurchaseStorefrontProduct(product({ stock_status: 'out_of_stock' })),
    false,
  )
  assert.equal(
    canPurchaseStorefrontProduct(product({ is_sold_out: undefined })),
    false,
  )
  assert.equal(
    canPurchaseStorefrontProduct(product({ stock_status: undefined })),
    false,
  )
  assert.equal(
    canPurchaseStorefrontProduct(product({ price_amount: '' })),
    false,
  )
  assert.equal(
    canPurchaseStorefrontProduct(product({ price_amount: '-1.00' })),
    false,
  )
  assert.equal(
    canPurchaseStorefrontProduct(product({ price_amount: '0.00' })),
    true,
  )
})

test('a product with SKUs requires at least one active, stocked SKU', () => {
  const stocked = {
    is_active: true,
    is_sold_out: false,
    stock_status: 'in_stock',
  }
  assert.equal(
    canPurchaseStorefrontProduct(
      product({ skus: [{ ...stocked, is_active: false }] }),
    ),
    false,
  )
  assert.equal(
    canPurchaseStorefrontProduct(
      product({ skus: [{ ...stocked, is_sold_out: true }] }),
    ),
    false,
  )
  assert.equal(
    canPurchaseStorefrontProduct(
      product({ skus: [{ ...stocked, is_sold_out: true }, stocked] }),
    ),
    true,
  )
})

test('the public price and promotion replace old display prices, including free promotions', () => {
  assert.equal(storefrontPrice(product({ price_amount: '99.90' })), 99.9)
  assert.equal(
    storefrontPrice(
      product({ price_amount: '99.90', promotion_price_amount: '79.90' }),
    ),
    79.9,
  )
  assert.equal(storefrontPrice(product({ promotion_price_amount: '0.00' })), 0)
  assert.equal(
    storefrontPrice(product({ promotion_price_amount: '100.00' })),
    88,
  )
  assert.equal(
    storefrontPrice(
      product({ price_amount: null, promotion_price_amount: '79.90' }),
    ),
    null,
  )
})

test('the catalog follows backend pagination and uses backend slugs without inventing ID mappings', async () => {
  const pages: number[] = []
  const rows = await loadStorefrontProducts(async (page) => {
    pages.push(page)
    return {
      status_code: 0,
      pagination: { total_page: 2 },
      data:
        page === 1
          ? [product({ id: 400, slug: 'new-account', price_amount: '120.00' })]
          : [product({ id: 500, slug: 'new-code', price_amount: '150.00' })],
    }
  })
  assert.deepEqual(pages, [1, 2])
  assert.deepEqual(
    rows.map((row) => row.slug),
    ['new-account', 'new-code'],
  )
  assert.deepEqual(rows.map(storefrontPrice), [120, 150])
})

test('a failed later page fails the whole catalog refresh so stale products cannot stay purchasable', async () => {
  await assert.rejects(
    loadStorefrontProducts(async (page) => {
      if (page === 2) throw new Error('network unavailable')
      return {
        status_code: 0,
        data: [product()],
        pagination: { total_page: 2 },
      }
    }),
    /network unavailable/,
  )
  await assert.rejects(
    loadStorefrontProducts(async () => ({
      status_code: 500,
      data: [product()],
    })),
    /Invalid public product response/,
  )
})
