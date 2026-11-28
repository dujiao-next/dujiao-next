// 管理端会话恢复。
//
// token 失效时不再整页跳转登录页（那会销毁编辑器里的未提交内容），而是：
// 1) 先静默调用 /admin/auth/refresh 续期；
// 2) 续期失败（token 过期/被吊销/超过会话上限）再原地弹出重新登录；
// 3) 用户放弃登录时返回 null，页面状态原样保留。

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || ''
const API_PREFIX = '/api/v1'
const TOKEN_KEY = 'admin_token'

export type ReloginHandler = () => Promise<boolean>

let reloginHandler: ReloginHandler | null = null
let recoverPromise: Promise<string | null> | null = null

/** 由 ReloginDialogHost 注册，保证同一时刻只有一个重新登录弹窗。 */
export const registerReloginHandler = (handler: ReloginHandler | null) => {
  reloginHandler = handler
}

const readToken = () => localStorage.getItem(TOKEN_KEY) || ''

// 只读 JWT 的 exp 用于安排续期时间，不做任何签名校验（校验在服务端）。
const readExpiryMs = (token: string): number | null => {
  const payload = token.split('.')[1]
  if (!payload) return null
  try {
    const normalized = payload.replace(/-/g, '+').replace(/_/g, '/')
    const padded = normalized + '='.repeat((4 - (normalized.length % 4)) % 4)
    const claims = JSON.parse(atob(padded))
    return typeof claims?.exp === 'number' ? claims.exp * 1000 : null
  } catch {
    return null
  }
}

const REFRESH_AHEAD_MS = 5 * 60 * 1000
const MAX_TIMER_MS = 24 * 60 * 60 * 1000

let keepAliveTimer: ReturnType<typeof setTimeout> | null = null

/**
 * 在 token 到期前提前续期，让会话在浏览器标签页开着时保持登录态。
 * 续期仍受服务端 max_session_hours 绝对上限约束，超过上限会走重新登录。
 */
export function scheduleSessionKeepAlive() {
  if (keepAliveTimer) {
    clearTimeout(keepAliveTimer)
    keepAliveTimer = null
  }
  const token = readToken()
  const expiresAt = token ? readExpiryMs(token) : null
  if (!expiresAt) return

  const delay = expiresAt - Date.now() - REFRESH_AHEAD_MS
  if (delay <= 0) return

  keepAliveTimer = setTimeout(() => {
    keepAliveTimer = null
    void recoverSession().finally(() => scheduleSessionKeepAlive())
  }, Math.min(delay, MAX_TIMER_MS))
}

// 后台标签页的定时器会被浏览器节流，回到前台时补一次检查。
if (typeof document !== 'undefined') {
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState !== 'visible') return
    const token = readToken()
    const expiresAt = token ? readExpiryMs(token) : null
    if (!expiresAt) return
    if (expiresAt - Date.now() < REFRESH_AHEAD_MS * 2) {
      void recoverSession().finally(() => scheduleSessionKeepAlive())
    }
  })
}

// 续期后同步 pinia 里的 token，避免路由守卫和后续逻辑读到旧值。
const syncAuthStore = async (token: string) => {
  try {
    const { useAdminAuthStore } = await import('@/stores/auth')
    useAdminAuthStore().token = token
  } catch {
    // pinia 未就绪时忽略：所有接口请求都从 localStorage 读取 token
  }
}

async function requestRefresh(): Promise<string | null> {
  const token = readToken()
  if (!token) return null

  try {
    const response = await fetch(`${API_BASE_URL}${API_PREFIX}/admin/auth/refresh`, {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${token}`,
        'Content-Type': 'application/json',
      },
      body: '{}',
    })
    if (!response.ok) return null

    const payload = await response.json().catch(() => null)
    const next = payload?.data?.token
    if (payload?.status_code !== 0 || typeof next !== 'string' || !next) return null

    localStorage.setItem(TOKEN_KEY, next)
    await syncAuthStore(next)
    return next
  } catch {
    return null
  }
}

async function doRecover(): Promise<string | null> {
  const refreshed = await requestRefresh()
  if (refreshed) {
    scheduleSessionKeepAlive()
    return refreshed
  }
  if (!reloginHandler) return null

  const ok = await reloginHandler()
  const token = ok ? readToken() || null : null
  if (token) scheduleSessionKeepAlive()
  return token
}

/**
 * 恢复登录态：静默续期优先，失败再原地重新登录。
 * 返回可用 token；用户放弃登录时返回 null。
 */
export function recoverSession(): Promise<string | null> {
  if (recoverPromise) return recoverPromise
  recoverPromise = doRecover().finally(() => {
    recoverPromise = null
  })
  return recoverPromise
}
