import test from 'node:test'
import assert from 'node:assert/strict'
import { codexCreditPoints, exactCreditStock, splitCodexCredits } from '../src/utils/codexCredits.ts'

test('credit section preserves the filtered input, regular order and original array', () => {
  const input = [{ slug: 'qzweb-78' }, { slug: 'zxa-8' }, { slug: 'qzweb-73' }, { slug: 'qzweb-6' }]
  const original = [...input]
  const { credits, other } = splitCodexCredits(input)
  assert.deepEqual(credits.map(codexCreditPoints), [250, 25000])
  assert.deepEqual(other.map(p => p.slug), ['zxa-8', 'qzweb-6'])
  assert.deepEqual(input, original)
  assert.equal(codexCreditPoints({ slug: 'codex-subscription' }), undefined)
})

test('stock comes from the API and respects hidden inventory and sold-out zero', () => {
  assert.equal(exactCreditStock({ stock_display_mode: 'exact', auto_stock_available: 9999 }), 9999)
  assert.equal(exactCreditStock({ stock_display_mode: 'exact', auto_stock_available: 0 }), 0)
  assert.equal(exactCreditStock({ stock_display_mode: 'exact', stock_quantity_hidden: true, auto_stock_available: 1 }), undefined)
  assert.equal(exactCreditStock({ stock_display_mode: 'hidden', auto_stock_available: 1 }), undefined)
  assert.equal(exactCreditStock({ stock_display_mode: 'exact', auto_stock_available: -1 }), undefined)
})
