import test from 'node:test'
import assert from 'node:assert/strict'
import {
  canPurchaseStorefrontProduct,
  loadStorefrontProducts,
  storefrontPrice,
  sortStorefrontProducts,
  storefrontStockQuantity,
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

test('price sorting uses numeric displayed prices, includes credits and preserves API input', () => {
  const rows = [
    product({ id: 1, price_amount: '100' }),
    product({ id: 2, price_amount: '9.99' }),
    product({ id: 3, slug: 'qzweb-78', price_amount: '200', effective_price_amount: '8' }),
    product({ id: 4, price_amount: '30', promotion_price_amount: '0' }),
  ]
  assert.deepEqual(sortStorefrontProducts(rows).map(p => p.id), [4, 3, 2, 1])
  assert.deepEqual(sortStorefrontProducts(rows, 'desc').map(p => p.id), [1, 2, 3, 4])
  assert.deepEqual(rows.map(p => p.id), [1, 2, 3, 4])
})

test('unknown prices stay last in either direction and ties preserve source order', () => {
  const rows = [
    product({ id: 1, price_amount: null }),
    product({ id: 2, price_amount: '10' }),
    product({ id: 3, price_amount: '10.00', is_sold_out: true }),
    product({ id: 4, price_amount: 'invalid' }),
    product({ id: 5, price_amount: '0' }),
  ]
  assert.deepEqual(sortStorefrontProducts(rows).map(p => p.id), [5, 2, 3, 1, 4])
  assert.deepEqual(sortStorefrontProducts(rows, 'desc').map(p => p.id), [2, 3, 5, 1, 4])
  assert.deepEqual(sortStorefrontProducts([]), [])
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

test('exact stock shows the current aggregate quantity for automatic and manual delivery', () => {
  const exact = { stock_display_mode: 'exact', stock_quantity_hidden: false }
  assert.equal(storefrontStockQuantity(product({ ...exact, fulfillment_type: 'auto', auto_stock_available: 9979 })), 9979)
  assert.equal(storefrontStockQuantity(product({ ...exact, fulfillment_type: 'manual', manual_stock_available: '4' })), 4)
  assert.equal(storefrontStockQuantity(product({ ...exact, fulfillment_type: 'auto', auto_stock_available: 0 })), 0)
})

test('hidden, unlimited, missing or malformed quantities do not become invented stock counts', () => {
  const exact = { stock_display_mode: 'exact', stock_quantity_hidden: false, fulfillment_type: 'auto' }
  for (const value of [undefined, null, '', 'unknown', -1, 1.5, Infinity]) {
    assert.equal(storefrontStockQuantity(product({ ...exact, auto_stock_available: value })), null)
  }
  assert.equal(storefrontStockQuantity(product({ ...exact, stock_display_mode: 'status', auto_stock_available: 9999 })), null)
  assert.equal(storefrontStockQuantity(product({ ...exact, stock_quantity_hidden: true, auto_stock_available: 9999 })), null)
})
