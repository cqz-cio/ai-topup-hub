<script setup lang="ts">
import { ArrowUpNarrowWide, ArrowDownWideNarrow, Check, ChevronDown } from 'lucide-vue-next'
import {
  SelectRoot, SelectTrigger, SelectValue, SelectIcon, SelectPortal,
  SelectContent, SelectViewport, SelectItem, SelectItemText, SelectItemIndicator,
} from 'reka-ui'
import { useI18n } from 'vue-i18n'
import type { StorefrontPriceOrder } from '../../utils/storefront'

defineProps<{ modelValue: StorefrontPriceOrder; category: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: StorefrontPriceOrder] }>()
const { t } = useI18n()
const options = [
  { value: 'asc', label: 'shopUi.priceLowToHigh', icon: ArrowUpNarrowWide },
  { value: 'desc', label: 'shopUi.priceHighToLow', icon: ArrowDownWideNarrow },
] as const
</script>

<template>
  <div class="ocean-price-sort">
    <span class="ocean-price-sort-label">{{ t('shopUi.priceSort') }}</span>
    <SelectRoot :model-value="modelValue" @update:model-value="emit('update:modelValue', $event === 'desc' ? 'desc' : 'asc')">
      <SelectTrigger class="ocean-sort-trigger" :aria-label="t('shopUi.categoryPriceSort', { category })">
        <component :is="modelValue === 'desc' ? ArrowDownWideNarrow : ArrowUpNarrowWide" :size="18" aria-hidden="true" />
        <SelectValue class="ocean-sort-value" />
        <SelectIcon as-child><ChevronDown class="ocean-sort-chevron" :size="16" aria-hidden="true" /></SelectIcon>
      </SelectTrigger>
      <SelectPortal>
        <SelectContent class="ocean-sort-menu" position="popper" align="end" :side-offset="6" :collision-padding="12">
          <SelectViewport class="ocean-sort-viewport">
            <SelectItem v-for="option in options" :key="option.value" :value="option.value" class="ocean-sort-option">
              <component :is="option.icon" :size="18" aria-hidden="true" />
              <SelectItemText>{{ t(option.label) }}</SelectItemText>
              <SelectItemIndicator class="ocean-sort-check"><Check :size="17" aria-hidden="true" /></SelectItemIndicator>
            </SelectItem>
          </SelectViewport>
        </SelectContent>
      </SelectPortal>
    </SelectRoot>
  </div>
</template>

<style>
.ocean-price-sort { display: flex; align-items: center; gap: 12px; max-width: 100%; color: var(--ui-text-primary); font-size: 13px; font-weight: 600; }
.ocean-price-sort-label { flex-shrink: 0; }
.ocean-sort-trigger, .ocean-sort-menu { color: #103f42; border: 1px solid rgb(255 255 255 / 92%); border-radius: 12px; background-color: rgb(166 231 247 / 66%); box-shadow: inset 0 0 0 1px rgb(255 255 255 / 38%), 0 3px 10px rgb(16 100 130 / 13%); backdrop-filter: blur(12px); }
.ocean-sort-trigger { position: relative; isolation: isolate; display: flex; align-items: center; gap: 10px; min-width: 190px; max-width: 100%; min-height: 44px; padding: 9px 14px; font: inherit; font-size: 14px; font-weight: 650; cursor: pointer; text-align: start; }
/* Reuse the approved wave/bubble/shell artwork, framing its transparent margins. */
.ocean-sort-trigger::before, .ocean-sort-menu::before { content: ''; pointer-events: none; position: absolute; z-index: -1; inset: 0; border-radius: inherit; background: url('/brand/ocean-buttons/add-cart-20261022.png') 45.45% 53.03% / 101.02% 157.39% no-repeat; opacity: .62; }
.ocean-sort-trigger > svg { flex-shrink: 0; }
.ocean-sort-value { flex: 1; white-space: nowrap; }
.ocean-sort-chevron { margin-left: auto; transition: transform .15s ease; }
.ocean-sort-trigger[data-state=open] .ocean-sort-chevron { transform: rotate(180deg); }
.ocean-sort-trigger:hover { background-color: rgb(198 244 251 / 80%); }
.ocean-sort-trigger:focus-visible { outline: 2px solid #14768a; outline-offset: 3px; }
.ocean-sort-menu { position: relative; isolation: isolate; z-index: 80; width: var(--reka-select-trigger-width); min-width: 190px; max-width: calc(100vw - 24px); max-height: var(--reka-select-content-available-height); overflow: hidden; background-color: rgb(164 229 244 / 90%); font-family: inherit; font-size: 14px; font-weight: 650; }
.ocean-sort-menu::before { top: auto; height: 28px; opacity: .55; background-size: 101.02% auto; background-position: 45.45% calc(100% + 11px); }
.ocean-sort-viewport { padding: 4px 4px 10px; }
.ocean-sort-option { display: flex; align-items: center; gap: 10px; position: relative; min-height: 44px; padding: 9px 9px; border: 1px solid transparent; border-radius: 9px; cursor: pointer; user-select: none; outline: none; }
.ocean-sort-option + .ocean-sort-option { margin-top: 2px; }
.ocean-sort-option > svg { flex-shrink: 0; }
.ocean-sort-option[data-state=checked] { background-color: rgb(236 253 255 / 43%); border-color: rgb(255 255 255 / 90%); }
.ocean-sort-option[data-highlighted] { background-color: rgb(238 254 255 / 65%); box-shadow: inset 0 0 0 1px rgb(27 112 132 / 28%); }
.ocean-sort-check { display: flex; margin-left: auto; }
.dark .ocean-sort-trigger, .dark .ocean-sort-menu { color: #e5fbff; background-color: rgb(15 62 79 / 94%); border-color: rgb(183 235 250 / 60%); }
.dark .ocean-sort-trigger::before, .dark .ocean-sort-menu::before { opacity: .18; }
.dark .ocean-sort-option[data-state=checked] { background-color: rgb(112 211 236 / 16%); border-color: rgb(183 235 250 / 58%); }
.dark .ocean-sort-option[data-highlighted] { background-color: rgb(112 211 236 / 28%); }
.dark .ocean-sort-trigger:focus-visible { outline-color: #a7edff; }
@media (max-width: 767px) {
  .ocean-price-sort { width: 100%; gap: 10px; }
  .ocean-sort-trigger { flex: 1; min-width: 0; gap: 8px; padding-inline: 10px; }
  .ocean-sort-menu { min-width: 0; }
}
@media (prefers-reduced-motion: reduce) { .ocean-sort-chevron { transition: none; } }
</style>
