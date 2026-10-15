import test from 'node:test'
import assert from 'node:assert/strict'
import { createPinia } from 'pinia'
import { useBuyNowStore } from '../src/stores/buyNow.ts'
import { BUY_NOW_SESSION_KEY, BUY_NOW_MAX_AGE, readBuyNowSession, saveBuyNowSession, clearBuyNowSession } from '../src/utils/buyNowSession.ts'

const storage = () => {
  const values = new Map<string, string>()
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => { values.set(key, value) },
    removeItem: (key: string) => { values.delete(key) },
  }
}
const item = {
  productId: 105, skuId: 1105, skuCode: 'MONTH', slug: 'qzweb-5',
  title: { 'zh-CN': 'ChatGPT Plus' }, priceAmount: '130.00', quantity: 2,
  skuSpecValues: { period: '一个月' }, purchaseType: 'guest', fulfillmentType: 'auto',
  paymentChannelIds: [1, 2], skuStockEnforced: true, skuAutoStockAvailable: 3,
}

test('buy-now survives a fresh Pinia instance with the same SKU and quantity, replaces, and clears', () => {
  const sessionStorage = storage()
  const oldWindow = Object.getOwnPropertyDescriptor(globalThis, 'window')
  Object.defineProperty(globalThis, 'window', { configurable: true, value: { sessionStorage } })
  try {
    useBuyNowStore(createPinia()).setItem(item)
    const refreshed = useBuyNowStore(createPinia())
    assert.deepEqual(JSON.parse(JSON.stringify(refreshed.item)), item)
    refreshed.setItem({ ...item, productId: 155, skuId: 1155, slug: 'qzweb-55', quantity: 1 })
    assert.equal(useBuyNowStore(createPinia()).item?.productId, 155)
    refreshed.clear()
    assert.equal(useBuyNowStore(createPinia()).item, null)
    assert.equal(sessionStorage.getItem(BUY_NOW_SESSION_KEY), null)
  } finally {
    if (oldWindow) Object.defineProperty(globalThis, 'window', oldWindow)
    else Reflect.deleteProperty(globalThis, 'window')
  }
})

test('buy-now session excludes checkout secrets and leaves the regular cart untouched', () => {
  const s = storage()
  s.setItem('cart_items', 'unchanged')
  saveBuyNowSession({ ...item, email: 'private@example.invalid', order_password: 'private', manualFormValues: { account: 'private' } } as typeof item, s, 1000)
  assert.equal(s.getItem(BUY_NOW_SESSION_KEY)?.includes('private'), false)
  clearBuyNowSession(s)
  assert.equal(s.getItem('cart_items'), 'unchanged')
  assert.equal(readBuyNowSession(storage(), 1000), null)
})

test('buy-now removes malformed, incompatible, future and expired snapshots', () => {
  for (const raw of ['broken', 'null', '{}',
    JSON.stringify({ version: 2, savedAt: 1000, item }),
    JSON.stringify({ version: 1, savedAt: 3000, item }),
    JSON.stringify({ version: 1, savedAt: 2000 - BUY_NOW_MAX_AGE, item })]) {
    const s = storage(); s.setItem(BUY_NOW_SESSION_KEY, raw)
    assert.equal(readBuyNowSession(s, 2000), null)
    assert.equal(s.getItem(BUY_NOW_SESSION_KEY), null)
  }
})

test('buy-now rejects invalid identity, quantity or price instead of restoring a misleading item', () => {
  for (const patch of [{ quantity: 0 }, { quantity: 1.5 }, { productId: -1 }, { skuId: -2 }, { priceAmount: 'NaN' }, { slug: '' }]) {
    const s = storage()
    s.setItem(BUY_NOW_SESSION_KEY, JSON.stringify({ version: 1, savedAt: 1000, item: { ...item, ...patch } }))
    assert.equal(readBuyNowSession(s, 2000), null)
  }
})

test('disabled storage does not break buying; a failed write removes the previous selection', () => {
  const s = storage(); saveBuyNowSession(item, s, 1000)
  s.setItem = () => { throw new Error('Quota exceeded') }
  assert.doesNotThrow(() => saveBuyNowSession({ ...item, quantity: 3 }, s, 2000))
  assert.equal(readBuyNowSession(s, 2000), null)
  const denied = { getItem: () => { throw new Error('Denied') }, setItem: () => { throw new Error('Denied') }, removeItem: () => { throw new Error('Denied') } }
  assert.equal(readBuyNowSession(denied), null)
  assert.doesNotThrow(() => saveBuyNowSession(item, denied))
  assert.doesNotThrow(() => clearBuyNowSession(denied))
})
