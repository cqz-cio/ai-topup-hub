<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { productPurchaseGuide } from '../../utils/productPurchaseGuide'
import { purchaseClarityCopy } from '../../i18n/purchaseClarity'

const props = defineProps<{ slug: string }>()
const { locale } = useI18n()
const copy = computed(() => purchaseClarityCopy[locale.value] || purchaseClarityCopy['zh-CN']!)
const guide = computed(() => productPurchaseGuide(props.slug))
</script>

<template>
  <div v-if="guide" data-product-redemption class="text-sm leading-relaxed">
    <p class="text-muted-foreground">{{ guide.rules.includes('credits') ? copy.creditsDelivery : guide.kind === 'code' ? copy.codeDelivery : copy.linkDelivery }}</p>
  </div>
</template>
