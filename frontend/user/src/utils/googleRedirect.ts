export const GOOGLE_REDIRECT_API_PATHS = {
  credentialCallback: '/auth/google/redirect/callback',
  loginIntent: '/auth/google/redirect/intent',
  loginExchange: '/auth/google/redirect/exchange',
  bindIntent: '/me/google/redirect/intent',
  bindExchange: '/me/google/redirect/exchange',
} as const

export const GOOGLE_REDIRECT_FRONTEND_CALLBACK_PATH = '/auth/google/callback'
export const GOOGLE_REDIRECT_INTENT_STORAGE_KEY = 'google_redirect_intent'
export const GOOGLE_REDIRECT_INTENT_TTL_MS = 10 * 60 * 1000
export const GOOGLE_REDIRECT_INTENT_REFRESH_MS = 8 * 60 * 1000

export type GoogleRedirectFlow = 'login' | 'bind'

export const GOOGLE_REDIRECT_ERRORS = [
  'invalid_request',
  'csrf_mismatch',
  'session_expired',
  'tenant_mismatch',
  'auth_disabled',
  'configuration_error',
  'credential_invalid',
  'credential_expired',
  'email_unverified',
  'service_unavailable',
  'internal_error',
] as const

export type GoogleRedirectError = (typeof GOOGLE_REDIRECT_ERRORS)[number]

export interface GoogleRedirectCallback {
  flow: GoogleRedirectFlow
  error: GoogleRedirectError | null
}

export interface GoogleRedirectIntent {
  flow: GoogleRedirectFlow
  returnPath: string
  issuedAt: number
}

export interface GoogleRedirectPreparedIntent {
  state: string
  issuedAt: number
}

interface StorageLike {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
  removeItem(key: string): void
}

const GOOGLE_REDIRECT_FLOWS = new Set<GoogleRedirectFlow>(['login', 'bind'])
const GOOGLE_REDIRECT_ERROR_SET = new Set<string>(GOOGLE_REDIRECT_ERRORS)
// 32 random bytes encoded as canonical, unpadded base64url. The final
// character is restricted to indices whose two padding bits are zero.
const GOOGLE_REDIRECT_STATE_PATTERN = /^[A-Za-z0-9_-]{42}[AEIMQUYcgkosw048]$/
const CALLBACK_QUERY_KEYS = new Set(['flow', 'error'])
const DEFAULT_LOGIN_RETURN_PATH = '/me/orders'
const BIND_RETURN_PATH = '/me/security'

export const normalizeGoogleRedirectReturnPath = (
  value: unknown,
  fallback = DEFAULT_LOGIN_RETURN_PATH,
): string => {
  const path = typeof value === 'string' ? value.trim() : ''
  if (!path.startsWith('/') || path.startsWith('//') || path.includes('\\')) {
    return fallback
  }
  return path
}

export const createGoogleRedirectIntent = (
  flow: GoogleRedirectFlow,
  returnPath: unknown,
  issuedAt = Date.now(),
): GoogleRedirectIntent => ({
  flow,
  returnPath: flow === 'bind'
    ? BIND_RETURN_PATH
    : normalizeGoogleRedirectReturnPath(returnPath),
  issuedAt,
})

export const storeGoogleRedirectIntent = (
  storage: StorageLike | null | undefined,
  intent: GoogleRedirectIntent,
): boolean => {
  if (!storage) return false
  try {
    storage.setItem(GOOGLE_REDIRECT_INTENT_STORAGE_KEY, JSON.stringify(intent))
    return true
  } catch {
    return false
  }
}

/**
 * Read-once by design: even malformed or expired data is removed before it is
 * interpreted so a callback cannot retry a stale client intent.
 */
export const consumeGoogleRedirectIntent = (
  storage: StorageLike | null | undefined,
  now = Date.now(),
): GoogleRedirectIntent | null => {
  if (!storage) return null

  let raw = ''
  try {
    raw = storage.getItem(GOOGLE_REDIRECT_INTENT_STORAGE_KEY) || ''
    storage.removeItem(GOOGLE_REDIRECT_INTENT_STORAGE_KEY)
  } catch {
    return null
  }
  if (raw === '') return null

  try {
    const parsed = JSON.parse(raw) as Partial<GoogleRedirectIntent>
    if (!GOOGLE_REDIRECT_FLOWS.has(parsed.flow as GoogleRedirectFlow)) return null
    if (!Number.isFinite(parsed.issuedAt)) return null
    const age = now - Number(parsed.issuedAt)
    if (age < 0 || age >= GOOGLE_REDIRECT_INTENT_TTL_MS) return null

    const flow = parsed.flow as GoogleRedirectFlow
    return createGoogleRedirectIntent(flow, parsed.returnPath, Number(parsed.issuedAt))
  } catch {
    return null
  }
}

export const resolveGoogleRedirectIntentRefreshDelay = (
  issuedAt: number,
  now = Date.now(),
): number => {
  if (!Number.isFinite(issuedAt) || !Number.isFinite(now)) return 0
  const age = Math.max(0, now - issuedAt)
  return Math.max(0, GOOGLE_REDIRECT_INTENT_REFRESH_MS - age)
}

export const createGoogleRedirectIntentRefreshScheduler = (
  setTimer: (callback: () => void, delay: number) => unknown,
  clearTimer: (handle: unknown) => void,
  now: () => number = Date.now,
) => {
  let timerHandle: unknown | null = null

  const clear = () => {
    if (timerHandle === null) return
    clearTimer(timerHandle)
    timerHandle = null
  }

  const schedule = (issuedAt: number, callback: () => void) => {
    clear()
    timerHandle = setTimer(() => {
      timerHandle = null
      callback()
    }, resolveGoogleRedirectIntentRefreshDelay(issuedAt, now()))
  }

  return { schedule, clear }
}

export const normalizeGoogleRedirectState = (value: unknown): string => {
  const state = typeof value === 'string' ? value.trim() : ''
  return GOOGLE_REDIRECT_STATE_PATTERN.test(state) ? state : ''
}

export const createGoogleRedirectPreparedIntent = (
  responseData: unknown,
  issuedAt = Date.now(),
): GoogleRedirectPreparedIntent | null => {
  if (!responseData || typeof responseData !== 'object' || !Number.isFinite(issuedAt)) {
    return null
  }
  const state = normalizeGoogleRedirectState((responseData as { state?: unknown }).state)
  if (state === '') return null
  return { state, issuedAt }
}

/**
 * Async intent refreshes are versioned by the button component. A response
 * from an older request is discarded so it cannot overwrite a newer state.
 */
export const acceptGoogleRedirectPreparedIntent = (
  intent: GoogleRedirectPreparedIntent | null,
  requestVersion: number,
  activeVersion: number,
): GoogleRedirectPreparedIntent | null =>
  intent && requestVersion === activeVersion ? intent : null

/**
 * The backend callback contract deliberately permits only two fixed keys.
 * Any credential, state, token, unknown flow, or unknown error in the URL makes
 * the callback invalid and prevents an exchange request.
 */
export const parseGoogleRedirectCallbackQuery = (
  query: Record<string, unknown>,
): GoogleRedirectCallback | null => {
  if (Object.keys(query).some((key) => !CALLBACK_QUERY_KEYS.has(key))) return null

  const flow = query.flow
  if (typeof flow !== 'string' || !GOOGLE_REDIRECT_FLOWS.has(flow as GoogleRedirectFlow)) {
    return null
  }

  const rawError = query.error
  if (rawError === undefined || rawError === null) {
    return { flow: flow as GoogleRedirectFlow, error: null }
  }
  if (typeof rawError !== 'string' || !GOOGLE_REDIRECT_ERROR_SET.has(rawError)) {
    return null
  }
  return {
    flow: flow as GoogleRedirectFlow,
    error: rawError as GoogleRedirectError,
  }
}

export const shouldResumeGoogleRedirect2FA = (
  marker: unknown,
  challengeToken: unknown,
): boolean => marker === '1' && typeof challengeToken === 'string' && challengeToken.trim() !== ''

const runtimeAPIBaseURL = (): string => {
  const meta = import.meta as ImportMeta & { env?: Record<string, string | undefined> }
  return String(meta.env?.VITE_API_BASE_URL || '').trim()
}

export const buildGoogleRedirectCredentialCallbackURL = (
  apiBaseURL: unknown = runtimeAPIBaseURL(),
  pageOrigin: unknown = typeof window === 'undefined' ? '' : window.location.origin,
): string => {
  const origin = String(pageOrigin || '').trim()
  if (origin === '') {
    throw new Error('Page origin is required for Google redirect mode')
  }
  const pageURL = new URL(origin)
  const configuredBase = String(apiBaseURL || '').trim()
  let basePath = ''
  if (configuredBase !== '') {
    const configuredURL = new URL(configuredBase, pageURL)
    if (configuredURL.origin !== pageURL.origin) {
      throw new Error('Google redirect mode requires a same-origin API')
    }
    if (configuredURL.search !== '' || configuredURL.hash !== '') {
      throw new Error('Google redirect API base must not contain a query or fragment')
    }
    basePath = configuredURL.pathname.replace(/\/+$/, '')
  }
  return new URL(
    `${basePath}/api/v1${GOOGLE_REDIRECT_API_PATHS.credentialCallback}`,
    pageURL,
  ).toString()
}

export const tryBuildGoogleRedirectCredentialCallbackURL = (): string => {
  try {
    return buildGoogleRedirectCredentialCallbackURL()
  } catch {
    return ''
  }
}

export const getGoogleRedirectSessionStorage = (): Storage | null => {
  if (typeof window === 'undefined') return null
  try {
    return window.sessionStorage
  } catch {
    return null
  }
}
