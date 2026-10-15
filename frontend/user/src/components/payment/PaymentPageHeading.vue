<script setup lang="ts">
import { computed, reactive } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowLeft } from 'lucide-vue-next'
import CheckoutSteps from '../checkout/CheckoutSteps.vue'
import type { usePayment } from '../../composables/usePayment'
import { orderDeliveryState } from '../../utils/orderDelivery'

const props = defineProps<{ checkout: ReturnType<typeof usePayment> }>()
const p = reactive(props.checkout)
const { t, locale } = useI18n()
const delivery = computed(() => orderDeliveryState(p.order))
const messages = {
  'zh-CN': { title: '完成付款', back: '订单详情', hint: '请按照下方信息完成付款，到账后订单会自动更新。' },
  'zh-TW': { title: '完成付款', back: '訂單詳情', hint: '請按照下方資訊完成付款，到帳後訂單會自動更新。' },
  'en-US': { title: 'Complete payment', back: 'Order details', hint: 'Pay using the details below. Your order updates automatically once payment arrives.' },
} as const
const copy = computed(() => messages[locale.value as keyof typeof messages] || messages['zh-CN'])
</script>

<template>
  <header class="payment-heading">
    <RouterLink :to="p.backLink" class="payment-back"><ArrowLeft :size="15" aria-hidden="true" />{{ copy.back }}</RouterLink>
    <div class="payment-heading-row">
      <h1>{{ p.showResultView ? copy.title : t('payment.title') }}</h1>
      <CheckoutSteps :step-keys="['checkout', 'payment', 'delivery']"
        :current-step="['waiting', 'partial', 'ready'].includes(delivery) ? 'delivery' : 'payment'"
        :complete="delivery === 'ready'" />
    </div>
    <p>{{ p.showResultView ? copy.hint : t('shopUi.paymentHint') }}</p>
  </header>
</template>

<style scoped>
.payment-heading { margin-bottom: 28px; }
.payment-back { display: inline-flex; align-items: center; gap: 8px; min-height: 36px; color: var(--ui-text-muted); font-size: 13px; }
.payment-back:hover { color: var(--ui-accent); }
.payment-back:focus-visible { outline: 2px solid var(--ui-accent); outline-offset: 3px; border-radius: 4px; }
.payment-heading-row { display: flex; align-items: center; justify-content: space-between; gap: 32px; margin-top: 12px; }
.payment-heading h1 { font-size: clamp(32px, 3.8vw, 52px); font-weight: 750; line-height: 1.15; letter-spacing: -.04em; }
.payment-heading p { color: var(--ui-text-muted); font-size: 16px; line-height: 1.7; margin-top: 14px; }
.payment-heading :deep(.commerce-steps) { padding: 0; border: 0; border-radius: 0; background: none; gap: 22px; flex-shrink: 0; }
.payment-heading :deep(.commerce-step) { flex: none; }
.payment-heading :deep(.commerce-step:not(:last-child)) { padding-right: 52px; }
.payment-heading :deep(.commerce-step:not(:last-child)::after) { left: auto; right: 0; width: 32px; }
.payment-heading :deep(.commerce-step-label) { font-size: 13px; }
@media (max-width: 960px) {
  .payment-heading-row { align-items: flex-start; flex-direction: column; gap: 18px; }
}
@media (max-width: 640px) {
  .payment-heading { margin-bottom: 14px; }
  .payment-back { font-size: 12px; min-height: 24px; }
  .payment-heading-row { margin-top: 8px; gap: 10px; }
  .payment-heading h1 { font-size: 30px; }
  .payment-heading p { font-size: 13px; margin-top: 8px; }
  .payment-heading :deep(.commerce-steps) { width: 100%; gap: 8px; }
  .payment-heading :deep(.commerce-step) { flex: 1; }
  .payment-heading :deep(.commerce-step:not(:last-child)) { padding-right: 0; }
  .payment-heading :deep(.commerce-step-content) { flex-direction: row; gap: 6px; }
  .payment-heading :deep(.commerce-step-number) { width: 24px; height: 24px; font-size: 11px; }
  .payment-heading :deep(.commerce-step-label) { font-size: 11px; }
  .payment-heading :deep(.commerce-step:not(:last-child)::after) { display: none; }
}
</style>
