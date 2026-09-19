import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const read = (path: string) => fs.readFileSync(new URL(`../${path}`, import.meta.url), 'utf8')
const products = read('src/views/Products.vue')
const router = read('src/router/index.ts')

test('product-first homepage does not load quick-buy or cart UI', () => {
  assert.doesNotMatch(products, /ProductQuickBuy/)
  assert.doesNotMatch(products, /quickBuy|quick-buy/)
})

test('idle prefetch only warms routes used by the focused storefront', () => {
  const warmups = router.match(/const routeWarmupLoaders:[\s\S]*?\n\]/)?.[0] ?? ''
  assert.doesNotMatch(warmups, /productsViewLoader/)
  assert.doesNotMatch(warmups, /cartViewLoader/)
  assert.doesNotMatch(warmups, /blogViewLoader/)
  assert.doesNotMatch(warmups, /noticeViewLoader/)
  assert.match(warmups, /productDetailViewLoader/)
  assert.match(warmups, /checkoutViewLoader/)
  assert.match(warmups, /paymentViewLoader/)
  assert.match(warmups, /loginViewLoader/)
})
