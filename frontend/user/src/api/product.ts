import { api } from './client'

export const productAPI = {
    list: (params?: any) => api.get('/public/products', { params }),
    detail: (slug: string) => api.get(`/public/products/${slug}`),
}

export const postAPI = {
    list: (params?: any) => api.get('/public/posts', { params }),
    // opts 可传 { silentBusinessError: true }，用于「没有这个页面」这类预期内的 404
    detail: (slug: string, opts?: Record<string, unknown>) => api.get(`/public/posts/${slug}`, opts),
}

export const bannerAPI = {
    list: (params?: any) => api.get('/public/banners', { params }),
}

export const categoryAPI = {
    list: (params?: any) => api.get('/public/categories', { params }),
}

export const memberLevelAPI = {
    list: () => api.get('/public/member-levels'),
}
