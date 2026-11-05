import test from 'node:test'
import assert from 'node:assert/strict'
import { publicDocumentSources } from '../build/publicDocumentEntry.ts'

test('public documents boot the same hashed modules and styles as index.html', () => {
  const entry = publicDocumentSources('<script data-cfasync="false" type="module" crossorigin src="/assets/index-123.js"></script><link rel="modulepreload" href="/assets/vendor-456.js"><link rel="stylesheet" crossorigin href="/assets/index-789.css">')
  assert.equal(entry.js, 'import "/assets/index-123.js";\n')
  assert.equal(entry.css, '@import "/assets/index-789.css";\n')
})

test('build fails instead of shipping a public document with broken boot assets', () => {
  assert.throws(() => publicDocumentSources('<div id="app"></div>'), /missing/)
  assert.throws(() => publicDocumentSources('<script type="module" src="https://external.invalid/app.js"></script><link rel="stylesheet" href="/assets/app.css">'), /Unexpected/)
})
