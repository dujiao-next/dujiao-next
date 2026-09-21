import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const read = (path: string) => readFileSync(new URL(path, import.meta.url), 'utf8')

test('guest order details offer a direct change-payment-method route', () => {
  for (const path of ['../src/views/GuestOrderDetail.vue', '../src/templates/vault/GuestOrderDetail.vue']) {
    const source = read(path)
    assert.match(source, /change_method=1/)
    assert.match(source, /payment\.changeMethod/)
  }
})

test('payment page does not restore or auto-open the previous payment when change is requested', () => {
  const source = read('../src/composables/usePayment.ts')
  assert.match(source, /changePaymentMethodRequested/)
  assert.match(source, /!changePaymentMethodRequested\.value[\s\S]*?loadLatestPayment\(\)/)
})
