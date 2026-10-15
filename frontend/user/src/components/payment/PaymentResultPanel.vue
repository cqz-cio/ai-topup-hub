<script setup lang="ts">
import { computed, reactive } from 'vue'
import { useI18n } from 'vue-i18n'
import { Box, Check, Clock3, Copy, ExternalLink, Headset, Info, RefreshCw, ArrowLeftRight } from 'lucide-vue-next'
import type { usePayment } from '../../composables/usePayment'
import { resolveUsdtTransfer } from '../../utils/usdtTransferNotice'
import UsdtPaymentGuides from './UsdtPaymentGuides.vue'
import UsdtTransferNotice from './UsdtTransferNotice.vue'
import CryptoPaymentDetails from './CryptoPaymentDetails.vue'
import PaymentAmountBreakdown from './PaymentAmountBreakdown.vue'

// Presentation only: polling, expiry, QR generation and copying stay in usePayment.
const props = defineProps<{ checkout: ReturnType<typeof usePayment> }>()
const p = reactive(props.checkout)
const { t, locale } = useI18n()
const transfer = computed(() => resolveUsdtTransfer(p.usdtPayment))
const address = computed(() => p.cryptoPaymentDetails.find(item => item.key === 'wallet_address')?.value || '')
const network = computed(() => p.cryptoPaymentDetails.find(item => item.key === 'chain')?.value || '')
const tokenDetail = computed(() => p.cryptoPaymentDetails.find(item => item.key === 'token')?.detail || '')
// A hosted gateway may supply only a payment URL; never invent a chain or amount.
const directTransfer = computed(() => Boolean(transfer.value?.amount && address.value && network.value))
const messages = {
  'zh-CN': { pay: '使用', payTail: '付款', networkHint: '请使用所选网络转账', orderAmount: '订单金额', amount: '应付数量', network: '所选网络', fees: '到账数量须完全一致，提现手续费另付。', details: '如何核对到账数量', waiting: '等待到账', auto: '到账后订单会自动更新，请勿重复转账。', breakdown: '查看金额明细', support: '联系客服', qr: '付款二维码', address: '收款地址', copyAddress: '复制收款地址', copyAmount: '复制金额' },
  'zh-TW': { pay: '使用', payTail: '付款', networkHint: '請使用所選網路轉帳', orderAmount: '訂單金額', amount: '應付數量', network: '所選網路', fees: '到帳數量須完全一致，提現手續費另付。', details: '如何核對到帳數量', waiting: '等待到帳', auto: '到帳後訂單會自動更新，請勿重複轉帳。', breakdown: '查看金額明細', support: '聯絡客服', qr: '付款 QR Code', address: '收款地址', copyAddress: '複製收款地址', copyAmount: '複製金額' },
  'en-US': { pay: 'Pay with', payTail: '', networkHint: 'Transfer using the selected network', orderAmount: 'Order total', amount: 'Amount to send', network: 'Network', fees: 'The received amount must match exactly. Pay withdrawal fees separately.', details: 'How to check the received amount', waiting: 'Awaiting payment', auto: 'Your order updates when payment arrives. Do not pay twice.', breakdown: 'View amount breakdown', support: 'Contact support', qr: 'Payment QR code', address: 'Recipient address', copyAddress: 'Copy address', copyAmount: 'Copy amount' },
} as const
const copy = computed(() => messages[locale.value as keyof typeof messages] || messages['zh-CN'])
</script>

<template>
  <section v-if="p.order && p.paymentResult" class="payment-result" aria-labelledby="payment-method-heading">
    <UsdtPaymentGuides :payment="p.usdtPayment" />
    <div class="payment-layout" :class="{ 'is-transfer': directTransfer }">
      <div class="payment-qr-column">
        <h2 id="payment-method-heading">{{ directTransfer ? `${copy.pay} ${transfer?.token} ${copy.payTail}` : p.paymentResultTitle }}</h2>
        <p class="payment-method-hint">{{ directTransfer ? copy.networkHint : p.paymentGuideTip }}</p>
        <template v-if="p.showQRCode">
          <div class="payment-qr-frame"><img :src="p.qrImageUrl" :alt="copy.qr" width="256" height="256" /></div>
          <p v-if="p.qrUsingPayLinkFallback" class="payment-caption">{{ t('payment.qrFallbackHint') }}</p>
        </template>
        <div v-if="!p.showQRCode || p.showPayLink" class="payment-link-actions">
          <button class="payment-button is-primary" @click="p.handleOpenPayLink"><ExternalLink :size="18" aria-hidden="true" />{{ t('payment.openPayLink') }}</button>
          <button class="payment-button" @click="p.handleCopyPayLink"><Copy :size="18" aria-hidden="true" />{{ p.copied ? t('payment.copied') : t('payment.copyPayLink') }}</button>
          <p v-if="p.openedPayWindow" class="payment-caption" role="status">{{ p.payLinkOpenedTip }}</p>
          <p v-if="p.showTelegramPayHint" class="payment-caption">{{ t('payment.telegramExternalHint') }}</p>
        </div>
        <div class="payment-waiting"><Clock3 :size="23" aria-hidden="true" /><span>{{ copy.waiting }}<template v-if="p.showCountdown"> · <span class="payment-timer">{{ p.countdownText }}</span></template></span></div>
        <p class="payment-caption">{{ copy.auto }}</p>
      </div>

      <div class="payment-amount-section">
        <div class="payment-label">{{ directTransfer ? copy.amount : t('payment.payableAmountLabel') }}</div>
        <div v-if="directTransfer" class="payment-crypto-amount" data-payment-exact-amount><strong>{{ transfer?.amount }}</strong><span>{{ transfer?.token }}</span></div>
        <div v-else class="payment-fiat-amount">{{ p.payableAmountDisplay }}</div>
        <div class="payment-order-total"><span>{{ copy.orderAmount }}</span><strong>{{ p.formatMoney(p.order.total_amount, p.order.currency) }}</strong></div>
        <div v-if="p.customerFeeApplied" class="payment-financial-row"><span>{{ t('payment.feeAmountLabel') }}</span><strong>{{ p.customerFeeAmountDisplay }}</strong></div>
        <template v-if="p.hasDiscountAmount(p.paymentResult.wallet_paid_amount)">
          <div class="payment-financial-row"><span>{{ t('payment.walletDeductLabel') }}</span><strong>{{ p.paymentWalletPaidDisplay }}</strong></div>
          <div class="payment-financial-row"><span>{{ t('payment.onlinePayLabel') }}</span><strong>{{ p.paymentOnlinePayDisplay }}</strong></div>
        </template>
      </div>

      <div class="payment-network-section">
        <template v-if="directTransfer">
          <div class="payment-network"><span class="payment-label">{{ copy.network }}</span><strong><Box :size="22" aria-hidden="true" />{{ network }}</strong></div>
          <p v-if="tokenDetail" class="payment-caption">{{ tokenDetail }}</p>
          <details class="payment-fee-details"><summary class="payment-fee-notice"><Info :size="19" aria-hidden="true" /><span>{{ copy.fees }}</span></summary><UsdtTransferNotice :payment="p.usdtPayment" /></details>
        </template>
        <template v-else>
          <div class="payment-network"><span class="payment-label">{{ t('payment.methodLabel') }}</span><strong>{{ p.resultChannelName }}</strong></div>
          <CryptoPaymentDetails v-if="p.hasCryptoPaymentDetails" :details="p.cryptoPaymentDetails" :usdt-payment="p.usdtPayment" :amount-copied="p.cryptoAmountCopied" :address-copied="p.walletAddressCopied" @copy-amount="p.handleCopyCryptoAmount" @copy-address="p.handleCopyWalletAddress" />
          <UsdtTransferNotice v-else :payment="p.usdtPayment" />
        </template>
      </div>

      <div v-if="directTransfer" class="payment-address-section">
        <div class="payment-label" id="payment-wallet-label">{{ copy.address }}</div>
        <div class="payment-address" aria-labelledby="payment-wallet-label"><span>{{ address }}</span><button :aria-label="copy.copyAddress" @click="p.handleCopyWalletAddress"><Check v-if="p.walletAddressCopied" :size="18" aria-hidden="true" /><Copy v-else :size="18" aria-hidden="true" /></button></div>
        <div class="payment-copy-actions" aria-live="polite">
          <button class="payment-button is-primary" @click="p.handleCopyWalletAddress"><Check v-if="p.walletAddressCopied" :size="18" aria-hidden="true" /><Copy v-else :size="18" aria-hidden="true" />{{ p.walletAddressCopied ? t('payment.copied') : copy.copyAddress }}</button>
          <button class="payment-button" @click="p.handleCopyCryptoAmount"><Check v-if="p.cryptoAmountCopied" :size="18" aria-hidden="true" /><Copy v-else :size="18" aria-hidden="true" />{{ p.cryptoAmountCopied ? t('payment.copied') : copy.copyAmount }}</button>
        </div>
      </div>

      <footer class="payment-meta-section">
        <div class="payment-order-meta"><span>{{ t('payment.orderNo') }}</span><strong>{{ p.order.order_no }}</strong><span class="payment-status">{{ p.statusLabel(p.order.status) }}</span></div>
        <div class="payment-meta-actions">
          <button :disabled="p.loading" @click="p.handleRefresh"><RefreshCw :size="17" aria-hidden="true" />{{ t('payment.refreshStatus') }}</button>
          <button @click="p.handleChangePaymentMethod"><ArrowLeftRight :size="17" aria-hidden="true" />{{ t('payment.changeMethod') }}</button>
          <RouterLink to="/about"><Headset :size="17" aria-hidden="true" />{{ copy.support }}</RouterLink>
        </div>
        <div class="payment-meta-bottom"><p v-if="p.paymentResult.expires_at" class="payment-caption">{{ t('payment.expiresAt') }}：{{ p.formatDate(p.paymentResult.expires_at) }}</p>
        <details class="payment-help-details payment-breakdown">
          <summary>{{ copy.breakdown }}</summary>
          <PaymentAmountBreakdown :order="p.order" :payment-result="p.paymentResult" :customer-fee-applied="p.customerFeeApplied" :customer-fee-amount-display="p.customerFeeAmountDisplay" :payable-amount-display="p.payableAmountDisplay" :wallet-paid-display="p.paymentWalletPaidDisplay" :online-pay-display="p.paymentOnlinePayDisplay" :show-countdown="false" :countdown-text="p.countdownText" :polling-active="false" :format-money="p.formatMoney" :format-discount-money="p.formatDiscountMoney" :has-discount-amount="p.hasDiscountAmount" />
        </details></div>
      </footer>
    </div>
  </section>
</template>

<style scoped>
.payment-result { color: var(--ui-text-primary); }
.payment-layout { display: grid; grid-template-columns: minmax(280px, 40%) minmax(0, 1fr); column-gap: 6.4%; margin-top: 28px; align-items: start; }
.payment-layout > * { min-width: 0; }
.payment-qr-column { grid-column: 1; grid-row: 1 / 5; align-self: stretch; border-right: 1px solid var(--ui-border); padding: 0 40px 24px 0; text-align: center; }
.payment-qr-column h2 { font-size: 27px; line-height: 1.3; font-weight: 750; letter-spacing: -.025em; }
.payment-method-hint { margin: 10px auto 0; font-size: 15px; line-height: 1.6; color: var(--ui-text-muted); max-width: 360px; }
.payment-qr-frame { width: 272px; max-width: 100%; padding: 8px; border-radius: 10px; background: white; margin: 18px auto 0; }
.payment-qr-frame img { width: 100%; height: auto; aspect-ratio: 1; display: block; object-fit: contain; }
.payment-waiting { display: flex; align-items: center; justify-content: center; gap: 8px; color: var(--ui-accent); font-size: 20px; font-weight: 650; line-height: 1.5; margin: 20px 0 12px; }
.payment-timer { font-variant-numeric: tabular-nums; }
.payment-caption { color: var(--ui-text-muted); font-size: 13px; line-height: 1.7; margin-top: 8px; overflow-wrap: anywhere; }
.payment-amount-section, .payment-network-section, .payment-address-section, .payment-meta-section { grid-column: 2; }
.payment-label { color: var(--ui-text-muted); font-size: 14px; line-height: 1.6; }
.payment-amount-section { grid-row: 1; padding-bottom: 18px; border-bottom: 1px solid var(--ui-border); }
.payment-crypto-amount { display: flex; flex-wrap: wrap; align-items: baseline; gap: 8px 16px; margin-top: 4px; }
.payment-crypto-amount strong, .payment-fiat-amount { font-size: clamp(36px, 4vw, 54px); font-weight: 750; letter-spacing: -.04em; line-height: 1.2; font-variant-numeric: tabular-nums; overflow-wrap: anywhere; }
.payment-crypto-amount span { font-size: 23px; font-weight: 600; color: var(--ui-text-secondary); }
.payment-order-total { display: flex; flex-wrap: wrap; align-items: baseline; gap: 8px 16px; margin-top: 10px; font-size: 15px; color: var(--ui-text-muted); }
.payment-order-total strong { font-size: 21px; color: var(--ui-text-primary); font-variant-numeric: tabular-nums; }
.payment-financial-row { display: flex; justify-content: space-between; flex-wrap: wrap; gap: 6px 16px; margin-top: 8px; font-size: 13px; }
.payment-network-section { grid-row: 2; padding-top: 18px; }
.payment-network { display: flex; align-items: center; flex-wrap: wrap; gap: 10px 16px; }
.payment-network strong { display: inline-flex; align-items: center; gap: 10px; padding: 10px 14px; border-radius: 8px; background: color-mix(in srgb, var(--ui-accent) 5%, var(--ui-bg-page)); font-size: 14px; font-weight: 600; overflow-wrap: anywhere; }
.payment-network svg { color: var(--ui-accent); flex-shrink: 0; }
.payment-fee-notice { display: flex; align-items: flex-start; gap: 9px; margin-top: 10px; padding: 11px 14px; border-radius: 8px; background: color-mix(in srgb, var(--ui-warning) 10%, var(--ui-bg-page)); color: var(--ui-warning); font-size: 13px; line-height: 1.6; }
.payment-fee-details summary { cursor: pointer; list-style: none; }
.payment-fee-details summary::-webkit-details-marker { display: none; }
.payment-fee-details summary:hover span { text-decoration: underline; }
.payment-fee-notice svg { flex-shrink: 0; margin-top: 1px; }
.payment-address-section { grid-row: 3; padding-top: 12px; }
.payment-address { display: flex; align-items: center; gap: 12px; border: 1px solid var(--ui-border-strong, var(--ui-border)); border-radius: 8px; padding: 6px 6px 6px 16px; margin-top: 5px; background: var(--ui-bg-elevated); }
.payment-address > span { min-width: 0; flex: 1; overflow-wrap: anywhere; font-size: 14px; line-height: 1.7; font-variant-numeric: tabular-nums; user-select: all; }
.payment-address button { display: flex; align-items: center; justify-content: center; flex-shrink: 0; width: 40px; min-height: 40px; border: 1px solid var(--ui-border); border-radius: 6px; background: var(--ui-bg-page); }
.payment-copy-actions { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; margin-top: 10px; }
.payment-button { display: inline-flex; justify-content: center; align-items: center; gap: 10px; min-height: 52px; padding: 10px 16px; border: 1px solid var(--ui-border-strong, var(--ui-border)); border-radius: 8px; font-size: 14px; line-height: 1.5; font-weight: 650; background: var(--ui-bg-elevated); color: var(--ui-text-primary); }
.payment-button svg { flex-shrink: 0; }
.payment-button.is-primary { background: var(--ui-accent); border-color: var(--ui-accent); color: var(--ui-text-on-accent); }
.payment-button:hover, .payment-address button:hover { filter: brightness(.94); }
.payment-link-actions { display: grid; gap: 10px; margin-top: 20px; }
.payment-meta-section { grid-row: 4; border-top: 1px solid var(--ui-border); margin-top: 18px; padding-top: 14px; }
.payment-order-meta { display: flex; flex-wrap: wrap; align-items: baseline; gap: 6px 12px; font-size: 13px; }
.payment-order-meta > span:first-child { color: var(--ui-text-muted); }
.payment-order-meta strong { overflow-wrap: anywhere; }
.payment-status { color: var(--ui-accent); font-weight: 650; }
.payment-meta-actions { display: flex; flex-wrap: wrap; gap: 6px 24px; margin-top: 6px; }
.payment-meta-actions button, .payment-meta-actions a { display: inline-flex; align-items: center; gap: 8px; min-height: 40px; font-size: 13px; }
.payment-meta-actions button:hover, .payment-meta-actions a:hover { color: var(--ui-accent); text-decoration: underline; }
.payment-meta-bottom { display: flex; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: 0 16px; }
.payment-meta-bottom:has(details[open]) { display: block; }
.payment-help-details { margin-top: 10px; color: var(--ui-text-muted); font-size: 12px; line-height: 1.7; }
.payment-help-details summary { width: fit-content; cursor: pointer; padding: 5px 0; }
.payment-breakdown :deep(> div) { margin-top: 8px; }
.payment-result button:focus-visible, .payment-result a:focus-visible, .payment-result summary:focus-visible { outline: 2px solid var(--ui-accent); outline-offset: 3px; }
.payment-result button:disabled { opacity: .5; cursor: wait; }
@media (max-width: 960px) {
  .payment-layout { grid-template-columns: minmax(220px, 38%) minmax(0, 1fr); column-gap: 28px; }
  .payment-qr-column { padding-right: 28px; }
  .payment-crypto-amount strong, .payment-fiat-amount { font-size: 38px; }
  .payment-copy-actions { gap: 8px; }
  .payment-button { gap: 6px; padding: 10px; }
}
@media (max-width: 640px) {
  .payment-layout { grid-template-columns: minmax(0, 1fr); margin-top: 16px; }
  .payment-layout > * { grid-column: 1; }
  .payment-amount-section { grid-row: 1; padding-bottom: 10px; }
  .payment-crypto-amount { gap: 6px 12px; }
  .payment-crypto-amount strong, .payment-fiat-amount { font-size: clamp(32px, 9vw, 40px); }
  .payment-crypto-amount span { font-size: 18px; }
  .payment-order-total { font-size: 13px; margin-top: 6px; gap: 10px; }
  .payment-order-total strong { font-size: 17px; }
  .payment-label { font-size: 12px; }
  .payment-network-section { grid-row: 2; padding-top: 8px; }
  .payment-network { gap: 8px; }
  .payment-network strong { padding: 7px 10px; font-size: 12px; gap: 7px; }
  .payment-network svg { width: 18px; height: 18px; }
  .payment-fee-notice { font-size: 12px; padding: 8px 10px; margin-top: 8px; }
  .payment-qr-column { grid-row: 3; border: 0; padding: 14px 0 8px; }
  .payment-qr-column h2 { font-size: 19px; }
  .payment-method-hint { font-size: 12px; margin-top: 5px; }
  .is-transfer .payment-qr-column h2, .is-transfer .payment-method-hint { display: none; }
  .payment-qr-frame { width: 192px; margin-top: 10px; }
  .is-transfer .payment-qr-frame { margin-top: 0; }
  .payment-waiting { font-size: 16px; margin: 12px 0 4px; }
  .payment-waiting svg { width: 19px; height: 19px; }
  .payment-caption { font-size: 12px; margin-top: 5px; }
  .payment-address-section { grid-row: 4; padding-top: 12px; }
  .payment-address { padding-left: 12px; gap: 8px; }
  .payment-address > span { font-size: 13px; }
  .payment-copy-actions { gap: 8px; }
  .payment-button { min-height: 48px; font-size: 13px; }
  .payment-meta-section { grid-row: 5; margin-top: 16px; }
  .payment-meta-actions { gap: 4px 20px; }
  .payment-meta-actions button, .payment-meta-actions a { min-height: 44px; font-size: 12px; gap: 6px; }
}
</style>
