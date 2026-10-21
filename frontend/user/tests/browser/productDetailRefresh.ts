// Run via the local Vite server: /tests/browser/productDetailRefresh.html.
// Real Vue lifecycle, stores, composable and API client; only responses/time are
// controlled. This fixture is not a production build entry and sends no orders.
import { createApp, h, nextTick } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createMemoryHistory, RouterView } from 'vue-router'
import { createHead } from '@unhead/vue/client'
import i18n from '../../src/i18n'
import { productAPI, api } from '../../src/api'
import { useProductDetail } from '../../src/composables/useProductDetail'
import { useUserAuthStore } from '../../src/stores/userAuth'

const results: string[] = []
const output = document.querySelector('#results')!
const assert = (condition: unknown, message: string) => { if (!condition) throw new Error(message) }
const pass = (name: string) => { results.push(name); output.textContent = JSON.stringify({ passed: results.length, tests: results }, null, 2) }
const flush = async () => { for (let i = 0; i < 8; i++) await nextTick() }
const pending: { slug: string; resolve: (data: any) => void; reject: (error: any) => void }[] = []
const originalDetail = productAPI.detail
productAPI.detail = (slug: string) => new Promise((resolve, reject) => pending.push({ slug, resolve, reject }))
const originalInterval = window.setInterval
const originalClearInterval = window.clearInterval
let timer: (() => void) | undefined
let timerCleared = false
window.setInterval = ((handler: TimerHandler, delay?: number, ...args: any[]) => {
  if (delay === 60000 && typeof handler === 'function') { timer = handler as () => void; return 987654 }
  return originalInterval(handler, delay, ...args)
}) as typeof window.setInterval
window.clearInterval = ((id?: number) => { if (id === 987654) timerCleared = true; else originalClearInterval(id) })

function product(slug = 'refresh-test', price = '100.00', stock = 10) {
  return { id: 901, slug, title: { 'zh-CN': slug }, images: ['/one.png', '/two.png'],
    purchase_type: 'guest', fulfillment_type: 'manual', price_amount: price,
    stock_status: stock ? 'in_stock' : 'out_of_stock', stock_display_mode: 'exact',
    skus: [11, 12].map(id => ({ id, is_active: true, price_amount: price,
      manual_stock_total: stock, stock_status: stock ? 'in_stock' : 'out_of_stock' })) }
}
let state: ReturnType<typeof useProductDetail>
let auth: ReturnType<typeof useUserAuthStore>
let loaded = 0
const Probe = {
  setup() {
    auth = useUserAuthStore()
    auth.token = '' // In-memory fixture only; never persist a test login.
    state = useProductDetail({ onLoaded: () => { loaded++ } })
    return () => state.loading.value ? h('div', { id: 'skeleton' }, 'loading') :
      state.product.value ? h('section', { id: 'product' }, [
        h('h2', state.product.value.slug), h('span', { id: 'price' }, state.selectedSku.value?.price_amount),
        h('input', { id: 'quantity', value: state.quantity.value }),
        h('button', { disabled: !state.canPurchase.value }, '购买'),
      ]) : h('div', { id: 'unavailable' }, 'unavailable')
  },
}
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/products/:slug', component: Probe }] })
const app = createApp({ render: () => h(RouterView) })
app.use(createPinia()).use(createHead()).use(i18n).use(router)
const resolveNext = async (value = product()) => {
  const request = pending.shift(); assert(request, 'expected pending request')
  request!.resolve({ data: { data: value } }); await flush()
}
const rejectNext = async (error: Error) => { pending.shift()!.reject(error); await flush() }
const focus = async () => { window.dispatchEvent(new Event('focus')); await flush() }
let mounted = false
try {
  await router.push('/products/refresh-test'); await router.isReady()
  app.mount('#fixture'); mounted = true; await flush()
  assert(state!.loading.value && document.querySelector('#skeleton'), 'first entry must show skeleton')
  await resolveNext()
  assert(!state!.loading.value && state!.canPurchase.value && loaded === 1, 'initial render/callback')
  pass('首次加载及渲染后回调')
  const node = document.querySelector('#product')
  state!.selectedSkuId.value = 12; await flush()
  state!.quantity.value = 3; state!.currentImage.value = state!.images.value[1]!; await flush()
  await focus()
  assert(!state!.loading.value && document.querySelector('#product') === node, 'focus must preserve DOM')
  timer!(); await focus()
  assert(pending.length === 1, 'overlapping automatic refreshes must coalesce')
  await resolveNext(product('refresh-test', '88.00', 8))
  assert(document.querySelector('#product') === node && !document.querySelector('#skeleton'), 'refresh must not remount content')
  assert(state!.selectedSku.value?.price_amount === '88.00' && state!.quantityEffectiveLimit.value === 8, 'fresh price/stock')
  assert(state!.selectedSkuId.value === 12 && state!.quantity.value === 3 && state!.currentImage.value === state!.images.value[1], 'selection must survive refresh')
  assert(loaded === 1, 'background refresh must not recreate observers')
  pass('窗口恢复静默更新价格库存，保留 DOM、规格、图片、数量')
  pass('重叠定时器和焦点请求合并')
  timer!(); await flush()
  assert(document.querySelector('#product') === node && !state!.loading.value, 'timer must be silent')
  await resolveNext(product('refresh-test', '90.00', 2))
  assert(state!.quantity.value === 2 && state!.selectedSku.value?.price_amount === '90.00', 'stock shrink clamps quantity')
  pass('60 秒定时回调静默刷新且库存减少时限制数量')
  await focus(); await rejectNext(new Error('simulated timeout'))
  assert(document.querySelector('#product') === node && !state!.loading.value && !state!.canPurchase.value, 'transient failure retains view, disables stale purchase')
  assert(state!.cannotPurchaseReason.value, 'failed refresh must explain disabled purchase')
  await focus(); await resolveNext()
  assert(state!.canPurchase.value && !state!.cannotPurchaseReason.value && document.querySelector('#product') === node, 'recovery')
  pass('网络失败保留页面，成功重试恢复购买')
  await focus(); await resolveNext(product('refresh-test', '100.00', 0))
  assert(!state!.canPurchase.value && document.querySelector('#product') === node, 'sold out disables purchase in place')
  pass('缺货正常更新且禁止购买')
  await focus(); await rejectNext(Object.assign(new Error('removed'), { status: 404 }))
  assert(!state!.product.value && document.querySelector('#unavailable'), 'removed product must not remain purchasable')
  pass('明确下架响应清除旧商品')
  const retry = state!.loadProduct(); await resolveNext(); await retry
  await focus()
  const oldProductRequest = pending.shift()!
  await router.push('/products/another-product'); await flush()
  assert(!state!.product.value && state!.loading.value, 'new route must clear previous product')
  await resolveNext(product('another-product', '200.00'))
  oldProductRequest.resolve({ data: { data: product('refresh-test', '1.00') } }); await flush()
  assert(state!.product.value.slug === 'another-product' && state!.selectedSku.value?.price_amount === '200.00', 'stale route response must be ignored')
  pass('切换商品重新加载并拒绝旧请求覆盖')
  await focus(); const oldAccountRequest = pending.shift()!
  auth!.token = 'fixture-in-memory-only'; await flush()
  assert(!state!.product.value && state!.loading.value, 'account switch must clear private pricing')
  await resolveNext(product('another-product', '170.00'))
  oldAccountRequest.resolve({ data: { data: product('another-product', '1.00') } }); await flush()
  assert(state!.selectedSku.value?.price_amount === '170.00', 'stale account response must be ignored')
  pass('切换账号清除旧价格并拒绝迟到响应')
  await focus(); const lateRequest = pending.shift()!
  const previousProduct = state!.product.value
  app.unmount(); mounted = false
  lateRequest.resolve({ data: { data: product('another-product', '1.00') } }); await flush()
  await focus(); await state!.loadProduct()
  assert(timerCleared && pending.length === 0 && state!.product.value === previousProduct, 'unmount must cancel polling and ignore responses')
  pass('卸载清理定时器、焦点监听和迟到响应')
  const originalFetch = window.fetch
  try {
    for (const [status, body] of [
      [404, JSON.stringify({ msg: 'removed' })], [403, 'not json'],
      [503, JSON.stringify({ msg: 'temporary' })], [200, JSON.stringify({ status_code: 404, msg: 'removed' })],
    ] as const) {
      window.fetch = async () => new Response(body, { status })
      let received: any
      try { await api.get('/public/products/fixture') } catch (error) { received = error }
      assert(received?.status === (status === 200 ? 404 : status), 'API must retain error status')
    }
  } finally { window.fetch = originalFetch }
  pass('真实 API 客户端保留 HTTP 和业务错误状态')
  output.textContent = JSON.stringify({ status: 'passed', passed: results.length, tests: results }, null, 2)
} catch (error) {
  output.textContent = JSON.stringify({ passed: results.length, tests: results, error: String(error) }, null, 2)
  throw error
} finally {
  if (mounted) app.unmount()
  productAPI.detail = originalDetail
  window.setInterval = originalInterval
  window.clearInterval = originalClearInterval
}
