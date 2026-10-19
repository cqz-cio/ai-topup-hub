<script setup lang="ts">
import SupportContacts from './SupportContacts.vue'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { AlertTriangle, MailCheck, Mail, Copy, X } from 'lucide-vue-next'
import { DialogClose, DialogContent, DialogDescription, DialogOverlay, DialogPortal, DialogRoot, DialogTitle } from 'reka-ui'
import { Button } from '@/components/ui/button'
import { usePaymentBlock } from '@/composables/usePaymentBlock'
import { copyText } from '@/utils/clipboard'

const { t } = useI18n()
const { open, block, close, restoreFocus } = usePaymentBlock()
const contact = 'Cqz0924ttxk'
const copied = ref(false)
const copyFailed = ref(false)
const confirmButton = ref<InstanceType<typeof Button> | null>(null)
const insufficient = computed(() => block.value?.code === 'card_platform_insufficient_balance')
const sent = computed(() => block.value?.notification_status === 'sent')
watch(open, () => { copied.value = false; copyFailed.value = false })
const copyContact = async () => {
  try { await copyText(contact); copied.value = true; copyFailed.value = false }
  catch { copyFailed.value = true }
}
const focusConfirm = (event: Event) => { event.preventDefault(); confirmButton.value?.$el.focus() }
</script>

<template>
  <DialogRoot :open="open" @update:open="value => { if (!value) close() }">
    <DialogPortal>
      <DialogOverlay class="fixed inset-0 z-[150] bg-black/40 backdrop-blur-sm" />
      <DialogContent aria-modal="true" class="fixed left-1/2 top-1/2 z-[150] max-h-[calc(100dvh-2rem)] w-[calc(100%-2rem)] max-w-[440px] -translate-x-1/2 -translate-y-1/2 overflow-y-auto rounded-2xl border bg-card p-6 text-card-foreground shadow-2xl outline-none sm:p-8" @open-auto-focus="focusConfirm" @close-auto-focus="restoreFocus">
        <DialogClose as-child>
          <button type="button" class="absolute right-4 top-4 flex size-8 items-center justify-center rounded-full text-muted-foreground hover:bg-secondary focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring" :aria-label="t('common.close')"><X class="size-4" /></button>
        </DialogClose>
        <div class="mb-5 flex size-12 items-center justify-center rounded-xl bg-amber-50 text-amber-600" aria-hidden="true"><AlertTriangle class="size-6" :stroke-width="1.8" /></div>
        <DialogTitle class="text-xl font-bold leading-7">{{ t(`paymentBalance.${insufficient ? 'title' : 'unavailableTitle'}`) }}</DialogTitle>
        <DialogDescription class="mt-3 text-sm leading-6 text-muted-foreground">{{ t(`paymentBalance.${insufficient ? 'description' : 'unavailableDescription'}`) }}</DialogDescription>
        <div v-if="insufficient" role="status" aria-live="polite" class="mt-5 flex items-start gap-2.5 rounded-xl px-3.5 py-3 text-sm leading-5" :class="sent ? 'bg-emerald-50 text-emerald-800' : 'bg-secondary text-muted-foreground'">
          <component :is="sent ? MailCheck : Mail" class="mt-0.5 size-4 shrink-0" aria-hidden="true" />
          <span>{{ t(`paymentBalance.notice.${block?.notification_status || 'not_sent'}`) }}</span>
        </div>
        <div class="mt-4 flex flex-wrap items-center justify-between gap-2 rounded-xl bg-secondary px-4 py-3.5 text-sm"><span class="text-muted-foreground">{{ t('paymentBalance.contact') }}</span><strong class="select-all font-semibold">{{ contact }}</strong></div>
        <SupportContacts />
        <p v-if="copyFailed" role="status" class="mt-2 text-xs text-muted-foreground">{{ t('paymentBalance.copyFailed') }}</p>
        <div class="mt-6 grid grid-cols-2 gap-3">
          <DialogClose as-child><Button ref="confirmButton" type="button" variant="outline" class="h-11">{{ t('paymentBalance.confirm') }}</Button></DialogClose>
          <Button type="button" class="h-11 gap-2 px-2" @click="copyContact"><Copy class="size-4 shrink-0" aria-hidden="true" />{{ t(`paymentBalance.${copied ? 'copied' : 'copy'}`) }}</Button>
        </div>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
