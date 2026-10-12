import test from 'node:test'
import assert from 'node:assert/strict'
import { orderDeliveryGroups, orderDeliveryState } from '../src/utils/orderDelivery.ts'

test('a paid order waits for fulfillment instead of claiming keys are ready', () => {
  assert.equal(orderDeliveryState({ status: 'paid' }), 'waiting')
  assert.equal(orderDeliveryState({ status: 'delivered', fulfillment: null }), 'waiting')
  assert.equal(orderDeliveryState({ status: 'pending_payment' }), 'unpaid')
})

test('split orders show a partial delivery until every fulfillment is delivered', () => {
  const order = { order_no: 'parent', status: 'partially_delivered', children: [
    { order_no: 'first', fulfillment: { status: 'delivered', payload: 'demo-key' } },
    { order_no: 'second', fulfillment: { status: 'pending' } },
  ] }
  assert.equal(orderDeliveryState(order), 'partial')
  assert.deepEqual(orderDeliveryGroups(order).map(group => [group.orderNo, group.delivered]), [['first', true], ['second', false]])
  order.children[1].fulfillment.status = 'delivered'
  assert.equal(orderDeliveryState(order), 'ready')
})

test('closed orders retain their actual state and historical delivery data', () => {
  for (const status of ['canceled', 'expired', 'refunded']) {
    assert.equal(orderDeliveryState({ status, fulfillment: { status: 'delivered', payload: 'historical-key' } }), 'closed')
  }
  assert.equal(orderDeliveryGroups({ status: 'refunded', fulfillment: { status: 'delivered', payload: 'historical-key' } })[0]?.fulfillment.payload, 'historical-key')
})

test('missing order and malformed child collection do not invent a delivery', () => {
  assert.deepEqual(orderDeliveryGroups(null), [])
  assert.equal(orderDeliveryState(null), 'unpaid')
  assert.equal(orderDeliveryState({ status: 'paid', children: 'invalid' }), 'waiting')
})

test('a split order preserves both parent and child fulfillment records', () => {
  const order = { order_no: 'parent', status: 'delivered', fulfillment: { status: 'delivered', payload: 'parent-key' }, children: [
    { order_no: 'child', fulfillment: { status: 'delivered', payload: 'child-key' } },
  ] }
  assert.deepEqual(orderDeliveryGroups(order).map(group => group.fulfillment.payload), ['parent-key', 'child-key'])
  assert.equal(orderDeliveryState(order), 'ready')
})
