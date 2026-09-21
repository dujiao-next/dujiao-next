import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const read = (path: string) => readFileSync(new URL(path, import.meta.url), 'utf8')

test('every active payment page exposes cancel for pending orders', () => {
  for (const path of ['../src/views/Payment.vue', '../src/templates/vault/Payment.vue']) {
    const source = read(path)
    assert.match(source, /canCancelOrder/)
    assert.match(source, /@click="cancelOrder"/)
    assert.match(source, /orderDetail\.cancel/)
  }
})

test('guest order details expose the same pending-order cancellation action', () => {
  for (const path of ['../src/views/GuestOrderDetail.vue', '../src/templates/vault/GuestOrderDetail.vue']) {
    const source = read(path)
    assert.match(source, /order\.status === 'pending_payment'/)
    assert.match(source, /@click="cancelOrder"/)
    assert.match(source, /orderDetail\.cancel/)
  }
})

test('guest API carries credentials when canceling an order', () => {
  const api = read('../src/api/order.ts')
  assert.match(api, /cancel:\s*\(orderNo: string, data: GuestAuthInput\)/)
  assert.match(api, /guest\/orders\/\$\{encodeURIComponent\(orderNo\)\}\/cancel/)
})
