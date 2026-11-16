import test from 'node:test'
import assert from 'node:assert/strict'
import vm from 'node:vm'
import { publicDocumentSources } from '../build/publicDocumentEntry.ts'
import { publicDocumentBootScript } from '../build/publicDocumentBoot.ts'

test('public documents boot the same hashed modules and styles as index.html', () => {
  const entry = publicDocumentSources('<script data-cfasync="false" type="module" crossorigin src="/assets/index-123.js"></script><link rel="modulepreload" href="/assets/vendor-456.js"><link rel="stylesheet" crossorigin href="/assets/index-789.css">')
  assert.match(entry.js, /import\("\/assets\/index-123\.js"\)/)
  assert.match(entry.js, /\.catch\(/)
  assert.match(entry.js, /data-public-document-boot-failed/)
  assert.deepEqual(entry.styles, ['/assets/index-789.css', entry.bootStyle])
  assert.match(entry.bootStyle, /^\/assets\/public-document-[a-f0-9]{16}\.css$/)
  assert.equal(entry.css.includes('@import'), false)
  assert.ok(entry.css.length < 5000, 'stable fallback contains only boot styling')
  assert.match(entry.js, /\/assets\/index-789\.css/)
  assert.match(entry.css, /@media \(scripting: enabled\)/)
  assert.match(entry.css, /public-document-reveal 0s 10s forwards/)
  assert.match(entry.css, /#app:has\(> #public-document\)/)
  assert.equal(/url\(/.test(entry.css), false, 'loading shell must not download extra images')
  assert.ok(entry.css.includes('meta[property="og:site_name"][content="π星卡社"]'), 'do not apply main-store branding to other tenants')
})

test('build fails instead of shipping a public document with broken boot assets', () => {
  assert.throws(() => publicDocumentSources('<div id="app"></div>'), /missing/)
  assert.throws(() => publicDocumentSources('<script type="module" src="https://external.invalid/app.js"></script><link rel="stylesheet" href="/assets/app.css">'), /Unexpected/)
})

test('a stylesheet that failed before the stable entry ran gets a fresh load attempt', async () => {
  const existing = { sheet: { cssRules: [] }, getAttribute: () => '/assets/app-new.css', hasAttribute: () => true }
  const listeners: Record<string, () => void> = {}
  let appended = 0, failed = false
  const document = {
    querySelectorAll: () => [existing],
    createElement: () => ({ setAttribute: () => {}, addEventListener: (event: string, callback: () => void) => { listeners[event] = callback } }),
    head: { appendChild: () => { appended++; queueMicrotask(() => listeners.load!()) } },
    documentElement: { setAttribute: () => { failed = true } },
  }
  await vm.runInNewContext(publicDocumentBootScript([], ['/assets/app-new.css']), { document })
  assert.equal(appended, 1)
  assert.equal(failed, false)
})

test('an unavailable current stylesheet restores the readable public document', async () => {
  const listeners: Record<string, () => void> = {}
  let failed = false
  const document = {
    querySelectorAll: () => [],
    createElement: () => ({ setAttribute: () => {}, addEventListener: (event: string, callback: () => void) => { listeners[event] = callback } }),
    head: { appendChild: () => { queueMicrotask(() => listeners.error!()) } },
    documentElement: { setAttribute: (name: string) => { failed = name === 'data-public-document-boot-failed' } },
  }
  await vm.runInNewContext(publicDocumentBootScript([], ['/assets/app-new.css']), { document })
  assert.equal(failed, true)
})

test('an already loaded current stylesheet needs no additional request', async () => {
  const document = {
    querySelectorAll: () => [{sheet:{cssRules:[{}]},getAttribute:()=>'/assets/app-new.css',hasAttribute:()=>true}],
    createElement: () => { throw new Error('loaded CSS must be reused') },
    documentElement: { setAttribute: () => { throw new Error('loaded CSS is not a failure') } },
  }
  await vm.runInNewContext(publicDocumentBootScript([], ['/assets/app-new.css']), { document })
})

for (const replacementLoads of [true, false]) test(`a cached document ${replacementLoads ? 'retires old owned CSS only after replacement loads' : 'keeps old CSS when replacement fails'}`, async () => {
  const removed: string[] = []
  const link = (href: string, owned: boolean) => ({
    sheet: { cssRules: [{}] },
    getAttribute: () => href,
    hasAttribute: () => owned,
    remove: () => removed.push(href),
  })
  const existing = [
    link('/assets/index-old.css', false), // Older unmarked SSR output.
    link('/assets/public-document-old.css', true),
    link('/assets/partner-widget.css', false),
    link('https://other.invalid/theme.css', false),
  ]
  const listeners: Record<string, () => void> = {}
  let failed = false
  const document = {
    querySelectorAll: () => existing,
    createElement: () => ({ setAttribute: () => {}, addEventListener: (event: string, callback: () => void) => { listeners[event] = callback } }),
    head: { appendChild: () => {
      assert.deepEqual(removed, [], 'readable document retains old styles while loading')
      queueMicrotask(() => listeners[replacementLoads ? 'load' : 'error']!())
    } },
    documentElement: { setAttribute: () => { failed = true } },
  }
  await vm.runInNewContext(publicDocumentBootScript([], ['/assets/index-new.css']), { document })
  assert.deepEqual(removed, replacementLoads ? ['/assets/index-old.css', '/assets/public-document-old.css'] : [])
  assert.equal(failed, !replacementLoads)
})
