import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const quickBuySource = readFileSync(
  new URL('../src/components/ProductQuickBuy.vue', import.meta.url),
  'utf8',
)

const quickBuyTemplate = quickBuySource.match(/<template>([\s\S]*?)<\/template>/)?.[1]

test('quick buy overlay and panel animate when initially mounted as visible', () => {
  assert.ok(quickBuyTemplate, 'ProductQuickBuy template should exist')

  const transitionAttributes = [...quickBuyTemplate.matchAll(/<Transition\b([^>]*)>/g)].map(
    (match) => match[1],
  )

  assert.equal(transitionAttributes.length, 2, 'quick buy should contain overlay and panel transitions')
  transitionAttributes.forEach((attributes, index) => {
    assert.match(
      attributes,
      /(?:^|\s)appear(?:\s|$)/,
      `quick buy transition ${index + 1} should animate on initial render`,
    )
  })
})
