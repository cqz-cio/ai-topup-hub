<template>
  <label
    class="payment-channel-option"
    :class="{ 'is-selected': selected && !disabled, 'is-disabled': disabled }"
    :title="disabled && disabledHint ? `${channel.name}\n${disabledHint}` : channel.name"
  >
    <img v-if="network" :src="usdtIcon" alt="" aria-hidden="true" class="payment-channel-icon" width="36" height="36" />
    <img v-else-if="channel.icon" :src="getImageUrl(channel.icon)" alt="" aria-hidden="true" class="payment-channel-icon" width="36" height="36" />
    <CreditCard v-else class="payment-channel-icon text-muted-foreground" aria-hidden="true" />
    <span class="payment-channel-content">
      <span class="payment-channel-title">{{ network?.title || channel.name }}</span>
      <span v-if="network" class="payment-channel-network">{{ network.subtitle }}</span>
      <span v-if="$slots.fees" class="payment-channel-note"><slot name="fees" /></span>
      <span v-if="disabled && disabledHint" class="payment-channel-note">{{ disabledHint }}</span>
    </span>
    <input
      type="radio"
      class="payment-channel-radio"
      :name="name"
      :value="channel.id"
      :checked="selected && !disabled"
      :disabled="disabled"
      @change="emit('select')"
    />
  </label>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { CreditCard } from 'lucide-vue-next'
import usdtIcon from '../../assets/payment/usdt.svg'
import { getImageUrl } from '../../utils/image'
import { isBep20PaymentChannel } from '../../utils/paymentChannelPresentation'

const props = defineProps<{
  channel: { id: number; name: string; icon?: string; provider_type?: string; channel_type?: string }
  name: string
  selected: boolean
  disabled?: boolean
  disabledHint?: string
}>()

const emit = defineEmits<{ select: [] }>()
const { t } = useI18n()
const network = computed(() => {
  if (isBep20PaymentChannel(props.channel)) return { title: 'USDT · BEP20', subtitle: 'BNB Smart Chain' }
  if (props.channel.provider_type?.toLowerCase() === 'plasmausdt'
    && props.channel.channel_type?.toLowerCase() === 'usdt-plasma') {
    return { title: 'USDT · Plasma', subtitle: t('payment.plasmaNetwork') }
  }
  return null
})
</script>

<style scoped>
.payment-channel-option {
  display: flex;
  align-items: center;
  gap: 14px;
  width: 100%;
  min-width: 0;
  min-height: 70px;
  padding: 12px 14px;
  border: 1px solid var(--ui-border-strong);
  border-radius: 10px;
  background: var(--ui-bg-elevated);
  cursor: pointer;
  transition: border-color 150ms, background-color 150ms;
}

.payment-channel-option.is-selected {
  border-color: var(--ui-accent);
  background: var(--ui-accent-soft);
}

.payment-channel-option:hover:not(.is-disabled) {
  border-color: var(--ui-accent);
}

.payment-channel-option:has(:focus-visible) {
  outline: 2px solid var(--ui-accent);
  outline-offset: 3px;
}

.payment-channel-option.is-disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.payment-channel-icon {
  flex: none;
  width: 36px;
  height: 36px;
  object-fit: contain;
}

.payment-channel-content {
  flex: 1;
  min-width: 0;
  text-align: left;
  overflow-wrap: anywhere;
}

.payment-channel-title {
  display: block;
  color: var(--ui-text-primary);
  font-size: 15px;
  font-weight: 650;
  line-height: 22px;
}

.is-selected .payment-channel-title {
  color: var(--ui-accent);
}

.payment-channel-network {
  display: block;
  margin-top: 2px;
  color: var(--ui-text-muted);
  font-size: 12px;
  line-height: 18px;
}

.payment-channel-note {
  display: block;
  margin-top: 4px;
  color: var(--ui-warning);
  font-size: 12px;
  line-height: 18px;
}

.payment-channel-radio {
  flex: none;
  width: 20px;
  height: 20px;
  margin: 0;
  accent-color: var(--ui-accent);
  cursor: inherit;
}

.payment-channel-radio:focus-visible {
  /* The entire option shows the keyboard focus ring. */
  outline: none;
}

.dark .payment-channel-radio {
  color-scheme: dark;
}

@media (prefers-reduced-motion: reduce) {
  .payment-channel-option { transition: none; }
}
</style>
