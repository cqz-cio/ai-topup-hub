<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { purchaseClarityCopy } from '../../i18n/purchaseClarity'
import { amountToCents, centsToAmount } from '../../utils/money'
import { checkoutPaymentSummary } from '../../utils/checkoutPaymentSummary'
import { useLocalized } from '../../composables/useProduct'

const props = defineProps<{
  subtotal: number | string; currency: string; walletCents: number; onlineCents: number; feeCents: number
  channel?: { name?: string; provider_type?: string }; requiresChannel: boolean; updating: boolean
}>()
const { locale } = useI18n()
const { formatPrice } = useLocalized()
const copy = computed(() => purchaseClarityCopy[locale.value] || purchaseClarityCopy['zh-CN']!)
const summary = computed(() => checkoutPaymentSummary(amountToCents(props.subtotal) ?? 0, props.walletCents, props.onlineCents, props.feeCents))
const ready = computed(() => !props.updating && (!props.requiresChannel || !!props.channel))
const isUSDT = computed(() => props.requiresChannel && ['bscusdt', 'plasmausdt', 'tronusdt'].includes(props.channel?.provider_type || ''))
const price = (cents: number) => formatPrice(centsToAmount(cents), props.currency)
</script>

<template>
  <section class="mb-4 rounded-xl border border-primary/30 bg-primary/5 p-4" data-payment-summary aria-live="polite" aria-atomic="true" :aria-busy="updating">
    <dl class="grid gap-2 text-sm">
      <div class="flex flex-wrap justify-between gap-x-3 gap-y-1"><dt class="text-muted-foreground">{{ copy.subtotal }}</dt><dd class="font-semibold tabular-nums">{{ price(summary.subtotal) }}</dd></div>
      <div class="flex flex-wrap justify-between gap-x-3 gap-y-1"><dt class="text-muted-foreground">{{ copy.fee }}</dt><dd class="font-semibold tabular-nums">{{ ready ? price(summary.fee) : '—' }}</dd></div>
      <div class="mt-1 flex flex-wrap items-baseline justify-between gap-2 border-t pt-3"><dt class="font-bold">{{ copy.total }}</dt><dd class="text-2xl font-extrabold text-primary tabular-nums" data-payable-total>{{ ready ? price(summary.total) : '—' }}</dd></div>
      <template v-if="ready && walletCents > 0">
        <div class="flex flex-wrap justify-between gap-2"><dt class="text-muted-foreground">{{ copy.wallet }}</dt><dd class="tabular-nums">{{ price(summary.wallet) }}</dd></div>
        <div class="flex flex-wrap justify-between gap-2"><dt class="text-muted-foreground">{{ copy.online }}</dt><dd class="font-bold tabular-nums">{{ price(summary.online) }}</dd></div>
      </template>
    </dl>
    <p v-if="!ready" class="mt-3 text-xs text-muted-foreground">{{ updating ? copy.updating : copy.choose }}</p>
    <p v-else-if="requiresChannel && channel?.name" class="mt-3 text-xs text-muted-foreground">{{ copy.channel }}：{{ channel.name }}</p>
    <p v-if="isUSDT" class="mt-2 text-xs leading-relaxed text-muted-foreground">{{ copy.usdt }}</p>
  </section>
</template>
