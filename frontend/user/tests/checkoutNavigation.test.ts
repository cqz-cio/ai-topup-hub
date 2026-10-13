import test from 'node:test'
import assert from 'node:assert/strict'
import { checkoutReturnPath, completedCheckoutReturnPath } from '../src/utils/checkoutNavigation.ts'

test('a single product or several SKUs of it return to that product detail', () => {
  assert.equal(checkoutReturnPath([{ slug: 'qzweb-plus' }]), '/products/qzweb-plus')
  assert.equal(checkoutReturnPath([{ slug: 'qzweb-plus' }, { slug: 'qzweb-plus' }]), '/products/qzweb-plus')
  assert.equal(checkoutReturnPath([{ slug: 'Plus 月卡' }]), '/products/Plus%20%E6%9C%88%E5%8D%A1')
})

test('mixed or missing products return to the product list', () => {
  assert.equal(checkoutReturnPath([{ slug: 'plus' }, { slug: 'gemini' }]), '/products')
  assert.equal(checkoutReturnPath([{ slug: '' }]), '/products')
  assert.equal(checkoutReturnPath([]), '/products')
  assert.equal(checkoutReturnPath([{ slug: '..' }]), '/products')
})

test('returning to or reloading a completed checkout skips it', () => {
  const state = { checkoutReturnPath: '/products/qzweb-plus' }
  assert.equal(completedCheckoutReturnPath('/checkout?mode=buynow', '/checkout?mode=buynow', state), '/products/qzweb-plus')
  assert.equal(completedCheckoutReturnPath('/checkout', '/checkout', { checkoutReturnPath: '/products' }), '/products')
})

test('a new checkout or unrelated navigation is not redirected by old history state', () => {
  const state = { checkoutReturnPath: '/products/qzweb-plus' }
  assert.equal(completedCheckoutReturnPath('/checkout', '/products/qzweb-plus', state), null)
  assert.equal(completedCheckoutReturnPath('/checkout', '/checkout', {}), null)
  assert.equal(completedCheckoutReturnPath('/me/orders', '/me/orders', state), null)
})

test('history return targets are restricted to local product routes', () => {
  for (const checkoutReturnPath of ['https://example.com', '//example.com', '/checkout', '/pay', '/products/\\example.com', '/products?redirect=other', 123]) {
    assert.equal(completedCheckoutReturnPath('/checkout', '/checkout', { checkoutReturnPath }), null)
  }
})
