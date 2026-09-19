import { productAPI } from '../api'

const detailRequests = new Map<string, Promise<any>>()

export function prefetchProductDetail(slug: string): Promise<any> {
  const normalized = String(slug || '').trim()
  if (!normalized) return Promise.resolve(null)
  const existing = detailRequests.get(normalized)
  if (existing) return existing
  const request = productAPI.detail(normalized)
    .then((response) => response.data.data || null)
    .catch((error) => {
      detailRequests.delete(normalized)
      throw error
    })
  detailRequests.set(normalized, request)
  return request
}

export function takeProductDetailRequest(slug: string): Promise<any> {
  return prefetchProductDetail(slug)
}
