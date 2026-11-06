import test from 'node:test'
import assert from 'node:assert/strict'
import { pickPetQuote, canOfferPetQuote, petQuoteDelay, type QuoteActivity } from '../src/components/support/petQuotes.ts'

test('quote selection never immediately repeats, including random boundaries', () => {
  let previous = ''
  const seen = new Set<string>()
  for (const random of [0, 0, .1, .2, .3, .4, .5, .6, .7, .8, .9, 1, 1, -1, 5, NaN]) {
    const quote = pickPetQuote('zh-CN', previous, random)
    assert.notEqual(quote.id, previous)
    assert.ok(quote.text && quote.label)
    previous = quote.id; seen.add(quote.id)
  }
  assert.ok(seen.size >= 8)
})

test('language changes retain the selected quote and distinguish original chatter', () => {
  for (let i = 0; i < 10; i++) {
    const random = i / 10
    const cn = pickPetQuote('zh-CN', '', random), tw = pickPetQuote('zh-TW', '', random), en = pickPetQuote('en-US', '', random)
    assert.equal(cn.id, tw.id); assert.equal(cn.id, en.id)
    assert.ok(cn.text && tw.text && en.text)
    assert.equal(cn.label.includes('原创'), i >= 2)
    assert.equal(en.label.includes('original'), i >= 2)
    assert.deepEqual(pickPetQuote('unknown', '', random), cn)
  }
})

test('idle quotes pause during chat, focus loss, rest and every active interaction', () => {
  const quiet: QuoteActivity = { hidden: false, focused: true, resting: false, chatting: false, busy: false, dragging: false, menu: false, preview: false, occupied: false }
  assert.equal(canOfferPetQuote(quiet), true)
  for (const flag of Object.keys(quiet) as (keyof QuoteActivity)[]) {
    assert.equal(canOfferPetQuote({ ...quiet, [flag]: flag !== 'focused' }), false, flag)
  }
})

test('chatter is infrequent even with out-of-range random input', () => {
  assert.equal(petQuoteDelay(0), 45000); assert.equal(petQuoteDelay(1), 80000)
  for (const random of [-1, 0, .5, 1, 7, NaN, Infinity]) {
    assert.ok(petQuoteDelay(random) >= 45000 && petQuoteDelay(random) <= 80000)
  }
})
