import test from 'node:test'
import assert from 'node:assert/strict'
import { createRedemptionAPI, RedemptionError } from '../src/api/redemption.ts'
import { normalizeRedemptionCode, validSession, shouldPoll, needsAccountCheck, statusPresentation, isRedemptionPath, planLabel, subscriptionLabel } from '../src/utils/redemption.ts'

const code = 'K7M2P-R8W4X-6NQ9T-H3V5C-Y2D8F'
test('purchased plans and detected subscriptions remain distinct', () => {
  assert.equal(planLabel('chatgptplusplan'), 'ChatGPT Plus')
  assert.equal(planLabel('chatgptprolite'), 'ChatGPT Pro 5x')
  assert.equal(planLabel('chatgptpro'), 'ChatGPT Pro 20x')
  assert.equal(subscriptionLabel('free'), '免费版')
  assert.equal(subscriptionLabel('plus'), 'ChatGPT Plus')
  assert.equal(subscriptionLabel('pro'), 'ChatGPT Pro')
  for (const plan of ['', 'unknown', 'private-provider-value']) {
    assert.equal(subscriptionLabel(plan), '其他订阅（类型待核实）')
  }
})
test('redeem code normalization matches the backend format', () => {
  for (const input of [code, code.toLowerCase(), code.replaceAll('-', ''), ` \n${code}\t`]) assert.equal(normalizeRedemptionCode(input), code)
  for (const input of ['', code.slice(1), code.replace('7', '0'), code.replace('K', 'I'), code.replace('K', 'ſ'), code.replace('-', ''), 'AAAAA-AAAAA-AAAAA-AAAAA-AAAAA', '22222-22222-22222-22222-22222']) assert.equal(normalizeRedemptionCode(input), null)
})
test('full session validation rejects oversized and incomplete credentials', () => {
  assert.equal(validSession('{"accessToken":"test","sessionToken":"test"}'), true)
  for (const value of ['null', '{}', '{"accessToken":"test"}', '{"accessToken":1,"sessionToken":"test"}', '{"accessToken":" ","sessionToken":"test"}', 'x'.repeat(65537)]) assert.equal(validSession(value), false)
})
test('task states never turn unknown results into success or offer recharging', () => {
  for (const state of ['unknown', 'action_required', 'waiting_card', 'running', 'confirming_subscription']) {
    assert.equal(shouldPoll(state), true)
    assert.equal(needsAccountCheck(state), false)
    assert.notEqual(statusPresentation(state).tone, 'success')
  }
  for (const state of ['failed', 'manual_review', 'completed', 'canceled', 'future_state']) assert.equal(shouldPoll(state), false)
  assert.equal(needsAccountCheck('awaiting_confirmation'), true)
  assert.equal(statusPresentation('completed').tone, 'success')
  assert.equal(statusPresentation('future_state').tone, 'warning')
})
test('sensitive route matching works with base paths and trailing slashes', () => {
  for (const path of ['/redeem', '/redeem/', '/shop/redeem']) assert.equal(isRedemptionPath(path), true)
  for (const path of ['/products/redeemer', '/redeem/history', '/']) assert.equal(isRedemptionPath(path), false)
})
test('API sends credentials only in POST bodies and rejects response extras', async () => {
  const requests: Array<{ url: string; init: RequestInit }> = []
  const api = createRedemptionAPI('https://shop.example.test/', async (url, init) => {
    requests.push({ url: String(url), init: init! })
    return new Response(JSON.stringify({ data: { state: 'awaiting_confirmation', plan: 'chatgptplusplan', session: 'must-not-be-returned', account: { account_id: 'acct-test', current_plan: 'free', accessToken: 'private' } } }))
  })
  const result = await api.account(code, 'authorized-session')
  assert.equal(requests[0]?.url, 'https://shop.example.test/api/v1/recharge/redemptions/account')
  const init = requests[0]!.init
  assert.equal(init.method, 'POST')
  assert.equal(init.credentials, 'omit')
  assert.equal(init.cache, 'no-store')
  assert.equal(init.redirect, 'error')
  assert.equal(init.referrerPolicy, 'no-referrer')
  assert.deepEqual(JSON.parse(String(init.body)), { code, session: 'authorized-session', authorized: true })
  assert.equal(JSON.stringify(result).includes('private'), false)
  assert.equal(JSON.stringify(result).includes('must-not-be-returned'), false)
})
test('confirmation errors do not expose raw provider messages or automatically retry', async () => {
  let calls = 0
  const api = createRedemptionAPI('', async () => { calls++; throw new Error('raw-token-private') })
  await assert.rejects(api.confirm(code, 'proof-token'), (error: unknown) => error instanceof RedemptionError && error.code === 'network' && !error.message.includes('private'))
  assert.equal(calls, 1)
  const failing = createRedemptionAPI('', async () => new Response(JSON.stringify({ code: 'unexpected-private-token', error: 'session-secret' }), { status: 500 }))
  await assert.rejects(failing.lookup(code), (error: unknown) => error instanceof RedemptionError && error.code === 'unavailable' && !error.message.includes('secret'))
})
test('HTTP limits and missing API routes produce safe recoverable errors', async () => {
  const limited = createRedemptionAPI('', async () => new Response('too many', { status: 429 }))
  await assert.rejects(limited.lookup(code), (error: unknown) => error instanceof RedemptionError && error.code === 'rate_limited')
  const absent = createRedemptionAPI('', async () => new Response('<html>not found</html>', { status: 404 }))
  await assert.rejects(absent.lookup(code), (error: unknown) => error instanceof RedemptionError && error.code === 'unavailable')
})
