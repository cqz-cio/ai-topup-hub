<script setup lang="ts">
import { Check, Copy } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'

defineProps<{
  details: Array<{ key: string; label: string; value: string; detail?: string }>
  amountCopied: boolean
  addressCopied: boolean
}>()

defineEmits<{
  'copy-amount': []
  'copy-address': []
}>()

const { t } = useI18n()
</script>

<template>
  <div class="space-y-3 rounded-xl border bg-background/60 p-3 text-left">
    <div
      v-for="item in details"
      :key="item.key"
      class="flex flex-col gap-2 border-b pb-3 last:border-b-0 last:pb-0"
    >
      <div class="flex min-w-0 flex-col gap-1 sm:flex-row sm:justify-between sm:gap-4">
        <span class="shrink-0 text-xs text-muted-foreground">{{ item.label }}</span>
        <span class="min-w-0 break-all text-sm font-semibold text-foreground sm:text-right">
          {{ item.value }}
          <span v-if="item.detail" class="ml-1 font-normal text-muted-foreground">({{ item.detail }})</span>
        </span>
      </div>
      <Button
        v-if="item.key === 'amount'"
        type="button"
        variant="outline"
        size="sm"
        class="self-end border-foreground/25 bg-background font-semibold"
        :aria-label="t('payment.copyCryptoAmount')"
        @click="$emit('copy-amount')"
      >
        <Check v-if="amountCopied" aria-hidden="true" />
        <Copy v-else aria-hidden="true" />
        <span aria-live="polite">{{ amountCopied ? t('payment.copied') : t('payment.copyCryptoAmount') }}</span>
      </Button>
      <Button
        v-if="item.key === 'wallet_address'"
        type="button"
        variant="outline"
        size="sm"
        class="self-end border-foreground/25 bg-background font-semibold"
        :aria-label="t('payment.copyWalletAddress')"
        @click="$emit('copy-address')"
      >
        <Check v-if="addressCopied" aria-hidden="true" />
        <Copy v-else aria-hidden="true" />
        <span aria-live="polite">{{ addressCopied ? t('payment.copied') : t('payment.copyWalletAddress') }}</span>
      </Button>
    </div>
  </div>
</template>
