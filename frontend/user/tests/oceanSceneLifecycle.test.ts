import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { runInNewContext } from 'node:vm'

const bubbleSource = readFileSync(new URL('../src/utils/oceanScene/bubbleWorld.js', import.meta.url), 'utf8').replace('export class', 'class')
const spriteData = readFileSync(new URL('../src/utils/oceanScene/bubbleSprites.json', import.meta.url), 'utf8')
const source = bubbleSource + '\nconst spriteData = ' + spriteData + ';\n' + readFileSync(new URL('../src/utils/oceanScene/renderer.js', import.meta.url), 'utf8')
  .replace(/^import .*;$/gm, '')
  .replace('export function mountOceanScene', 'function mountOceanScene') + '\nglobalThis.mount = mountOceanScene;'

function setup(options: { unavailable?: boolean; shaderFailure?: boolean; width?: number; height?: number } = {}) {
  const callbacks = new Map<number, (time: number) => void>(), timers = new Map<number, () => void>()
  let id = 0, draws = 0, released = 0, disconnected = false, ready = 0, failed = 0
  const uniforms: Record<string, unknown> = {}
  function target() {
    const listeners = new Map<string, Function>()
    return { listeners, addEventListener: (k: string, cb: Function) => listeners.set(k, cb),
      removeEventListener: (k: string) => listeners.delete(k), emit: (k: string, e = {}) => listeners.get(k)?.(e) }
  }
  const document = { ...target(), hidden: false }, window = target()
  const images: Array<{ onload: Function | null; onerror: Function | null; src: string }> = []
  const gl = new Proxy({
    createShader: () => ({}), createProgram: () => ({}), createBuffer: () => ({}), createTexture: () => ({}),
    getShaderParameter: () => !options.shaderFailure, getProgramParameter: () => true,
    getUniformLocation: (_: unknown, name: string) => name, getAttribLocation: () => 0,
    uniform1f: (key: string, value: number) => uniforms[key] = value,
    uniform2f: (key: string, x: number, y: number) => uniforms[key] = [x, y],
    drawArrays: () => draws++, deleteShader: () => released++, deleteProgram: () => released++,
    deleteTexture: () => released++, deleteBuffer: () => released++, getExtension: () => null,
  }, { get: (o, key) => key in o ? o[key as keyof typeof o] : () => {} })
  const canvas = { ...target(), width: 0, height: 0, getContext: () => options.unavailable ? null : gl,
    getBoundingClientRect: () => ({ width: options.width ?? 1280, height: options.height ?? 720 }) }
  const bubbles = { width: 0, height: 0, getContext: () => new Proxy({}, { get: () => () => {} }) }
  const sandbox: Record<string, any> = { document, window, devicePixelRatio: 3, Float32Array,
    Image: class { src = ''; naturalWidth = 1672; naturalHeight = 941; onload = null; onerror = null; constructor() { images.push(this) } },
    ResizeObserver: class { observe() {} disconnect() { disconnected = true } },
    requestAnimationFrame: (cb: (t: number) => void) => { const n = ++id; callbacks.set(n, cb); return n },
    cancelAnimationFrame: (n: number) => callbacks.delete(n),
    setTimeout: (cb: () => void) => { const n = ++id; timers.set(n, cb); return n }, clearTimeout: (n: number) => timers.delete(n),
  }
  runInNewContext(source, sandbox)
  return { canvas, document, window, images, callbacks, timers, uniforms,
    mount: (key = 'garden') => sandbox.mount(canvas, bubbles, key, { onReady: () => ready++, onFallback: () => failed++ }),
    frame: (t: number) => { const pending = [...callbacks.values()]; callbacks.clear(); pending.forEach(cb => cb(t)) },
    stats: () => ({ draws, released, disconnected, ready, failed }),
  }
}

test('ocean scene pauses hidden/BFCache pages and resumes without a time jump', () => {
  const f = setup(), scene = f.mount()
  f.images.forEach(image => image.onload!()); f.frame(0); f.frame(16)
  const initial = f.stats().draws
  f.document.hidden = true; f.document.emit('visibilitychange'); f.frame(10000)
  assert.equal(f.callbacks.size, 0); assert.equal(f.stats().draws, initial)
  f.document.hidden = false; f.document.emit('visibilitychange'); f.frame(11000)
  assert.equal(f.uniforms.t, .016)
  f.window.emit('pagehide'); assert.equal(f.callbacks.size, 0)
  f.window.emit('pageshow'); assert.equal(f.callbacks.size, 1)
  scene.dispose(); scene.dispose()
  assert.equal(f.callbacks.size, 0); assert.equal(f.document.listeners.size, 0)
  assert.equal(f.window.listeners.size, 0); assert.equal(f.canvas.listeners.size, 0)
  assert.equal(f.stats().released, 5); assert.equal(f.stats().disconnected, true)
})

test('switching away during loading cancels timeout and ignores a stale image callback', () => {
  const f = setup(), scene = f.mount(), stale = f.images[0].onload!
  scene.dispose(); stale()
  assert.equal(f.timers.size, 0); assert.equal(f.callbacks.size, 0)
  assert.equal(f.stats().ready, 0); assert.equal(f.stats().failed, 0)
  assert.equal(f.images[0].src, '')
})

test('context loss releases resources and falls back exactly once', () => {
  const f = setup(); f.mount(); f.images[0].onload!()
  let prevented = false
  f.canvas.emit('webglcontextlost', { preventDefault: () => prevented = true })
  assert.equal(prevented, true); assert.equal(f.stats().failed, 1)
  assert.equal(f.callbacks.size, 0); assert.equal(f.stats().released, 5)
})

test('unavailable WebGL, shader failure, failed image retain static fallback', () => {
  for (const kind of ['unavailable', 'shaderFailure', 'image']) {
    const f = setup({ unavailable: kind === 'unavailable', shaderFailure: kind === 'shaderFailure' })
    f.mount()
    if (kind === 'image') f.images[0].onerror!()
    assert.equal(f.stats().failed, 1, kind); assert.equal(f.stats().ready, 0, kind)
    assert.equal(f.callbacks.size, 0, kind); assert.equal(f.timers.size, 0, kind)
  }
})

test('four scenes preserve static mobile crops and limit high-DPI canvas cost', () => {
  for (const [key, alignment] of Object.entries({ garden: .1, street: .85, home: .85, detail: .7 })) {
    const f = setup({ width: 390, height: 844 }), scene = f.mount(key)
    assert.match(f.images[0].src, /^\/media\/ocean-20261023\//)
    f.images.forEach(image => image.onload!()); assert.equal(f.uniforms.alignment, alignment); scene.dispose()
  }
  const f = setup({ width: 3840, height: 2160 }), scene = f.mount()
  f.images.forEach(image => image.onload!()); assert.ok(f.canvas.width * f.canvas.height <= 2403000); scene.dispose()
})

test('clean background and original bubble atlas must both load before revealing animation', () => {
  for (const order of [[0, 1], [1, 0]]) {
    const f = setup(), scene = f.mount('detail');
    assert.match(f.images[0].src, /detail-clean.webp$/);
    assert.match(f.images[1].src, /detail-bubbles.webp$/);
    f.images[order[0]].onload!();
    assert.equal(f.stats().ready, 0); assert.equal(f.callbacks.size, 0);
    f.images[order[1]].onload!();
    assert.equal(f.stats().ready, 1); assert.equal(f.callbacks.size, 1);
    scene.dispose();
  }
});

test('a failed or stale atlas keeps the static wallpaper and clears both image handlers', () => {
  const f = setup(); f.mount();
  const stale = f.images[1].onload!;
  f.images[0].onload!(); f.images[1].onerror!(); stale();
  assert.equal(f.stats().failed, 1); assert.equal(f.stats().ready, 0);
  assert.equal(f.callbacks.size, 0); assert.equal(f.timers.size, 0);
  for (const image of f.images) {
    assert.equal(image.src, ''); assert.equal(image.onload, null); assert.equal(image.onerror, null);
  }
});

test('slow successful images retain motion instead of falling back at an arbitrary deadline', () => {
  const f = setup(), scene = f.mount();
  assert.equal(f.timers.size, 0);
  f.frame(30000);
  f.images.forEach(image => image.onload!());
  assert.equal(f.stats().ready, 1); assert.equal(f.stats().failed, 0);
  scene.dispose();
});

test('high refresh displays cap GPU paints without slowing elapsed animation time', () => {
  for (const hz of [60, 120, 144, 240]) {
    const f = setup(), scene = f.mount();
    f.images.forEach(image => image.onload!());
    const before = f.stats().draws;
    for (let n = 0; n <= hz; n++) f.frame(n * 1000 / hz);
    const paints = f.stats().draws - before;
    assert.ok(paints >= 58 && paints <= 64, `${hz}Hz produced ${paints} paints`);
    assert.ok(Math.abs(Number(f.uniforms.t) - 1) < .02);
    scene.dispose();
  }
});
