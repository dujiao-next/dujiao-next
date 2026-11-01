import i18n from '../i18n'
import { isPublicAuthEndpoint } from '../utils/authEndpoints'

export const t = (key: string, params?: Record<string, any>) =>
    (params ? i18n.global.t(key, params) : i18n.global.t(key)) as string

export interface ApiResponse<T = any> {
    status_code: number
    msg: string
    data: T
    pagination?: {
        page: number
        page_size: number
        total: number
        total_page: number
    }
}

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || ''
const API_PREFIX = '/api/v1'

interface RequestOptions {
    params?: Record<string, any>
    headers?: Record<string, string>
    blob?: boolean
    silentBusinessError?: boolean
    credentials?: RequestCredentials
    [key: string]: any
}

function buildUrl(base: string, path: string, params?: Record<string, any>): string {
    const url = `${base}${path}`
    if (!params) return url
    const searchParams = new URLSearchParams()
    for (const [key, value] of Object.entries(params)) {
        if (value !== undefined && value !== null) {
            searchParams.append(key, String(value))
        }
    }
    const qs = searchParams.toString()
    return qs ? `${url}?${qs}` : url
}

function getLocale(): string {
    return (i18n.global.locale as any).value || i18n.global.locale || ''
}

function getHttpErrorMessage(status: number): string {
    switch (status) {
        case 401: return t('common.api.unauthorized')
        case 403: return t('common.api.forbidden')
        case 404: return t('common.api.notFound')
        case 500: return t('common.api.serverError')
        case 502: return t('common.api.badGateway')
        case 503: return t('common.api.serviceUnavailable')
        default: return t('common.api.requestFailedStatus', { status })
    }
}

function createClient(injectAuth: boolean) {
    const baseURL = `${API_BASE_URL}${API_PREFIX}`
    const timeout = 10000

    async function request(method: string, path: string, bodyOrOptions?: any, options?: RequestOptions): Promise<{ data: any }> {
        let body: any = undefined
        let opts: RequestOptions = {}

        if (method === 'GET' || method === 'DELETE') {
            opts = bodyOrOptions || {}
        } else {
            body = bodyOrOptions
            opts = options || {}
        }

        const url = buildUrl(baseURL, path, opts.params)
        const headers: Record<string, string> = { ...opts.headers }

        const locale = getLocale()
        if (locale) {
            headers['X-Lang'] = locale
        }

        if (injectAuth) {
            const token = localStorage.getItem('user_token')
            if (token && !headers['Authorization']) {
                headers['Authorization'] = `Bearer ${token}`
            }
        }

        if (body !== undefined && !(body instanceof FormData)) {
            headers['Content-Type'] = 'application/json'
        }

        const controller = new AbortController()
        const timer = setTimeout(() => controller.abort(), timeout)

        let response: Response
        try {
            response = await fetch(url, {
                method,
                headers,
                body: body instanceof FormData ? body : body !== undefined ? JSON.stringify(body) : undefined,
                credentials: opts.credentials,
                cache: opts.cache,
                signal: controller.signal,
            })
        } catch (err: any) {
            if (err.name === 'AbortError') {
                const message = t('common.api.networkError')
                console.error('Request Timeout:', message)
                return Promise.reject(new Error(message))
            }
            const message = t('common.api.networkError')
            console.error('Network Error:', message)
            return Promise.reject(new Error(message))
        } finally {
            clearTimeout(timer)
        }

        // Blob response
        if (opts.blob) {
            if (!response.ok) {
                const message = getHttpErrorMessage(response.status)
                console.error('HTTP Error:', message)
                return Promise.reject(new Error(message))
            }
            const blob = await response.blob()
            return { data: blob }
        }

        // Parse JSON
        let data: ApiResponse
        try {
            data = await response.json()
        } catch {
            if (!response.ok) {
                const status = response.status
                const message = getHttpErrorMessage(status)
                if (status === 401) {
                    if (injectAuth && !isPublicAuthEndpoint(path)) {
                        localStorage.removeItem('user_token')
                        localStorage.removeItem('user_profile')
                        window.location.href = '/auth/login'
                    }
                }
                console.error('HTTP Error:', message)
                return Promise.reject(new Error(message))
            }
            console.error('API Error: No response data')
            return Promise.reject(new Error(t('common.api.responseMissing')))
        }

        // HTTP error with JSON body
        if (!response.ok) {
            const status = response.status
            const message = data?.msg || getHttpErrorMessage(status)
            if (status === 401 && injectAuth && !isPublicAuthEndpoint(path)) {
                localStorage.removeItem('user_token')
                localStorage.removeItem('user_profile')
                window.location.href = '/auth/login'
            }
            console.error('HTTP Error:', message)
            return Promise.reject(new Error(message))
        }

        // Business error check
        if (typeof data.status_code !== 'undefined' && data.status_code !== 0) {
            if (data.status_code === 401 && injectAuth && !isPublicAuthEndpoint(path)) {
                localStorage.removeItem('user_token')
                localStorage.removeItem('user_profile')
                window.location.href = '/auth/login'
                return Promise.reject(new Error(t('common.api.unauthorized')))
            }
            const fallbackMessage = t('common.api.requestFailed')
            const errorMessage = data.msg || fallbackMessage
            const silentBusinessError = Boolean(opts.silentBusinessError)
            if (!silentBusinessError) {
                console.error('API Error:', errorMessage)
            }
            const businessError = new Error(errorMessage) as Error & { silentBusinessError?: boolean }
            businessError.silentBusinessError = silentBusinessError
            return Promise.reject(businessError)
        }

        return { data }
    }

    return {
        get: (path: string, options?: RequestOptions) => request('GET', path, options),
        post: (path: string, body?: any, options?: RequestOptions) => request('POST', path, body, options),
        put: (path: string, body?: any, options?: RequestOptions) => request('PUT', path, body, options),
        patch: (path: string, body?: any, options?: RequestOptions) => request('PATCH', path, body, options),
        delete: (path: string, options?: RequestOptions) => request('DELETE', path, options),
    }
}

const api = createClient(false)
const userApi = createClient(true)

export { api, userApi }
export default api
