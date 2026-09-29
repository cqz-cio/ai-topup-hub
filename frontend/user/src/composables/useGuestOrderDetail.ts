import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { guestOrderAPI } from '../api'
import { debounceAsync } from '../utils/debounce'
import { clearGuestOrderAuth, loadGuestOrderAuth, saveGuestOrderAuth } from '../utils/guestOrderAuth'
import { resolveGuestOrderDetailViewState } from '../utils/guestOrderDetailState'
import { useOrderDisplayHelpers } from './useOrderDisplayHelpers'
import { createOrderDetailPolling, needsOrderDetailRefresh } from '../utils/orderDetailPolling'

/**
 * 游客订单详情逻辑（classic + vault 共用）。
 */
export function useGuestOrderDetail() {
  const route = useRoute()
  const router = useRouter()
  const { t } = useI18n()

  const loading = ref(true)
  const order = ref<any>(null)
  const authError = ref('')
  const auth = ref({
    email: '',
    order_password: '',
  })
  const fulfillmentDownloading = ref(false)
  let disposed = false
  let requestVersion = 0

  const helpers = useOrderDisplayHelpers(order)

  const handleDownloadFulfillment = async (orderNo: string) => {
    if (fulfillmentDownloading.value) return
    fulfillmentDownloading.value = true
    try {
      const res = await guestOrderAPI.downloadFulfillment(orderNo, {
        email: auth.value.email,
        order_password: auth.value.order_password,
      })
      const blob = new Blob([res.data], { type: 'text/plain; charset=utf-8' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `fulfillment-${orderNo}.txt`
      a.click()
      URL.revokeObjectURL(url)
    } catch {} finally {
      fulfillmentDownloading.value = false
    }
  }

  const loadSavedAuth = () => {
    auth.value = loadGuestOrderAuth()
  }

  const hasAuth = computed(() => Boolean(auth.value.email && auth.value.order_password))
  const showAuthForm = computed(() => !hasAuth.value || authError.value !== '')
  const viewState = computed(() => resolveGuestOrderDetailViewState({
    loading: loading.value,
    order: order.value,
    showAuthForm: showAuthForm.value,
  }))

  const loadOrder = async (options?: { silent?: boolean }) => {
    const silent = options?.silent === true
    const version = ++requestVersion
    const orderNo = String(route.params.order_no || '').trim()
    const credentials = { ...auth.value }
    const isCurrent = () => !disposed && version === requestVersion
      && orderNo === String(route.params.order_no || '').trim()
      && credentials.email === auth.value.email && credentials.order_password === auth.value.order_password
    if (!silent) loading.value = true
    try {
      if (!hasAuth.value) {
        order.value = null
        authError.value = t('guestOrderDetail.authRequired')
        return
      }
      const response = await guestOrderAPI.detail(orderNo, credentials)
      if (!isCurrent()) return
      order.value = response.data.data
      authError.value = ''
    } catch (error) {
      if (isCurrent() && !silent) {
        order.value = null
        authError.value = t('guestOrderDetail.authInvalid')
      }
    } finally {
      if (isCurrent()) {
        if (!silent) loading.value = false
        polling.schedule()
      }
    }
  }

  const polling = createOrderDetailPolling(
    () => loadOrder({ silent: true }),
    () => !loading.value && hasAuth.value && !authError.value && needsOrderDetailRefresh(order.value),
  )

  const debouncedLoadOrder = debounceAsync(loadOrder, 300)

  const persistAuth = () => {
    saveGuestOrderAuth({
      email: auth.value.email,
      order_password: auth.value.order_password,
    })
  }

  const handleAuthSubmit = async () => {
    authError.value = ''
    if (!hasAuth.value) {
      authError.value = t('guestOrderDetail.authRequired')
      return
    }
    persistAuth()
    loading.value = true
    await debouncedLoadOrder()
  }

  const clearAuth = () => {
    requestVersion += 1
    loading.value = false
    clearGuestOrderAuth()
    auth.value = { email: '', order_password: '' }
    order.value = null
    authError.value = t('guestOrderDetail.authRequired')
  }

  onMounted(() => {
    if (!route.params.order_no) {
      router.push('/guest/orders')
      return
    }
    loadSavedAuth()
    loadOrder()
  })

  onUnmounted(() => {
    disposed = true
    polling.stop()
    debouncedLoadOrder.cancel()
  })

  return {
    loading,
    order,
    authError,
    auth,
    showAuthForm,
    viewState,
    handleAuthSubmit,
    clearAuth,
    fulfillmentDownloading,
    handleDownloadFulfillment,
    ...helpers,
  }
}
