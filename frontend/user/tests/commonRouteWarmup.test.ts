import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const router = readFileSync(new URL('../src/router/index.ts', import.meta.url), 'utf8')

test('idle route warmup prioritizes the two mobile bottom-navigation destinations', () => {
  const queue = router.match(/const routeWarmupLoaders:[\s\S]*?= \[([\s\S]*?)\]/)?.[1] || ''
  assert.match(router, /const guestOrdersViewLoader/)
  assert.match(router, /const personalCenterViewLoader/)
  assert.match(queue, /guestOrdersViewLoader[\s\S]*personalCenterViewLoader/)
  assert.ok(queue.indexOf('guestOrdersViewLoader') < queue.indexOf('productDetailViewLoader'))
  assert.ok(queue.indexOf('personalCenterViewLoader') < queue.indexOf('productDetailViewLoader'))
})

test('guest lookup and personal center routes reuse their warmed loaders', () => {
  assert.match(router, /path: '\/guest\/orders'[\s\S]*?templateView\('GuestOrders', guestOrdersViewLoader\)/)
  assert.match(router, /path: '\/me'[\s\S]*?templateView\('PersonalCenter', personalCenterViewLoader\)/)
})
