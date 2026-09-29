<template>
  <div class="mb-6 rounded-2xl border p-4">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div class="min-w-0">
        <h3 class="text-base font-semibold text-foreground">{{ t('personalCenter.security.googleTitle') }}</h3>
        <p class="mt-1 text-xs text-muted-foreground">
          {{ googleEnabled ? t('personalCenter.security.googleSubtitle') : t('personalCenter.security.googleDisabledTip') }}
        </p>
      </div>
      <Badge :variant="googleBound ? 'success' : 'warning'" size="sm">
        {{ googleBound ? t('personalCenter.security.googleBound') : t('personalCenter.security.googleUnbound') }}
      </Badge>
    </div>

    <div v-if="loadingGoogleBinding" class="mt-4 rounded-xl border border-dashed px-4 py-4 text-sm text-muted-foreground">
      {{ t('personalCenter.security.googleLoading') }}
    </div>

    <div v-else-if="googleBound" class="mt-4 space-y-4 rounded-xl border bg-card p-4">
      <div class="flex min-w-0 items-center gap-3">
        <img
          v-if="avatarUrl"
          :src="avatarUrl"
          :alt="t('personalCenter.security.googleAvatarAlt')"
          class="h-11 w-11 shrink-0 rounded-full border object-cover"
        />
        <div v-else class="flex h-11 w-11 shrink-0 items-center justify-center rounded-full border bg-background text-sm font-bold text-foreground">
          G
        </div>
        <div class="min-w-0">
          <p class="truncate text-sm font-semibold text-foreground">{{ googleDisplayName }}</p>
          <p v-if="email" class="break-all text-xs text-muted-foreground">{{ email }}</p>
          <p class="break-all text-xs text-muted-foreground">
            {{ t('personalCenter.security.googleBindID', { id: providerUserId }) }}
          </p>
        </div>
      </div>
      <p class="text-xs text-muted-foreground">
        {{ t('personalCenter.security.googleBindTime', { time: formattedAuthAt }) }}
      </p>
      <Button
        type="button"
        variant="outline"
        size="sm"
        :disabled="unbindingGoogle || !canUnbindGoogle"
        @click="$emit('unbind')"
      >
        {{ unbindingGoogle ? t('personalCenter.security.googleUnbinding') : t('personalCenter.security.googleUnbind') }}
      </Button>
      <p v-if="!canUnbindGoogle" class="text-xs text-muted-foreground">
        {{ t('personalCenter.security.googleUnbindDisabledTip') }}
      </p>
    </div>

    <div v-else class="mt-4 space-y-3">
      <p class="text-xs text-muted-foreground">
        {{
          !googleEnabled
            ? t('personalCenter.security.googleDisabledTip')
            : isTelegramMiniApp
              ? t('personalCenter.security.googleMiniAppUnavailable')
              : t('personalCenter.security.googleUnboundTip')
        }}
      </p>
      <GoogleIdentityButton
        v-if="googleEnabled && !isTelegramMiniApp"
        :client-id="clientId"
        :locale="locale"
        text="continue_with"
        :ux-mode="googleUxMode"
        :login-uri="googleLoginUri"
        :prepare-redirect="prepareRedirect"
        :disabled="bindingGoogle"
        :loading-label="t('personalCenter.security.googleButtonLoading')"
        @credential="$emit('credential', $event)"
        @error="$emit('scriptError')"
      />
      <p v-if="bindingGoogle" class="text-xs text-muted-foreground" role="status">
        {{ t('personalCenter.security.googleBinding') }}
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import GoogleIdentityButton from '../auth/GoogleIdentityButton.vue'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import type { GoogleRedirectPreparedIntent } from '../../utils/googleRedirect'

const { t } = useI18n()

defineProps<{
  googleEnabled: boolean
  googleBound: boolean
  loadingGoogleBinding: boolean
  clientId: string
  locale: string
  avatarUrl: string
  googleDisplayName: string
  email: string
  providerUserId: string
  formattedAuthAt: string
  bindingGoogle: boolean
  unbindingGoogle: boolean
  canUnbindGoogle: boolean
  isTelegramMiniApp: boolean
  googleUxMode: 'popup' | 'redirect'
  googleLoginUri: string
  prepareRedirect: () => Promise<GoogleRedirectPreparedIntent>
}>()

defineEmits<{
  credential: [credential: string]
  scriptError: []
  unbind: []
}>()
</script>
