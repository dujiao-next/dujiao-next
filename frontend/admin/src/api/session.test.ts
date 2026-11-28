import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/stores/auth', () => ({
  useAdminAuthStore: () => ({ token: '' }),
}))

const TOKEN_KEY = 'admin_token'

// session.ts 持有模块级的单飞状态，每个用例重新加载以隔离。
async function loadSession() {
  vi.resetModules()
  return import('./session')
}

const jsonResponse = (body: unknown) => ({ ok: true, json: async () => body })

beforeEach(() => {
  localStorage.clear()
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('recoverSession', () => {
  it('续期成功时写入并返回新 token', async () => {
    localStorage.setItem(TOKEN_KEY, 'old-token')
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse({ status_code: 0, data: { token: 'new-token' } }))
    vi.stubGlobal('fetch', fetchMock)

    const { recoverSession } = await loadSession()

    await expect(recoverSession()).resolves.toBe('new-token')
    expect(localStorage.getItem(TOKEN_KEY)).toBe('new-token')
    expect(fetchMock).toHaveBeenCalledTimes(1)
    const firstCall = fetchMock.mock.calls[0]
    expect(String(firstCall?.[0])).toContain('/admin/auth/refresh')
  })

  it('续期失败时回落到原地重新登录', async () => {
    localStorage.setItem(TOKEN_KEY, 'expired-token')
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, json: async () => ({}) }))

    const { recoverSession, registerReloginHandler } = await loadSession()
    registerReloginHandler(async () => {
      localStorage.setItem(TOKEN_KEY, 'relogged-token')
      return true
    })

    await expect(recoverSession()).resolves.toBe('relogged-token')
  })

  it('用户放弃登录时返回 null 且不篡改现有 token', async () => {
    localStorage.setItem(TOKEN_KEY, 'expired-token')
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, json: async () => ({}) }))

    const { recoverSession, registerReloginHandler } = await loadSession()
    registerReloginHandler(async () => false)

    await expect(recoverSession()).resolves.toBeNull()
    expect(localStorage.getItem(TOKEN_KEY)).toBe('expired-token')
  })

  it('body 里 status_code=401 也视为续期失败', async () => {
    localStorage.setItem(TOKEN_KEY, 'expired-token')
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(jsonResponse({ status_code: 401, msg: 'session expired' })),
    )

    const { recoverSession, registerReloginHandler } = await loadSession()
    const handler = vi.fn(async () => false)
    registerReloginHandler(handler)

    await expect(recoverSession()).resolves.toBeNull()
    expect(handler).toHaveBeenCalledTimes(1)
  })

  it('没有 token 时不发续期请求', async () => {
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)

    const { recoverSession, registerReloginHandler } = await loadSession()
    registerReloginHandler(async () => false)

    await expect(recoverSession()).resolves.toBeNull()
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('并发调用共享同一次续期（单飞）', async () => {
    localStorage.setItem(TOKEN_KEY, 'old-token')
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse({ status_code: 0, data: { token: 'new-token' } }))
    vi.stubGlobal('fetch', fetchMock)

    const { recoverSession } = await loadSession()

    const first = recoverSession()
    const second = recoverSession()
    expect(second).toBe(first)

    await expect(first).resolves.toBe('new-token')
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('续期请求抛异常时回落到重新登录', async () => {
    localStorage.setItem(TOKEN_KEY, 'old-token')
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('network down')))

    const { recoverSession, registerReloginHandler } = await loadSession()
    registerReloginHandler(async () => {
      localStorage.setItem(TOKEN_KEY, 'relogged-token')
      return true
    })

    await expect(recoverSession()).resolves.toBe('relogged-token')
  })
})
