import { GOOGLE_REDIRECT_API_PATHS } from './googleRedirect.ts'

const PUBLIC_AUTH_ENDPOINTS = new Set([
  '/auth/login',
  '/auth/login/verify-2fa',
  '/auth/register',
  '/auth/send-verify-code',
  '/auth/telegram/login',
  '/auth/telegram/miniapp/login',
  '/auth/telegram/oidc/start',
  '/auth/telegram/oidc/callback',
  '/auth/google/login',
  GOOGLE_REDIRECT_API_PATHS.loginIntent,
  GOOGLE_REDIRECT_API_PATHS.loginExchange,
  '/auth/forgot-password',
])

export const isPublicAuthEndpoint = (url: unknown): boolean => {
  const [path = ''] = String(url || '').split(/[?#]/, 1)
  return PUBLIC_AUTH_ENDPOINTS.has(path)
}
