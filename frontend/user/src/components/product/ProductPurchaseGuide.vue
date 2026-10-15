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
  <section v-if="guide" class="mb-6 rounded-xl border bg-secondary p-4 text-sm leading-relaxed" data-purchase-guide :aria-label="copy.title">
    <h2 class="mb-2 font-bold text-foreground">{{ copy.title }}</h2>
    <ul class="list-disc space-y-2 pl-4 text-foreground">
      <li v-for="rule in guide.rules" :key="rule">{{ copy.rules[rule] }}</li>
    </ul>
    <p class="mt-3 text-muted-foreground">{{ copy.help }}</p>
    <div class="mt-3 border-t pt-3">
      <a v-if="guide.redemptionUrl" :href="guide.redemptionUrl" target="_blank" rel="noopener noreferrer" class="inline-flex min-h-11 items-center gap-2 font-semibold text-primary underline underline-offset-4">
        {{ copy.redeem }} <ExternalLink class="h-4 w-4 shrink-0" aria-hidden="true" />
      </a>
      <p class="text-muted-foreground">{{ guide.rules.includes('credits') ? copy.creditsDelivery : guide.kind === 'code' ? copy.codeDelivery : copy.linkDelivery }}</p>
    </div>
  </section>
</template>
