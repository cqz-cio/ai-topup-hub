<template>
  <section id="order-delivery" class="order-delivery" :aria-label="t('shopUi.deliveryTitle')">
    <div class="delivery-heading" :class="{ 'is-ready': hasDelivery }" role="status" aria-live="polite">
      <div class="delivery-symbol"><PackageCheck v-if="hasDelivery" /><Clock3 v-else-if="state === 'waiting'" /><KeyRound v-else /></div>
      <div><p class="commerce-eyebrow">{{ t('shopUi.deliveryEyebrow') }}</p><h2>{{ t(headingKey) }}</h2><p>{{ t(hintKey) }}</p></div>
    </div>
    <template v-if="hasDelivery">
      <article v-for="group in groups" :key="group.orderNo" class="delivery-group">
        <div class="delivery-group-heading">
          <div><h3>{{ groupTitle(group) || t('shopUi.content') }}</h3><p v-if="groups.length > 1">{{ group.orderNo }}</p></div>
        </div>
        <p v-if="!group.delivered" class="delivery-waiting"><Clock3 :size="16" />{{ t('shopUi.deliveryPending') }}</p>
        <DeliveryContent v-else :order-no="group.orderNo" :downloading="downloading" :load-delivery="loadDelivery" :session-key="sessionKey" @download="$emit('download', $event)" />
      </article>
    </template>
    <div class="delivery-footer"><span><LockKeyhole :size="13" />{{ t('shopUi.deliveryDescription') }}</span><RouterLink to="/products">{{ t('shopUi.continueShopping') }}<ArrowUpRight :size="14" /></RouterLink></div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowUpRight, Clock3, KeyRound, LockKeyhole, PackageCheck } from 'lucide-vue-next'
import DeliveryContent from './DeliveryContent.vue'
import { useOrderDisplayHelpers } from '@/composables/useOrderDisplayHelpers'
import { orderDeliveryGroups, orderDeliveryState, hasCardPlatformBalanceIssue, type DeliveryGroup } from '@/utils/orderDelivery'

const props = defineProps<{ order: any; downloading: boolean; loadDelivery: (orderNo: string) => Promise<any>; sessionKey: string | number }>()
defineEmits<{ download: [orderNo: string] }>()
const { t } = useI18n()
const { getLocalizedText } = useOrderDisplayHelpers(computed(() => props.order))
const groups = computed(() => orderDeliveryGroups(props.order))
const state = computed(() => orderDeliveryState(props.order))
const hasDelivery = computed(() => groups.value.some(group => group.delivered))
const headingKey = computed(() => ({ unpaid: 'shopUi.deliveryUnpaid', waiting: 'shopUi.deliveryWaiting', partial: 'shopUi.deliveryPartial', ready: 'shopUi.deliveryReady', closed: 'shopUi.deliveryClosed' }[state.value]))
const hintKey = computed(() => hasCardPlatformBalanceIssue(props.order) ? 'shopUi.cardPlatformBalanceInsufficient' : ({ unpaid: 'shopUi.deliveryUnpaidHint', waiting: 'shopUi.deliveryWaitingHint', partial: 'shopUi.deliveryWaitingHint', ready: 'shopUi.deliveryReadyHint', closed: 'shopUi.deliveryClosedHint' }[state.value]))
const groupTitle = (group: DeliveryGroup) => group.items.map(item => getLocalizedText(item.title)).filter(Boolean).join(' / ')
</script>

<style scoped>
.order-delivery { background: var(--ui-bg-elevated); border: 1px solid var(--ui-border); border-radius: 18px; overflow: hidden; }
.delivery-heading { padding: 26px 28px; display: flex; align-items: flex-start; gap: 18px; background: var(--ui-bg-soft); }
.delivery-heading.is-ready { background: var(--ui-success-soft); }
.delivery-heading .commerce-eyebrow { font-size: 10px; margin-bottom: 7px; }
.delivery-heading h2 { font-size: 21px; font-weight: 700; line-height: 1.5; }
.delivery-heading div > p:last-child { color: var(--ui-text-muted); font-size: 12px; margin-top: 7px; line-height: 1.8; }
.delivery-symbol { display: flex; align-items: center; justify-content: center; background: var(--ui-bg-elevated); border: 1px solid var(--ui-border); border-radius: 12px; height: 46px; width: 46px; color: var(--ui-success); flex-shrink: 0; }
.delivery-symbol svg { height: 22px; width: 22px; }
.delivery-group { padding: 24px 28px; border-top: 1px solid var(--ui-border); }
.delivery-group-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 20px; margin-bottom: 16px; }
.delivery-group-heading h3 { font-size: 14px; font-weight: 650; line-height: 1.6; }
.delivery-group-heading p { font-size: 10px; color: var(--ui-text-muted); margin-top: 6px; word-break: break-all; }
.delivery-waiting, .delivery-hidden { color: var(--ui-text-muted); font-size: 12px; display: flex; align-items: center; gap: 10px; padding: 18px; background: var(--ui-bg-soft); border-radius: 10px; }
.delivery-footer { padding: 16px 28px; border-top: 1px solid var(--ui-border); font-size: 10px; display: flex; align-items: center; justify-content: space-between; gap: 18px; color: var(--ui-text-muted); }
.delivery-footer > span { display: flex; align-items: flex-start; gap: 7px; }
.delivery-footer svg { flex-shrink: 0; margin-top: 2px; }
.delivery-footer a { display: flex; align-items: center; gap: 5px; flex-shrink: 0; color: var(--ui-text-primary); }
@media (max-width: 767px) {
  .delivery-heading { padding: 22px 18px; gap: 12px; }
  .delivery-heading h2 { font-size: 18px; }
  .delivery-symbol { height: 40px; width: 40px; border-radius: 10px; }
  .delivery-heading div > p:last-child { font-size: 11px; }
  .delivery-group { padding: 20px 18px; }
  .delivery-group-heading { flex-direction: column; gap: 14px; }
  .delivery-footer { padding: 16px 18px; flex-direction: column; align-items: flex-start; gap: 14px; }
}
</style>
