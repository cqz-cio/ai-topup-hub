import { userApi } from './client'
import type {
    WalletRechargePayload,
    CaptchaPayload,
} from './types'

export const walletAPI = {
	withdrawPolicy: () => userApi.get('/wallet/withdraw-policy'),
	withdraws: (params?: { page?: number; page_size?: number }) => userApi.get('/wallet/withdraws', { params }),
	applyWithdraw: (data: { amount: string; channel: string; account: string; real_name: string; request_key: string }) => userApi.post('/wallet/withdraws', data),
    getPaymentChannels: (amount: string) => userApi.post('/wallet/payment-channels', { amount }),
    account: () => userApi.get('/wallet'),
    transactions: (params?: any) => userApi.get('/wallet/transactions', { params }),
    recharge: (data: WalletRechargePayload) => userApi.post('/wallet/recharge', data),
    rechargeOrders: (params?: any) => userApi.get('/wallet/recharges', { params }),
    rechargeStats: (params?: any) => userApi.get('/wallet/recharges/stats', { params }),
    rechargeDetail: (rechargeNo: string) =>
        userApi.get(`/wallet/recharges/${encodeURIComponent(rechargeNo)}`),
    captureRechargePayment: (paymentID: number) =>
        userApi.post(`/wallet/recharge/payments/${paymentID}/capture`),
}

export const giftCardAPI = {
    redeem: (data: { code: string; captcha_payload?: CaptchaPayload }) =>
        userApi.post('/gift-cards/redeem', data),
}
