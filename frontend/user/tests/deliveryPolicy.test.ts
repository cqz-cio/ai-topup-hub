import test from 'node:test'
import assert from 'node:assert/strict'
import { appendDeliveryPolicy, merchantReturnPolicy, offerMerchantIdentity } from '../src/utils/deliveryPolicy.ts'
import { appendRechargeLinks } from '../src/utils/gptRecharge.ts'
import { siteIdentitySchema } from '../src/utils/siteIdentity.ts'

test('main-site return policy uses the real policy URL without inventing refund or shipping terms', () => {
  const origin = 'https://www.qzweb.cloud/'
  const policy = merchantReturnPolicy(origin)
  const identity = siteIdentitySchema({ site_url: origin }, '', '', true)['@graph'][0]
  assert.deepEqual(identity.hasMerchantReturnPolicy, policy)
  assert.equal(policy.merchantReturnLink, 'https://www.qzweb.cloud/blog/purchase-and-agent-guide')
  assert.deepEqual(offerMerchantIdentity(origin, false).hasMerchantReturnPolicy, { '@id': policy['@id'] })
  assert.deepEqual(Object.keys(policy).sort(), ['@id', '@type', 'merchantReturnLink'])
})

test('reseller identity and product content do not inherit the main-store policy', () => {
  const origin = 'https://reseller.example'
  assert.equal(appendDeliveryPolicy('<p>Own terms</p>', true), '<p>Own terms</p>')
  assert.equal(siteIdentitySchema({ site_url: origin }, '')['@graph'][0].hasMerchantReturnPolicy, undefined)
  assert.deepEqual(offerMerchantIdentity(origin, true), { seller: { '@id': origin + '/#organization' } })
})

test('visible policy entry preserves existing conditions and section layout, and avoids duplicates', () => {
  const original = '<h2>Existing conditions</h2><p>Exact existing refund rules</p>'
  const content = appendDeliveryPolicy(original, false)
  assert.ok(content.startsWith(original))
  assert.equal(content.match(/<h2>/g)?.length, 1)
  assert.equal(appendDeliveryPolicy(content, false), content)
  const gpt = appendRechargeLinks(original, 'product', 'qzweb-5', 'gpt-recharge', false)
  assert.equal(appendDeliveryPolicy(gpt, false), gpt)
  assert.equal(gpt.match(/href="\/blog\/purchase-and-agent-guide"/g)?.length, 1)
})
