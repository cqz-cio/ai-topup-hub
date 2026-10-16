import test from 'node:test'
import assert from 'node:assert/strict'
import { parseAffiliateRules } from '../src/utils/affiliateRules.ts'

test('published rate, example and confirmation period follow the supplied configuration', () => {
  const initial = parseAffiliateRules({ enabled: true, commission_rate: 2, confirm_days: 30 })!
  assert.equal(initial.rateText, '2')
  assert.equal(initial.exampleCommission, '2.00')
  assert.equal(initial.confirmDays, 30)
  const changed = parseAffiliateRules({ enabled: true, commission_rate: 3.75, confirm_days: 7 })!
  assert.equal(changed.rateText, '3.75')
  assert.equal(changed.exampleCommission, '3.75')
  assert.equal(changed.confirmDays, 7)
})

test('zero rate and zero waiting days are valid and remain distinct from missing settings', () => {
  const rules = parseAffiliateRules({ enabled: true, commission_rate: 0, confirm_days: 0 })!
  assert.equal(rules.enabled, true)
  assert.equal(rules.exampleCommission, '0.00')
  assert.equal(rules.confirmDays, 0)
  assert.equal(parseAffiliateRules({ enabled: true }), null)
})

test('disabled program stays closed even with old or absent numerical settings', () => {
  assert.equal(parseAffiliateRules({ enabled: false, commission_rate: 99, confirm_days: 1 })?.enabled, false)
  assert.equal(parseAffiliateRules({ enabled: false })?.enabled, false)
})

test('invalid responses never present missing data as zero commission or immediate payment', () => {
  for (const raw of [null, undefined, {}, { enabled: 'false' }, { enabled: true, commission_rate: null, confirm_days: 0 }]) {
    assert.equal(parseAffiliateRules(raw), null)
  }
  for (const rate of [NaN, Infinity, -1, 101, '', '2', false]) {
    assert.equal(parseAffiliateRules({ enabled: true, commission_rate: rate, confirm_days: 30 }), null)
  }
  for (const days of [NaN, Infinity, -1, 1.5, 3651, null, '', '30', false]) {
    assert.equal(parseAffiliateRules({ enabled: true, commission_rate: 2, confirm_days: days }), null)
  }
})

test('percentage bounds and hundredths agree with the server configuration contract', () => {
  assert.equal(parseAffiliateRules({ enabled: true, commission_rate: 0.01, confirm_days: 3650 })?.exampleCommission, '0.01')
  assert.equal(parseAffiliateRules({ enabled: true, commission_rate: 100, confirm_days: 0 })?.exampleCommission, '100.00')
  assert.equal(parseAffiliateRules({ enabled: true, commission_rate: 2.755, confirm_days: 1 })?.rateText, '2.76')
})
