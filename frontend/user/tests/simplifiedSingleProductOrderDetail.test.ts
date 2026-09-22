import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const files = [
  '../src/views/OrderDetail.vue',
  '../src/views/GuestOrderDetail.vue',
  '../src/templates/vault/components/VaultOrderBody.vue',
]

test('buy-now order details hide internal child-order terminology and duplicate item blocks', () => {
  for (const path of files) {
    const source = readFileSync(new URL(path, import.meta.url), 'utf8')
    assert.doesNotMatch(source, /childOrdersTitle|childOrderNo|childOrderAmount|childItemsTitle|childFulfillmentTitle/)
    assert.match(source, /!order\.children \|\| order\.children\.length === 0/)
  }
})
