import test from 'node:test'
import assert from 'node:assert/strict'
import { loadSceneArtwork } from '../src/utils/jellyfishing/artwork.js'

function images(t: any) {
  const created: Array<{
    src: string; onload: (() => void) | null; onerror: (() => void) | null
  }> = []
  class FakeImage {
    src = ''
    onload = null
    onerror = null
    constructor() { created.push(this) }
  }
  const previous = Object.getOwnPropertyDescriptor(globalThis, 'Image')
  Object.defineProperty(globalThis, 'Image', { configurable: true, value: FakeImage })
  t.after(() => {
    if (previous) Object.defineProperty(globalThis, 'Image', previous)
    else Reflect.deleteProperty(globalThis, 'Image')
  })
  return created
}

test('artwork can finish after a slow download without permanently disabling motion', async t => {
  const created = images(t)
  t.mock.timers.enable({ apis: ['setTimeout'] })
  const pending = new Set<() => void>()
  let state = 'loading'
  const promise = loadSceneArtwork('/scene.webp', pending).then(
    image => { state = 'ready'; return image },
    error => { state = 'failed'; throw error },
  )
  t.mock.timers.tick(60000)
  await Promise.resolve()
  assert.equal(state, 'loading')
  assert.equal(pending.size, 1)
  assert.equal(created[0]!.src, '/scene.webp')
  created[0]!.onload!()
  assert.equal(await promise, created[0])
  assert.equal(state, 'ready')
  assert.equal(pending.size, 0)
})

test('disposing a scene cancels unfinished artwork and removes late callbacks', async t => {
  const created = images(t)
  const pending = new Set<() => void>()
  const promise = loadSceneArtwork('/scene.webp', pending)
  const rejected = assert.rejects(promise, /Scene disposed/)
  for (const cancel of [...pending]) cancel()
  await rejected
  assert.equal(pending.size, 0)
  assert.equal(created[0]!.src, '')
  assert.equal(created[0]!.onload, null)
  assert.equal(created[0]!.onerror, null)
})

test('an actual image error still reports failure and releases pending work', async t => {
  const created = images(t)
  const pending = new Set<() => void>()
  const promise = loadSceneArtwork('/missing.webp', pending)
  const rejected = assert.rejects(promise, /Scene artwork unavailable/)
  created[0]!.onerror!()
  await rejected
  assert.equal(pending.size, 0)
  assert.equal(created[0]!.onload, null)
  assert.equal(created[0]!.onerror, null)
})
