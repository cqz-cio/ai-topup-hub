import { onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { userOrderAPI } from '../api'
import { debounceAsync } from '../utils/debounce'
import { useConfirmDialog } from './useConfirmDialog'
import { toast } from './useToast'
import { useOrderDisplayHelpers } from './useOrderDisplayHelpers'
import { createOrderDetailPolling, needsOrderDetailRefresh } from '../utils/orderDetailPolling'

/**
 * 已登录用户订单详情逻辑（classic + vault 共用）。
 */
export function useOrderDetail() {
  const route = useRoute()
  const router = useRouter()
  const { confirm: showConfirm } = useConfirmDialog()
  const { t } = useI18n()

  const loading = ref(true)
  const order = ref<any>(null)
  const fulfillmentDownloading = ref(false)
  let disposed = false
  let requestVersion = 0

  const helpers = useOrderDisplayHelpers(order)

  const handleDownloadFulfillment = async (orderNo: string) => {
    if (fulfillmentDownloading.value) return
    fulfillmentDownloading.value = true
    try {
      const res = await userOrderAPI.downloadFulfillment(orderNo)
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

  const loadOrder = async (options?: { silent?: boolean }) => {
    const silent = options?.silent === true
    const version = ++requestVersion
    const orderNo = String(route.params.order_no || '').trim()
    const isCurrent = () => !disposed && version === requestVersion && orderNo === String(route.params.order_no || '').trim()
    if (!silent) loading.value = true
    try {
      const response = await userOrderAPI.detail(orderNo)
      if (!isCurrent()) return
      order.value = response.data.data
    } catch (error) {
      if (isCurrent() && !silent) order.value = null
    } finally {
      if (isCurrent()) {
        if (!silent) loading.value = false
        polling.schedule()
      }
    }
  }

  const polling = createOrderDetailPolling(
    () => loadOrder({ silent: true }),
    () => !loading.value && needsOrderDetailRefresh(order.value),
  )

  const debouncedLoadOrder = debounceAsync(loadOrder, 300)

  const cancelOrder = async () => {
    if (!order.value) return
    const confirmed = await showConfirm({
      title: t('orderDetail.cancel'),
      message: t('orderDetail.cancelConfirm'),
      confirmText: t('common.confirm'),
      cancelText: t('common.cancel'),
      variant: 'danger',
    })
    if (!confirmed) return
    try {
      await userOrderAPI.cancel(order.value.order_no)
      await debouncedLoadOrder()
    } catch {
      toast.error(t('orderDetail.cancelFailed'))
    }
  }

  onMounted(() => {
    if (!route.params.order_no) {
      router.push('/me/orders')
      return
    }
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
    debouncedLoadOrder,
    cancelOrder,
    fulfillmentDownloading,
    handleDownloadFulfillment,
    ...helpers,
  }
}
