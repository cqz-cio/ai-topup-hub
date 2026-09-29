import test from 'node:test'
import assert from 'node:assert/strict'
import {
  GOOGLE_REDIRECT_API_PATHS,
  GOOGLE_REDIRECT_INTENT_REFRESH_MS,
  GOOGLE_REDIRECT_INTENT_STORAGE_KEY,
  GOOGLE_REDIRECT_INTENT_TTL_MS,
  acceptGoogleRedirectPreparedIntent,
  buildGoogleRedirectCredentialCallbackURL,
  consumeGoogleRedirectIntent,
  createGoogleRedirectIntentRefreshScheduler,
  createGoogleRedirectIntent,
  createGoogleRedirectPreparedIntent,
  normalizeGoogleRedirectState,
  normalizeGoogleRedirectReturnPath,
  parseGoogleRedirectCallbackQuery,
  resolveGoogleRedirectIntentRefreshDelay,
  shouldResumeGoogleRedirect2FA,
  storeGoogleRedirectIntent,
} from '../src/utils/googleRedirect.ts'

const canonicalRedirectState = 'c3Nzc3Nzc3Nzc3Nzc3Nzc3Nzc3Nzc3Nzc3Nzc3Nzc3M'

const createMemoryStorage = () => {
  const values = new Map<string, string>()
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => { values.set(key, value) },
    removeItem: (key: string) => { values.delete(key) },
    has: (key: string) => values.has(key),
  }
}

test('Google redirect backend paths stay centralized and login_uri is same-origin only', () => {
  assert.deepEqual(GOOGLE_REDIRECT_API_PATHS, {
    credentialCallback: '/auth/google/redirect/callback',
    loginIntent: '/auth/google/redirect/intent',
    loginExchange: '/auth/google/redirect/exchange',
    bindIntent: '/me/google/redirect/intent',
    bindExchange: '/me/google/redirect/exchange',
  })
  assert.equal(
    buildGoogleRedirectCredentialCallbackURL('', 'https://shop.example.com'),
    'https://shop.example.com/api/v1/auth/google/redirect/callback',
  )
  assert.equal(
    buildGoogleRedirectCredentialCallbackURL('/backend', 'https://shop.example.com'),
    'https://shop.example.com/backend/api/v1/auth/google/redirect/callback',
  )
  assert.equal(
    buildGoogleRedirectCredentialCallbackURL(
      'https://shop.example.com/backend/',
      'https://shop.example.com',
    ),
    'https://shop.example.com/backend/api/v1/auth/google/redirect/callback',
  )
  assert.throws(
    () => buildGoogleRedirectCredentialCallbackURL(
      'https://api.example.com',
      'https://shop.example.com',
    ),
    /same-origin API/,
  )
})

test('callback query accepts only fixed flow/error markers and rejects credential or state', () => {
  assert.deepEqual(parseGoogleRedirectCallbackQuery({ flow: 'login' }), {
    flow: 'login',
    error: null,
  })
  assert.deepEqual(parseGoogleRedirectCallbackQuery({
    flow: 'bind',
    error: 'credential_expired',
  }), {
    flow: 'bind',
    error: 'credential_expired',
  })
  assert.equal(parseGoogleRedirectCallbackQuery({ flow: 'other' }), null)
  assert.equal(parseGoogleRedirectCallbackQuery({ flow: 'login', error: 'unknown' }), null)
  assert.equal(parseGoogleRedirectCallbackQuery({
    flow: 'login',
    credential: 'must-never-be-read',
  }), null)
  assert.equal(parseGoogleRedirectCallbackQuery({
    flow: 'login',
    state: 'must-never-be-read',
  }), null)
})

test('intent state round-trips into the active button version and stale responses are discarded', () => {
  const prepared = createGoogleRedirectPreparedIntent({
    state: canonicalRedirectState,
    expires_in: 600,
  }, 10_000)
  assert.deepEqual(prepared, {
    state: canonicalRedirectState,
    issuedAt: 10_000,
  })
  assert.equal(createGoogleRedirectPreparedIntent({ state: 'short' }, 10_000), null)
  assert.equal(
    normalizeGoogleRedirectState('c3Nzc3Nzc3Nzc3Nzc3Nzc3Nzc3Nzc3Nzc3Nzc3Nzc3N'),
    '',
  )
  assert.deepEqual(acceptGoogleRedirectPreparedIntent(prepared, 3, 3), prepared)
  assert.equal(acceptGoogleRedirectPreparedIntent(prepared, 2, 3), null)
})

test('client intent is read once, expires at ten minutes, and refreshes at eight minutes', () => {
  const storage = createMemoryStorage()
  const issuedAt = 1_000_000
  const intent = createGoogleRedirectIntent('login', '/checkout?step=payment', issuedAt)
  assert.equal(storeGoogleRedirectIntent(storage, intent), true)
  assert.equal(storage.has(GOOGLE_REDIRECT_INTENT_STORAGE_KEY), true)
  assert.deepEqual(
    consumeGoogleRedirectIntent(storage, issuedAt + GOOGLE_REDIRECT_INTENT_TTL_MS - 1),
    intent,
  )
  assert.equal(storage.has(GOOGLE_REDIRECT_INTENT_STORAGE_KEY), false)
  assert.equal(consumeGoogleRedirectIntent(storage, issuedAt + 1), null)

  storeGoogleRedirectIntent(storage, intent)
  assert.equal(
    consumeGoogleRedirectIntent(storage, issuedAt + GOOGLE_REDIRECT_INTENT_TTL_MS),
    null,
  )
  assert.equal(
    resolveGoogleRedirectIntentRefreshDelay(issuedAt, issuedAt),
    GOOGLE_REDIRECT_INTENT_REFRESH_MS,
  )
  assert.equal(
    resolveGoogleRedirectIntentRefreshDelay(
      issuedAt,
      issuedAt + GOOGLE_REDIRECT_INTENT_REFRESH_MS,
    ),
    0,
  )
})

test('intent refresh scheduler replaces stale timers and clears on lifecycle teardown', () => {
  const callbacks = new Map<number, () => void>()
  const delays = new Map<number, number>()
  const cleared: number[] = []
  let nextHandle = 1
  const scheduler = createGoogleRedirectIntentRefreshScheduler(
    (callback, delay) => {
      const handle = nextHandle++
      callbacks.set(handle, callback)
      delays.set(handle, delay)
      return handle
    },
    (handle) => {
      cleared.push(handle as number)
      callbacks.delete(handle as number)
    },
    () => 1_000,
  )

  scheduler.schedule(1_000, () => assert.fail('stale timer must be replaced'))
  assert.equal(delays.get(1), GOOGLE_REDIRECT_INTENT_REFRESH_MS)

  let refreshed = 0
  scheduler.schedule(1_000, () => { refreshed += 1 })
  assert.deepEqual(cleared, [1])
  callbacks.get(2)?.()
  assert.equal(refreshed, 1)

  scheduler.schedule(1_000, () => { refreshed += 1 })
  scheduler.clear()
  assert.deepEqual(cleared, [1, 3])
  assert.equal(callbacks.has(3), false)
})

test('return paths remain internal and Google redirect 2FA resumes only with a live challenge', () => {
  assert.equal(normalizeGoogleRedirectReturnPath('/me/orders?tab=paid'), '/me/orders?tab=paid')
  assert.equal(normalizeGoogleRedirectReturnPath('//evil.example'), '/me/orders')
  assert.equal(normalizeGoogleRedirectReturnPath('/\\evil.example'), '/me/orders')
  assert.equal(normalizeGoogleRedirectReturnPath('https://evil.example'), '/me/orders')

  assert.equal(shouldResumeGoogleRedirect2FA('1', 'challenge-token'), true)
  assert.equal(shouldResumeGoogleRedirect2FA('1', ''), false)
  assert.equal(shouldResumeGoogleRedirect2FA('0', 'challenge-token'), false)
})

test('session storage failure does not block server-authorized redirect login', () => {
  const throwingStorage = {
    getItem: () => { throw new Error('blocked') },
    setItem: () => { throw new Error('blocked') },
    removeItem: () => { throw new Error('blocked') },
  }
  const intent = createGoogleRedirectIntent('login', '/checkout', 1_000)
  assert.equal(storeGoogleRedirectIntent(throwingStorage, intent), false)
  assert.equal(consumeGoogleRedirectIntent(throwingStorage, 1_001), null)
})
