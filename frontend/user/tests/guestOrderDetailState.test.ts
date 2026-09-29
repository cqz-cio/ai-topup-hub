import test from 'node:test'
import assert from 'node:assert/strict'
import { resolveGuestOrderDetailViewState } from '../src/utils/guestOrderDetailState.ts'

test('guest order detail shows loading before authentication state is initialized', () => {
  assert.equal(
    resolveGuestOrderDetailViewState({
      loading: true,
      order: null,
      showAuthForm: true,
    }),
    'loading',
  )
})

test('guest order detail shows only authentication when credentials are missing or invalid', () => {
  assert.equal(
    resolveGuestOrderDetailViewState({
      loading: false,
      order: null,
      showAuthForm: true,
    }),
    'auth',
  )
  assert.equal(
    resolveGuestOrderDetailViewState({
      loading: false,
      order: { order_no: 'stale-order' },
      showAuthForm: true,
    }),
    'auth',
  )
})

test('guest order detail renders fields only when an order is present and authentication is valid', () => {
  assert.equal(
    resolveGuestOrderDetailViewState({
      loading: false,
      order: { order_no: 'DJ-1001' },
      showAuthForm: false,
    }),
    'detail',
  )
  assert.equal(
    resolveGuestOrderDetailViewState({
      loading: false,
      order: null,
      showAuthForm: false,
    }),
    'empty',
  )
})
