import { test } from 'node:test'
import assert from 'node:assert/strict'
import { createDeliveryAccess, type DeliveryAccessState } from '../../shared/deliveryAccess.ts'

test('delivery stays unloaded until requested and hiding forces a fresh request', async () => {
  let calls = 0
  let state: DeliveryAccessState<string> = {}
  const access = createDeliveryAccess(async () => { calls++; return 'TEST-NOT-REDEEMABLE' }, value => { state = value })
  assert.equal(calls, 0)
  await access.reveal('one')
  assert.equal(state.one?.data, 'TEST-NOT-REDEEMABLE')
  access.hide('one')
  assert.deepEqual(state, {})
  await access.reveal('one')
  assert.equal(calls, 2)
})

for (const action of ['hide', 'clear', 'dispose'] as const) {
  test(`${action} discards an in-flight sensitive response`, async () => {
    let resolve!: (value: string) => void
    let state: DeliveryAccessState<string> = {}
    const access = createDeliveryAccess(() => new Promise<string>(r => { resolve = r }), value => { state = value })
    const pending = access.reveal('one')
    assert.equal(state.one?.loading, true)
    access[action]('one')
    resolve('TEST-STALE-SECRET')
    await pending
    assert.deepEqual(state, {})
  })
}

test('duplicate clicks do not duplicate requests; failure can be retried', async () => {
  let calls = 0
  let state: DeliveryAccessState<string> = {}
  const access = createDeliveryAccess(async () => { if (++calls === 1) throw new Error('denied'); return 'TEST-RETRY' }, value => { state = value })
  await Promise.all([access.reveal('one'), access.reveal('one')])
  assert.equal(calls, 1)
  assert.equal(state.one?.failed, true)
  assert.equal(state.one?.data, undefined)
  await access.reveal('one')
  assert.equal(state.one?.data, 'TEST-RETRY')
})
