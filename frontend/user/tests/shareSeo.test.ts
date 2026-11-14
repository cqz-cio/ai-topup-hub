import test from 'node:test'
import assert from 'node:assert/strict'
import { identityLogo, shareMetadata, defaultShareImage } from '../src/utils/shareSeo.ts'
import assets from '../../../internal/platform/publiccontent/seo-assets.json' with { type: 'json' }
import generated from '../../../internal/platform/publiccontent/seo-share-images.json' with { type: 'json' }

const origin = 'https://www.qzweb.cloud'
test('sharing skips SVG and unsafe URLs, with no invented dimensions for raw rasters', () => {
  assert.deepEqual(shareMetadata(origin, ['/pro.svg', '/cover.png?version=2'], assets.shareImage, '商品'), {
    image: origin + '/cover.png?version=2', card: 'summary_large_image', width: 0, height: 0, alt: '商品',
  })
  for (const raw of ['/pro.svg', 'javascript:bad.png', 'data:image/png;base64,x', 'https://secret:password@example.com/a.png']) {
    assert.deepEqual(shareMetadata(origin, [raw], assets.shareImage, '品牌'), {
      image: origin + assets.shareImage, card: 'summary_large_image', width: 1200, height: 630, alt: '品牌',
    })
  }
  assert.equal(shareMetadata(origin, [], 'invalid.svg').image, '')
})
test('generated product cards and defaults agree on real dimensions', () => {
  const [source, image] = Object.entries(generated)[0]!
  const result = shareMetadata(origin, [source], assets.shareImage, '商品')
  assert.equal(result.image, origin + image)
  assert.equal(result.width, 1200)
  assert.equal(result.height, 630)
  assert.equal(result.card, 'summary_large_image')
  assert.equal(shareMetadata(origin, ['https://other.example'+source], assets.shareImage).width, 0)
  assert.equal(defaultShareImage(assets.logo, origin, true), assets.shareImage)
  assert.equal(defaultShareImage('/custom.jpg', origin, true), '/custom.jpg')
  assert.equal(defaultShareImage('/custom.jpg', origin, false), '/custom.jpg')
})
test('organization logo stays independent of share card and reseller branding', () => {
  assert.equal(identityLogo('/brand/patrick-logo.png', true), '/media/storefront-20261020/patrick-logo.webp')
  assert.equal(identityLogo('/brand/patrick-logo.png', false), '/brand/patrick-logo.png')
})
