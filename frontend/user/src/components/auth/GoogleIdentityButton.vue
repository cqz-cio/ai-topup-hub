<template>
  <div
    class="relative flex min-h-11 w-full justify-center overflow-hidden"
    :class="{ 'pointer-events-none opacity-60': disabled }"
    :aria-disabled="disabled"
    :inert="disabled"
  >
    <div ref="buttonContainerRef" class="flex min-h-11 w-full justify-center"></div>
    <div
      v-if="loading"
      class="absolute inset-0 flex items-center justify-center border bg-background text-muted-foreground"
      :class="shape === 'pill' ? 'rounded-full' : 'rounded-md'"
      role="status"
    >
      <span class="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent motion-reduce:animate-none"></span>
      <span class="sr-only">{{ loadingLabel }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  initializeGoogleIdentity,
  renderGoogleIdentityButton,
  resolveGoogleButtonWidth,
} from '../../utils/googleIdentity'
import {
  acceptGoogleRedirectPreparedIntent,
  createGoogleRedirectIntentRefreshScheduler,
  type GoogleRedirectPreparedIntent,
} from '../../utils/googleRedirect'
import type { GoogleAccountsID } from '../../types/google-identity'

const props = withDefaults(defineProps<{
  clientId: string
  locale?: string
  shape?: 'rectangular' | 'pill'
  text?: 'signin_with' | 'continue_with'
  disabled?: boolean
  loadingLabel?: string
  uxMode?: 'popup' | 'redirect'
  loginUri?: string
  prepareRedirect?: () => Promise<GoogleRedirectPreparedIntent>
}>(), {
  locale: '',
  shape: 'rectangular',
  text: 'signin_with',
  disabled: false,
  loadingLabel: 'Loading Google sign-in',
  uxMode: 'popup',
  loginUri: '',
})

const emit = defineEmits<{
  credential: [credential: string]
  error: [error: Error]
}>()

const buttonContainerRef = ref<HTMLDivElement | null>(null)
const loading = ref(true)
let accountsID: GoogleAccountsID | null = null
let resizeObserver: ResizeObserver | null = null
let initializeVersion = 0
let lastRenderedWidth = 0
let active = false
let redirectState = ''
let redirectPreparationPromise: Promise<GoogleRedirectPreparedIntent> | null = null

const redirectIntentScheduler = createGoogleRedirectIntentRefreshScheduler(
  (callback, delay) => window.setTimeout(callback, delay),
  (handle) => window.clearTimeout(handle as number),
)

const scheduleRedirectIntentRefresh = (version: number, issuedAt: number) => {
  redirectIntentScheduler.clear()
  if (props.uxMode !== 'redirect') return
  redirectIntentScheduler.schedule(issuedAt, () => {
    if (!active || version !== initializeVersion) return
    void initializeButton()
  })
}

const prepareRedirectIntent = (): Promise<GoogleRedirectPreparedIntent> => {
  if (redirectPreparationPromise) return redirectPreparationPromise
  if (!props.prepareRedirect) {
    return Promise.reject(new Error('Google redirect intent preparation is unavailable'))
  }
  redirectPreparationPromise = props.prepareRedirect().finally(() => {
    redirectPreparationPromise = null
  })
  return redirectPreparationPromise
}

const renderButton = () => {
  const container = buttonContainerRef.value
  if (!container || !accountsID || !active) return

  const width = resolveGoogleButtonWidth(container.getBoundingClientRect().width)
  try {
    renderGoogleIdentityButton({
      accountsID,
      container,
      locale: props.locale,
      shape: props.shape,
      text: props.text,
      width,
      state: redirectState,
    })
    lastRenderedWidth = width || 0
  } catch (error) {
    emit('error', error instanceof Error ? error : new Error(String(error)))
  }
}

const scheduleRender = () => {
  void nextTick(renderButton)
}

const initializeButton = async () => {
  const clientID = props.clientId.trim()
  const version = ++initializeVersion
  redirectIntentScheduler.clear()
  accountsID = null
  redirectState = ''
  lastRenderedWidth = 0
  buttonContainerRef.value?.replaceChildren()
  if (clientID === '') {
    loading.value = false
    return
  }

  loading.value = true
  try {
    let redirectIntentIssuedAt = 0
    if (props.uxMode === 'redirect') {
      const preparedIntent = acceptGoogleRedirectPreparedIntent(
        await prepareRedirectIntent(),
        version,
        initializeVersion,
      )
      if (!active || !preparedIntent) return
      redirectState = preparedIntent.state
      redirectIntentIssuedAt = preparedIntent.issuedAt
    }

    const initializedAccountsID = await initializeGoogleIdentity({
      clientID,
      onCredential: (credential) => {
        if (active && version === initializeVersion && !props.disabled) {
          emit('credential', credential)
        }
      },
      onInvalidCredential: () => {
        if (active && version === initializeVersion) {
          emit('error', new Error('Google credential is missing'))
        }
      },
      uxMode: props.uxMode,
      loginUri: props.loginUri,
    })
    if (!active || version !== initializeVersion) return
    accountsID = initializedAccountsID
    scheduleRender()
    if (redirectIntentIssuedAt > 0) {
      scheduleRedirectIntentRefresh(version, redirectIntentIssuedAt)
    }
  } catch (error) {
    if (!active || version !== initializeVersion) return
    emit('error', error instanceof Error ? error : new Error(String(error)))
  } finally {
    if (active && version === initializeVersion) {
      loading.value = false
    }
  }
}

onMounted(() => {
  active = true
  void initializeButton()
  if (typeof ResizeObserver !== 'undefined' && buttonContainerRef.value) {
    resizeObserver = new ResizeObserver((entries) => {
      const width = resolveGoogleButtonWidth(entries[0]?.contentRect.width)
      if (!width || Math.abs(width - lastRenderedWidth) < 8) return
      scheduleRender()
    })
    resizeObserver.observe(buttonContainerRef.value)
  }
})

watch(
  () => [props.clientId, props.uxMode, props.loginUri],
  () => { void initializeButton() },
)

watch(
  () => [props.locale, props.shape, props.text],
  scheduleRender,
)

onBeforeUnmount(() => {
  active = false
  initializeVersion += 1
  resizeObserver?.disconnect()
  resizeObserver = null
  redirectIntentScheduler.clear()
  accountsID?.cancel?.()
  accountsID = null
  buttonContainerRef.value?.replaceChildren()
})
</script>
