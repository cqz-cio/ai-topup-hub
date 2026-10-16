<template>
  <section class="mb-5 rounded-xl border border-primary/15 bg-secondary p-4 sm:p-5" :aria-label="copy.title" data-testid="affiliate-rules">
    <div v-if="loading && !rules" class="h-28 animate-pulse rounded-lg bg-muted" :aria-label="t('walletEarnings.loading')"></div>
    <div v-else-if="!rules" role="status" class="space-y-3 text-sm text-muted-foreground">
      <p>{{ copy.loadFailed }}</p>
      <Button type="button" variant="outline" size="sm" @click="refresh">{{ copy.retry }}</Button>
    </div>
    <p v-else-if="!rules.enabled" role="status" class="text-sm text-muted-foreground">{{ copy.closed }}</p>
    <template v-else>
      <div class="grid grid-cols-1 gap-3.5 sm:grid-cols-[1fr_1.25fr_1.3fr] sm:gap-5">
        <div class="min-w-0">
          <p class="text-xs font-semibold text-ink-secondary">{{ copy.rate }}</p>
          <p class="mt-1.5 text-[2rem] font-extrabold leading-tight text-primary" data-testid="affiliate-rate">{{ rules.rateText }}%</p>
          <p class="mt-1.5 text-xs leading-relaxed text-ink-secondary">{{ rules.ratePercent === 0 ? copy.zeroRate : copy.currentRate }}</p>
        </div>
        <div class="min-w-0 border-t border-primary/10 pt-3.5 sm:border-t-0 sm:border-l sm:pt-0 sm:pl-5">
          <p class="text-xs font-semibold text-ink-secondary">{{ copy.calculation }}</p>
          <p class="mt-1.5 flex flex-wrap items-baseline gap-x-2 text-[25px] font-extrabold leading-tight text-primary" data-testid="affiliate-example">
            <span><span class="text-base">¥</span>{{ AFFILIATE_EXAMPLE_PRINCIPAL }}</span>
            <span class="text-lg font-normal text-muted-foreground">→</span>
            <span><span class="text-base">¥</span>{{ rules.exampleCommission }}</span>
          </p>
          <p class="mt-2 text-xs leading-relaxed text-ink-secondary">{{ copy.formula(rules.rateText) }}</p>
        </div>
        <div class="min-w-0 border-t border-primary/10 pt-3.5 sm:border-t-0 sm:border-l sm:pt-0 sm:pl-5">
          <p class="text-xs font-semibold text-ink-secondary">{{ copy.timing }}</p>
          <p class="mt-1.5 text-[23px] font-extrabold leading-snug text-primary" data-testid="affiliate-confirmation">{{ rules.confirmDays === 0 ? copy.noWait : copy.paidAfter(rules.confirmDays) }}</p>
          <p class="mt-2 text-xs leading-relaxed text-ink-secondary">{{ copy.credited }}</p>
        </div>
      </div>
      <p class="mt-4 border-t border-primary/10 pt-3 text-xs leading-relaxed text-muted-foreground">{{ copy.note }}</p>
    </template>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { configAPI } from '../../api'
import { affiliateRulesCopy } from '../../i18n/affiliateRules'
import { AFFILIATE_EXAMPLE_PRINCIPAL, parseAffiliateRules, type AffiliateRules } from '../../utils/affiliateRules'
import { Button } from '@/components/ui/button'

const { t, locale } = useI18n()
const copy = computed(() => affiliateRulesCopy[locale.value] || affiliateRulesCopy['zh-CN']!)
const rules = ref<AffiliateRules | null>(null)
const loading = ref(true)
let pending: Promise<void> | null = null
let disposed = false

// Fetch on every entry and return to the page, rather than reusing the global
// configuration cache. A failed refresh removes stale numerical promises.
function refresh(): Promise<void> {
  if (pending) return pending
  loading.value = true
  pending = (async () => {
    try {
      const response = await configAPI.get()
      if (!disposed) rules.value = parseAffiliateRules(response.data.data?.affiliate)
    } catch {
      if (!disposed) rules.value = null
    } finally {
      if (!disposed) loading.value = false
      pending = null
    }
  })()
  return pending
}

const refreshWhenVisible = () => {
  if (document.visibilityState === 'visible') void refresh()
}

onMounted(() => {
  void refresh()
  window.addEventListener('focus', refreshWhenVisible)
  document.addEventListener('visibilitychange', refreshWhenVisible)
})
onUnmounted(() => {
  disposed = true
  window.removeEventListener('focus', refreshWhenVisible)
  document.removeEventListener('visibilitychange', refreshWhenVisible)
})
</script>
