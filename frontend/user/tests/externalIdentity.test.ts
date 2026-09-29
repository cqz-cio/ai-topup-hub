import test from 'node:test'
import assert from 'node:assert/strict'
import { canUnbindExternalIdentity } from '../src/utils/externalIdentity.ts'

test('external identity unbinding fails closed without explicit backend authorization', () => {
  assert.equal(canUnbindExternalIdentity(undefined), false)
  assert.equal(canUnbindExternalIdentity(null), false)
  assert.equal(canUnbindExternalIdentity({}), false)
  assert.equal(canUnbindExternalIdentity({ can_unbind: false }), false)
  assert.equal(canUnbindExternalIdentity({ can_unbind: 'true' }), false)
  assert.equal(canUnbindExternalIdentity({ can_unbind: true }), true)
})
