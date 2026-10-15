<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { TriangleAlert } from 'lucide-vue-next'
import { resolveUsdtTransfer, type UsdtPaymentSource } from '../../utils/usdtTransferNotice'

const props = defineProps<{ payment?: UsdtPaymentSource | null }>()
const transfer = computed(() => resolveUsdtTransfer(props.payment))
const { locale } = useI18n()
// Plain Vue text keeps these fixed messages compatible with the runtime-only i18n build.
const messages = {
    'zh-CN': {
      title: '手续费另付，到账须一致',
      amountLead: '我方钱包须实际到账',
      amountTail: '，与本页应付数量完全一致（含小数尾数）。',
      exactAmount: '我方钱包实际到账须与收银页应付 U 数量完全一致（含小数尾数）。',
      fees: '转账 / 提现手续费由您另行承担。若平台从转账金额中扣费，请加上手续费，确保扣费后到账金额不多不少。',
      check: '提交转账前，请核对平台显示的“实际到账”数量。',
    },
    'zh-TW': {
      title: '手續費另付，到帳須一致',
      amountLead: '我方錢包須實際到帳',
      amountTail: '，與本頁應付數量完全一致（含小數尾數）。',
      exactAmount: '我方錢包實際到帳須與收銀頁應付 U 數量完全一致（含小數尾數）。',
      fees: '轉帳 / 提現手續費由您另行承擔。若平台從轉帳金額中扣費，請加上手續費，確保扣費後到帳金額不多不少。',
      check: '提交轉帳前，請核對平台顯示的「實際到帳」數量。',
    },
    'en-US': {
      title: 'Pay fees separately; match the exact amount',
      amountLead: 'Our wallet must receive exactly',
      amountTail: ', matching the amount due on this page, including every decimal digit.',
      exactAmount: 'Our wallet must receive exactly the USDT amount shown at checkout, including every decimal digit.',
      fees: 'You are responsible for transfer / withdrawal fees. If your platform deducts the fee from the transfer amount, add enough to cover it so the amount received is neither more nor less than the amount due.',
      check: 'Before confirming the transfer, check the amount your platform says the recipient will receive.',
    },
} as const
const copy = computed(() => messages[locale.value as keyof typeof messages] || messages['zh-CN'])
</script>

<template>
  <aside v-if="transfer" class="usdt-transfer-notice" role="note" data-usdt-transfer-notice>
    <div class="usdt-transfer-heading">
      <TriangleAlert aria-hidden="true" />
      <strong>{{ copy.title }}</strong>
    </div>
    <p v-if="transfer.amount">
      {{ copy.amountLead }} <strong>{{ transfer.amount }} {{ transfer.token }}</strong>{{ copy.amountTail }}
    </p>
    <p v-else>{{ copy.exactAmount }}</p>
    <p>{{ copy.fees }}</p>
    <p class="usdt-transfer-check">{{ copy.check }}</p>
  </aside>
</template>

<style scoped>
.usdt-transfer-notice {
  box-sizing: border-box;
  border: 1px solid color-mix(in srgb, var(--ui-warning) 38%, transparent);
  background: color-mix(in srgb, var(--ui-warning) 8%, var(--ui-bg-page));
  border-radius: 10px;
  padding: 14px;
  margin-block: 12px;
  text-align: left;
  color: var(--ui-text-primary);
  font-size: 14px;
  line-height: 1.7;
  overflow-wrap: anywhere;
}
.usdt-transfer-heading {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  color: var(--ui-warning);
  line-height: 22px;
}
.usdt-transfer-heading svg {
  width: 18px;
  height: 18px;
  flex-shrink: 0;
  margin-top: 2px;
}
.usdt-transfer-notice strong { font-weight: 650; }
.usdt-transfer-notice p { margin: 8px 0 0; }
.usdt-transfer-check { font-size: 12px; color: var(--ui-text-muted); }
</style>
