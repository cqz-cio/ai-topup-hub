<script setup lang="ts">
import SupportContacts from '../SupportContacts.vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import ProductRedemptionGuide from './ProductRedemptionGuide.vue'
import { productPurchaseGuide } from '../../utils/productPurchaseGuide'
import { purchaseClarityCopy } from '../../i18n/purchaseClarity'

const props = defineProps<{ slug: string; hideRedemption?: boolean }>()
const { locale } = useI18n()
const copy = computed(() => purchaseClarityCopy[locale.value] || purchaseClarityCopy['zh-CN']!)
const guide = computed(() => productPurchaseGuide(props.slug))
</script>

<template>
  <section v-if="guide" class="mb-6 rounded-xl border bg-secondary p-4 text-sm leading-relaxed" data-purchase-guide :aria-label="copy.title">
    <h2 class="mb-2 font-bold text-foreground">{{ copy.title }}</h2>
    <ul class="list-disc space-y-2 pl-4 text-foreground">
      <li v-for="rule in guide.rules" :key="rule">{{ copy.rules[rule] }}</li>
    </ul>
    <p class="mt-3 text-muted-foreground">{{ copy.help }}</p>
    <SupportContacts />
    <ProductRedemptionGuide v-if="!hideRedemption" :slug="slug" class="mt-3 border-t pt-3" />
  </section>
</template>
