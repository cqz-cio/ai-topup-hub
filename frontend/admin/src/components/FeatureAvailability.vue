<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import type { RuntimeFeature, useRuntimeFeatures } from '@/composables/useRuntimeFeatures'

defineProps<{ state: ReturnType<typeof useRuntimeFeatures>; feature: RuntimeFeature }>()
const { t } = useI18n()
</script>

<template>
  <div v-if="!state.available(feature)" role="status" class="rounded-lg border border-border bg-muted/30 p-4 text-sm leading-relaxed">
    <p>{{ t(state.loading ? 'featureAvailability.loading' : state.error ? 'featureAvailability.loadFailed' : `featureAvailability.${feature}`) }}</p>
    <Button v-if="state.error" type="button" variant="outline" size="sm" class="mt-3" @click="state.reload">{{ t('featureAvailability.retry') }}</Button>
  </div>
</template>
