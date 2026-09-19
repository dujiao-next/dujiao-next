import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const read = (path: string) => fs.readFileSync(new URL(path, import.meta.url), 'utf8')

const router = read('../src/router/index.ts')
const app = read('../src/App.vue')
const navbar = read('../src/components/Navbar.vue')
const mobileNav = read('../src/components/MobileBottomNav.vue')
const navConfig = read('../src/composables/useNavConfig.ts')
const vaultLayout = read('../src/templates/vault/layout/VaultLayout.vue')
const security = read('../src/views/personal/SecurityPanel.vue')
const register = read('../src/composables/useRegister.ts')
const classicPersonalCenter = read('../src/views/PersonalCenter.vue')
const vaultPersonalCenter = read('../src/templates/vault/PersonalCenter.vue')
const enUS = read('../src/i18n/locales/en-US.json')
const zhCN = read('../src/i18n/locales/zh-CN.json')
const zhTW = read('../src/i18n/locales/zh-TW.json')

const routeBlock = (path: string) => {
  const escaped = path.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const match = router.match(new RegExp(`\\{\\s*path: '${escaped}',[\\s\\S]*?\\n\\s*\\},`))
  assert.ok(match, `route ${path} should exist`)
  return match[0]
}

test('root renders products directly and removed destinations are absent from storefront navigation', () => {
  const root = routeBlock('/')
  assert.match(root, /name: 'products'/)
  assert.match(root, /component: Products/)
  assert.doesNotMatch(root, /redirect:/)

  assert.doesNotMatch(app, /<Footer\b|import Footer from/)
  assert.doesNotMatch(navbar, /to="\/cart"|ShoppingCart|cartCount|useCartStore/)
  assert.doesNotMatch(mobileNav, /\/cart|\/products|ShoppingCart|cartCount|useCartStore/)
  assert.doesNotMatch(navConfig, /blog: \{|about: \{|key: 'products'/)
  assert.doesNotMatch(vaultLayout, /to="\/(?:cart|products)"|<footer\b|\/blog|\/about|ShoppingCart/)
})

test('legacy advanced account URLs redirect to the personal center', () => {
  for (const path of ['/me/api', '/me/affiliate', '/me/reseller']) {
    const route = routeBlock(path)
    assert.match(route, /redirect: '\/me'/)
    assert.doesNotMatch(route, /props: \{ section:/)
  }
})

test('classic and vault personal centers cannot render advanced account panels', () => {
  for (const source of [classicPersonalCenter, vaultPersonalCenter]) {
    assert.doesNotMatch(source, /AffiliatePanel|ApiPanel|currentSection === 'reseller'/)
  }
})

test('security keeps login history, password and 2FA, removes identity and email controls, and logs out through auth store', () => {
  for (const removed of ['TelegramBindingSection', 'GoogleBindingSection', 'EmailChangeForm']) {
    assert.doesNotMatch(security, new RegExp(removed))
  }
  for (const kept of ['LoginHistorySection', 'PasswordChangeForm', 'TwoFactorSection']) {
    assert.match(security, new RegExp(`<${kept}\\b`))
  }
  assert.match(security, /useUserAuthStore\(\)/)
  assert.match(security, /userAuthStore\.logout\(\)/)
  for (const messages of [enUS, zhCN, zhTW]) {
    const parsed = JSON.parse(messages)
    assert.doesNotMatch(parsed.personalCenter.security.subtitle, /email|邮箱|郵箱/i)
  }
})

test('registration verification remains backend-controlled through shared native registration logic', () => {
  assert.match(register, /emailVerificationEnabled = computed\(\(\) => appStore\.config\?\.email_verification_enabled !== false\)/)
  assert.match(register, /userAuthStore\.sendVerifyCode/)
  assert.match(register, /purpose: 'register'/)
  assert.match(register, /code: emailVerificationEnabled\.value \? code\.value : ''/)
})

test('route fade is keyed and cannot retain the prior route', () => {
  assert.equal((app.match(/class="route-page"/g) || []).length, 2)
  assert.doesNotMatch(app, /<Transition name="page-fade"|mode="out-in"/)
  assert.equal((app.match(/:key="route\.fullPath"/g) || []).length, 2)
  assert.match(app, /animation: page-fade-in 200ms ease both/)
})
