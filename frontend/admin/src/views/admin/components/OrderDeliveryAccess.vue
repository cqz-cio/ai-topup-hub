<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Copy, Download, Eye, EyeOff, LoaderCircle, LockKeyhole, ShieldCheck } from 'lucide-vue-next'
import DOMPurify from 'dompurify'
import { Button } from '@/components/ui/button'
import { adminAPI } from '@/api/admin'
import { useAdminAuthStore } from '@/stores/auth'
import { getLocalizedText } from '@/utils/format'
import { createDeliveryAccess, type DeliveryAccessState } from '../../../../../shared/deliveryAccess'

const props = withDefaults(defineProps<{ orderId: number; orderNo: string; source?: 'order' | 'procurement' }>(), { source: 'order' })
const { t } = useI18n()
const auth = useAdminAuthStore()
const key = computed(() => `${props.source}:${props.orderId}`)
const permission = computed(() => props.source === 'order' ? 'GET:/admin/orders/:id/fulfillment' : 'GET:/admin/procurement-orders/:id/upstream-payload/download')
const canRead = computed(() => auth.hasPermission(permission.value))
const records = ref<DeliveryAccessState<any>>({})
const downloading = ref(false)
const actionError = ref(false)
const copied = ref(false)
let copyTimer: ReturnType<typeof setTimeout> | undefined
let disposed = false
const downloadResponse = () => props.source === 'order' ? adminAPI.downloadFulfillment(props.orderId) : adminAPI.downloadProcurementUpstreamPayload(props.orderId)
const access = createDeliveryAccess(async () => {
  if (props.source === 'procurement') {
    const response = await downloadResponse()
    return { fulfillment: { payload: await new Blob([response.data]).text() }, items: [] }
  }
  const response = await adminAPI.getOrderDelivery(props.orderId)
  if (response.data.data?.order_no !== props.orderNo) throw new Error('Order changed')
  return response.data.data
}, value => { records.value = value })
const entry = computed(() => records.value[key.value])
const text = computed(() => {
  const f = entry.value?.data?.fulfillment
  const data = f?.delivery_data
  if (!data || typeof data !== 'object') return String(f?.payload || '')
  const lines = [String(data.note || '')]
  for (const item of Array.isArray(data.entries) ? data.entries : []) lines.push([item.key, item.value].filter(Boolean).join(': '))
  for (const [name, value] of Object.entries(data)) if (!['note', 'entries'].includes(name)) lines.push(`${name}: ${String(value ?? '')}`)
  return lines.filter(Boolean).join('\n') || String(f?.payload || '')
})
const instructions = computed(() => (entry.value?.data?.items || []).map((item: any) => DOMPurify.sanitize(getLocalizedText(item.instructions))).filter(Boolean))
const truncated = computed(() => (entry.value?.data?.fulfillment?.payload_line_count || 0) > 100)
const hide = () => { access.hide(key.value); copied.value = false; actionError.value = false }
const copy = async () => {
  try {
    await navigator.clipboard.writeText(text.value)
    copied.value = true
    if (copyTimer) clearTimeout(copyTimer)
    copyTimer = setTimeout(() => { copied.value = false }, 2200)
  } catch { actionError.value = true }
}
const download = async () => {
  downloading.value = true
  actionError.value = false
  try {
    const identity = key.value
    const token = auth.token
    const response = await downloadResponse()
    if (disposed || identity !== key.value || token !== auth.token || !canRead.value) return
    const url = URL.createObjectURL(new Blob([response.data], { type: 'text/plain; charset=utf-8' }))
    const a = document.createElement('a')
    a.href = url
    a.download = `fulfillment-${props.orderNo}.txt`
    a.click()
    URL.revokeObjectURL(url)
  } catch { actionError.value = true } finally { downloading.value = false }
}
watch(() => [key.value, auth.token, canRead.value], () => { access.clear(); copied.value = false }, { flush: 'sync' })
onUnmounted(() => { disposed = true; access.dispose(); if (copyTimer) clearTimeout(copyTimer) })
</script>

<template>
  <div class="mt-3">
    <div v-if="!entry?.data" class="flex flex-wrap items-center gap-3 rounded-lg border border-dashed border-border bg-background p-4">
      <LockKeyhole class="size-5 shrink-0 text-primary" />
      <div class="min-w-40 flex-1"><h4 class="text-sm font-semibold">{{ t('deliveryAccess.protected') }}</h4><p class="mt-1 text-xs leading-relaxed text-muted-foreground">{{ t(canRead ? 'deliveryAccess.description' : 'deliveryAccess.denied') }}</p></div>
      <Button v-if="canRead" size="sm" :disabled="entry?.loading" @click="access.reveal(key)"><LoaderCircle v-if="entry?.loading" class="animate-spin" /><Eye v-else />{{ t(entry?.loading ? 'deliveryAccess.loading' : entry?.failed ? 'deliveryAccess.retry' : 'deliveryAccess.reveal') }}</Button>
    </div>
    <p v-if="entry?.failed || actionError" role="alert" class="mt-2 text-xs text-destructive">{{ t('deliveryAccess.failed') }}</p>
    <template v-if="entry?.data">
      <div class="mb-3 flex flex-wrap items-center gap-2">
        <span class="mr-auto flex items-center gap-1 text-xs text-primary"><ShieldCheck class="size-4" />{{ t('deliveryAccess.verified') }}</span>
        <Button variant="ghost" size="sm" @click="hide"><EyeOff />{{ t('deliveryAccess.hide') }}</Button>
        <Button v-if="text && !truncated" size="sm" @click="copy"><Copy />{{ t(copied ? 'deliveryAccess.copied' : 'deliveryAccess.copy') }}</Button>
        <Button v-if="text" variant="outline" size="sm" :disabled="downloading" @click="download"><Download />{{ t('deliveryAccess.download') }}</Button>
      </div>
      <p v-if="truncated" class="mb-2 text-xs text-muted-foreground">{{ t('admin.orders.fulfillmentTruncatedHint') }}</p>
      <pre v-if="text" class="max-h-64 overflow-y-auto whitespace-pre-wrap break-all rounded-lg border border-border bg-background p-4 text-xs leading-relaxed">{{ text }}</pre>
      <div v-for="(html, index) in instructions" :key="index" class="prose prose-sm mt-4 max-w-none dark:prose-invert" v-html="html"></div>
    </template>
  </div>
</template>
