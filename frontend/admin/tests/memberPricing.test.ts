import test from 'node:test'
import assert from 'node:assert/strict'
import { buildMemberPricePayload, createMemberPriceRows } from '../src/utils/memberPricing.ts'

test('the matrix preserves product and variant overrides for every configured level', () => {
  const rows = createMemberPriceRows([{ id: 1 }, { id: 2 }], [{ id: 11 }, { id: 12 }], [
    { member_level_id: 2, sku_id: 0, price_amount: '75.00' },
    { member_level_id: 2, sku_id: 11, price_amount: '70.00' },
  ])
  assert.equal(rows.length, 6)
  const payload = buildMemberPricePayload(7, rows)
  assert.deepEqual(payload, { product_id: 7, prices: [
    { product_id: 7, member_level_id: 2, sku_id: 0, price_amount: 75 },
    { product_id: 7, member_level_id: 2, sku_id: 11, price_amount: 70 },
  ] })
})

test('clearing every input sends an empty replacement matrix', () => {
  assert.deepEqual(buildMemberPricePayload(7, [{ member_level_id: 2, sku_id: 0, price: ' ' }]), { product_id: 7, prices: [] })
})

test('invalid amounts, duplicate scopes and invalid IDs are rejected before saving', () => {
  for (const price of ['0', '-1', 'NaN', 'Infinity', '1.001', '1e2', '1,00']) {
    assert.throws(() => buildMemberPricePayload(7, [{ member_level_id: 2, sku_id: 0, price }]))
  }
  const row = { member_level_id: 2, sku_id: 0, price: '1.25' }
  assert.throws(() => buildMemberPricePayload(7, [row, row]))
  assert.throws(() => buildMemberPricePayload(0, [row]))
  assert.throws(() => buildMemberPricePayload(7, [{ ...row, sku_id: -1 }]))
})
