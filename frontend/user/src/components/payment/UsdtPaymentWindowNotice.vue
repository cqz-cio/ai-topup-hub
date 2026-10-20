<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { usdtPaymentWindowMessage } from '../../utils/usdtPaymentWindow'
defineProps<{ expired?: boolean }>()
const { locale } = useI18n()
</script>

<template>
  <p class="my-3 rounded-lg border border-warning/30 bg-warning/10 p-3 text-sm leading-relaxed text-foreground" role="note" data-usdt-payment-window>
    {{ usdtPaymentWindowMessage(locale, expired) }}
  </p>
</template>
