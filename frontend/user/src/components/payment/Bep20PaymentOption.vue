<template>
  <div class="min-w-0">
    <button
      type="button"
      class="bep20-payment-option"
      :class="{ 'is-selected': props.selected && !props.disabled }"
      :disabled="props.disabled"
      :title="props.disabled && props.disabledHint ? `${props.fullName}\n${props.disabledHint}` : props.fullName"
      :aria-label="props.fullName"
      :aria-pressed="props.selected && !props.disabled"
      @click="emit('select')"
    >
      <img :src="bepusdtIcon" alt="" aria-hidden="true" width="22" height="22" class="bep20-payment-icon" />
      <span>U-Bep20</span>
    </button>
    <div v-if="$slots.fees" class="mt-1 space-y-0.5 text-xs text-warning">
      <slot name="fees" />
    </div>
    <div v-if="props.disabled && props.disabledHint" class="mt-1 text-xs text-warning">
      {{ props.disabledHint }}
    </div>
  </div>
</template>

<script setup lang="ts">
// Official icon: https://github.com/v03413/BEpusdt/blob/main/static/checkout/official/assets/img/bepusdt.svg
import bepusdtIcon from '../../assets/payment/bepusdt.svg'

const props = defineProps<{
  fullName: string
  selected: boolean
  disabled?: boolean
  disabledHint?: string
}>()

const emit = defineEmits<{
  select: []
}>()
</script>

<style scoped>
.bep20-payment-option {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  height: 36px;
  min-width: 110px;
  max-width: 100%;
  padding: 0 13px;
  border: 1px solid var(--ui-border-strong);
  border-radius: 999px;
  background: var(--ui-bg-elevated);
  color: #64748b;
  font-size: 14px;
  font-weight: 650;
  line-height: 1;
  white-space: nowrap;
  box-shadow: 0 1px 4px rgba(20, 40, 60, 0.07);
  cursor: pointer;
  transition: border-color 150ms, background-color 150ms, box-shadow 150ms;
}

.bep20-payment-option.is-selected {
  border-color: #a6c8ee;
  background: #f8fcff;
}

.bep20-payment-option:hover:not(:disabled) {
  border-color: var(--ui-accent);
}

.bep20-payment-option:focus-visible {
  outline: 2px solid var(--ui-accent);
  outline-offset: 3px;
}

.bep20-payment-option:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.bep20-payment-icon {
  width: 22px;
  height: 22px;
  flex: none;
}

.dark .bep20-payment-option {
  color: var(--ui-text-secondary);
}

.dark .bep20-payment-option.is-selected {
  border-color: #476c98;
  background: #182638;
}

.dark .bep20-payment-icon {
  filter: invert(1);
}

@media (prefers-reduced-motion: reduce) {
  .bep20-payment-option {
    transition: none;
  }
}
</style>
