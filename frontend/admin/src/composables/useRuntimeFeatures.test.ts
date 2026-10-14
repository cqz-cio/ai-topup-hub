import { afterEach, describe, expect, it, vi } from 'vitest'
import { createApp, h, nextTick, type App } from 'vue'
import { useRuntimeFeatures, type RuntimeFeatureState } from './useRuntimeFeatures'
import FeatureAvailability from '@/components/FeatureAvailability.vue'

const getRuntimeFeatures = vi.hoisted(() => vi.fn())
vi.mock('@/api/admin', () => ({ adminAPI: { getRuntimeFeatures } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
let app: App | undefined
afterEach(() => { app?.unmount(); document.body.replaceChildren(); vi.resetAllMocks() })
function mount() {
  let state!: RuntimeFeatureState
  const host = document.createElement('div')
  document.body.append(host)
  app = createApp({ setup() { state = useRuntimeFeatures(); return () => h(FeatureAvailability, { state, feature: 'reconciliation' }) } })
  app.mount(host)
  return { state, host }
}

describe('runtime availability', () => {
  it('blocks actions until a fresh response confirms support', async () => {
    let resolve!: (value: unknown) => void
    getRuntimeFeatures.mockReturnValue(new Promise(r => { resolve = r }))
    const { state, host } = mount()
    expect(state.available('reconciliation')).toBe(false)
    expect(host.textContent).toContain('featureAvailability.loading')
    resolve({ data: { data: { reconciliation: { available: true } } } })
    await nextTick(); await nextTick()
    expect(state.available('reconciliation')).toBe(true)
    expect(state.available('telegram_broadcast')).toBe(false)
    expect(host.textContent).toBe('')
  })

  it('revokes cached permission on error and supports retry', async () => {
    getRuntimeFeatures.mockResolvedValue({ data: { data: { reconciliation: { available: true } } } })
    const { state, host } = mount()
    await nextTick(); await nextTick()
    expect(state.available('reconciliation')).toBe(true)
    getRuntimeFeatures.mockRejectedValueOnce(new Error('offline'))
    await state.reload(); await nextTick()
    expect(state.available('reconciliation')).toBe(false)
    expect(host.textContent).toContain('featureAvailability.loadFailed')
    getRuntimeFeatures.mockResolvedValueOnce({ data: { data: { reconciliation: { available: false } } } })
    host.querySelector('button')!.click()
    await nextTick(); await nextTick()
    expect(state.available('reconciliation')).toBe(false)
    expect(host.textContent).toContain('featureAvailability.reconciliation')
    expect(host.textContent).not.toContain('featureAvailability.loadFailed')
  })
})
