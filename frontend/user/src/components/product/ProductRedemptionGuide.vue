<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ExternalLink } from 'lucide-vue-next'
import { productPurchaseGuide } from '../../utils/productPurchaseGuide'
import { purchaseClarityCopy } from '../../i18n/purchaseClarity'

const props = defineProps<{ slug: string }>()
const { locale } = useI18n()
const copy = computed(() => purchaseClarityCopy[locale.value] || purchaseClarityCopy['zh-CN']!)
const guide = computed(() => productPurchaseGuide(props.slug))
</script>

<template>
  <div v-if="guide" data-product-redemption class="text-sm leading-relaxed">
    <a v-if="guide.redemptionUrl" :href="guide.redemptionUrl" target="_blank" rel="noopener noreferrer" class="inline-flex min-h-11 items-center gap-2 font-semibold text-primary underline underline-offset-4">
      {{ copy.redeem }} <ExternalLink class="h-4 w-4 shrink-0" aria-hidden="true" />
    </a>
    <p class="text-muted-foreground">{{ guide.rules.includes('credits') ? copy.creditsDelivery : guide.kind === 'code' ? copy.codeDelivery : copy.linkDelivery }}</p>
  </div>
</template>
