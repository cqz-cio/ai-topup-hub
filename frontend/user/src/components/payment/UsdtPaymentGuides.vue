<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { BookOpen, ExternalLink } from 'lucide-vue-next'
import { resolveUsdtTransfer, type UsdtPaymentSource } from '../../utils/usdtTransferNotice'

const props = defineProps<{ payment?: UsdtPaymentSource | null }>()
const transfer = computed(() => resolveUsdtTransfer(props.payment))
const { locale } = useI18n()
const messages = {
  'zh-CN': {
    title: '第一次用 U？',
    description: '先看付款教程，轻松完成转账。',
    binance: '币安教程',
    okx: '欧易教程',
    hint: '教程在新标签页打开，当前付款页会保留。',
  },
  'zh-TW': {
    title: '第一次用 U？',
    description: '先看付款教學，輕鬆完成轉帳。',
    binance: '幣安教學',
    okx: '歐易教學',
    hint: '教學在新分頁開啟，目前付款頁會保留。',
  },
  'en-US': {
    title: 'New to USDT?',
    description: 'Start with a payment guide.',
    binance: 'Binance guide',
    okx: 'OKX guide',
    hint: 'Guides open in a new tab, keeping this payment page open.',
  },
} as const
const copy = computed(() => messages[locale.value as keyof typeof messages] || messages['zh-CN'])
</script>

<template>
  <aside v-if="transfer" class="usdt-payment-guides" role="note" data-usdt-payment-guides>
    <div class="usdt-payment-guide-intro"><BookOpen aria-hidden="true" /><div><strong>{{ copy.title }}</strong><span>{{ copy.description }}</span></div></div>
    <div class="usdt-payment-guide-links" :aria-label="copy.hint">
      <a href="/blog/binance-usdt-payment-guide" target="_blank" rel="noopener noreferrer">
        {{ copy.binance }}
        <ExternalLink aria-hidden="true" />
      </a>
      <a href="/blog/okx-usdt-payment-guide" target="_blank" rel="noopener noreferrer">
        {{ copy.okx }}
        <ExternalLink aria-hidden="true" />
      </a>
    </div>
  </aside>
</template>

<style scoped>
.usdt-payment-guides {
  box-sizing: border-box;
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 22px;
  border-radius: 10px;
  background: color-mix(in srgb, var(--ui-accent) 6%, var(--ui-bg-page));
  color: var(--ui-text-primary);
  text-align: left;
  font-size: 14px;
  line-height: 1.7;
  overflow-wrap: anywhere;
}
.usdt-payment-guide-intro { display: flex; align-items: center; gap: 18px; }
.usdt-payment-guide-intro > svg { width: 25px; height: 25px; flex-shrink: 0; color: var(--ui-accent); }
.usdt-payment-guides strong { font-weight: 700; font-size: 17px; margin-right: 8px; }
.usdt-payment-guide-intro span { color: var(--ui-text-muted); }
.usdt-payment-guide-links { display: flex; gap: 14px; flex-shrink: 0; }
.usdt-payment-guide-links a {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-height: 48px;
  max-width: 100%;
  padding: 8px 24px;
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
@media (max-width: 960px) {
  .usdt-payment-guide-intro span { display: block; font-size: 12px; }
  .usdt-payment-guides strong { font-size: 15px; }
  .usdt-payment-guide-links a { padding-inline: 16px; }
}
@media (max-width: 640px) {
  .usdt-payment-guides { align-items: stretch; flex-direction: column; padding: 12px 14px; gap: 10px; }
  .usdt-payment-guide-intro { gap: 10px; }
  .usdt-payment-guide-intro > svg { width: 20px; height: 20px; }
  .usdt-payment-guide-intro span { display: none; }
  .usdt-payment-guides strong { font-size: 14px; }
  .usdt-payment-guide-links { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
  .usdt-payment-guide-links a { min-height: 44px; font-size: 13px; padding: 7px 8px; }
}
</style>
