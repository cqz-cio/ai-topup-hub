<template>
  <div class="delivery-access">
    <div v-if="!entry?.data" class="delivery-locked">
      <div class="delivery-lock-symbol"><LockKeyhole :size="21" /></div>
      <div class="delivery-lock-copy"><h4>{{ t('deliveryAccess.protected') }}</h4><p>{{ t('deliveryAccess.description') }}</p></div>
      <Button type="button" :disabled="entry?.loading" @click="access.reveal(orderNo)">
        <LoaderCircle v-if="entry?.loading" class="animate-spin" /><Eye v-else />
        {{ t(entry?.loading ? 'deliveryAccess.loading' : entry?.failed ? 'deliveryAccess.retry' : 'deliveryAccess.reveal') }}
      </Button>
    </div>
    <p v-if="entry?.failed" class="delivery-access-error" role="alert">{{ t('deliveryAccess.failed') }}</p>
    <template v-if="entry?.data">
      <div class="delivery-access-controls">
        <span><ShieldCheck :size="15" />{{ t('deliveryAccess.verified') }}</span>
        <Button type="button" variant="ghost" size="sm" @click="hide"><EyeOff />{{ t('shopUi.hide') }}</Button>
        <Button v-if="text && !truncated" type="button" size="sm" @click="copy"><Check v-if="copied" /><Copy v-else />{{ t(copied ? 'shopUi.copied' : 'shopUi.copy') }}</Button>
        <Button v-if="text && (entry.data.fulfillment?.type === 'auto' || truncated)" type="button" variant="outline" size="sm" :disabled="downloading" @click="$emit('download', orderNo)"><Download />{{ t(downloading ? 'orderDetail.fulfillmentDownloading' : 'shopUi.download') }}</Button>
      </div>
      <p v-if="truncated" class="delivery-preview-hint">{{ t('shopUi.copyPartialHint') }}</p>
      <pre v-if="text" class="delivery-content" tabindex="0">{{ text }}</pre>
      <p v-else class="delivery-empty">{{ t('shopUi.manualDelivery') }}</p>
      <div v-if="instructions.length" class="delivery-instructions">
        <h4><Info :size="15" />{{ t('orderDetail.instructionsTitle') }}</h4>
        <div v-for="(block, index) in instructions" :key="index" class="prose prose-sm max-w-none dark:prose-invert" v-html="block.html"></div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Check, Copy, Download, Eye, EyeOff, Info, LoaderCircle, LockKeyhole, ShieldCheck } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { useOrderDisplayHelpers } from '@/composables/useOrderDisplayHelpers'
import { toast } from '@/composables/useToast'
import { copyText } from '@/utils/clipboard'
import { createDeliveryAccess, type DeliveryAccessState } from '../../../../shared/deliveryAccess'

const props = defineProps<{ orderNo: string; downloading: boolean; loadDelivery: (orderNo: string) => Promise<any>; sessionKey: string | number }>()
defineEmits<{ download: [orderNo: string] }>()
const { t } = useI18n()
const { instructionBlocks, fulfillmentDeliveryLines, isFulfillmentTruncated } = useOrderDisplayHelpers(ref(null))
const records = ref<DeliveryAccessState<any>>({})
const access = createDeliveryAccess((key) => props.loadDelivery(key), value => { records.value = value })
const entry = computed(() => records.value[props.orderNo])
const text = computed(() => fulfillmentDeliveryLines(entry.value?.data?.fulfillment).join('\n') || String(entry.value?.data?.fulfillment?.payload || ''))
const truncated = computed(() => isFulfillmentTruncated(entry.value?.data?.fulfillment))
const instructions = computed(() => instructionBlocks(entry.value?.data?.items || []))
const copied = ref(false)
let copyTimer: ReturnType<typeof setTimeout> | undefined
const hide = () => { access.hide(props.orderNo); copied.value = false; if (copyTimer) clearTimeout(copyTimer) }
const copy = async () => {
  try {
    await copyText(text.value)
    copied.value = true
    if (copyTimer) clearTimeout(copyTimer)
    copyTimer = setTimeout(() => { copied.value = false }, 2200)
  } catch { toast.error(t('shopUi.copyFailed')) }
}
watch(() => [props.orderNo, props.sessionKey], () => { access.clear(); copied.value = false }, { flush: 'sync' })
onUnmounted(() => { access.dispose(); if (copyTimer) clearTimeout(copyTimer) })
</script>

<style scoped>
.delivery-locked { display: flex; align-items: center; gap: 16px; background: var(--ui-bg-soft, var(--muted)); border: 1px dashed var(--ui-border, var(--border)); border-radius: 12px; padding: 22px; }
.delivery-lock-symbol { display: flex; flex-shrink: 0; color: var(--ui-success, var(--primary)); padding: 12px; background: var(--ui-bg-elevated, var(--card)); border-radius: 12px; }
.delivery-lock-copy { flex: 1; }
.delivery-lock-copy h4 { font-size: 14px; font-weight: 650; }
.delivery-lock-copy p { font-size: 12px; color: var(--ui-text-muted, var(--muted-foreground)); line-height: 1.8; margin-top: 6px; }
.delivery-access-controls { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; margin-bottom: 14px; }
.delivery-access-controls > span { margin-right: auto; color: var(--ui-success, var(--primary)); display: flex; align-items: center; gap: 6px; font-size: 12px; }
.delivery-content { white-space: pre-wrap; overflow-wrap: anywhere; word-break: break-all; background: var(--ui-bg-soft, var(--muted)); border: 1px solid var(--ui-border, var(--border)); border-radius: 12px; padding: 20px; font-family: 'SFMono-Regular', Consolas, monospace; font-size: 13px; line-height: 1.9; max-height: 350px; overflow: auto; user-select: text; }
.delivery-content:focus-visible { outline: 2px solid var(--ui-success, var(--primary)); outline-offset: 2px; }
.delivery-instructions { margin-top: 24px; font-size: 12px; }
.delivery-instructions h4 { display: flex; align-items: center; gap: 8px; color: var(--ui-success, var(--primary)); margin-bottom: 16px; font-weight: 600; }
.delivery-instructions > div { overflow-wrap: anywhere; }
.delivery-access-error { color: var(--ui-danger, var(--destructive)); font-size: 12px; margin-top: 12px; line-height: 1.8; }
.delivery-preview-hint, .delivery-empty { color: var(--ui-text-muted, var(--muted-foreground)); font-size: 12px; margin-bottom: 12px; }
@media (max-width: 767px) {
  .delivery-locked { flex-wrap: wrap; padding: 18px; gap: 12px; }
  .delivery-lock-copy { min-width: 130px; }
  .delivery-locked > button { width: 100%; min-height: 42px; }
  .delivery-access-controls > span { width: 100%; margin-bottom: 6px; }
  .delivery-access-controls button { flex: 1; min-height: 40px; }
  .delivery-content { padding: 16px; font-size: 12px; }
}
</style>
