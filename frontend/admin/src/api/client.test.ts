import { afterEach, describe, expect, it, vi } from 'vitest'
import { api } from './client'

vi.mock('@/utils/notify', () => ({ notifyError: vi.fn() }))

afterEach(() => vi.unstubAllGlobals())

describe('payment compliance responses', () => {
  for (const status of [200, 403]) {
    for (const code of ['compliance_required', 'compliance_required_by_super_admin']) {
      it(`translates ${code} for HTTP ${status} and retains its code`, async () => {
        vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ status_code: 403, msg: code }), { status, headers: { 'Content-Type': 'application/json' } })))
        const error = await api.post('/admin/users/1/wallet/adjust', { amount: '100' }).catch((err: Error) => err)
        expect(error).toBeInstanceOf(Error)
        expect(error).toHaveProperty('code', code)
        expect((error as Error).message).not.toContain('compliance_required')
        expect((error as Error).message.length).toBeGreaterThan(10)
      })
    }
  }
})
