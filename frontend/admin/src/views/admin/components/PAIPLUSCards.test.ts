import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createApp, h, nextTick, type App } from 'vue'
import PAIPLUSCards from './PAIPLUSCards.vue'

const mock = vi.hoisted(() => ({ list: vi.fn(), reveal: vi.fn(), check: vi.fn(), revoke: vi.fn(), allowed: true, token: 'test-admin' }))
vi.mock('@/api/paiplus', () => ({ paiplusAPI: mock }))
vi.mock('@/stores/auth', () => ({ useAdminAuthStore: () => ({ hasPermission: () => mock.allowed, get token() { return mock.token } }) }))
vi.mock('@/components/ListPagination.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/ui/dialog', () => {
  const slot = { template: '<div><slot /></div>' }
  return { Dialog: { props: ['open'], template: '<div v-if="open"><slot /></div>' }, DialogScrollContent: slot, DialogDescription: slot, DialogHeader: slot, DialogTitle: slot }
})
vi.mock('@/components/ui/checkbox', () => ({ Checkbox: { props: ['modelValue'], emits: ['update:modelValue'], template: '<input type="checkbox" :checked="modelValue" @change="$emit(\'update:modelValue\', $event.target.checked)" />' } }))

const card = { id: 'test-card-12345678', code: 'PAIPLUS-••••••••••-••••••AB23', source_code: '••••••CD45', reference: 'shop:1:order:100:1', order_id: 100, order_no: 'ORDER-100', product_id: 30, sku_id: 30, label: 'ChatGPT Plus', status: 'unused', local_status: 'ready', created_at: 1, updated_at: 1, revoked_at: 0, upstream_checked_at: 0, upstream: null, submitted: false, attempt_state: null }
const full = { ...card, code: 'PAIPLUS-ABCDEFGH23-ABCDEFGH45', source_code: 'SOURCE-PRIVATE-TEST', operations: [], audit: [] }
let app: App | undefined
async function flush() { for (let i = 0; i < 5; i++) { await Promise.resolve(); await nextTick() } }
function button(host: HTMLElement, text: string) { return [...host.querySelectorAll('button')].find(el => el.textContent?.trim() === text)! }
async function mount() {
  const host = document.createElement('div'); document.body.append(host)
  app = createApp({ render: () => h(PAIPLUSCards) }); app.mount(host); await flush(); return host
}
async function confirmDestroy(host: HTMLElement) {
  const reason = host.querySelector<HTMLTextAreaElement>('#paiplus-revoke-reason')!
  reason.value = '客户退款'; reason.dispatchEvent(new Event('input'))
  const check = host.querySelector<HTMLInputElement>('input[type="checkbox"]')!; check.checked = true; check.dispatchEvent(new Event('change'))
  await flush(); button(host, '检查并销毁本站卡密').click(); await flush()
}
beforeEach(() => {
  vi.clearAllMocks(); mock.allowed = true; mock.token = 'test-admin'
  mock.list.mockResolvedValue({ cards: [{ ...card }], total: 1, page: 1, page_size: 20, counts: { unused: 1 } })
  mock.reveal.mockResolvedValue({ ...full })
})
afterEach(() => { app?.unmount(); app = undefined; document.body.replaceChildren() })

describe('PAIPLUS cards in shop administration', () => {
  it('hides both full codes until explicit detail access and clears them on close', async () => {
    const host = await mount(); expect(host.textContent).not.toContain(full.source_code)
    button(host, '查看双码与管理').click(); await flush(); expect(host.textContent).toContain(full.source_code); expect(host.textContent).toContain(full.code)
    button(host, '关闭并隐藏卡密').click(); await flush(); expect(host.textContent).not.toContain(full.source_code)
  })
  it('requires reason and confirmation, then shows successful permanent revocation', async () => {
    const host = await mount(); button(host, '查看双码与管理').click(); await flush()
    expect(button(host, '检查并销毁本站卡密').disabled).toBe(true)
    mock.revoke.mockResolvedValue({ destroyed: true, card: { ...card, status: 'revoked', revoked_at: 1000 } })
    mock.reveal.mockResolvedValue({ ...full, status: 'revoked', revoked_at: 1000 })
    await confirmDestroy(host)
    expect(mock.revoke).toHaveBeenCalledWith(card.id, '客户退款', expect.any(String))
    expect(host.textContent).toContain('本站卡密已永久销毁'); expect(host.textContent).toContain('此操作不会自动退款')
  })
  it('shows used-card rejection without claiming revocation succeeded', async () => {
    const host = await mount(); button(host, '查看双码与管理').click(); await flush()
    mock.revoke.mockRejectedValue(new Error('卡密无效，销毁失败：该卡密已使用或上游已失效。'))
    await confirmDestroy(host); expect(host.textContent).toContain('卡密无效，销毁失败'); expect(host.textContent).not.toContain('销毁成功：')
  })
  it('ignores a late full-code response after the dialog has closed', async () => {
    let resolve!: (value: unknown) => void; mock.reveal.mockReturnValue(new Promise(done => { resolve = done }))
    const host = await mount(); button(host, '查看双码与管理').click(); await flush()
    button(host, '关闭并隐藏卡密').click(); resolve(full); await flush(); expect(host.textContent).not.toContain(full.source_code)
  })
  it('read-only permissions do not expose pair details or revoke actions', async () => {
    mock.allowed = false; const host = await mount(); expect(button(host, '查看双码与管理')).toBeUndefined(); expect(mock.reveal).not.toHaveBeenCalled()
  })
})
