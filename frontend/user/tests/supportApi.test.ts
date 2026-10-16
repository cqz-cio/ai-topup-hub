import { test } from 'node:test'
import assert from 'node:assert/strict'
import { askSupport, agentError, SupportError } from '../src/components/support/supportApi.ts'

test('support sends only bounded user fields and filters untrusted navigation', async () => {
  const original = globalThis.fetch
  const calls: { url: string; options?: RequestInit }[] = []
  globalThis.fetch = async (url, options) => {
    calls.push({ url: String(url), options })
    return new Response(JSON.stringify(calls.length === 1 ? { ready: true } : {
      text: '查看订单', mode: 'model', links: [{ label: '订单', to: '/me/orders' }, { label: '兑换', to: 'https://untrusted.example' }, { label: '管理', to: '/admin' }],
    }), { status: 200 })
  }
  try {
    const response = await askSupport('我拿到码了', 'zh-CN', crypto.randomUUID(), '', new AbortController().signal)
    assert.equal(calls[0]?.url, '/api/v1/support/session')
    assert.equal(calls[1]?.url, '/api/v1/support/chat')
    const body = JSON.parse(String(calls[1]?.options?.body))
    assert.deepEqual(Object.keys(body).sort(), ['conversation_id', 'locale', 'message', 'product_slug', 'request_id'])
    assert.equal(body.message, '我拿到码了')
    assert.deepEqual(response.links, [{ label: '订单', to: '/me/orders' }])
  } finally { globalThis.fetch = original }
})

test('support never retries failed billable requests', async () => {
  const original = globalThis.fetch; let calls = 0
  globalThis.fetch = async () => new Response(JSON.stringify(++calls === 1 ? { ready: true } : { error: 'model_unavailable' }), { status: calls === 1 ? 200 : 502 })
  try {
    await assert.rejects(askSupport('怎么用', 'zh-CN', crypto.randomUUID(), '', new AbortController().signal), (e: unknown) => e instanceof SupportError && e.code === 'model_unavailable')
    assert.equal(calls, 2)
  } finally { globalThis.fetch = original }
})

test('limit and unknown errors produce useful localized messages without raw errors', () => {
  assert.match(agentError(new SupportError('daily_limit'), 'zh-CN'), /额度/)
  assert.match(agentError(new SupportError('rate_limit'), 'en-US'), /limit/)
  assert.match(agentError(new Error('private detail'), 'zh-TW'), /暫時/)
  assert.doesNotMatch(agentError(new Error('private detail'), 'en-US'), /private detail/)
})
