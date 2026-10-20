import test from 'node:test'
import assert from 'node:assert/strict'
import { afterPageLoad, canWarmMedia, catalogWarmupUrls, warmCatalogImages } from '../src/utils/mediaLoading.ts'

function browser(t: any) {
  const win = new EventTarget()
  const doc = Object.assign(new EventTarget(), { readyState: 'loading', visibilityState: 'visible' })
  let next = 0
  const timers = new Map<number, () => void>()
  const idle = new Map<number, () => void>()
  const images: Array<{ src: string; onload: (() => void) | null; onerror: (() => void) | null }> = []
  class FakeImage {
    src = ''
    onload = null
    onerror = null
    constructor() { images.push(this) }
  }
  const window = Object.assign(win, {
    setTimeout: (fn: () => void) => { timers.set(++next, fn); return next },
    clearTimeout: (id: number) => timers.delete(id),
    requestIdleCallback: (fn: () => void) => { idle.set(++next, fn); return next },
    cancelIdleCallback: (id: number) => idle.delete(id),
  })
  for (const [name, value] of Object.entries({ window, document: doc, navigator: { connection: { effectiveType: '4g' } }, Image: FakeImage })) {
    const previous = Object.getOwnPropertyDescriptor(globalThis, name)
    Object.defineProperty(globalThis, name, { configurable: true, value })
    t.after(() => {
      if (previous) Object.defineProperty(globalThis, name, previous)
      else Reflect.deleteProperty(globalThis, name)
    })
  }
  const flush = (queue: Map<number, () => void>) => {
    const entries = [...queue.values()]
    queue.clear()
    entries.forEach(fn => fn())
  }
  return { win, doc, timers, idle, images, flush }
}

test('slow networks and data saver skip speculative media requests', () => {
  assert.equal(canWarmMedia(), true)
  assert.equal(canWarmMedia({ effectiveType: '4g' }), true)
  for (const effectiveType of ['slow-2g', '2g', '3g']) assert.equal(canWarmMedia({ effectiveType }), false)
  assert.equal(canWarmMedia({ saveData: true, effectiveType: '4g' }), false)
})

test('warmup is bounded, deduplicated and limited to public local catalog media', () => {
  assert.deepEqual(catalogWarmupUrls([
    '/catalog-products/a.svg', '/catalog-products/a.svg', '/media/storefront-20261020/gpt.webp',
    'https://example.com/a.png', '//example.com/a.png', '/api/orders/1/delivery', '/uploads/private.png',
  ]), ['/catalog-products/a.svg', '/media/storefront-20261020/gpt.webp'])
  assert.equal(catalogWarmupUrls(Array.from({ length: 100 }, (_, n) => `/catalog-products/${n}.svg`)).length, 24)
})

test('initial load completes before background work starts, then runs once at idle', t => {
  const b = browser(t)
  let calls = 0
  afterPageLoad(() => calls++)
  assert.equal(b.timers.size, 0)
  b.doc.readyState = 'complete'
  b.win.dispatchEvent(new Event('load'))
  b.flush(b.timers)
  assert.equal(calls, 0)
  b.flush(b.idle)
  assert.equal(calls, 1)
  b.doc.dispatchEvent(new Event('visibilitychange'))
  assert.equal(b.timers.size, 0)
})

test('hidden tabs defer work until visible, including a tab hidden during idle scheduling', t => {
  const b = browser(t)
  b.doc.readyState = 'complete'
  let calls = 0
  afterPageLoad(() => calls++)
  b.flush(b.timers)
  b.doc.visibilityState = 'hidden'
  b.flush(b.idle)
  assert.equal(calls, 0)
  b.doc.visibilityState = 'visible'
  b.doc.dispatchEvent(new Event('visibilitychange'))
  b.flush(b.timers)
  b.flush(b.idle)
  assert.equal(calls, 1)
})

test('unmount cancels pending load listeners, timers and idle work', t => {
  const b = browser(t)
  let calls = 0
  const cancel = afterPageLoad(() => calls++)
  cancel()
  b.doc.readyState = 'complete'
  b.win.dispatchEvent(new Event('load'))
  assert.equal(b.timers.size, 0)
  const cancelIdle = afterPageLoad(() => calls++)
  b.flush(b.timers)
  cancelIdle()
  b.flush(b.idle)
  assert.equal(calls, 0)
})

test('catalog fetches one image at a time and survives failures without a request burst', t => {
  const b = browser(t)
  b.doc.readyState = 'complete'
  const cancel = warmCatalogImages(['/catalog-products/a.svg', '/catalog-products/b.svg'])
  b.flush(b.timers)
  b.flush(b.idle)
  assert.equal(b.images.length, 1)
  assert.equal(b.images[0]?.src, '/catalog-products/a.svg')
  b.images[0]?.onerror?.()
  b.flush(b.timers)
  b.flush(b.idle)
  assert.equal(b.images.length, 2)
  cancel()
  assert.equal(b.images[1]?.onload, null)
  assert.equal(b.timers.size, 0)
})

test('a stalled media download stops speculation instead of starting more requests', t => {
  const b = browser(t)
  b.doc.readyState = 'complete'
  warmCatalogImages(['/catalog-products/a.svg', '/catalog-products/b.svg'])
  b.flush(b.timers)
  b.flush(b.idle)
  b.flush(b.timers)
  assert.equal(b.images.length, 1)
  assert.equal(b.images[0]?.onload, null)
  assert.equal(b.idle.size, 0)
})
