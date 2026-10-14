<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ReceiptText } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { copyText } from '@/utils/clipboard'
import { toast } from '@/composables/useToast'

const props = defineProps<{ order?: { status?: string; order_no?: string } | null }>()
const { t } = useI18n()
const supportWechat = 'Cqz0924ttxk'
// Payment confirmation is independent of delivery; unpaid and closed orders do not offer invoicing.
const paidStatuses = ['paid', 'fulfilling', 'partially_delivered', 'delivered', 'completed', 'partially_refunded']
const orderNo = computed(() => String(props.order?.order_no || '').trim())
const visible = computed(() => Boolean(orderNo.value) && paidStatuses.includes(props.order?.status || ''))
const copied = ref<'wechat' | 'order' | null>(null)
let copyTimer: ReturnType<typeof setTimeout> | undefined
let disposed = false

const resetCopy = () => {
  if (copyTimer) clearTimeout(copyTimer)
  copyTimer = undefined
  copied.value = null
}

const copyContact = async (target: 'wechat' | 'order') => {
  const currentOrderNo = orderNo.value
  if (!visible.value) return
  try {
    await copyText(target === 'wechat' ? supportWechat : currentOrderNo)
    if (disposed || orderNo.value !== currentOrderNo || !visible.value) return
    resetCopy()
    copied.value = target
    copyTimer = setTimeout(resetCopy, 2200)
  } catch {
    if (disposed) return
    resetCopy()
    toast.error(t('invoiceSupport.copyFailed'))
  }
}

watch([() => props.order?.order_no, () => props.order?.status], resetCopy)
onUnmounted(() => { disposed = true; resetCopy() })
</script>

<template>
  <section v-if="visible" class="invoice-support" :aria-label="t('invoiceSupport.title')" data-invoice-support>
    <div class="invoice-support-details">
      <span class="invoice-support-icon" aria-hidden="true"><ReceiptText /></span>
      <div class="invoice-support-copy">
        <h2>{{ t('invoiceSupport.title') }}</h2>
        <p>{{ t('invoiceSupport.description') }}</p>
        <div class="invoice-support-wechat">
          <span>{{ t('shopUi.wechat') }}</span>
          <strong>{{ supportWechat }}</strong>
        </div>
      </div>
    </div>
    <div class="invoice-support-actions">
      <Button type="button" @click="copyContact('wechat')">
        {{ t(copied === 'wechat' ? 'invoiceSupport.wechatCopied' : 'invoiceSupport.copyWechat') }}
      </Button>
      <Button type="button" variant="outline" @click="copyContact('order')">
        {{ t(copied === 'order' ? 'invoiceSupport.orderCopied' : 'invoiceSupport.copyOrder') }}
      </Button>
    </div>
    <span class="sr-only" role="status" aria-live="polite">
      {{ copied ? t(copied === 'wechat' ? 'invoiceSupport.wechatCopied' : 'invoiceSupport.orderCopied') : '' }}
    </span>
  </section>
</template>

<style scoped>
.invoice-support {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 28px;
  padding: 28px;
  border: 1px solid var(--ui-border);
  border-radius: 16px;
  background: var(--ui-bg-elevated);
  color: var(--ui-text-primary);
}
.invoice-support-details { display: flex; align-items: center; gap: 20px; min-width: 0; }
.invoice-support-icon { display: grid; place-items: center; flex: 0 0 64px; width: 64px; height: 64px; border-radius: 50%; background: var(--ui-bg-soft); color: var(--ui-accent); }
.invoice-support-icon svg { width: 30px; height: 30px; stroke-width: 1.8; }
.invoice-support-copy { min-width: 0; overflow-wrap: anywhere; }
.invoice-support-copy h2 { margin: 0; font-size: 20px; font-weight: 700; line-height: 1.5; }
.invoice-support-copy p { margin: 5px 0 0; color: var(--ui-text-secondary); font-size: 14px; line-height: 1.7; }
.invoice-support-wechat { display: flex; align-items: baseline; flex-wrap: wrap; gap: 6px 16px; margin-top: 10px; font-size: 14px; color: var(--ui-text-secondary); }
.invoice-support-wechat strong { color: var(--ui-accent); font-size: 20px; font-weight: 700; line-height: 1.4; }
.invoice-support-actions { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; flex: 0 0 324px; }
.invoice-support-actions button { min-width: 0; min-height: 44px; height: auto; padding: 10px 12px; font-size: 14px; line-height: 20px; white-space: normal; }
@media (max-width: 1023px) {
  .invoice-support { flex-direction: column; align-items: stretch; gap: 20px; }
  .invoice-support-actions { flex: none; width: 324px; max-width: 100%; margin-left: auto; }
}
@media (max-width: 767px) {
  .invoice-support { padding: 20px; }
  .invoice-support-details { align-items: flex-start; gap: 12px; }
  .invoice-support-icon { flex-basis: 40px; width: 40px; height: 40px; }
  .invoice-support-icon svg { width: 22px; height: 22px; }
  .invoice-support-copy h2 { font-size: 18px; }
  .invoice-support-wechat { gap: 2px 10px; }
  .invoice-support-wechat strong { font-size: 18px; }
  .invoice-support-actions { width: 100%; gap: 10px; }
}
</style>
