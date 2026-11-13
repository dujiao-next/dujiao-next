import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createApp, nextTick, type App } from 'vue'
import AdminAPITokens from './AdminAPITokens.vue'
import { adminAPI } from '@/api/admin'
import { confirmAction } from '@/utils/confirm'

const permissions = vi.hoisted(() => ({ rotate: true }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/stores/auth', () => ({ useAdminAuthStore: () => ({
  hasPermission: (permission: string) => !permission.endsWith('/rotate') || permissions.rotate,
}) }))
vi.mock('@/api/admin', () => ({
  adminAPI: { listAdminAPITokens: vi.fn(), createAdminAPIToken: vi.fn(), revokeAdminAPIToken: vi.fn(), rotateAdminAPIToken: vi.fn() },
}))
vi.mock('@/utils/confirm', () => ({ confirmAction: vi.fn() }))
vi.mock('@/utils/notify', () => ({ notifyError: vi.fn(), notifySuccess: vi.fn() }))
vi.mock('@/utils/format', () => ({ formatDate: (value: string) => value }))
vi.mock('@/utils/clipboard', () => ({ copyText: vi.fn() }))

let app: App | undefined
let root: HTMLDivElement
const item = {
  id: 7, admin_id: 2, name: 'inventory', status: 'active',
  ip_whitelist: ['198.51.100.0/24'], revoked_at: null, last_used_at: null,
  created_at: '2026-11-13T00:00:00Z', expires_at: '2027-01-01T00:00:00Z',
}

function mount(isSuper = false) {
  root = document.createElement('div')
  document.body.appendChild(root)
  app = createApp(AdminAPITokens, { admin: { id: 2, username: 'staff', is_super: isSuper } })
  app.mount(root)
}

function button(key: string) {
  const result = [...root.querySelectorAll('button')].find((node) => node.textContent?.includes(key))
  expect(result).toBeDefined()
  return result!
}

beforeEach(() => {
  vi.clearAllMocks()
  permissions.rotate = true
  vi.mocked(adminAPI.listAdminAPITokens).mockResolvedValue({ data: { data: [] } } as never)
})

afterEach(() => {
  app?.unmount()
  app = undefined
  root?.remove()
})

describe('subaccount API token management', () => {
  it('creates a token without a financial switch and dismisses the one-time secret', async () => {
    vi.mocked(adminAPI.createAdminAPIToken).mockResolvedValue({ data: { data: { token: item, secret: 'test-one-time-secret' } } } as never)
    mount()
    await vi.waitFor(() => expect(adminAPI.listAdminAPITokens).toHaveBeenCalledWith(2))
    const name = root.querySelector<HTMLInputElement>('#admin-api-token-name')!
    name.value = 'inventory'
    name.dispatchEvent(new Event('input', { bubbles: true }))
    await nextTick()
    root.querySelector('form')!.dispatchEvent(new Event('submit', { cancelable: true, bubbles: true }))
    await vi.waitFor(() => expect(root.textContent).toContain('test-one-time-secret'))
    expect(adminAPI.createAdminAPIToken).toHaveBeenCalledWith(2, expect.objectContaining({
      name: 'inventory', ip_whitelist: [],
    }))
    expect(vi.mocked(adminAPI.createAdminAPIToken).mock.calls[0]![1]).not.toHaveProperty('allow_financial_writes')
    expect(root.querySelector('#admin-api-token-financial')).toBeNull()
    expect(root.querySelector('form')).toBeNull()
    expect(document.activeElement).toBe(root.querySelector('h3'))
    button('adminApiTokens.saved').click()
    await nextTick()
    expect(root.textContent).not.toContain('test-one-time-secret')
    expect(root.querySelector('form')).not.toBeNull()
    expect(document.activeElement).toBe(root.querySelector('#admin-api-token-name'))
  })

  it('sends one IP or CIDR per non-empty line and displays the persisted rules', async () => {
    vi.mocked(adminAPI.listAdminAPITokens).mockResolvedValue({ data: { data: [item] } } as never)
    vi.mocked(adminAPI.createAdminAPIToken).mockResolvedValue({ data: { data: { token: item, secret: 'one-time-value' } } } as never)
    mount()
    await vi.waitFor(() => expect(root.textContent).toContain('198.51.100.0/24'))
    const name = root.querySelector<HTMLInputElement>('#admin-api-token-name')!
    name.value = 'restricted'
    name.dispatchEvent(new Event('input', { bubbles: true }))
    const rules = root.querySelector<HTMLTextAreaElement>('#admin-api-token-ip-whitelist')!
    rules.value = ' 203.0.113.10 \n198.51.100.0/24\n\n2001:db8::/32\n'
    rules.dispatchEvent(new Event('input', { bubbles: true }))
    await nextTick()
    root.querySelector('form')!.dispatchEvent(new Event('submit', { cancelable: true, bubbles: true }))
    await vi.waitFor(() => expect(adminAPI.createAdminAPIToken).toHaveBeenCalledWith(2, expect.objectContaining({
      ip_whitelist: ['203.0.113.10', '198.51.100.0/24', '2001:db8::/32'],
    })))
  })

  it('requires confirmation before revoking the selected account token', async () => {
    vi.mocked(adminAPI.listAdminAPITokens).mockResolvedValue({ data: { data: [item] } } as never)
    vi.mocked(adminAPI.revokeAdminAPIToken).mockResolvedValue({ data: {} } as never)
    mount()
    await vi.waitFor(() => expect(root.textContent).toContain('inventory'))
    vi.mocked(confirmAction).mockResolvedValueOnce(false)
    button('adminApiTokens.revoke').click()
    await nextTick()
    expect(adminAPI.revokeAdminAPIToken).not.toHaveBeenCalled()
    vi.mocked(confirmAction).mockResolvedValueOnce(true)
    button('adminApiTokens.revoke').click()
    await vi.waitFor(() => expect(adminAPI.revokeAdminAPIToken).toHaveBeenCalledWith(2, 7))
    expect(confirmAction).toHaveBeenLastCalledWith(expect.objectContaining({
      title: 'adminApiTokens.revokeTitle',
      confirmText: 'adminApiTokens.revokeTitle',
    }))
  })

  it('allows historical token revocation but no issuance for a promoted super admin', async () => {
    vi.mocked(adminAPI.listAdminAPITokens).mockResolvedValue({ data: { data: [item] } } as never)
    mount(true)
    await vi.waitFor(() => expect(root.textContent).toContain('inventory'))
    expect(root.querySelector('form')).toBeNull()
    expect(root.textContent).toContain('adminApiTokens.superHint')
    expect(button('adminApiTokens.revoke')).toBeDefined()
  })

  it('shows a recoverable load error instead of an empty account', async () => {
    vi.mocked(adminAPI.listAdminAPITokens).mockRejectedValueOnce(new Error('offline'))
    mount()
    await vi.waitFor(() => expect(root.querySelector('[role="alert"]')?.textContent).toContain('adminApiTokens.loadFailed'))
    expect(root.textContent).not.toContain('adminApiTokens.empty')
    vi.mocked(adminAPI.listAdminAPITokens).mockResolvedValueOnce({ data: { data: [item] } } as never)
    button('adminApiTokens.retry').click()
    await vi.waitFor(() => expect(root.textContent).toContain('inventory'))
    expect(root.querySelector('[role="alert"]')).toBeNull()
  })

  it('retains existing records when refreshing fails', async () => {
    vi.mocked(adminAPI.listAdminAPITokens).mockResolvedValueOnce({ data: { data: [item] } } as never)
    mount()
    await vi.waitFor(() => expect(root.textContent).toContain('inventory'))
    vi.mocked(adminAPI.listAdminAPITokens).mockRejectedValueOnce(new Error('offline'))
    button('adminApiTokens.refresh').click()
    await vi.waitFor(() => expect(root.querySelector('[role="alert"]')).not.toBeNull())
    expect(root.textContent).toContain('inventory')
  })

  it('keeps the newly created secret when a different token is revoked', async () => {
    vi.mocked(adminAPI.listAdminAPITokens).mockResolvedValue({ data: { data: [item] } } as never)
    vi.mocked(adminAPI.createAdminAPIToken).mockResolvedValue({ data: { data: { token: { ...item, id: 8 }, secret: 'fixture-new-secret' } } } as never)
    vi.mocked(adminAPI.revokeAdminAPIToken).mockResolvedValue({ data: {} } as never)
    vi.mocked(confirmAction).mockResolvedValue(true)
    mount()
    await vi.waitFor(() => expect(root.textContent).toContain('inventory'))
    const name = root.querySelector<HTMLInputElement>('#admin-api-token-name')!
    name.value = 'new token'
    name.dispatchEvent(new Event('input', { bubbles: true }))
    await nextTick()
    root.querySelector('form')!.dispatchEvent(new Event('submit', { cancelable: true, bubbles: true }))
    await vi.waitFor(() => expect(root.textContent).toContain('fixture-new-secret'))
    await vi.waitFor(() => expect(button('adminApiTokens.revoke').disabled).toBe(false))
    button('adminApiTokens.revoke').click()
    await vi.waitFor(() => expect(adminAPI.revokeAdminAPIToken).toHaveBeenCalledWith(2, 7))
    await nextTick()
    expect(root.textContent).toContain('fixture-new-secret')
  })

  it('keeps long IP lists collapsed with an explicit disclosure', async () => {
    const rules = Array.from({ length: 100 }, (_, i) => `2001:db8:${i.toString(16)}::/48`)
    vi.mocked(adminAPI.listAdminAPITokens).mockResolvedValue({ data: { data: [{ ...item, ip_whitelist: rules }] } } as never)
    mount()
    await vi.waitFor(() => expect(root.querySelector('details')).not.toBeNull())
    const details = root.querySelector('details')!
    expect(details.open).toBe(false)
    expect(details.querySelector('summary')?.textContent).toBe('adminApiTokens.showRules')
    expect(details.querySelectorAll('li')).toHaveLength(100)
  })

  it('confirms rotation and presents the replacement secret only once', async () => {
    vi.mocked(adminAPI.listAdminAPITokens).mockResolvedValue({ data: { data: [item] } } as never)
    vi.mocked(adminAPI.rotateAdminAPIToken).mockResolvedValue({ data: { data: { token: item, secret: 'rotated-fixture' } } } as never)
    mount()
    await vi.waitFor(() => expect(root.textContent).toContain('inventory'))
    vi.mocked(confirmAction).mockResolvedValueOnce(false)
    button('adminApiTokens.rotate').click()
    await nextTick()
    expect(adminAPI.rotateAdminAPIToken).not.toHaveBeenCalled()
    vi.mocked(confirmAction).mockResolvedValueOnce(true)
    button('adminApiTokens.rotate').click()
    await vi.waitFor(() => expect(root.textContent).toContain('rotated-fixture'))
    expect(adminAPI.rotateAdminAPIToken).toHaveBeenCalledWith(2, item.id)
    expect(confirmAction).toHaveBeenLastCalledWith(expect.objectContaining({
      title: 'adminApiTokens.rotateTitle', description: 'adminApiTokens.rotateConfirm',
    }))
    expect(document.activeElement).toBe(root.querySelector('h3'))
    expect(button('adminApiTokens.rotate').disabled).toBe(true)
    button('adminApiTokens.saved').click()
    await nextTick()
    expect(root.textContent).not.toContain('rotated-fixture')
  })

  it('preserves the list and permits retry when rotation fails', async () => {
    vi.mocked(adminAPI.listAdminAPITokens).mockResolvedValue({ data: { data: [item] } } as never)
    vi.mocked(adminAPI.rotateAdminAPIToken).mockRejectedValueOnce(new Error('offline'))
    vi.mocked(confirmAction).mockResolvedValueOnce(true)
    mount()
    await vi.waitFor(() => expect(root.textContent).toContain('inventory'))
    button('adminApiTokens.rotate').click()
    await vi.waitFor(() => expect(adminAPI.rotateAdminAPIToken).toHaveBeenCalled())
    await vi.waitFor(() => expect(button('adminApiTokens.rotate').disabled).toBe(false))
    expect(root.querySelector('code')).toBeNull()
    expect(root.textContent).toContain('inventory')
  })

  it('does not offer rotation for inactive tokens or super administrators', async () => {
    const inactive = ['expired', 'revoked', 'invalidated'].map((status, i) => ({ ...item, id: i + 10, status }))
    vi.mocked(adminAPI.listAdminAPITokens).mockResolvedValue({ data: { data: inactive } } as never)
    mount()
    await vi.waitFor(() => expect(root.textContent).toContain('inventory'))
    expect(root.textContent).not.toContain('adminApiTokens.rotate')
    app?.unmount()
    root.remove()
    vi.mocked(adminAPI.listAdminAPITokens).mockResolvedValue({ data: { data: [item] } } as never)
    mount(true)
    await vi.waitFor(() => expect(root.textContent).toContain('inventory'))
    expect(root.textContent).not.toContain('adminApiTokens.rotate')
  })

  it('requires the rotation permission even when revocation is allowed', async () => {
    permissions.rotate = false
    vi.mocked(adminAPI.listAdminAPITokens).mockResolvedValue({ data: { data: [item] } } as never)
    mount()
    await vi.waitFor(() => expect(root.textContent).toContain('inventory'))
    expect(root.textContent).not.toContain('adminApiTokens.rotate')
    expect(button('adminApiTokens.revoke')).toBeDefined()
  })
})
