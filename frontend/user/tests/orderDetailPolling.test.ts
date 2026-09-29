import test from 'node:test'
import assert from 'node:assert/strict'
import { setImmediate as settle } from 'node:timers/promises'
import { createOrderDetailPolling, needsOrderDetailRefresh } from '../src/utils/orderDetailPolling.ts'

test('only orders awaiting payment or delivery need automatic refresh', () => {
  for (const status of ['pending_payment', 'paid', 'fulfilling', 'partially_delivered']) {
    assert.equal(needsOrderDetailRefresh({ status }), true)
  }
  for (const status of ['delivered', 'completed', 'canceled', 'refunded', '']) {
    assert.equal(needsOrderDetailRefresh({ status }), false)
  }
  assert.equal(needsOrderDetailRefresh(null), false)
})

test('a customer redirected before delivery sees the code without reloading the page', async (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] })
  let order = { status: 'paid', payload: '' }
  let calls = 0
  const polling = createOrderDetailPolling(async () => {
    calls += 1
    order = calls === 1
      ? { status: 'fulfilling', payload: '' }
      : { status: 'delivered', payload: 'ABCDE-FG234-HJK56-LMN78-PQR9S' }
  }, () => needsOrderDetailRefresh(order))

  polling.schedule()
  polling.schedule()
  t.mock.timers.tick(5000)
  await settle()
  assert.equal(calls, 1)
  assert.equal(order.status, 'fulfilling')
  t.mock.timers.tick(5000)
  await settle()
  assert.equal(order.payload, 'ABCDE-FG234-HJK56-LMN78-PQR9S')
  t.mock.timers.tick(30000)
  await settle()
  assert.equal(calls, 2, 'delivered orders stop polling')
})

test('a temporary network failure retries and eventually displays delivery', async (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] })
  let order = { status: 'paid' }
  let calls = 0
  const polling = createOrderDetailPolling(async () => {
    calls += 1
    if (calls === 1) throw new Error('temporarily unavailable')
    order = { status: 'delivered' }
  }, () => needsOrderDetailRefresh(order))
  polling.schedule()
  t.mock.timers.tick(5000)
  await settle()
  assert.equal(order.status, 'paid')
  t.mock.timers.tick(5000)
  await settle()
  assert.equal(order.status, 'delivered')
  assert.equal(calls, 2)
})

test('slow requests do not overlap and leaving the page stops further requests', async (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] })
  let release!: () => void
  let calls = 0
  const polling = createOrderDetailPolling(() => {
    calls += 1
    return new Promise<void>((resolve) => { release = resolve })
  }, () => true)
  polling.schedule()
  t.mock.timers.tick(5000)
  await settle()
  polling.schedule()
  t.mock.timers.tick(60000)
  await settle()
  assert.equal(calls, 1)
  release()
  await settle()
  t.mock.timers.tick(4999)
  await settle()
  assert.equal(calls, 1)
  t.mock.timers.tick(1)
  await settle()
  assert.equal(calls, 2)
  polling.stop()
  release()
  await settle()
  t.mock.timers.tick(60000)
  await settle()
  assert.equal(calls, 2)
})

test('clearing guest access prevents queued polling and signing in can resume it', async (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] })
  let authorized = true
  let calls = 0
  const polling = createOrderDetailPolling(async () => { calls += 1 }, () => authorized)
  polling.schedule()
  authorized = false
  t.mock.timers.tick(5000)
  await settle()
  assert.equal(calls, 0)
  authorized = true
  polling.schedule()
  t.mock.timers.tick(5000)
  await settle()
  assert.equal(calls, 1)
  polling.stop()
  t.mock.timers.tick(5000)
  await settle()
  assert.equal(calls, 1)
})
