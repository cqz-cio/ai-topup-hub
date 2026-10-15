import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createApp, h, nextTick, type App } from 'vue'
import WalletWithdraws from './WalletWithdraws.vue'

const mock = vi.hoisted(() => ({ list: vi.fn(), review: vi.fn(), error: vi.fn(), allowed: true }))
vi.mock('@/api/admin', () => ({ adminAPI: { getWalletWithdraws: mock.list, reviewWalletWithdraw: mock.review } }))
vi.mock('@/stores/auth', () => ({ useAdminAuthStore: () => ({ hasPermission: () => mock.allowed }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/utils/notify', () => ({ notifyError: mock.error, notifySuccess: vi.fn() }))
// Only the isolated component fixture skips the declaration dialog; the real
// route and API retain the compliance guard and financial permissions.
vi.mock('@/components/ComplianceGuardWrapper.vue', () => ({ default: { template: '<slot />' } }))
vi.mock('@/components/ui/dialog', () => {
 const slot = { template: '<div><slot /></div>' }
 return { Dialog: { props: ['open'], template: '<div v-if="open"><slot /></div>' }, DialogContent: slot, DialogDescription: slot, DialogFooter: slot, DialogHeader: slot, DialogTitle: slot }
})
let app: App | undefined
afterEach(() => { app?.unmount(); document.body.replaceChildren() })
beforeEach(() => { vi.clearAllMocks(); mock.allowed = true; mock.review.mockResolvedValue({ data: {} }) })
async function flush() { await Promise.resolve(); await nextTick(); await Promise.resolve(); await nextTick() }
async function mount(status = 'pending', debt = '0.00') {
 mock.list.mockResolvedValue({ data: { data: [{ id: 7, user_id: 11, amount: '20.00', channel: 'wechat', account: 'test-recipient', real_name: 'Local fixture', status, created_at: '2026-10-15T00:00:00Z', wallet: { earnings_debt: debt } }], pagination: { total_page: 1 } } })
 const host = document.createElement('div'); document.body.append(host)
 app = createApp({ render: () => h(WalletWithdraws) }); app.mount(host); await flush()
 return host
}
function button(host: HTMLElement, key: string) { return [...host.querySelectorAll('button')].find(el => el.textContent?.trim() === `walletEarnings.${key}`) }
describe('manual wallet withdrawal review', () => {
 it('requires a payout reference before recording manual payment', async () => {
  const host = await mount('approved')
  button(host, 'actions.paid')!.click(); await flush()
  button(host, 'confirm')!.click(); await flush()
  expect(mock.review).not.toHaveBeenCalled(); expect(mock.error).toHaveBeenCalledWith('walletEarnings.required')
  const input = host.querySelector<HTMLInputElement>('#payout-ref')!; input.value = 'MANUAL-TEST-001'; input.dispatchEvent(new Event('input')); await flush()
  button(host, 'confirm')!.click(); await flush()
  expect(mock.review).toHaveBeenCalledExactlyOnceWith(7, { action: 'paid', payout_reference: 'MANUAL-TEST-001', remark: '' })
 })
 it('requires a reason to reject and cannot mark a pending request paid', async () => {
  const host = await mount()
  expect(button(host, 'actions.paid')).toBeUndefined()
  button(host, 'actions.reject')!.click(); await flush(); button(host, 'confirm')!.click(); await flush()
  expect(mock.review).not.toHaveBeenCalled()
  const input = host.querySelector<HTMLInputElement>('#withdraw-remark')!; input.value = 'Incorrect recipient'; input.dispatchEvent(new Event('input')); await flush()
  button(host, 'confirm')!.click(); await flush()
  expect(mock.review).toHaveBeenCalledWith(7, { action: 'reject', payout_reference: '', remark: 'Incorrect recipient' })
 })
 it('hides review controls for read-only staff', async () => {
  mock.allowed = false; const host = await mount()
  expect(button(host, 'actions.approve')).toBeUndefined(); expect(button(host, 'actions.reject')).toBeUndefined()
 })
 it('blocks approval with refund debt while allowing rejection', async () => {
  const host = await mount('pending', '5.00')
  expect(button(host, 'actions.approve')?.disabled).toBe(true)
  expect(button(host, 'actions.reject')?.disabled).toBe(false)
 })
})
