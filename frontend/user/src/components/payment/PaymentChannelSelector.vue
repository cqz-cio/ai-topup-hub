<template>
  <div v-if="props.channels.length > 0" class="grid grid-cols-1 md:grid-cols-2 gap-4">
    <button v-for="channel in props.channels" :key="channel.id"
      :disabled="isDisabled(channel)"
      :title="isDisabled(channel) ? channelHint(channel) : ''"
      @click="handleSelect(channel)"
      class="text-left border rounded-xl p-4 transition-colors disabled:cursor-not-allowed disabled:opacity-60"
      :class="props.modelValue === channel.id && !isDisabled(channel) ? 'border-primary/45 bg-primary/10' : 'bg-card hover:border-foreground/25'">
      <div class="flex items-center justify-between gap-2">
        <div class="flex items-center gap-2">
          <img v-if="channel.icon" :src="getImageUrl(channel.icon)" loading="lazy" class="h-5 w-5 rounded object-contain shrink-0" />
          <div class="text-foreground font-medium">{{ channel.name }}</div>
        </div>
        <Badge v-if="props.modelValue === channel.id && !isDisabled(channel)" variant="accent" size="xs">
          {{ t('payment.selected') }}
        </Badge>
      </div>
      <div v-if="channel.fee_policy === 'customer_surcharge'" class="mt-2 text-xs font-medium text-warning">
        {{ t('payment.feeLabel') }}：{{ customerFeeDescription(channel) }}
      </div>
      <div v-if="isDisabled(channel)" class="mt-2 text-xs text-warning">
        {{ channelHint(channel) }}
      </div>
    </button>
  </div>
  <div v-else-if="props.showBalanceOption" class="text-sm text-muted-foreground">
    {{ t('payment.channelEmptyUseBalance') }}
  </div>
  <div v-else class="text-sm text-muted-foreground">
    {{ t('payment.channelEmpty') }}
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { getImageUrl } from '../../utils/image'
import { Badge } from '@/components/ui/badge'

const emit = defineEmits<{
  'update:modelValue': [value: number]
}>()

const { t } = useI18n()

const props = defineProps<{
  channels: any[]
  modelValue: number | null
  showBalanceOption: boolean
  isChannelDisabledForAmount?: (channel?: any) => boolean
  channelAmountLimitHint?: (channel?: any) => string
}>()

const isDisabled = (channel?: any) => {
  if (!props.isChannelDisabledForAmount) return false
  return Boolean(props.isChannelDisabledForAmount(channel))
}

const channelHint = (channel?: any) => {
  if (!props.channelAmountLimitHint) return ''
  return String(props.channelAmountLimitHint(channel) || '')
}

const customerFeeDescription = (channel?: any) => {
  const parts: string[] = []
  const rate = Number(channel?.fee_rate || 0)
  const fixed = Number(channel?.fixed_fee || 0)
  if (rate > 0) parts.push(`${rate.toFixed(2)}%`)
  if (fixed > 0) parts.push(fixed.toFixed(2))
  return parts.join(' + ') || t('payment.feeFree')
}

const handleSelect = (channel?: any) => {
  if (!channel || isDisabled(channel)) return
  const id = Number(channel.id)
  if (!Number.isFinite(id) || id <= 0) return
  emit('update:modelValue', id)
}
</script>
