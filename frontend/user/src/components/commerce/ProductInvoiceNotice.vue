<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ReceiptText } from 'lucide-vue-next'

const { t } = useI18n()
</script>

<template>
  <p class="product-invoice-notice" role="note" data-product-invoice-notice>
    <ReceiptText aria-hidden="true" />
    <span><strong>{{ t('invoiceSupport.available') }}</strong>{{ t('invoiceSupport.afterPayment') }}</span>
  </p>
</template>

<style scoped>
.product-invoice-notice {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  margin: 16px 0 0;
  color: var(--ui-text-secondary);
  font-size: 14px;
  line-height: 22px;
  overflow-wrap: anywhere;
}
.product-invoice-notice svg { flex: none; width: 18px; height: 18px; margin-top: 2px; color: var(--ui-accent); }
.product-invoice-notice strong { color: var(--ui-accent); font-weight: 650; }
</style>
