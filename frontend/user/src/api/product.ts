import { api } from './client'

export const productAPI = {
    list: (params?: any) => api.get('/public/products', { params }),
    detail: (slug: string) => api.get(`/public/products/${slug}`),
}

export const postAPI = {
    list: (params?: any) => api.get('/public/posts', { params }),
    detail: (slug: string) => api.get(`/public/posts/${slug}`),
}

export const bannerAPI = {
    list: (params?: any) => api.get('/public/banners', { params }),
}

/**
 * 分类 slug 归一化：去掉前后空白。
 *
 * 后台的分类表单没有 trim，slug 可能被存成带前后空格 —— 线上就有一个
 * `"Firstmail邮箱 "`。slug 是 URL 片段，空白永远没有意义；但各消费方都是拿
 * `route.params.slug.trim()` 去比 `c.slug`，一边 trim 一边不 trim，结果就是
 * 这个分类永远匹配不上：点它只会看到「全部商品」，表现上像「跳错了页面」。
 *
 * 放在 API 出口统一处理，而不是让 Home / Products / AtlasLayout /
 * useProductList / vault Home 这 5 个消费方各自 trim 一遍。
 */
const trimCategorySlug = (item: any) =>
    item && typeof item.slug === 'string' ? { ...item, slug: item.slug.trim() } : item

export const categoryAPI = {
    list: (params?: any) =>
        api.get('/public/categories', { params }).then((res: any) => {
            const list = res?.data?.data
            if (Array.isArray(list)) res.data.data = list.map(trimCategorySlug)
            return res
        }),
}

export const memberLevelAPI = {
    list: () => api.get('/public/member-levels'),
}
