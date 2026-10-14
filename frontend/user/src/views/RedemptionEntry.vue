<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { configAPI } from '../api'
import { Button } from '../components/ui/button'
import Redeem from './Redeem.vue'

const { t } = useI18n()
const enabled = ref<boolean | null>(null)
const loading = ref(true)
const reload = async () => {
  enabled.value = null
  loading.value = true
  try {
    const response = await configAPI.get()
    enabled.value = response.data?.data?.features?.redemption === true
  } catch {
    enabled.value = null
  } finally { loading.value = false }
}
onMounted(reload)
</script>

<template>
  <Redeem v-if="enabled === true" />
  <main v-else class="mx-auto flex min-h-[60vh] max-w-2xl items-center px-4 py-12">
    <section class="w-full rounded-xl border border-border bg-card p-6 sm:p-10" role="status">
      <h1 class="text-2xl font-bold">{{ t('featureAvailability.redemptionTitle') }}</h1>
      <p class="mt-4 text-sm leading-7 text-muted-foreground">{{ t(loading ? 'featureAvailability.loading' : enabled === false ? 'featureAvailability.redemptionClosed' : 'featureAvailability.loadFailed') }}</p>
      <div class="mt-6 flex flex-wrap gap-3">
        <Button as-child><a href="/">{{ t('featureAvailability.backToStore') }}</a></Button>
        <Button v-if="enabled === null" variant="outline" :disabled="loading" @click="reload">{{ t('orders.filters.refresh') }}</Button>
      </div>
    </section>
  </main>
</template>
