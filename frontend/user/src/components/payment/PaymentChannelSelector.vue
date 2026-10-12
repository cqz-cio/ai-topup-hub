<template>
  <div v-if="props.channels.length > 0" class="grid grid-cols-1 gap-2.5" role="group" :aria-label="t('checkout.paymentMethod')">
    <PaymentChannelOption
      v-for="channel in props.channels"
      :key="channel.id"
      :channel="channel"
      :name="groupName"
      :selected="props.modelValue === channel.id"
      :disabled="isDisabled(channel)"
      :disabled-hint="channelHint(channel)"
      @select="handleSelect(channel)"
    >
      <template v-if="channel.fee_policy === 'customer_surcharge'" #fees>
        <span>{{ t('payment.feeLabel') }}：{{ customerFeeDescription(channel) }}</span>
      </template>
    </PaymentChannelOption>
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
import { useId } from 'vue'
import PaymentChannelOption from './PaymentChannelOption.vue'

const emit = defineEmits<{
  'update:modelValue': [value: number]
}>()

const { t } = useI18n()
const groupName = `payment-channel-${useId()}`

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
