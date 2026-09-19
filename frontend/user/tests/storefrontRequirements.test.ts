import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const read = (path: string) => fs.readFileSync(new URL(`../${path}`, import.meta.url), 'utf8')

test('order lookup keeps browser, credential and order-number modes', () => {
  const api = read('src/api/order.ts')
  const composable = read('src/composables/useGuestOrders.ts')
  const classic = read('src/views/GuestOrders.vue')
  const vault = read('src/templates/vault/GuestOrders.vue')

  assert.match(api, /browserOrders/)
  assert.match(api, /\/guest\/orders\/browser/)
  assert.match(composable, /activeTab/)
  assert.match(composable, /loadBrowserOrders/)
  assert.match(composable, /searchByCredentials/)
  assert.match(composable, /searchByOrderNo/)
  for (const view of [classic, vault]) {
    assert.match(view, /guestOrders\.tabs\.browser/)
    assert.match(view, /guestOrders\.tabs\.credentials/)
    assert.match(view, /guestOrders\.tabs\.orderNo/)
  }
})

test('mobile navigation keeps product, order lookup and account', () => {
  const mobile = read('src/components/MobileBottomNav.vue')
  assert.match(mobile, /\/guest\/orders/)
  assert.match(mobile, /ClipboardList/)
  assert.match(mobile, /bottomNav\.orders/)
})

test('static document carries Chinese brand metadata without runtime patches', () => {
  const html = read('index.html')
  assert.match(html, /<html lang="zh-CN">/)
  assert.match(html, /<title>雪糕数卡<\/title>/)
  assert.match(html, /href="\/favicon\.svg"/)
  assert.doesNotMatch(html, /MutationObserver|XMLHttpRequest/)
})
