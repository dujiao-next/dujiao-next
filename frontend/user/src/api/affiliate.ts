import { api, userApi } from './client'
import type { AffiliateWithdrawApplyPayload } from './types'

export const affiliateAPI = {
    trackClick: (data: { affiliate_code: string; visitor_key?: string; landing_path?: string; referrer?: string }) =>
        api.post('/public/affiliate/click', data),
    open: () => userApi.post('/affiliate/open'),
    dashboard: () => userApi.get('/affiliate/dashboard'),
    productRates: (productId: number) => userApi.get(`/affiliate/products/${productId}/rates`),
    commissions: (params?: any) => userApi.get('/affiliate/commissions', { params }),
    withdraws: (params?: any) => userApi.get('/affiliate/withdraws', { params }),
    applyWithdraw: (data: AffiliateWithdrawApplyPayload) =>
        userApi.post('/affiliate/withdraws', data),
}
