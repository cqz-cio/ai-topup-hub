import type {
  GoogleAccountsID,
  GoogleCredentialResponse,
  GoogleIdentityButtonConfiguration,
  GoogleIdentityConfiguration,
} from '../types/google-identity'
import { normalizeGoogleRedirectState } from './googleRedirect.ts'

export const GOOGLE_IDENTITY_SCRIPT_URL = 'https://accounts.google.com/gsi/client'
const GOOGLE_IDENTITY_SCRIPT_ID = 'google-identity-services'
const GOOGLE_IDENTITY_SCRIPT_TIMEOUT_MS = 10000
const GOOGLE_IDENTITY_BUTTON_MAX_WIDTH = 400

let googleIdentityScriptPromise: Promise<GoogleAccountsID> | null = null

export const normalizeGoogleClientID = (value: unknown): string => String(value || '').trim()

export interface GoogleIdentityNavigator {
  userAgent?: unknown
  platform?: unknown
  maxTouchPoints?: unknown
}

export const isIOSOrIPadOS = (source: GoogleIdentityNavigator): boolean => {
  const userAgent = String(source.userAgent || '')
  const platform = String(source.platform || '')
  const maxTouchPoints = Number(source.maxTouchPoints || 0)
  return /iPad|iPhone|iPod/i.test(userAgent)
    || ((/Macintosh/i.test(userAgent) || /Mac/i.test(platform)) && maxTouchPoints > 1)
}

export const resolveGoogleIdentityUXMode = (
  source: GoogleIdentityNavigator,
): 'popup' | 'redirect' => isIOSOrIPadOS(source) ? 'redirect' : 'popup'

export const detectGoogleIdentityUXMode = (): 'popup' | 'redirect' => {
  if (typeof navigator === 'undefined') return 'popup'
  return resolveGoogleIdentityUXMode(navigator)
}

export const canShowGoogleIdentityButton = (
  enabled: unknown,
  clientID: unknown,
  isTelegramWebView: unknown,
): boolean => Boolean(enabled) && normalizeGoogleClientID(clientID) !== '' && !Boolean(isTelegramWebView)

export const hasThirdPartyLoginOption = (...options: unknown[]): boolean => options.some(Boolean)

/**
 * GIS returns an encoded ID token in `credential`. The browser treats it as an
 * opaque value; only the backend is allowed to verify or trust its claims.
 */
export const extractGoogleCredential = (response: GoogleCredentialResponse | null | undefined): string =>
  String(response?.credential || '').trim()

export const resolveGoogleButtonWidth = (availableWidth: unknown): number | undefined => {
  const width = Number(availableWidth)
  if (!Number.isFinite(width) || width <= 0) return undefined
  return Math.min(GOOGLE_IDENTITY_BUTTON_MAX_WIDTH, Math.floor(width))
}

const GOOGLE_BUTTON_LOCALE_MAP: Record<string, string> = {
  'zh-CN': 'zh_CN',
  'zh-TW': 'zh_TW',
  'en-US': 'en_US',
}

export const resolveGoogleButtonLocale = (locale: unknown): string => {
  const normalized = String(locale || '').trim()
  return GOOGLE_BUTTON_LOCALE_MAP[normalized] || normalized
}

export const createGoogleIdentityConfiguration = (
  clientID: string,
  onCredential: (credential: string) => void,
  onInvalidCredential?: () => void,
  options: {
    uxMode?: 'popup' | 'redirect'
    loginUri?: string
  } = {},
): GoogleIdentityConfiguration => {
  const uxMode = options.uxMode || 'popup'
  if (uxMode === 'redirect') {
    const loginURI = String(options.loginUri || '').trim()
    if (loginURI === '') {
      throw new Error('Google redirect login URI is missing')
    }
    return {
      client_id: normalizeGoogleClientID(clientID),
      ux_mode: 'redirect',
      login_uri: loginURI,
      auto_select: false,
    }
  }

  return {
    client_id: normalizeGoogleClientID(clientID),
    callback: (response) => {
      const credential = extractGoogleCredential(response)
      if (credential === '') {
        onInvalidCredential?.()
        return
      }
      onCredential(credential)
    },
    ux_mode: 'popup',
    auto_select: false,
    cancel_on_tap_outside: true,
    use_fedcm_for_button: true,
  }
}

export const createGoogleButtonConfiguration = (options: {
  locale?: string
  shape?: 'rectangular' | 'pill'
  text?: 'signin_with' | 'continue_with'
  width?: number
  state?: string
} = {}): GoogleIdentityButtonConfiguration => {
  const width = resolveGoogleButtonWidth(options.width)
  const locale = resolveGoogleButtonLocale(options.locale)
  const state = normalizeGoogleRedirectState(options.state)
  return {
    type: 'standard',
    theme: 'outline',
    size: 'large',
    text: options.text || 'signin_with',
    shape: options.shape || 'rectangular',
    logo_alignment: 'left',
    ...(width ? { width: String(width) } : {}),
    ...(locale ? { locale } : {}),
    ...(state ? { state } : {}),
  }
}

const getGoogleAccountsID = (): GoogleAccountsID | null => {
  if (typeof window === 'undefined') return null
  return window.google?.accounts?.id || null
}

const waitForGoogleIdentityScript = (script: HTMLScriptElement): Promise<GoogleAccountsID> =>
  new Promise((resolve, reject) => {
    let settled = false

    const settle = (callback: () => void) => {
      if (settled) return
      settled = true
      window.clearTimeout(timer)
      script.removeEventListener('load', handleLoad)
      script.removeEventListener('error', handleError)
      callback()
    }

    const handleLoad = () => {
      const accountsID = getGoogleAccountsID()
      if (!accountsID) {
        settle(() => {
          script.remove()
          reject(new Error('Google Identity Services is unavailable'))
        })
        return
      }
      settle(() => resolve(accountsID))
    }

    const handleError = () => {
      settle(() => {
        script.remove()
        reject(new Error('Failed to load Google Identity Services'))
      })
    }

    const timer = window.setTimeout(() => {
      settle(() => {
        script.remove()
        reject(new Error('Google Identity Services load timed out'))
      })
    }, GOOGLE_IDENTITY_SCRIPT_TIMEOUT_MS)

    script.addEventListener('load', handleLoad)
    script.addEventListener('error', handleError)
  })

export const loadGoogleIdentityScript = (): Promise<GoogleAccountsID> => {
  const existingAccountsID = getGoogleAccountsID()
  if (existingAccountsID) return Promise.resolve(existingAccountsID)
  if (googleIdentityScriptPromise) return googleIdentityScriptPromise
  if (typeof window === 'undefined' || typeof document === 'undefined') {
    return Promise.reject(new Error('Google Identity Services requires a browser'))
  }

  let script = document.getElementById(GOOGLE_IDENTITY_SCRIPT_ID) as HTMLScriptElement | null
  let shouldAppendScript = false
  if (!script) {
    script = document.createElement('script')
    script.id = GOOGLE_IDENTITY_SCRIPT_ID
    script.src = GOOGLE_IDENTITY_SCRIPT_URL
    script.async = true
    script.defer = true
    shouldAppendScript = true
  }

  googleIdentityScriptPromise = waitForGoogleIdentityScript(script).catch((error) => {
    googleIdentityScriptPromise = null
    throw error
  })
  if (shouldAppendScript) {
    document.head.appendChild(script)
  }
  return googleIdentityScriptPromise
}

/**
 * GIS stores its initialization configuration globally. Call this once for
 * each active component/client lifecycle; later size/locale changes must only
 * rerender the button so they cannot replace the credential callback.
 */
export const initializeGoogleIdentity = async (options: {
  clientID: string
  onCredential: (credential: string) => void
  onInvalidCredential?: () => void
  uxMode?: 'popup' | 'redirect'
  loginUri?: string
}): Promise<GoogleAccountsID> => {
  const clientID = normalizeGoogleClientID(options.clientID)
  if (clientID === '') {
    throw new Error('Google client ID is missing')
  }

  const accountsID = await loadGoogleIdentityScript()
  accountsID.initialize(createGoogleIdentityConfiguration(
    clientID,
    options.onCredential,
    options.onInvalidCredential,
    {
      uxMode: options.uxMode,
      loginUri: options.loginUri,
    },
  ))
  return accountsID
}

/** Render (or resize) the official button without reinitializing GIS. */
export const renderGoogleIdentityButton = (options: {
  accountsID: GoogleAccountsID
  container: HTMLElement
  locale?: string
  shape?: 'rectangular' | 'pill'
  text?: 'signin_with' | 'continue_with'
  width?: number
  state?: string
}): void => {
  if (!options.container.isConnected) {
    throw new Error('Google sign-in button container is unavailable')
  }
  options.container.replaceChildren()
  options.accountsID.renderButton(options.container, createGoogleButtonConfiguration({
    locale: options.locale,
    shape: options.shape,
    text: options.text,
    width: options.width,
    state: options.state,
  }))
}
