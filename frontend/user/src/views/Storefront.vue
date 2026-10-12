<template>
  <div class="storefront">
    <a class="skip-link" href="#catalog-content">{{ t('shopUi.skipCatalog') }}</a>
    <section class="store-hero" aria-labelledby="store-title">
      <div class="hero-copy">
        <p class="commerce-eyebrow"><span class="hero-dot" aria-hidden="true"></span>{{ t('shopUi.heroEyebrow') }}</p>
        <h1 id="store-title">{{ t('shopUi.heroTitle') }}<br /><span>{{ t('shopUi.heroAccent') }}</span></h1>
        <p class="hero-description">{{ t('shopUi.heroDescription') }}</p>
        <div class="hero-actions">
          <a href="#catalog-content" class="store-button is-primary">{{ t('shopUi.browse') }}<ArrowRight :size="17" aria-hidden="true" /></a>
          <RouterLink :to="authStore.isAuthenticated ? '/me/orders' : '/guest/orders'" class="store-text-link">{{ t('shopUi.lookupAction') }}<ArrowUpRight :size="16" aria-hidden="true" /></RouterLink>
        </div>
      </div>
      <div class="hero-guide">
        <div class="hero-guide-heading"><ShoppingBag :size="19" aria-hidden="true" /><span>{{ siteName }}</span><span class="guide-caption">{{ t('shopUi.brandLine') }}</span></div>
        <h2>{{ t('shopUi.flowTitle') }}</h2>
        <ol class="hero-flow">
          <li v-for="(key, index) in flowKeys" :key="key">
            <span class="hero-flow-number">0{{ index + 1 }}</span>
            <div><strong>{{ t(`shopUi.steps.${key}`) }}</strong><p>{{ t(`shopUi.stepHints.${key}`) }}</p></div>
            <ArrowUpRight v-if="index === 3" :size="18" aria-hidden="true" />
          </li>
        </ol>
      </div>
    </section>

    <main id="catalog-content" class="catalog-section" tabindex="-1">
      <div class="catalog-toolbar">
        <div><p class="commerce-eyebrow">{{ t('shopUi.catalog') }}</p><h2>{{ t('shopUi.allProducts') }}<span v-if="!initialLoading && !loadFailed">{{ t('shopUi.productCount', { count: products.length }) }}</span></h2></div>
        <label class="catalog-search">
          <Search :size="18" aria-hidden="true" /><span class="sr-only">{{ t('shopUi.search') }}</span>
          <input v-model="search" type="search" :placeholder="t('shopUi.search')" />
        </label>
      </div>
      <div class="catalog-shell">
        <aside class="catalog-sidebar" :aria-label="t('shopUi.categories')">
          <h3>{{ t('shopUi.categories') }}</h3>
          <nav class="category-nav" :aria-label="t('shopUi.categories')">
            <button type="button" :aria-pressed="selectedCategory === ''" :class="{ active: selectedCategory === '' }" @click="selectedCategory = ''">{{ t('shopUi.allProducts') }}<span>{{ products.length }}</span></button>
            <button v-for="category in categories" :key="category" type="button" :aria-pressed="selectedCategory === category" :class="{ active: selectedCategory === category }" @click="selectedCategory = category">{{ category }}<ArrowUpRight :size="13" aria-hidden="true" /></button>
          </nav>
          <div class="purchase-help">
            <ClipboardList :size="21" aria-hidden="true" /><h3>{{ t('shopUi.helpTitle') }}</h3><p>{{ t('shopUi.helpDescription') }}</p>
            <template v-if="supportWechat"><span>{{ t('shopUi.wechat') }}</span><button type="button" class="wechat-copy" @click="copyWechat">{{ supportWechat }}<Copy :size="13" aria-hidden="true" /></button><span v-if="copyMessage" role="status">{{ copyMessage }}</span></template>
          </div>
        </aside>
        <div class="catalog-main">
          <div class="delivery-notice"><Info :size="15" aria-hidden="true" /><p>{{ t('shopUi.deliveryNotice') }}</p></div>
          <div v-if="initialLoading" class="product-grid" :aria-label="t('shopUi.loading')" aria-busy="true">
            <div v-for="index in 6" :key="index" class="product-skeleton"><div></div><p></p><p></p><p></p></div>
          </div>
          <div v-else-if="loadFailed" class="catalog-empty" role="alert">
            <PackageOpen :size="36" aria-hidden="true" /><h3>{{ t('shopUi.loadFailed') }}</h3><p>{{ t('shopUi.loadFailedHint') }}</p>
            <button type="button" class="store-button is-primary" :disabled="refreshing" @click="refreshProducts">{{ refreshing ? t('shopUi.reloading') : t('shopUi.reload') }}</button>
          </div>
          <div v-else-if="visibleProducts.length === 0" class="catalog-empty">
            <PackageOpen :size="36" aria-hidden="true" /><h3>{{ t(products.length ? 'shopUi.noMatch' : 'shopUi.noProducts') }}</h3><p>{{ t(products.length ? 'shopUi.noMatchHint' : 'shopUi.noProductsHint') }}</p>
            <button v-if="products.length" type="button" class="store-button" @click="clearFilters">{{ t('shopUi.allProducts') }}</button>
          </div>
          <div v-else class="product-grid">
            <article v-for="product in visibleProducts" :key="product.slug" class="catalog-card">
              <RouterLink :to="productPath(product)" class="product-image-link" :aria-label="t('shopUi.detailsLabel', { name: productTitle(product) })">
                <img v-if="productImage(product) && !failedImages.has(product.slug)" :src="productImage(product)" :alt="productTitle(product)" loading="lazy" decoding="async" @error="failedImages.add(product.slug)" />
                <div v-else class="product-image-placeholder"><PackageOpen :size="36" aria-hidden="true" /></div>
                <span class="image-category">{{ productCategory(product) }}</span>
              </RouterLink>
              <div class="product-body">
                <h3><RouterLink :to="productPath(product)">{{ productTitle(product) }}</RouterLink></h3>
                <dl v-if="displayDetails(product)" class="product-facts">
                  <div><dt>{{ t('shopUi.serviceType') }}</dt><dd>{{ displayDetails(product)?.deliveryType }}</dd></div>
                  <div><dt>{{ displayDetails(product)?.periodLabel }}</dt><dd>{{ displayDetails(product)?.period }}</dd></div>
                  <div><dt>{{ t('shopUi.warranty') }}</dt><dd>{{ displayDetails(product)?.warranty }}</dd></div>
                </dl>
                <dl v-else class="product-facts">
                  <div><dt>{{ t('shopUi.deliveryMethod') }}</dt><dd>{{ t(product.fulfillment_type === 'auto' ? 'shopUi.automatic' : 'shopUi.manual') }}</dd></div>
                  <div><dt>{{ t('shopUi.purchaseMethod') }}</dt><dd>{{ t(product.purchase_type === 'member' ? 'shopUi.memberOnly' : 'shopUi.guestAvailable') }}</dd></div>
                  <div><dt>{{ t('shopUi.usageWarranty') }}</dt><dd>{{ t('shopUi.checkDetails') }}</dd></div>
                </dl>
                <div class="product-price"><strong>{{ formattedPrice(product) }}</strong><small v-if="(product.skus?.length || 0) > 1">{{ t('shopUi.from') }}</small><span class="stock-label" :class="{ 'is-out': !canPurchase(product) }">{{ stockLabel(product) }}</span></div>
                <div class="product-actions">
                  <RouterLink v-if="canPurchase(product)" :to="productPath(product)" class="store-button is-primary" :aria-label="t('shopUi.buyLabel', { name: productTitle(product) })">{{ t('shopUi.buy') }}<ArrowUpRight :size="14" aria-hidden="true" /></RouterLink>
                  <button v-else type="button" class="store-button is-primary" disabled>{{ t('shopUi.unavailable') }}</button>
                  <RouterLink :to="productPath(product)" class="store-text-link">{{ t('shopUi.details') }}</RouterLink>
                </div>
              </div>
            </article>
          </div>
          <p class="catalog-note">{{ t('shopUi.catalogHint') }}</p>
        </div>
      </div>
    </main>
    <section class="order-lookup" aria-labelledby="lookup-title">
      <div class="lookup-copy"><p class="commerce-eyebrow">{{ t('shopUi.deliveryEyebrow') }}</p><h2 id="lookup-title">{{ t('shopUi.lookupTitle') }}</h2><p>{{ t('shopUi.lookupHint') }}</p></div>
      <div class="lookup-actions"><RouterLink to="/me/orders" class="store-button is-primary">{{ t('shopUi.memberOrders') }}<ArrowUpRight :size="16" aria-hidden="true" /></RouterLink><RouterLink to="/guest/orders" class="store-button">{{ t('shopUi.guestOrders') }}</RouterLink></div>
    </section>
    <p class="store-service-note"><Check :size="14" aria-hidden="true" />{{ t('shopUi.flowDescription') }}</p>
    <AnnouncementModal
      v-if="activeAnnouncement"
      :announcement="activeAnnouncement"
      v-model:visible="announcementVisible"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { ArrowUpRight, ArrowRight, Check, Copy, Info, PackageOpen, Search, ShoppingBag, ClipboardList } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'
import { categoryAPI, productAPI } from '../api/product'
import { useAppStore } from '../stores/app'
import { useUserAuthStore } from '../stores/userAuth'
import { usePageSeo } from '../composables/usePageSeo'
import AnnouncementModal from '../components/AnnouncementModal.vue'
import { useAnnouncement, type HomeAnnouncement } from '../composables/useAnnouncement'
import { getImageUrl } from '../utils/image'
import { storefrontDisplayDetails } from '../data/storefrontCatalog'
import {
  canPurchaseStorefrontProduct,
  loadStorefrontProducts,
  storefrontPrice,
  storefrontStockQuantity,
  storefrontText,
  type StorefrontProduct,
} from '../utils/storefront'

const appStore = useAppStore()
const { shouldShow } = useAnnouncement()
const activeAnnouncement = computed<HomeAnnouncement | undefined>(() => appStore.config?.announcement)
const announcementVisible = ref(false)
watch(activeAnnouncement, (announcement) => {
  announcementVisible.value = shouldShow(announcement)
}, { immediate: true })
const authStore = useUserAuthStore()
const { t } = useI18n()
const flowKeys = ['cart', 'checkout', 'payment', 'delivery'] as const
const siteName = computed(
  () => String(appStore.config?.brand?.site_name || '').trim() || 'π星卡社',
)
const supportWechat = computed(
  () =>
    String(appStore.config?.contact?.wechat || '').trim() ||
    (appStore.isResellerTenant ? '' : '15824485317'),
)
const products = ref<StorefrontProduct[]>([])
const categoryRows = ref<Array<{ name?: unknown }>>([])
const initialLoading = ref(true)
const refreshing = ref(false)
const loadFailed = ref(false)
const selectedCategory = ref('')
const search = ref('')
const failedImages = ref(new Set<string>())
const copyMessage = ref('')
const displayDetails = (product: StorefrontProduct) =>
  appStore.isResellerTenant ? undefined : storefrontDisplayDetails[product.slug]
const productTitle = (product: StorefrontProduct) =>
  storefrontText(product.title, appStore.locale) || t('shopUi.product')
const productCategory = (product: StorefrontProduct) =>
  storefrontText(product.category?.name, appStore.locale) ||
  displayDetails(product)?.category ||
  t('shopUi.otherProducts')
const categories = computed(() => [
  ...new Set([
    ...categoryRows.value
      .map((category) => storefrontText(category?.name, appStore.locale))
      .filter(Boolean),
    ...products.value.map(productCategory),
  ]),
])
const visibleProducts = computed(() =>
  products.value.filter((product) => {
    if (
      selectedCategory.value &&
      productCategory(product) !== selectedCategory.value
    )
      return false
    const keyword = search.value.trim().toLocaleLowerCase()
    return (
      !keyword ||
      [
        productTitle(product),
        productCategory(product),
        displayDetails(product)?.deliveryType || '',
      ]
        .join(' ')
        .toLocaleLowerCase()
        .includes(keyword)
    )
  }),
)
const productPath = (product: StorefrontProduct) =>
  `/products/${encodeURIComponent(product.slug)}`
const canPurchase = (product: StorefrontProduct) =>
  !loadFailed.value && canPurchaseStorefrontProduct(product)
const stockLabel = (product: StorefrontProduct) => {
  if (!canPurchase(product)) return t('shopUi.outOfStock')
  const quantity = storefrontStockQuantity(product)
  if (quantity !== null) return t('productDetail.skuStockRemaining', { count: quantity })
  return t(product.stock_status === 'unlimited'
    ? 'productDetail.skuStockUnlimited'
    : 'productDetail.skuStockInStock')
}
const productImage = (product: StorefrontProduct) => {
  const images = Array.isArray(product.images)
    ? product.images
    : product.images?.images
  // The imported Gmail catalog accidentally referenced a Gemini thumbnail.
  if (
    product.slug === 'zxa-23' &&
    images?.[0]?.endsWith('/294d28a8dc515316e378f8e150fa6d.jpg') &&
    displayDetails(product)
  )
    return displayDetails(product)?.image || ''
  return images?.[0]
    ? getImageUrl(images[0])
    : displayDetails(product)?.image || ''
}
const formattedPrice = (product: StorefrontProduct) => {
  const amount = storefrontPrice(product)
  if (amount === null) return '—'
  const currency = String(appStore.config?.currency || 'CNY').toUpperCase()
  try {
    return new Intl.NumberFormat(appStore.locale, {
      style: 'currency',
      currency,
      useGrouping: false,
      minimumFractionDigits: 0,
      maximumFractionDigits: 2,
    }).format(amount)
  } catch {
    return `${amount} ${currency}`
  }
}
const clearFilters = () => {
  selectedCategory.value = ''
  search.value = ''
}
let disposed = false
let refreshTimer: ReturnType<typeof setInterval> | undefined
let copyTimer: ReturnType<typeof setTimeout> | undefined
let productRequest = 0

const refreshProducts = async () => {
  if (disposed) return
  const request = ++productRequest
  const token = authStore.token
  refreshing.value = true
  try {
    const [next, categoryResponse] = await Promise.all([
      loadStorefrontProducts(
        async (page) => (await productAPI.list({ page, page_size: 100 })).data,
      ),
      categoryAPI.list().catch(() => undefined),
    ])
    if (disposed || request !== productRequest || token !== authStore.token) return
    products.value = next
    categoryRows.value = Array.isArray(categoryResponse?.data?.data)
      ? categoryResponse.data.data
      : []
    loadFailed.value = false
    if (
      selectedCategory.value &&
      !categories.value.includes(selectedCategory.value)
    )
      selectedCategory.value = ''
  } catch {
    if (disposed || request !== productRequest || token !== authStore.token) return
    products.value = []
    categoryRows.value = []
    loadFailed.value = true
  } finally {
    if (!disposed && request === productRequest) {
      initialLoading.value = false
      refreshing.value = false
    }
  }
}
watch(() => authStore.token, () => {
  products.value = []
  initialLoading.value = true
  void refreshProducts()
})
const refreshOnFocus = () => {
  if (document.visibilityState === 'visible') void refreshProducts()
}
const copyWechat = async () => {
  try {
    await navigator.clipboard.writeText(supportWechat.value)
    copyMessage.value = t('shopUi.wechatCopied')
  } catch {
    copyMessage.value = t('shopUi.wechatCopyFailed')
  }
  if (copyTimer) clearTimeout(copyTimer)
  copyTimer = setTimeout(() => {
    copyMessage.value = ''
  }, 3000)
}
usePageSeo({
  title: () => siteName.value,
  description: () =>
    t('shopUi.heroDescription'),
  canonicalPath: () => '/',
})
onMounted(() => {
  void refreshProducts()
  refreshTimer = setInterval(refreshOnFocus, 60000)
  window.addEventListener('focus', refreshOnFocus)
  document.addEventListener('visibilitychange', refreshOnFocus)
})
onUnmounted(() => {
  disposed = true
  if (refreshTimer) clearInterval(refreshTimer)
  if (copyTimer) clearTimeout(copyTimer)
  window.removeEventListener('focus', refreshOnFocus)
  document.removeEventListener('visibilitychange', refreshOnFocus)
})
</script>

<style scoped>
.storefront { max-width: 1240px; margin: 0 auto; padding: 122px 32px 48px; color: var(--ui-text-primary); }
.storefront a { text-decoration: none; }
.storefront :is(a, button, input):focus-visible { outline: 2px solid var(--ui-success); outline-offset: 4px; }
.storefront button, .storefront a { touch-action: manipulation; }
.skip-link { position: fixed; top: -100px; left: 20px; z-index: 100; padding: 12px; background: var(--ui-bg-elevated); }
.skip-link:focus { top: 12px; }
.store-hero { display: grid; grid-template-columns: 1.25fr 1fr; gap: 72px; align-items: center; padding-bottom: 48px; }
.hero-copy .commerce-eyebrow { display: flex; align-items: center; gap: 7px; margin-bottom: 22px; }
.hero-dot { height: 6px; width: 6px; border-radius: 50%; background: var(--ui-success); }
.hero-copy h1 { font-size: clamp(32px, 4vw, 52px); letter-spacing: -.04em; line-height: 1.38; font-weight: 750; }
.hero-copy h1 span { color: var(--ui-success); }
.hero-description { margin-top: 20px; color: var(--ui-text-muted); font-size: 14px; max-width: 435px; line-height: 1.9; }
.hero-actions { display: flex; align-items: center; gap: 25px; margin-top: 30px; flex-wrap: wrap; }
.store-button { display: inline-flex; align-items: center; justify-content: center; gap: 16px; border: 1px solid var(--ui-border-strong); background: var(--ui-bg-elevated); color: var(--ui-text-primary); border-radius: 9px; min-height: 44px; padding: 10px 18px; font-size: 12px; font-weight: 600; cursor: pointer; transition: background .2s; }
.store-button:hover { background: var(--ui-bg-soft); }
.store-button.is-primary { color: var(--ui-text-on-accent); background: var(--ui-accent); border-color: var(--ui-accent); }
.store-button.is-primary:hover { background: var(--ui-accent-hover); }
.store-button:disabled { color: var(--ui-text-muted); background: var(--ui-bg-muted); border-color: var(--ui-border); cursor: not-allowed; }
.store-text-link { display: inline-flex; align-items: center; justify-content: center; gap: 6px; font-size: 12px; color: var(--ui-text-secondary); white-space: nowrap; }
.store-text-link:hover { color: var(--ui-success); }
.hero-guide { background: #263e2e; color: #edf3e9; border-radius: 20px; padding: 28px 30px 24px; }
.hero-guide-heading { display: flex; gap: 9px; align-items: center; font-size: 13px; padding-bottom: 20px; border-bottom: 1px solid #ffffff21; }
.guide-caption { margin-left: auto; color: #b5c8b6; font-size: 9px; letter-spacing: .03em; }
.hero-guide h2 { font-size: 16px; margin: 24px 0 18px; font-weight: 600; line-height: 1.6; }
.hero-flow { list-style: none; padding: 0; display: grid; gap: 15px; }
.hero-flow li { display: flex; align-items: center; gap: 15px; }
.hero-flow-number { flex-shrink: 0; display: flex; align-items: center; justify-content: center; height: 31px; width: 31px; border: 1px solid #9fbc9a55; border-radius: 9px; font-size: 11px; color: #bfd3b4; font-variant-numeric: tabular-nums; }
.hero-flow strong { font-size: 12px; font-weight: 500; }
.hero-flow p { font-size: 10px; color: #b5c8b6; margin-top: 2px; }
.hero-flow li:last-child { border-top: 1px solid #ffffff21; padding-top: 16px; margin-top: 1px; }
.hero-flow li:last-child .hero-flow-number { background: #c9deb9; color: #263e2e; border-color: transparent; }
.hero-flow li:last-child > svg { margin-left: auto; color: #c9deb9; }
.catalog-section { scroll-margin-top: 100px; outline: none; }
.catalog-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 25px; padding-top: 32px; border-top: 1px solid var(--ui-border); margin-bottom: 28px; }
.catalog-toolbar .commerce-eyebrow { margin-bottom: 8px; }
.catalog-toolbar h2 { font-size: 25px; font-weight: 700; }
.catalog-toolbar h2 span { font-size: 10px; font-weight: 400; color: var(--ui-text-muted); margin-left: 16px; vertical-align: middle; }
.catalog-search { display: flex; align-items: center; gap: 10px; border: 1px solid var(--ui-border); background: var(--ui-bg-elevated); padding: 12px 14px; border-radius: 9px; color: var(--ui-text-muted); width: 290px; }
.catalog-search input { width: 100%; min-width: 0; background: transparent; font-size: 12px; border: none; outline: none; color: var(--ui-text-primary); }
.catalog-search:focus-within { outline: 2px solid var(--ui-success); outline-offset: 2px; }
.catalog-shell { display: grid; grid-template-columns: 175px minmax(0, 1fr); gap: 30px; }
.catalog-sidebar { position: sticky; top: 100px; align-self: start; max-height: calc(100dvh - 124px); overflow-y: auto; }
.catalog-sidebar h3 { font-size: 12px; font-weight: 500; color: var(--ui-text-muted); margin-bottom: 12px; }
.category-nav { display: grid; gap: 5px; }
.category-nav button { border: 0; background: transparent; color: var(--ui-text-secondary); padding: 12px 13px; border-radius: 8px; cursor: pointer; text-align: left; display: flex; align-items: center; justify-content: space-between; font-size: 13px; gap: 10px; overflow-wrap: anywhere; }
.category-nav button > svg { flex-shrink: 0; }
.category-nav button:hover { background: var(--ui-bg-muted); }
.category-nav button.active { color: var(--ui-text-on-accent); background: var(--ui-accent); font-weight: 600; }
.category-nav button span { font-size: 10px; opacity: .7; }
.purchase-help { border-top: 1px solid var(--ui-border); margin-top: 28px; padding-top: 24px; }
.purchase-help > svg { color: var(--ui-success); margin-bottom: 14px; }
.purchase-help h3 { color: var(--ui-text-primary); font-size: 12px; font-weight: 600; }
.purchase-help p { font-size: 11px; color: var(--ui-text-muted); line-height: 1.85; margin-bottom: 20px; }
.purchase-help > span { display: block; font-size: 10px; color: var(--ui-text-muted); }
.wechat-copy { display: flex; gap: 8px; align-items: center; border: none; background: transparent; padding: 4px 0; color: var(--ui-text-primary); font-size: 14px; cursor: pointer; font-weight: 600; word-break: break-all; }
.catalog-main { min-width: 0; }
.delivery-notice { display: flex; align-items: flex-start; gap: 8px; color: var(--ui-text-muted); font-size: 12px; line-height: 1.7; margin-bottom: 16px; }
.delivery-notice > svg { color: var(--ui-success); flex-shrink: 0; }
.product-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 18px; }
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
.catalog-note { font-size: 10px; color: var(--ui-text-muted); margin: 20px 0 0; }
.catalog-empty { border: 1px dashed var(--ui-border-strong); border-radius: 13px; padding: 60px 24px; display: flex; align-items: center; flex-direction: column; gap: 14px; color: var(--ui-text-muted); text-align: center; }
.catalog-empty h3 { font-size: 18px; color: var(--ui-text-primary); }
.catalog-empty p { font-size: 12px; }
.product-skeleton { border-radius: 13px; border: 1px solid var(--ui-border); background: var(--ui-bg-elevated); overflow: hidden; }
.product-skeleton div { aspect-ratio: 5/3; background: var(--ui-bg-muted); }
.product-skeleton p { background: var(--ui-bg-soft); height: 16px; margin: 16px; }
.order-lookup { background: var(--ui-bg-muted); border-radius: 16px; padding: 28px 32px; display: flex; align-items: center; justify-content: space-between; gap: 32px; margin-top: 48px; }
.lookup-copy .commerce-eyebrow { margin-bottom: 10px; }
.lookup-copy h2 { font-size: 23px; font-weight: 650; }
.lookup-copy > p:last-child { font-size: 12px; color: var(--ui-text-muted); margin-top: 10px; line-height: 1.8; }
.lookup-actions { display: flex; gap: 10px; flex-shrink: 0; }
.store-service-note { margin: 24px 0 0; text-align: center; color: var(--ui-text-muted); font-size: 10px; display: flex; align-items: center; justify-content: center; gap: 6px; }
.store-service-note svg { color: var(--ui-success); }
@media (max-width: 1050px) {
  .store-hero { gap: 36px; grid-template-columns: 1fr 1fr; }
  .hero-guide { padding: 24px; }
  .guide-caption { display: none; }
  .product-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .catalog-shell { grid-template-columns: 155px minmax(0, 1fr); gap: 24px; }
  .order-lookup { padding: 26px; gap: 20px; }
}
@media (max-width: 767px) {
  .storefront { padding: 102px 18px 32px; }
  .store-hero { grid-template-columns: 1fr; gap: 30px; padding-bottom: 30px; }
  .hero-copy h1 { font-size: 34px; }
  .hero-copy .commerce-eyebrow { margin-bottom: 16px; }
  .hero-description { font-size: 13px; margin-top: 14px; }
  .hero-actions { margin-top: 22px; gap: 20px; }
  .hero-guide { padding: 22px; border-radius: 14px; }
  .hero-guide-heading { padding-bottom: 14px; }
  .guide-caption { display: block; }
  .hero-guide h2 { margin: 17px 0; font-size: 15px; }
  .hero-flow { grid-template-columns: 1fr 1fr; gap: 20px 12px; }
  .hero-flow li { align-items: flex-start; gap: 9px; }
  .hero-flow li:last-child { margin: 0; padding: 0; border: 0; }
  .hero-flow li:last-child > svg { display: none; }
  .hero-flow strong { font-size: 11px; }
  .hero-flow p { font-size: 9px; }
  .hero-flow-number { height: 27px; width: 27px; border-radius: 7px; }
  .catalog-toolbar { flex-direction: column; align-items: stretch; gap: 18px; padding-top: 26px; margin-bottom: 18px; }
  .catalog-toolbar h2 { font-size: 23px; }
  .catalog-search { width: 100%; }
  .catalog-shell { display: block; }
  .catalog-sidebar { position: static; max-height: none; overflow: visible; }
  .catalog-sidebar > h3, .purchase-help { display: none; }
  .category-nav { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 18px; }
  .category-nav button { max-width: 100%; min-height: 40px; padding: 10px 13px; white-space: normal; font-size: 12px; }
  .category-nav button > svg { display: none; }
  .delivery-notice { font-size: 12px; align-items: flex-start; margin-bottom: 15px; }
  .delivery-notice svg { margin-top: 3px; }
  .product-grid { gap: 12px; }
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
  .order-lookup { margin-top: 30px; padding: 23px; flex-direction: column; align-items: stretch; }
  .lookup-copy h2 { font-size: 22px; }
  .lookup-actions .store-button { flex: 1; gap: 8px; }
  .store-service-note { font-size: 9px; }
}
@media (max-width: 479px) {
  .product-grid { grid-template-columns: minmax(0, 1fr); }
  .product-body { padding: 18px; }
  .product-body h3 { font-size: 16px; min-height: auto; }
  .product-facts { font-size: 12px; }
  .product-price strong { font-size: 26px; }
  .stock-label { width: auto; margin-left: auto; }
  .product-actions { flex-direction: row; align-items: center; gap: 12px; }
  .product-actions .store-text-link { min-height: 44px; }
}
@media (max-width: 360px) { .hero-copy h1 { font-size: 29px; } .guide-caption { display: none; } }
</style>
