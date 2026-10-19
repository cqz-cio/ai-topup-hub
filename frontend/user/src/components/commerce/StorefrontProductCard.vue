<template>
  <article class="catalog-card" :class="{ 'catalog-card-credits': credits }">
    <RouterLink :to="path" class="product-image-link" :aria-label="t('shopUi.detailsLabel', { name: title })">
      <img v-if="image" :src="displayImage" :class="{ 'is-glass-cover': usingGlassCover }" :alt="title" loading="lazy" decoding="async" @error="handleImageError" />
      <div v-else class="product-image-placeholder"><PackageOpen :size="36" aria-hidden="true" /></div>
      <span v-if="!credits" class="image-category">{{ category }}</span>
    </RouterLink>
    <div class="product-body">
      <h3><RouterLink :to="path">{{ title }}</RouterLink></h3>
      <dl v-if="details" class="product-facts">
        <div><dt>{{ t('shopUi.serviceType') }}</dt><dd>{{ details.deliveryType }}</dd></div>
        <div><dt>{{ details.periodLabel }}</dt><dd>{{ details.period }}</dd></div>
        <div><dt>{{ t('shopUi.warranty') }}</dt><dd>{{ details.warranty }}</dd></div>
      </dl>
      <dl v-else class="product-facts">
        <div><dt>{{ t('shopUi.deliveryMethod') }}</dt><dd>{{ t(fulfillmentType === 'auto' ? 'shopUi.automatic' : 'shopUi.manual') }}</dd></div>
        <div><dt>{{ t('shopUi.purchaseMethod') }}</dt><dd>{{ t(purchaseType === 'member' ? 'shopUi.memberOnly' : 'shopUi.guestAvailable') }}</dd></div>
        <div><dt>{{ t('shopUi.usageWarranty') }}</dt><dd>{{ t('shopUi.checkDetails') }}</dd></div>
      </dl>
      <div class="product-price"><strong>{{ price }}</strong><small v-if="multipleSkus">{{ t('shopUi.from') }}</small><span class="stock-label" :class="{ 'is-out': !purchasable }">{{ stock }}</span></div>
      <div class="product-actions">
        <RouterLink v-if="purchasable" :to="path" class="store-button is-primary" :aria-label="t('shopUi.buyLabel', { name: title })">{{ t('shopUi.buy') }}<ArrowUpRight :size="14" aria-hidden="true" /></RouterLink>
        <button v-else type="button" class="store-button is-primary" disabled>{{ t('shopUi.unavailable') }}</button>
        <RouterLink :to="path" class="store-text-link">{{ t('shopUi.details') }}</RouterLink>
      </div>
    </div>
  </article>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ArrowUpRight, PackageOpen } from 'lucide-vue-next'
import type { StorefrontDisplayDetails } from '../../data/storefrontCatalog'
import { useAppStore } from '../../stores/app'
import { getCatalogGlassCover } from '../../utils/catalogGlass'

const props = defineProps<{
  slug: string
  title: string
  category: string
  image: string
  details?: StorefrontDisplayDetails
  fulfillmentType?: string
  purchaseType?: string
  price: string
  stock: string
  purchasable: boolean
  multipleSkus: boolean
  credits?: boolean
}>()
const emit = defineEmits<{ imageError: [] }>()
const { t } = useI18n()
const appStore = useAppStore()
const path = computed(() => `/products/${encodeURIComponent(props.slug)}`)
const glassCoverFailed = ref(false)
const glassCover = computed(() => appStore.isResellerTenant ? '' : getCatalogGlassCover(props.image))
const usingGlassCover = computed(() => !!glassCover.value && !glassCoverFailed.value)
const displayImage = computed(() => usingGlassCover.value ? glassCover.value : props.image)
watch(() => props.image, () => { glassCoverFailed.value = false })
function handleImageError() {
  if (usingGlassCover.value) glassCoverFailed.value = true
  else emit('imageError')
}
</script>

<style scoped>
.store-button { display: inline-flex; align-items: center; justify-content: center; gap: 16px; border: 1px solid var(--ui-border-strong); background: var(--ui-bg-elevated); color: var(--ui-text-primary); border-radius: 9px; min-height: 44px; padding: 10px 18px; font-size: 12px; font-weight: 600; cursor: pointer; transition: background .2s; }
.store-button:hover { background: var(--ui-bg-soft); }
.store-button.is-primary { color: var(--ui-text-on-accent); background: var(--ui-accent); border-color: var(--ui-accent); }
.store-button.is-primary:hover { background: var(--ui-accent-hover); }
.store-button:disabled { color: var(--ui-text-muted); background: var(--ui-bg-muted); border-color: var(--ui-border); cursor: not-allowed; }
.store-text-link { display: inline-flex; align-items: center; justify-content: center; gap: 6px; font-size: 12px; color: var(--ui-text-secondary); white-space: nowrap; }
.store-text-link:hover { color: var(--ui-success); }
.catalog-card { display: flex; flex-direction: column; border: 1px solid var(--ui-border); border-radius: 13px; overflow: hidden; background: var(--ui-bg-elevated); transition: border-color .2s, box-shadow .2s; }
.catalog-card:hover { border-color: var(--ui-border-strong); box-shadow: var(--ui-shadow-soft); }
.product-image-link { position: relative; display: block; aspect-ratio: 5/3; background: var(--ui-bg-soft); overflow: hidden; }
.product-image-link img { display: block; height: 100%; width: 100%; object-fit: contain; object-position: center; }
.image-category { position: absolute; top: 10px; left: 10px; max-width: calc(100% - 20px); background: #ffffffed; color: #384338; padding: 4px 8px; border-radius: 5px; font-size: 10px; line-height: 1.5; overflow-wrap: anywhere; }
.product-image-placeholder { height: 100%; display: flex; align-items: center; justify-content: center; color: var(--ui-text-muted); }
.product-body { min-width: 0; padding: 18px; flex: 1; display: flex; flex-direction: column; }
.product-body h3 { font-size: 16px; line-height: 1.6; min-height: 3.2em; font-weight: 650; margin-bottom: 14px; overflow-wrap: anywhere; }
.product-body h3 a { color: var(--ui-text-primary); }
.product-facts { display: grid; gap: 8px; font-size: 12px; line-height: 1.65; margin-bottom: 22px; }
.product-facts > div { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1.4fr); align-items: start; gap: 10px; }
.product-facts dt { color: var(--ui-text-muted); overflow-wrap: anywhere; }
.product-facts dd { min-width: 0; color: var(--ui-text-secondary); text-align: right; overflow-wrap: anywhere; }
.product-price { margin-top: auto; display: flex; flex-wrap: wrap; align-items: baseline; gap: 8px; margin-bottom: 16px; }
.product-price strong { font-size: 26px; font-weight: 700; letter-spacing: -.04em; font-variant-numeric: tabular-nums; }
.product-price small { font-size: 10px; color: var(--ui-text-muted); }
.stock-label { font-size: 12px; color: var(--ui-text-secondary); margin-left: auto; overflow-wrap: anywhere; }
.stock-label.is-out { color: var(--ui-warning); }
.product-actions { display: flex; gap: 12px; align-items: center; }
.product-actions .store-button { min-width: 0; flex: 1; padding: 10px 12px; min-height: 44px; gap: 8px; font-size: 12px; }
.product-actions .store-button svg { flex-shrink: 0; }
.product-actions .store-text-link { min-height: 44px; flex-shrink: 0; font-size: 12px; }

.catalog-card { min-width: 0; }
.catalog-card-credits .product-image-link { background: #213d2c; }

/* Thin glass: the surface and rim reflect light without fading the content. */
.brand-scene .catalog-card {
  position: relative;
  isolation: isolate;
  border-radius: 18px;
  border-color: rgba(255, 255, 255, .88) rgba(255, 255, 255, .7) rgba(255, 255, 255, .84) rgba(255, 255, 255, .78);
  background: linear-gradient(160deg, rgba(255, 255, 255, .18), rgba(255, 255, 255, .07) 38%, rgba(255, 255, 255, .16) 74%, rgba(255, 255, 255, .24));
  -webkit-backdrop-filter: blur(3px) saturate(1.04);
  backdrop-filter: blur(3px) saturate(1.04);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, .78), inset 1px 0 0 rgba(255, 255, 255, .3), inset 0 -1px 0 rgba(255, 255, 255, .62), 0 8px 24px -18px rgba(13, 59, 62, .32);
}
.brand-scene .catalog-card::before,
.brand-scene .catalog-card::after {
  content: '';
  position: absolute;
  pointer-events: none;
  border-radius: inherit;
}
.brand-scene .catalog-card::before {
  z-index: 2;
  inset: 2px;
  border: 1px solid rgba(255, 255, 255, .72);
  border-radius: 15px;
  -webkit-mask-image: linear-gradient(135deg, #000, rgba(0, 0, 0, .22) 24%, transparent 44%, transparent 62%, rgba(0, 0, 0, .5) 86%, #000);
  mask-image: linear-gradient(135deg, #000, rgba(0, 0, 0, .22) 24%, transparent 44%, transparent 62%, rgba(0, 0, 0, .5) 86%, #000);
}
.brand-scene .catalog-card::after {
  z-index: 0;
  inset: 0;
  background:
    linear-gradient(45deg, transparent 32%, rgba(255, 255, 255, .95) 42%, rgba(255, 255, 255, .95) 58%, transparent 68%) right 5px top 6px / 11px 9px no-repeat,
    linear-gradient(154deg, rgba(255, 255, 255, .24), rgba(255, 255, 255, .06) 14%, transparent 28%);
}
.brand-scene .catalog-card > * { position: relative; z-index: 1; }
.brand-scene .catalog-card:hover {
  border-color: rgba(255, 255, 255, .95);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, .9), inset 1px 0 0 rgba(255, 255, 255, .4), inset 0 -1px 0 rgba(255, 255, 255, .7), 0 12px 28px -18px rgba(13, 59, 62, .36);
}
.brand-scene .product-image-link { background: transparent; }
.brand-scene .product-body h3 { font-size: 17px; line-height: 1.5; min-height: 3em; font-weight: 700; }
.brand-scene .product-facts { font-size: 13px; line-height: 1.55; font-weight: 550; }
.brand-scene .product-facts dt, .brand-scene .product-facts dd { color: var(--ui-text-primary); }
.brand-scene .product-price strong { font-size: 28px; font-weight: 750; color: var(--ui-text-primary); }
.brand-scene .stock-label { font-size: 13px; font-weight: 500; }
.brand-scene .product-actions .store-text-link { font-size: 13px; font-weight: 600; }

.brand-scene .image-category {
  background: rgba(255, 255, 255, .66);
  border: 1px solid rgba(255, 255, 255, .55);
  padding: 3px 7px;
}
.brand-scene .stock-label.is-out { color: #754000; }
.dark .brand-scene .catalog-card {
  border-color: rgba(220, 247, 237, .3);
  background: linear-gradient(145deg, rgba(20, 43, 37, .58), rgba(14, 33, 31, .38) 42%, rgba(20, 43, 37, .56));
  box-shadow: inset 0 1px 0 rgba(224, 255, 244, .22), 0 10px 28px -16px rgba(0, 0, 0, .42);
}
.dark .brand-scene .catalog-card::before { opacity: .38; }
.dark .brand-scene .catalog-card::after { opacity: .28; }
.dark .brand-scene .is-glass-cover { filter: brightness(0) invert(.94); }
.dark .brand-scene .stock-label.is-out { color: #ffdc8b; }
@supports not ((backdrop-filter: blur(5px)) or (-webkit-backdrop-filter: blur(5px))) {
  .brand-scene .catalog-card { background: rgba(239, 250, 246, .84); }
  .dark .brand-scene .catalog-card { background: rgba(20, 43, 37, .9); }
}
@media (max-width: 767px) {
  .catalog-card { border-radius: 10px; }
  .product-body { padding: 16px 14px; }
  .product-body h3 { font-size: 14px; line-height: 1.6; margin-bottom: 12px; }
  .product-facts { font-size: 11px; gap: 7px; }
  .product-facts > div { gap: 8px; }
  .image-category { top: 8px; left: 8px; font-size: 10px; padding: 3px 7px; }
  .product-price strong { font-size: 22px; }
  .product-price { flex-wrap: wrap; }
  .stock-label { display: block; margin-left: 0; width: 100%; }
  .product-actions { flex-direction: column; align-items: stretch; gap: 9px; }
  .product-actions .store-button { min-height: 44px; }
  .product-actions .store-text-link { min-height: 36px; }
}
@media (max-width: 479px) {
  .product-body { padding: 18px; }
  .product-body h3 { font-size: 16px; min-height: auto; }
  .brand-scene .product-body h3 { font-size: 16px; min-height: auto; }
  .product-facts { font-size: 12px; }
  .product-price strong { font-size: 26px; }
  .stock-label { width: auto; margin-left: auto; }
  .product-actions { flex-direction: row; align-items: center; gap: 12px; }
  .product-actions .store-text-link { min-height: 44px; }
}
</style>
