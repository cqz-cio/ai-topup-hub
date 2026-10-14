import test from 'node:test'
import assert from 'node:assert/strict'
import {
  canRenderResellerConsoleModule,
  getResellerConsoleState,
  resolveResellerConsoleModule,
} from '../src/utils/resellerConsole.ts'

test('application form obeys server admission for new, rejected, pending and disabled accounts', () => {
  for (const status of ['not_opened', 'rejected', 'pending_review', 'active', 'disabled']) {
    const state = getResellerConsoleState({
      opened: status !== 'not_opened',
      can_apply: false,
      profile: status === 'not_opened' ? undefined : { status },
      domains: [],
    } as any)
    assert.equal(state.modules.apply.enabled, false, status)
  }
  for (const status of ['not_opened', 'rejected']) {
    const state = getResellerConsoleState({
      opened: status !== 'not_opened',
      can_apply: true,
      profile: status === 'not_opened' ? undefined : { status },
      domains: [],
    } as any)
    assert.equal(state.modules.apply.enabled, true, status)
  }
  assert.equal(getResellerConsoleState(null).modules.apply.enabled, false)
  assert.equal(getResellerConsoleState(undefined).modules.apply.enabled, false)
})

test('reseller console route gate blocks active-only modules until profile is active', () => {
  const pendingState = getResellerConsoleState({
    opened: true,
    can_apply: false,
    profile: { status: 'pending_review' },
    domains: [],
  } as any)
  const activeState = getResellerConsoleState({
    opened: true,
    can_apply: false,
    profile: { status: 'active' },
    domains: [],
  } as any)

  assert.equal(resolveResellerConsoleModule('/reseller/products'), 'products')
  assert.equal(canRenderResellerConsoleModule('/reseller/apply', pendingState), true)
  assert.equal(canRenderResellerConsoleModule('/reseller', pendingState), true)
  assert.equal(canRenderResellerConsoleModule('/reseller/products', pendingState), false)
  assert.equal(canRenderResellerConsoleModule('/reseller/products', activeState), true)
})
