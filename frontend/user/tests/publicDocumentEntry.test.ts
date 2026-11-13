import test from 'node:test'
import assert from 'node:assert/strict'
import { publicDocumentSources } from '../build/publicDocumentEntry.ts'

test('public documents boot the same hashed modules and styles as index.html', () => {
  const entry = publicDocumentSources('<script data-cfasync="false" type="module" crossorigin src="/assets/index-123.js"></script><link rel="modulepreload" href="/assets/vendor-456.js"><link rel="stylesheet" crossorigin href="/assets/index-789.css">')
  assert.match(entry.js, /import\("\/assets\/index-123\.js"\)/)
  assert.match(entry.js, /\.catch\(/)
  assert.match(entry.js, /data-public-document-boot-failed/)
  assert.ok(entry.css.startsWith('@import "/assets/index-789.css";\n'))
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
