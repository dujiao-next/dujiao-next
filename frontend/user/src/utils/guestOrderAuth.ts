export interface GuestOrderAuth {
  email: string
  order_password: string
}

export const MIN_GUEST_ORDER_PASSWORD_LENGTH = 12

const GUEST_ORDER_AUTH_KEY = 'guest_order_auth'
const EMPTY_GUEST_ORDER_AUTH: GuestOrderAuth = {
  email: '',
  order_password: '',
}

type GuestOrderAuthStorage = 'sessionStorage' | 'localStorage'

let volatileGuestOrderAuth: GuestOrderAuth | null = null

const normalizeGuestOrderAuth = (auth: Partial<GuestOrderAuth>): GuestOrderAuth => ({
  email: typeof auth.email === 'string' ? auth.email : '',
  order_password: typeof auth.order_password === 'string' ? auth.order_password : '',
})

const cloneGuestOrderAuth = (auth: GuestOrderAuth): GuestOrderAuth => ({ ...auth })

const parseGuestOrderAuth = (raw: string | null): GuestOrderAuth | null => {
  if (!raw) return null
  try {
    const parsed = JSON.parse(raw) as Partial<GuestOrderAuth>
    return normalizeGuestOrderAuth(parsed)
  } catch {
    return null
  }
}

const readStorage = (storageName: GuestOrderAuthStorage, key: string): string | null => {
  try {
    return window[storageName].getItem(key)
  } catch {
    return null
  }
}

const writeStorage = (storageName: GuestOrderAuthStorage, key: string, value: string) => {
  try {
    window[storageName].setItem(key, value)
  } catch {
    // 浏览器禁用存储时由当前页面内存状态继续承接。
  }
}

const removeStorage = (storageName: GuestOrderAuthStorage, key: string) => {
  try {
    window[storageName].removeItem(key)
  } catch {
    // 浏览器禁用存储时只保留当前页面内存状态。
  }
}

// loadGuestOrderAuth 优先读取当前标签页的 sessionStorage，并执行一次旧版
// localStorage -> sessionStorage 迁移。迁移后立即删除长期存储中的游客凭据。
export const loadGuestOrderAuth = (): GuestOrderAuth => {
  if (typeof window === 'undefined') {
    return cloneGuestOrderAuth(EMPTY_GUEST_ORDER_AUTH)
  }
  if (volatileGuestOrderAuth) {
    return cloneGuestOrderAuth(volatileGuestOrderAuth)
  }

  const sessionRaw = readStorage('sessionStorage', GUEST_ORDER_AUTH_KEY)
  const legacyRaw = readStorage('localStorage', GUEST_ORDER_AUTH_KEY)
  const sessionAuth = parseGuestOrderAuth(sessionRaw)
  const legacyAuth = parseGuestOrderAuth(legacyRaw)
  const parsed = sessionAuth || legacyAuth

  if (parsed) {
    volatileGuestOrderAuth = cloneGuestOrderAuth(parsed)
  }
  if (!sessionAuth && legacyAuth) {
    writeStorage('sessionStorage', GUEST_ORDER_AUTH_KEY, JSON.stringify(legacyAuth))
  }
  if (legacyRaw !== null) {
    removeStorage('localStorage', GUEST_ORDER_AUTH_KEY)
  }
  return cloneGuestOrderAuth(volatileGuestOrderAuth || EMPTY_GUEST_ORDER_AUTH)
}

export const saveGuestOrderAuth = (auth: GuestOrderAuth) => {
  if (typeof window === 'undefined') return
  const normalized = normalizeGuestOrderAuth(auth)
  volatileGuestOrderAuth = cloneGuestOrderAuth(normalized)
  writeStorage('sessionStorage', GUEST_ORDER_AUTH_KEY, JSON.stringify(normalized))
  // 不回退到 localStorage，避免把游客订单凭据重新变成长生命周期数据。
  removeStorage('localStorage', GUEST_ORDER_AUTH_KEY)
}

export const clearGuestOrderAuth = () => {
  volatileGuestOrderAuth = null
  if (typeof window === 'undefined') return
  removeStorage('sessionStorage', GUEST_ORDER_AUTH_KEY)
  removeStorage('localStorage', GUEST_ORDER_AUTH_KEY)
}
