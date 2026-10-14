import { onMounted, reactive } from 'vue'
import { adminAPI } from '@/api/admin'

export type RuntimeFeature = 'reseller_settlement' | 'reconciliation' | 'system_subdomain' | 'stock_sync_settings' | 'telegram_broadcast' | 'inventory_alerts' | 'redemption'
export type RuntimeFeatures = Partial<Record<RuntimeFeature, { available: boolean; reason?: string }>>
export interface RuntimeFeatureState {
  loading: boolean
  error: boolean
  features: RuntimeFeatures | null
  available: (feature: RuntimeFeature) => boolean
  reload: () => Promise<void>
}

// Missing, failed and still-loading capability responses all block submission.
// Reload on each page visit so an old tab does not advertise cached capabilities.
export function useRuntimeFeatures() {
  const state: RuntimeFeatureState = reactive({
    loading: true,
    error: false,
    features: null as RuntimeFeatures | null,
    available: (feature: RuntimeFeature): boolean => !state.loading && !state.error && state.features?.[feature]?.available === true,
    reload: async (): Promise<void> => {
      state.loading = true
      state.error = false
      state.features = null
      try {
        const response = await adminAPI.getRuntimeFeatures()
        state.features = response.data?.data || null
        if (!state.features) state.error = true
      } catch {
        state.error = true
      } finally {
        state.loading = false
      }
    },
  })
  onMounted(state.reload)
  return state
}
