import test from 'node:test'
import assert from 'node:assert/strict'
import { isPublicAuthEndpoint } from '../src/utils/authEndpoints.ts'
import {
  GOOGLE_IDENTITY_SCRIPT_URL,
  canShowGoogleIdentityButton,
  createGoogleButtonConfiguration,
  createGoogleIdentityConfiguration,
  isIOSOrIPadOS,
  extractGoogleCredential,
  hasThirdPartyLoginOption,
  normalizeGoogleClientID,
  renderGoogleIdentityButton,
  resolveGoogleButtonLocale,
  resolveGoogleButtonWidth,
  resolveGoogleIdentityUXMode,
} from '../src/utils/googleIdentity.ts'

const canonicalRedirectState = 'c3Nzc3Nzc3Nzc3Nzc3Nzc3Nzc3Nzc3Nzc3Nzc3Nzc3M'

test('Google Identity Services uses the official script and popup button contract', () => {
  assert.equal(GOOGLE_IDENTITY_SCRIPT_URL, 'https://accounts.google.com/gsi/client')

  const seenCredentials: string[] = []
  const configuration = createGoogleIdentityConfiguration(
    ' client-id.apps.googleusercontent.com ',
    (credential) => seenCredentials.push(credential),
  )

  assert.equal(configuration.client_id, 'client-id.apps.googleusercontent.com')
  assert.equal(configuration.ux_mode, 'popup')
  assert.equal(configuration.auto_select, false)
  assert.equal(configuration.cancel_on_tap_outside, true)
  assert.equal(configuration.use_fedcm_for_button, true)

  configuration.callback?.({ credential: ' encoded-id-token ', select_by: 'btn' })
  assert.deepEqual(seenCredentials, ['encoded-id-token'])
})

test('GIS redirect mode posts to the backend without exposing a JavaScript credential callback', () => {
  const configuration = createGoogleIdentityConfiguration(
    'client-id',
    () => assert.fail('redirect mode must not invoke a browser credential callback'),
    undefined,
    {
      uxMode: 'redirect',
      loginUri: 'https://shop.example.com/api/v1/auth/google/redirect/callback',
    },
  )

  assert.equal(configuration.ux_mode, 'redirect')
  assert.equal(configuration.login_uri, 'https://shop.example.com/api/v1/auth/google/redirect/callback')
  assert.equal(configuration.callback, undefined)
  assert.equal(configuration.use_fedcm_for_button, undefined)
})

test('iOS and desktop-mode iPadOS use GIS redirect while other platforms keep popup', () => {
  assert.equal(isIOSOrIPadOS({
    userAgent: 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X)',
    platform: 'iPhone',
    maxTouchPoints: 5,
  }), true)
  assert.equal(resolveGoogleIdentityUXMode({
    userAgent: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15) Version/18.0 Safari/605.1.15',
    platform: 'MacIntel',
    maxTouchPoints: 5,
  }), 'redirect')
  assert.equal(resolveGoogleIdentityUXMode({
    userAgent: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 14_5)',
    platform: 'MacIntel',
    maxTouchPoints: 0,
  }), 'popup')
  assert.equal(resolveGoogleIdentityUXMode({
    userAgent: 'Mozilla/5.0 (Linux; Android 15)',
    platform: 'Linux armv8l',
    maxTouchPoints: 5,
  }), 'popup')
})

test('credential extraction treats the ID token as opaque and ignores untrusted claims', () => {
  const response = {
    credential: ' opaque.jwt.value ',
    email: 'attacker-controlled@example.com',
    name: 'Untrusted browser claim',
  }

  assert.equal(extractGoogleCredential(response), 'opaque.jwt.value')
  assert.equal(extractGoogleCredential({ email: 'missing@example.com' } as any), '')
  assert.equal(normalizeGoogleClientID('  client-id  '), 'client-id')
})

test('invalid GIS responses do not call the credential handler', () => {
  let credentials = 0
  let invalidCredentials = 0
  const configuration = createGoogleIdentityConfiguration(
    'client-id',
    () => { credentials += 1 },
    () => { invalidCredentials += 1 },
  )

  configuration.callback?.({ credential: '   ' })

  assert.equal(credentials, 0)
  assert.equal(invalidCredentials, 1)
})

test('official button configuration stays responsive and within the GIS width limit', () => {
  assert.equal(resolveGoogleButtonWidth(327.8), 327)
  assert.equal(resolveGoogleButtonWidth(640), 400)
  assert.equal(resolveGoogleButtonWidth(0), undefined)
  assert.equal(resolveGoogleButtonLocale('zh-CN'), 'zh_CN')
  assert.equal(resolveGoogleButtonLocale('zh-TW'), 'zh_TW')
  assert.equal(resolveGoogleButtonLocale('en-US'), 'en_US')
  assert.equal(resolveGoogleButtonLocale('fr'), 'fr')

  assert.deepEqual(
    createGoogleButtonConfiguration({
      locale: 'zh-CN',
      shape: 'pill',
      text: 'continue_with',
      width: 640,
    }),
    {
      type: 'standard',
      theme: 'outline',
      size: 'large',
      text: 'continue_with',
      shape: 'pill',
      logo_alignment: 'left',
      width: '400',
      locale: 'zh_CN',
    },
  )

  assert.deepEqual(
    createGoogleButtonConfiguration({
      state: canonicalRedirectState,
    }),
    {
      type: 'standard',
      theme: 'outline',
      size: 'large',
      text: 'signin_with',
      shape: 'rectangular',
      logo_alignment: 'left',
      state: canonicalRedirectState,
    },
  )
})

test('resizing rerenders the official button without reinitializing GIS', () => {
  let initializes = 0
  let renders = 0
  let replacements = 0
  const accountsID = {
    initialize: () => { initializes += 1 },
    renderButton: () => { renders += 1 },
  }
  const container = {
    isConnected: true,
    replaceChildren: () => { replacements += 1 },
  } as unknown as HTMLElement

  renderGoogleIdentityButton({
    accountsID,
    container,
    width: 320,
  })
  renderGoogleIdentityButton({
    accountsID,
    container,
    width: 360,
  })

  assert.equal(initializes, 0)
  assert.equal(renders, 2)
  assert.equal(replacements, 2)
})

test('Google login is classified as a public auth endpoint', () => {
  assert.equal(isPublicAuthEndpoint('/auth/google/login'), true)
  assert.equal(isPublicAuthEndpoint('/auth/google/redirect/intent'), true)
  assert.equal(isPublicAuthEndpoint('/auth/google/redirect/exchange'), true)
  assert.equal(isPublicAuthEndpoint('/auth/google/login?source=button'), true)
  assert.equal(isPublicAuthEndpoint('/auth/login/verify-2fa'), true)
  assert.equal(isPublicAuthEndpoint('/auth/send-verify-code'), true)
  assert.equal(isPublicAuthEndpoint('/me/google'), false)
  assert.equal(isPublicAuthEndpoint('/me/google/bind'), false)
  assert.equal(isPublicAuthEndpoint('/me/google/redirect/exchange'), false)
})

test('Google stays hidden in Telegram WebView while providers share one third-party section', () => {
  assert.equal(canShowGoogleIdentityButton(true, 'client-id', false), true)
  assert.equal(canShowGoogleIdentityButton(true, 'client-id', true), false)
  assert.equal(canShowGoogleIdentityButton(true, '   ', false), false)
  assert.equal(hasThirdPartyLoginOption(true, true), true)
  assert.equal(hasThirdPartyLoginOption(false, false), false)
})
