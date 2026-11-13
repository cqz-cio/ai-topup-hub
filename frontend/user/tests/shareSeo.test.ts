import test from 'node:test'
import assert from 'node:assert/strict'
import { identityLogo, shareMetadata } from '../src/utils/shareSeo.ts'

test('sharing skips SVG and unsafe URLs while preserving supported raster images', () => {
  const origin = 'https://www.qzweb.cloud'
  assert.deepEqual(shareMetadata(origin, ['/pro.svg', '/cover.png?version=2'], '/logo.webp'), {
    image: origin + '/cover.png?version=2', card: 'summary_large_image',
  })
  for (const raw of ['/pro.svg', 'javascript:bad.png', 'data:image/png;base64,x', 'https://secret:password@example.com/a.png']) {
    assert.deepEqual(shareMetadata(origin, [raw], '/logo.webp'), { image: origin + '/logo.webp', card: 'summary' })
  }
  assert.deepEqual(shareMetadata(origin, [], 'invalid.svg'), { image: '', card: 'summary' })
})

test('main site reuses its lossless logo without applying it to reseller branding', () => {
  assert.equal(identityLogo('/brand/patrick-logo.png', true), '/media/storefront-20261020/patrick-logo.webp')
  assert.equal(identityLogo('/brand/patrick-logo.png', false), '/brand/patrick-logo.png')
  assert.equal(identityLogo('/my-logo.png', true), '/my-logo.png')
})
