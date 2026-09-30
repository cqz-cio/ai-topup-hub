<template>
  <div class="storefront">
    <a class="skip-link" href="#catalog-content">跳到商品目录</a>
    <header class="storefront-header">
      <RouterLink to="/" class="brand" :aria-label="`${siteName}首页`">
        <strong>{{ siteName }}</strong>
        <span>AI SERVICES</span>
      </RouterLink>
      <nav aria-label="商城导航">
        <a href="#catalog-content">商品目录</a>
        <RouterLink to="/me/orders">我的订单</RouterLink>
        <RouterLink to="/guest/orders">游客查单</RouterLink>
        <RouterLink to="/cart">
          购物车
          <span v-if="cartStore.totalItems" class="cart-count">
            {{ cartStore.totalItems }}
          </span>
        </RouterLink>
        <RouterLink
          :to="authStore.isAuthenticated ? '/me' : '/auth/login'"
          class="account-link"
        >
          {{ authStore.isAuthenticated ? '个人中心' : '登录 / 注册' }}
        </RouterLink>
      </nav>
    </header>

    <div class="catalog-shell">
      <aside class="catalog-sidebar" aria-label="商品分类与帮助">
        <h2>商品分类</h2>
        <nav class="category-nav" aria-label="商品分类">
          <button
            type="button"
            :aria-pressed="selectedCategory === ''"
            :class="{ active: selectedCategory === '' }"
            @click="selectedCategory = ''"
          >
            全部商品
          </button>
          <button
            v-for="category in categories"
            :key="category"
            type="button"
            :aria-pressed="selectedCategory === category"
            :class="{ active: selectedCategory === category }"
            @click="selectedCategory = category"
          >
            {{ category }}
          </button>
        </nav>
        <div class="purchase-help">
          <h2>购买与售后帮助</h2>
          <p>
            规格、支付或使用问题，
            <br />
            可联系微信客服。
          </p>
          <template v-if="supportWechat">
            <span>客服微信</span>
            <button type="button" class="wechat-copy" @click="copyWechat">
              {{ supportWechat }}
            </button>
            <span v-if="copyMessage" role="status">{{ copyMessage }}</span>
          </template>
        </div>
      </aside>

      <main id="catalog-content" class="catalog-main" tabindex="-1">
        <section class="catalog-intro" aria-labelledby="catalog-title">
          <div>
            <p class="eyebrow">自助下单 · 账号与卡密交付</p>
            <h1 id="catalog-title">选好商品，自助下单。</h1>
          </div>
          <label class="catalog-search">
            <Search :size="18" aria-hidden="true" />
            <span class="sr-only">搜索商品名称或服务类型</span>
            <input
              v-model="search"
              type="search"
              placeholder="搜索商品名称或服务类型"
            />
          </label>
          <p class="intro-description">
            在线选择规格并支付，到账确认后自动交付账号或卡密。
          </p>
        </section>
        <p class="delivery-notice">
          <Info :size="18" aria-hidden="true" />
          <span>
            交付内容在订单详情中查看。游客购买请保存下单邮箱与查询密码。
          </span>
        </p>
        <div class="catalog-heading">
          <h2>{{ selectedCategory || '全部商品' }}</h2>
          <p>使用期限与质保期不同，请分别确认。</p>
        </div>

        <div
          v-if="initialLoading"
          class="product-grid"
          aria-label="正在加载商品"
          aria-busy="true"
        >
          <div v-for="index in 6" :key="index" class="product-skeleton">
            <div></div>
            <p></p>
            <p></p>
            <p></p>
          </div>
        </div>
        <div v-else-if="loadFailed" class="catalog-empty" role="alert">
          <PackageOpen :size="36" aria-hidden="true" />
          <h3>商品暂时加载失败</h3>
          <p>请稍后重试，价格和库存以后台实时信息为准。</p>
          <button
            type="button"
            class="primary-button"
            :disabled="refreshing"
            @click="refreshProducts"
          >
            {{ refreshing ? '正在加载…' : '重新加载' }}
          </button>
        </div>
        <div v-else-if="visibleProducts.length === 0" class="catalog-empty">
          <PackageOpen :size="36" aria-hidden="true" />
          <h3>{{ products.length ? '没有找到相关商品' : '暂无上架商品' }}</h3>
          <p>
            {{
              products.length
                ? '试试其他关键词或查看全部商品。'
                : '商品上架后会在这里显示。'
            }}
          </p>
          <button
            v-if="products.length"
            type="button"
            class="secondary-button"
            @click="clearFilters"
          >
            查看全部商品
          </button>
        </div>
        <div v-else class="product-grid">
          <article
            v-for="product in visibleProducts"
            :key="product.slug"
            class="catalog-card"
            :class="{
              'compact-product-image': ['zxa-8', 'zxa-23'].includes(
                product.slug,
              ),
            }"
          >
            <RouterLink
              :to="productPath(product)"
              class="product-image-link"
              :aria-label="`${productTitle(product)}商品详情`"
            >
              <img
                v-if="productImage(product) && !failedImages.has(product.slug)"
                :src="productImage(product)"
                :alt="productTitle(product)"
                loading="lazy"
                decoding="async"
                @error="failedImages.add(product.slug)"
              />
              <div v-else class="product-image-placeholder">
                <PackageOpen :size="40" aria-hidden="true" />
                <span>{{ productTitle(product) }}</span>
              </div>
            </RouterLink>
            <div class="product-body">
              <h3>
                <RouterLink :to="productPath(product)">
                  {{ productTitle(product) }}
                </RouterLink>
                <span
                  v-if="
                    displayDetails(product) &&
                    ['zxa-1', 'zxa-2'].includes(product.slug)
                  "
                  class="title-warranty"
                >
                  （{{ displayDetails(product)?.warranty }}）
                </span>
              </h3>
              <dl v-if="displayDetails(product)" class="product-facts">
                <div>
                  <dt>服务类型</dt>
                  <dd>{{ displayDetails(product)?.deliveryType }}</dd>
                </div>
                <div>
                  <dt>{{ displayDetails(product)?.periodLabel }}</dt>
                  <dd>{{ displayDetails(product)?.period }}</dd>
                </div>
                <div>
                  <dt>质保范围</dt>
                  <dd>{{ displayDetails(product)?.warranty }}</dd>
                </div>
              </dl>
              <dl v-else class="product-facts">
                <div>
                  <dt>交付方式</dt>
                  <dd>
                    {{
                      product.fulfillment_type === 'auto'
                        ? '自动交付'
                        : '按商品说明交付'
                    }}
                  </dd>
                </div>
                <div>
                  <dt>购买方式</dt>
                  <dd>
                    {{
                      product.purchase_type === 'member'
                        ? '登录后购买'
                        : '支持游客购买'
                    }}
                  </dd>
                </div>
                <div>
                  <dt>使用与质保</dt>
                  <dd>请查看商品详情</dd>
                </div>
              </dl>
              <div class="product-price">
                <span>{{ formattedPrice(product) }}</span>
                <small v-if="(product.skus?.length || 0) > 1">起</small>
                <span v-if="!canPurchase(product)" class="stock-label">
                  暂时缺货
                </span>
              </div>
              <div class="product-actions">
                <RouterLink
                  v-if="canPurchase(product)"
                  :to="productPath(product)"
                  class="primary-button"
                  :aria-label="`立即购买${productTitle(product)}`"
                >
                  立即购买
                </RouterLink>
                <button v-else type="button" class="primary-button" disabled>
                  暂不可购买
                </button>
                <RouterLink :to="productPath(product)" class="secondary-button">
                  查看详情
                </RouterLink>
              </div>
            </div>
          </article>
        </div>
      </main>
    </div>

    <section class="purchase-steps" aria-labelledby="purchase-steps-title">
      <h2 id="purchase-steps-title">自助购买流程</h2>
      <ol>
        <li>
          <strong>01</strong>
          <div>
            <h3>选择商品</h3>
            <p>确认规格、数量、使用期限与质保。</p>
          </div>
          <ChevronRight :size="16" aria-hidden="true" />
        </li>
        <li>
          <strong>02</strong>
          <div>
            <h3>下单并支付</h3>
            <p>填写订单信息，按页面指引完成支付。</p>
          </div>
          <ChevronRight :size="16" aria-hidden="true" />
        </li>
        <li>
          <strong>03</strong>
          <div>
            <h3>领取交付内容</h3>
            <p>到账确认后，在订单页面查看账号或卡密。</p>
          </div>
        </li>
      </ol>
    </section>
    <section class="order-lookup" aria-labelledby="lookup-title">
      <h2 id="lookup-title">查看订单与交付内容</h2>
      <p>登录用户进入我的订单，游客使用邮箱与查询密码查单。</p>
      <div>
        <RouterLink to="/me/orders" class="secondary-button">
          我的订单
        </RouterLink>
        <RouterLink to="/guest/orders" class="secondary-button">
          游客查单
        </RouterLink>
      </div>
      <span v-if="supportWechat" class="lookup-support">
        售后微信
        <strong>{{ supportWechat }}</strong>
      </span>
    </section>
    <footer class="storefront-footer">
      <p>{{ siteName }} · 自助下单与账号卡密交付</p>
      <a
        v-if="!appStore.isResellerTenant"
        href="https://beian.miit.gov.cn/"
        target="_blank"
        rel="noopener noreferrer"
      >
        浙ICP备2026050415号-2
      </a>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { ChevronRight, Info, PackageOpen, Search } from 'lucide-vue-next'
import { productAPI } from '../api/product'
import { useAppStore } from '../stores/app'
import { useUserAuthStore } from '../stores/userAuth'
import { useCartStore } from '../stores/cart'
import { usePageSeo } from '../composables/usePageSeo'
import { getImageUrl } from '../utils/image'
import { storefrontDisplayDetails } from '../data/storefrontCatalog'
import {
  canPurchaseStorefrontProduct,
  loadStorefrontProducts,
  storefrontPrice,
  storefrontText,
  type StorefrontProduct,
} from '../utils/storefront'

const appStore = useAppStore()
const authStore = useUserAuthStore()
const cartStore = useCartStore()
const siteName = computed(
  () => String(appStore.config?.brand?.site_name || '').trim() || '智享A社',
)
const supportWechat = computed(
  () =>
    String(appStore.config?.contact?.wechat || '').trim() ||
    (appStore.isResellerTenant ? '' : '15824485317'),
)
const products = ref<StorefrontProduct[]>([])
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
  storefrontText(product.title, appStore.locale) || '商品'
const productCategory = (product: StorefrontProduct) =>
  storefrontText(product.category?.name, appStore.locale) ||
  displayDetails(product)?.category ||
  '其他商品'
const categories = computed(() => [
  ...new Set(products.value.map(productCategory)),
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
    return new Intl.NumberFormat('zh-CN', {
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

const refreshProducts = async () => {
  if (refreshing.value || disposed) return
  refreshing.value = true
  try {
    const next = await loadStorefrontProducts(
      async (page) => (await productAPI.list({ page, page_size: 100 })).data,
    )
    if (disposed) return
    products.value = next
    loadFailed.value = false
    if (
      selectedCategory.value &&
      !categories.value.includes(selectedCategory.value)
    )
      selectedCategory.value = ''
  } catch {
    if (disposed) return
    products.value = []
    loadFailed.value = true
  } finally {
    if (!disposed) {
      initialLoading.value = false
      refreshing.value = false
    }
  }
}
const refreshOnFocus = () => {
  if (document.visibilityState === 'visible') void refreshProducts()
}
const copyWechat = async () => {
  try {
    await navigator.clipboard.writeText(supportWechat.value)
    copyMessage.value = '已复制微信号'
  } catch {
    copyMessage.value = '请复制上方微信号'
  }
  if (copyTimer) clearTimeout(copyTimer)
  copyTimer = setTimeout(() => {
    copyMessage.value = ''
  }, 3000)
}
usePageSeo({
  title: () => siteName.value,
  description: () =>
    '在线选择商品规格，自助下单支付，到账确认后领取账号或卡密。',
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
.storefront {
  width: 100%;
  max-width: 1440px;
  margin: auto;
  background: #fff;
  color: #171717;
  font-family: Arial, 'Microsoft YaHei', sans-serif;
  font-size: 14px;
  line-height: 1.5;
  min-height: 100vh;
}
.storefront a {
  color: inherit;
  text-decoration: none;
}
.storefront button,
.storefront input {
  font: inherit;
}
.storefront button,
.storefront a {
  touch-action: manipulation;
}
.storefront :is(a, button, input):focus-visible {
  outline: 2px solid #171717;
  outline-offset: 4px;
}
.storefront .skip-link {
  position: fixed;
  top: -100px;
  left: 16px;
  z-index: 100;
  background: #fff;
  padding: 12px;
}
.storefront .skip-link:focus {
  top: 12px;
}
.storefront-header {
  min-height: 60px;
  padding: 10px 32px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  border-bottom: 1px solid #e6e8ec;
}
.brand {
  display: flex;
  align-items: center;
  gap: 16px;
  white-space: nowrap;
}
.brand strong {
  line-height: 1.2;
  font-size: 30px;
  font-weight: 800;
  letter-spacing: -1px;
}
.brand span {
  color: #666b77;
  font-size: 14px;
}
.storefront-header nav {
  display: flex;
  align-items: center;
  gap: 24px;
  font-size: 13px;
}
.storefront-header nav a:hover {
  text-decoration: underline;
  text-underline-offset: 4px;
}
.storefront-header .account-link {
  background: #171717;
  color: #fff;
  border-radius: 5px;
  padding: 8px 20px;
  font-weight: 700;
  white-space: nowrap;
}
.cart-count {
  display: inline-block;
  margin-left: 4px;
  font-size: 11px;
}
.catalog-shell {
  display: grid;
  grid-template-columns: 194px minmax(0, 1fr);
}
.catalog-sidebar {
  padding: 28px 16px 28px 30px;
  border-right: 1px solid #e6e8ec;
}
.catalog-sidebar h2 {
  font-size: 15px;
  font-weight: 700;
  margin: 0 0 10px;
}
.category-nav {
  display: grid;
  gap: 6px;
}
.category-nav button {
  border: 0;
  background: transparent;
  color: #4d5360;
  text-align: left;
  padding: 10px 13px;
  border-radius: 5px;
  cursor: pointer;
}
.category-nav button:hover {
  background: #f2f3f5;
}
.category-nav button.active {
  background: #222;
  color: #fff;
  font-weight: 700;
}
.purchase-help {
  border-top: 1px solid #e6e8ec;
  margin-top: 24px;
  padding-top: 24px;
}
.purchase-help p {
  font-size: 13px;
  color: #747984;
  margin: 0 0 14px;
}
.purchase-help > span {
  display: block;
  font-size: 12px;
}
.wechat-copy {
  display: block;
  border: 0;
  background: none;
  padding: 0;
  margin-top: 2px;
  font-size: 19px !important;
  font-weight: 700;
  cursor: pointer;
  word-break: break-all;
}
.catalog-main {
  padding: 20px 30px 12px;
  min-width: 0;
}
.catalog-intro {
  display: grid;
  grid-template-columns: 1fr 236px;
  align-items: center;
  column-gap: 20px;
}
.eyebrow {
  color: #848995;
  font-size: 13px;
  margin: 0 0 2px;
}
.catalog-intro h1 {
  font-size: 34px;
  line-height: 1.35;
  font-weight: 800;
  letter-spacing: -1px;
  margin: 0;
}
.catalog-search {
  display: flex;
  align-items: center;
  gap: 8px;
  border: 1px solid #dfe2e8;
  border-radius: 5px;
  padding: 8px 10px;
  color: #858b96;
  min-width: 0;
}
.catalog-search input {
  border: 0;
  outline: none;
  background: none;
  width: 100%;
  min-width: 0;
  font-size: 12px;
  color: #171717;
}
.catalog-search:focus-within {
  outline: 2px solid #171717;
  outline-offset: 2px;
}
.intro-description {
  grid-column: 1/-1;
  font-size: 18px;
  color: #626775;
  margin: 4px 0 14px;
}
.delivery-notice {
  margin: 0 0 20px;
  padding: 12px 16px;
  background: #f4f5f6;
  border-radius: 6px;
  color: #757b87;
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 12px;
}
.delivery-notice svg {
  flex-shrink: 0;
}
.catalog-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}
.catalog-heading h2 {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
}
.catalog-heading p {
  margin: 0;
  color: #707583;
  font-size: 12px;
}
.product-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px 16px;
}
.catalog-card.compact-product-image .product-image-link {
  aspect-ratio: 16/9;
}
.catalog-card {
  border: 1px solid #e3e5e9;
  border-radius: 6px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  background: #fff;
}
.product-image-link {
  display: block;
  aspect-ratio: 3/2;
  overflow: hidden;
  background: #f4f5f6;
}
.product-image-link img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.product-image-placeholder {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  gap: 10px;
  padding: 20px;
  color: #8b919a;
  text-align: center;
}
.product-body {
  padding: 8px 10px 12px;
  display: flex;
  flex-direction: column;
  flex: 1;
}
.product-body h3 {
  font-size: 15px;
  font-weight: 700;
  line-height: 1.45;
  margin: 0 0 8px;
  min-height: 0;
}
.title-warranty {
  display: block;
}
.product-facts {
  margin: 0 0 5px;
  font-size: 12px;
  color: #565d69;
}
.product-facts > div {
  display: grid;
  grid-template-columns: 80px 1fr;
  gap: 4px;
  border-top: 1px solid #eef0f3;
  padding: 2px 0;
}
.product-facts dd,
.product-facts dt {
  margin: 0;
}
.product-price {
  display: flex;
  align-items: baseline;
  gap: 4px;
  margin-top: auto;
  margin-bottom: 4px;
  font-size: 25px;
  font-weight: 700;
  line-height: 1.4;
}
.product-price small {
  font-size: 12px;
  font-weight: 400;
}
.stock-label {
  margin-left: auto;
  font-size: 11px !important;
  color: #8c6262;
  font-weight: 400;
}
.product-actions {
  display: grid;
  grid-template-columns: 1.3fr 1fr;
  gap: 8px;
}
.storefront .primary-button,
.storefront .secondary-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  text-align: center;
  min-height: 35px;
  padding: 6px 12px;
  border-radius: 5px;
  font-size: 12px;
  font-weight: 700;
  cursor: pointer;
  border: 1px solid #171717;
  line-height: 1.4;
}
.storefront .primary-button {
  background: #171717;
  color: #fff;
}
.storefront .primary-button:hover {
  background: #333;
}
.storefront .secondary-button {
  background: #fff;
  color: #171717;
  border-color: #8a8f9a;
}
.storefront .secondary-button:hover {
  background: #f3f4f6;
}
.storefront .primary-button:disabled {
  background: #e5e7eb;
  color: #8d9199;
  border-color: #e5e7eb;
  cursor: not-allowed;
}
.catalog-empty {
  padding: 70px 24px;
  display: flex;
  align-items: center;
  flex-direction: column;
  text-align: center;
  border: 1px dashed #dfe2e8;
  border-radius: 6px;
  color: #727986;
  gap: 12px;
}
.catalog-empty h3 {
  margin: 0;
  font-size: 18px;
  color: #171717;
}
.catalog-empty p {
  margin: 0;
}
.product-skeleton {
  border: 1px solid #e3e5e9;
  border-radius: 6px;
  padding-bottom: 12px;
}
.product-skeleton div {
  aspect-ratio: 3/2;
  background: #f0f1f3;
}
.product-skeleton p {
  height: 22px;
  background: #f0f1f3;
  margin: 10px;
}
.purchase-steps {
  border-top: 1px solid #e6e8ec;
  padding: 20px 32px 14px;
  display: flex;
  align-items: center;
  gap: 24px;
}
.purchase-steps h2 {
  font-size: 20px;
  font-weight: 700;
  margin: 0;
  padding-right: 24px;
  border-right: 1px solid #e6e8ec;
  white-space: nowrap;
}
.purchase-steps ol {
  list-style: none;
  padding: 0;
  margin: 0;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
  flex: 1;
}
.purchase-steps li {
  display: flex;
  align-items: flex-start;
  gap: 8px;
}
.purchase-steps li > strong {
  font-size: 25px;
  line-height: 1.1;
}
.purchase-steps li > div {
  flex: 1;
}
.purchase-steps h3 {
  font-size: 14px;
  font-weight: 700;
  margin: 0 0 2px;
}
.purchase-steps p {
  font-size: 10.5px;
  color: #818692;
  margin: 0;
}
.purchase-steps svg {
  margin-top: 6px;
  flex-shrink: 0;
}
.order-lookup {
  margin: 0 28px;
  padding: 12px 20px;
  background: #f5f6f7;
  border-radius: 6px;
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}
.order-lookup h2 {
  margin: 0;
  font-size: 18px;
  font-weight: 700;
}
.order-lookup p {
  margin: 0;
  flex: 1;
  font-size: 11px;
  color: #818692;
}
.order-lookup > div {
  display: flex;
  gap: 10px;
}
.order-lookup .secondary-button {
  min-width: 94px;
}
.lookup-support {
  border-left: 1px solid #dde0e6;
  padding-left: 16px;
  font-size: 11px;
  color: #818692;
}
.lookup-support strong {
  font-size: 16px;
  color: #171717;
}
.storefront-footer {
  display: flex;
  justify-content: space-between;
  gap: 20px;
  padding: 10px 32px 14px;
  font-size: 11px;
  color: #818692;
}
.storefront-footer p {
  margin: 0;
}
.storefront-footer a:hover {
  text-decoration: underline;
}
@media (min-width: 1200px) {
  .catalog-main {
    padding-top: 30px;
  }
  .product-body {
    padding: 12px 14px 14px;
  }
  .product-body h3 {
    font-size: 17px;
  }
  .product-facts {
    font-size: 13px;
  }
  .product-facts > div {
    grid-template-columns: 90px 1fr;
    padding: 4px 0;
  }
  .product-actions .primary-button,
  .product-actions .secondary-button {
    min-height: 38px;
    font-size: 13px;
  }
}
@media (max-width: 1000px) {
  .storefront-header {
    padding-left: 24px;
    padding-right: 24px;
  }
  .storefront-header nav {
    gap: 16px;
  }
  .brand strong {
    font-size: 26px;
  }
  .brand span {
    font-size: 12px;
  }
  .catalog-intro {
    grid-template-columns: 1fr;
  }
  .catalog-search {
    grid-row: 3;
    margin-bottom: 12px;
  }
  .catalog-intro h1 {
    font-size: 30px;
  }
  .intro-description {
    font-size: 16px;
  }
  .catalog-shell {
    grid-template-columns: 170px minmax(0, 1fr);
  }
  .catalog-sidebar {
    padding-left: 24px;
  }
  .catalog-main {
    padding: 24px 24px 12px;
  }
  .product-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .purchase-steps {
    gap: 18px;
    padding-left: 24px;
    padding-right: 24px;
  }
  .purchase-steps h2 {
    font-size: 18px;
    padding-right: 18px;
  }
  .purchase-steps ol {
    gap: 14px;
  }
  .purchase-steps li {
    gap: 10px;
  }
  .purchase-steps li > strong {
    font-size: 22px;
  }
  .lookup-support {
    border: 0;
    padding-left: 0;
  }
}
@media (max-width: 680px) {
  .storefront-header {
    padding: 14px 18px;
    display: block;
  }
  .brand strong {
    font-size: 28px;
  }
  .storefront-header nav {
    gap: 18px;
    margin-top: 12px;
    overflow-x: auto;
    white-space: nowrap;
    padding-bottom: 2px;
    font-size: 12px;
  }
  .storefront-header .account-link {
    padding: 6px 12px;
    margin-left: auto;
  }
  .catalog-shell {
    display: block;
  }
  .catalog-sidebar {
    border-right: 0;
    border-bottom: 1px solid #e6e8ec;
    padding: 14px 18px;
  }
  .catalog-sidebar > h2 {
    display: none;
  }
  .category-nav {
    display: flex;
    overflow-x: auto;
    gap: 8px;
  }
  .category-nav button {
    white-space: nowrap;
    padding: 8px 14px;
    font-size: 13px;
  }
  .purchase-help {
    display: none;
  }
  .catalog-main {
    padding: 20px 18px;
  }
  .catalog-intro h1 {
    font-size: 29px;
  }
  .eyebrow {
    font-size: 12px;
  }
  .intro-description {
    font-size: 14px;
    line-height: 1.6;
    margin-bottom: 12px;
  }
  .delivery-notice {
    font-size: 11px;
    align-items: flex-start;
    padding: 10px 12px;
    margin-bottom: 18px;
  }
  .catalog-heading {
    align-items: flex-start;
    flex-direction: column;
    gap: 2px;
    margin-bottom: 12px;
  }
  .catalog-heading h2 {
    font-size: 19px;
  }
  .catalog-heading p {
    font-size: 11px;
  }
  .product-grid {
    gap: 12px;
  }
  .product-body {
    padding: 8px;
  }
  .product-body h3 {
    font-size: 13px;
    min-height: 38px;
  }
  .product-facts {
    font-size: 10px;
  }
  .product-facts > div {
    grid-template-columns: 56px 1fr;
  }
  .product-price {
    font-size: 22px;
  }
  .stock-label {
    display: none;
  }
  .product-actions {
    gap: 6px;
    grid-template-columns: 1fr;
  }
  .storefront .primary-button,
  .storefront .secondary-button {
    min-height: 36px;
    font-size: 11px;
    padding: 6px;
  }
  .purchase-steps {
    display: block;
    padding: 20px 18px;
  }
  .purchase-steps h2 {
    border: 0;
    padding: 0;
    margin-bottom: 16px;
  }
  .purchase-steps ol {
    grid-template-columns: 1fr;
    gap: 16px;
  }
  .purchase-steps li > strong {
    font-size: 24px;
    min-width: 28px;
  }
  .purchase-steps li > svg {
    display: none;
  }
  .purchase-steps h3 {
    font-size: 14px;
  }
  .purchase-steps p {
    font-size: 12px;
  }
  .order-lookup {
    margin: 0 18px;
    display: block;
    padding: 16px;
  }
  .order-lookup h2 {
    font-size: 18px;
  }
  .order-lookup p {
    font-size: 12px;
    margin: 6px 0 12px;
  }
  .order-lookup > div {
    margin-bottom: 10px;
  }
  .order-lookup .secondary-button {
    flex: 1;
  }
  .storefront-footer {
    display: block;
    padding: 14px 18px;
    font-size: 10px;
  }
  .storefront-footer p {
    margin-bottom: 4px;
  }
}
@media (prefers-reduced-motion: reduce) {
  .storefront * {
    scroll-behavior: auto;
  }
}
</style>
