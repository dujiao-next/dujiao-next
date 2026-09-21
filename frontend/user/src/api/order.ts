import { userApi } from './client'
import type { CreatePaymentPayload } from './types'

type GuestAuthInput = {
    email?: string
    order_password?: string
    [key: string]: any
}

function encodeGuestAuthorization(email: string, orderPassword: string): string {
    const bytes = new TextEncoder().encode(`${email.trim().toLowerCase()}\n${orderPassword.trim()}`)
    let binary = ''
    for (const byte of bytes) {
        binary += String.fromCharCode(byte)
    }
    const token = btoa(binary)
        .replace(/\+/g, '-')
        .replace(/\//g, '_')
        .replace(/=+$/g, '')
    return `Guest ${token}`
}

function splitGuestAuth(input: GuestAuthInput = {}) {
    const { email = '', order_password = '', ...payload } = input
    return {
        payload,
        headers: {
            Authorization: encodeGuestAuthorization(String(email), String(order_password)),
        },
    }
}

function withGuestAuth(input: GuestAuthInput, options: Record<string, any> = {}) {
    const { payload, headers } = splitGuestAuth(input)
    return {
        payload,
        options: {
            ...options,
            headers: { ...(options.headers || {}), ...headers },
        },
    }
}

export const userOrderAPI = {
    preview: (data: any) => userApi.post('/orders/preview', data),
    getPaymentChannels: (data: any) => userApi.post('/order/payment-channels', data),
    create: (data: any) => userApi.post('/orders', data),
    createAndPay: (data: any) => userApi.post('/orders/create-and-pay', data),
    list: (params?: any) => userApi.get('/orders', { params }),
    stats: (params?: any) => userApi.get('/orders/stats', { params }),
    detail: (orderNo: string, options?: any) => userApi.get(`/orders/${encodeURIComponent(orderNo)}`, options),
    cancel: (orderNo: string) => userApi.post(`/orders/${encodeURIComponent(orderNo)}/cancel`),
    downloadFulfillment: (orderNo: string) => userApi.get(`/orders/${encodeURIComponent(orderNo)}/fulfillment/download`, { blob: true }),
}

export const guestOrderAPI = {
    preview: (data: any) => userApi.post('/guest/orders/preview', data),
    create: (data: any) => userApi.post('/guest/orders', data),
    createAndPay: (data: any) => userApi.post('/guest/orders/create-and-pay', data),
    browserOrders: (params?: any) => userApi.get('/guest/orders/browser', {
        params,
        credentials: 'include',
    }),
    list: (params: GuestAuthInput) => {
        const request = withGuestAuth(params)
        return userApi.get('/guest/orders', { ...request.options, params: request.payload })
    },
    detail: (orderNo: string, params: GuestAuthInput, options?: any) => {
        const request = withGuestAuth(params, options)
        return userApi.get(`/guest/orders/${encodeURIComponent(orderNo)}`, { ...request.options, params: request.payload })
    },
    downloadFulfillment: (orderNo: string, params: GuestAuthInput) => {
        const request = withGuestAuth(params, { blob: true })
        return userApi.get(`/guest/orders/${encodeURIComponent(orderNo)}/fulfillment/download`, { ...request.options, params: request.payload })
    },
    cancel: (orderNo: string, data: GuestAuthInput) => {
        const request = withGuestAuth(data)
        return userApi.post(`/guest/orders/${encodeURIComponent(orderNo)}/cancel`, request.payload, request.options)
    },
    createPayment: (data: GuestAuthInput) => {
        const request = withGuestAuth(data)
        return userApi.post('/guest/payments', request.payload, request.options)
    },
    capturePayment: (id: number, data: GuestAuthInput) => {
        const request = withGuestAuth(data)
        return userApi.post(`/guest/payments/${id}/capture`, request.payload, request.options)
    },
    latestPayment: (params: GuestAuthInput) => {
        const request = withGuestAuth(params, { silentBusinessError: true })
        return userApi.get('/guest/payments/latest', { ...request.options, params: request.payload })
    },
}

export const paymentAPI = {
    create: (data: CreatePaymentPayload) => userApi.post('/payments', data),
    capture: (id: number) => userApi.post(`/payments/${id}/capture`),
    latest: (params: any) => userApi.get('/payments/latest', { params, silentBusinessError: true }),
}
