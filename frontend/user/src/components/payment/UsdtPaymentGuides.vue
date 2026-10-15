<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ExternalLink } from 'lucide-vue-next'
import { resolveUsdtTransfer, type UsdtPaymentSource } from '../../utils/usdtTransferNotice'

const props = defineProps<{ payment?: UsdtPaymentSource | null }>()
const transfer = computed(() => resolveUsdtTransfer(props.payment))
const { locale } = useI18n()
const messages = {
  'zh-CN': {
    title: '不会用 U？先看付款教程',
    binance: '币安教程',
    okx: '欧易教程',
    hint: '教程在新标签页打开，当前付款页会保留。',
  },
  'zh-TW': {
    title: '不會用 U？先看付款教學',
    binance: '幣安教學',
    okx: '歐易教學',
    hint: '教學在新分頁開啟，目前付款頁會保留。',
  },
  'en-US': {
    title: 'New to USDT? View a payment guide',
    binance: 'Binance guide',
    okx: 'OKX guide',
    hint: 'Guides open in a new tab, keeping this payment page open.',
  },
} as const
const copy = computed(() => messages[locale.value as keyof typeof messages] || messages['zh-CN'])
</script>

<template>
  <aside v-if="transfer" class="usdt-payment-guides" role="note" data-usdt-payment-guides>
    <strong>{{ copy.title }}</strong>
    <div class="usdt-payment-guide-links">
      <a href="/blog/binance-usdt-payment-guide" target="_blank" rel="noopener noreferrer">
        {{ copy.binance }}
        <ExternalLink aria-hidden="true" />
      </a>
      <a href="/blog/okx-usdt-payment-guide" target="_blank" rel="noopener noreferrer">
        {{ copy.okx }}
        <ExternalLink aria-hidden="true" />
      </a>
    </div>
    <p>{{ copy.hint }}</p>
  </aside>
</template>

<style scoped>
.usdt-payment-guides {
  box-sizing: border-box;
  width: 100%;
  padding: 14px;
  border: 1px solid color-mix(in srgb, var(--ui-accent) 28%, transparent);
  border-radius: 10px;
  background: color-mix(in srgb, var(--ui-accent) 6%, var(--ui-bg-page));
  color: var(--ui-text-primary);
  text-align: left;
  font-size: 14px;
  line-height: 1.7;
  overflow-wrap: anywhere;
}
.usdt-payment-guides strong { font-weight: 650; }
.usdt-payment-guides p { margin: 8px 0 0; font-size: 12px; color: var(--ui-text-muted); }
.usdt-payment-guide-links { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 8px; }
.usdt-payment-guide-links a {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-height: 44px;
  max-width: 100%;
  padding: 8px 12px;
  border: 1px solid var(--ui-border);
  border-radius: 8px;
  background: var(--ui-bg-page);
  color: var(--ui-text-primary);
  font-weight: 600;
  text-decoration: none;
}
.usdt-payment-guide-links a:hover { text-decoration: underline; }
.usdt-payment-guide-links a:focus-visible { outline: 2px solid currentColor; outline-offset: 3px; }
.usdt-payment-guide-links svg { width: 14px; height: 14px; flex-shrink: 0; }
</style>
