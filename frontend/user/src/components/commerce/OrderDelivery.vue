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
          <div v-if="group.delivered" class="delivery-controls">
            <Button type="button" variant="ghost" size="sm" :aria-expanded="!hiddenGroups.has(group.orderNo)" @click="toggleVisibility(group.orderNo)">
              <Eye v-if="hiddenGroups.has(group.orderNo)" /><EyeOff v-else />{{ t(hiddenGroups.has(group.orderNo) ? 'shopUi.reveal' : 'shopUi.hide') }}
            </Button>
            <Button v-if="!isFulfillmentTruncated(group.fulfillment) && deliveryText(group.fulfillment)" type="button" size="sm" @click="copyDelivery(group)">
              <Check v-if="copiedOrder === group.orderNo" /><Copy v-else />{{ t(copiedOrder === group.orderNo ? 'shopUi.copied' : 'shopUi.copy') }}
            </Button>
            <Button v-if="group.fulfillment?.type === 'auto' || isFulfillmentTruncated(group.fulfillment)" type="button" variant="outline" size="sm" :disabled="downloading" @click="$emit('download', group.orderNo)">
              <Download />{{ downloading ? t('orderDetail.fulfillmentDownloading') : t('shopUi.download') }}
            </Button>
          </div>
        </div>
        <p v-if="!group.delivered" class="delivery-waiting"><Clock3 :size="16" />{{ t('shopUi.deliveryPending') }}</p>
        <template v-else>
          <p v-if="isFulfillmentTruncated(group.fulfillment)" class="delivery-preview-hint"><Info :size="15" />{{ t('shopUi.copyPartialHint') }}</p>
          <p v-if="hiddenGroups.has(group.orderNo)" class="delivery-hidden"><LockKeyhole :size="18" />{{ t('shopUi.hiddenContent') }}</p>
          <pre v-else-if="deliveryText(group.fulfillment)" class="delivery-content" tabindex="0">{{ deliveryText(group.fulfillment) }}</pre>
          <p v-else class="delivery-waiting">{{ t('shopUi.manualDelivery') }}</p>
          <div v-if="instructionBlocks(group.items).length" class="delivery-instructions">
            <h4><Info :size="15" />{{ t('orderDetail.instructionsTitle') }}</h4>
            <div v-for="(block, index) in instructionBlocks(group.items)" :key="index" class="prose prose-sm max-w-none dark:prose-invert" v-html="block.html"></div>
          </div>
        </template>
      </article>
    </template>
    <div class="delivery-footer"><span><LockKeyhole :size="13" />{{ t('shopUi.deliveryDescription') }}</span><RouterLink to="/products">{{ t('shopUi.continueShopping') }}<ArrowUpRight :size="14" /></RouterLink></div>
  </section>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowUpRight, Check, Clock3, Copy, Download, Eye, EyeOff, Info, KeyRound, LockKeyhole, PackageCheck } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { useOrderDisplayHelpers } from '@/composables/useOrderDisplayHelpers'
import { toast } from '@/composables/useToast'
import { copyText } from '@/utils/clipboard'
import { orderDeliveryGroups, orderDeliveryState, hasCardPlatformBalanceIssue, type DeliveryGroup } from '@/utils/orderDelivery'

const props = defineProps<{ order: any; downloading: boolean }>()
defineEmits<{ download: [orderNo: string] }>()
const { t } = useI18n()
const orderRef = computed(() => props.order)
const { getLocalizedText, instructionBlocks, fulfillmentDeliveryLines, isFulfillmentTruncated } = useOrderDisplayHelpers(orderRef)
const groups = computed(() => orderDeliveryGroups(props.order))
const state = computed(() => orderDeliveryState(props.order))
const hasDelivery = computed(() => groups.value.some(group => group.delivered))
const headingKey = computed(() => ({ unpaid: 'shopUi.deliveryUnpaid', waiting: 'shopUi.deliveryWaiting', partial: 'shopUi.deliveryPartial', ready: 'shopUi.deliveryReady', closed: 'shopUi.deliveryClosed' }[state.value]))
const hintKey = computed(() => hasCardPlatformBalanceIssue(props.order) ? 'shopUi.cardPlatformBalanceInsufficient' : ({ unpaid: 'shopUi.deliveryUnpaidHint', waiting: 'shopUi.deliveryWaitingHint', partial: 'shopUi.deliveryWaitingHint', ready: 'shopUi.deliveryReadyHint', closed: 'shopUi.deliveryClosedHint' }[state.value]))
const hiddenGroups = ref(new Set<string>())
const copiedOrder = ref('')
let copyTimer: ReturnType<typeof setTimeout> | undefined
const groupTitle = (group: DeliveryGroup) => group.items.map(item => getLocalizedText(item.title)).filter(Boolean).join(' / ')
const deliveryText = (fulfillment: any) => fulfillmentDeliveryLines(fulfillment).join('\n') || String(fulfillment?.payload || '')
const toggleVisibility = (orderNo: string) => {
  if (hiddenGroups.value.has(orderNo)) hiddenGroups.value.delete(orderNo)
  else hiddenGroups.value.add(orderNo)
}
const copyDelivery = async (group: DeliveryGroup) => {
  try {
    await copyText(deliveryText(group.fulfillment))
    copiedOrder.value = group.orderNo
    if (copyTimer) clearTimeout(copyTimer)
    copyTimer = setTimeout(() => { copiedOrder.value = '' }, 2200)
  } catch { toast.error(t('shopUi.copyFailed')) }
}
watch(() => props.order?.order_no, () => { hiddenGroups.value.clear(); copiedOrder.value = '' })
onUnmounted(() => { if (copyTimer) clearTimeout(copyTimer) })
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
.delivery-controls { display: flex; flex-wrap: wrap; gap: 6px; flex-shrink: 0; }
.delivery-content { white-space: pre-wrap; overflow-wrap: anywhere; word-break: break-all; background: var(--ui-bg-soft); border: 1px solid var(--ui-border); border-radius: 12px; padding: 20px; color: var(--ui-text-primary); font-family: 'SFMono-Regular', Consolas, monospace; font-size: 13px; line-height: 1.9; max-height: 350px; overflow: auto; user-select: text; }
.delivery-content:focus-visible { outline: 2px solid var(--ui-success); outline-offset: 2px; }
.delivery-waiting, .delivery-hidden { color: var(--ui-text-muted); font-size: 12px; display: flex; align-items: center; gap: 10px; padding: 18px; background: var(--ui-bg-soft); border-radius: 10px; }
.delivery-preview-hint { color: var(--ui-warning); background: var(--ui-warning-soft); border-radius: 9px; padding: 12px; font-size: 11px; display: flex; align-items: flex-start; gap: 8px; margin-bottom: 14px; }
.delivery-preview-hint svg { flex-shrink: 0; }
.delivery-instructions { margin-top: 24px; font-size: 12px; }
.delivery-instructions h4 { display: flex; align-items: center; gap: 8px; color: var(--ui-success); margin-bottom: 16px; font-weight: 600; }
.delivery-instructions > div { color: var(--ui-text-secondary); overflow-wrap: anywhere; }
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
  .delivery-controls { width: 100%; }
  .delivery-controls button { flex: 1; min-height: 40px; }
  .delivery-content { padding: 16px; font-size: 12px; }
  .delivery-footer { padding: 16px 18px; flex-direction: column; align-items: flex-start; gap: 14px; }
}
</style>
