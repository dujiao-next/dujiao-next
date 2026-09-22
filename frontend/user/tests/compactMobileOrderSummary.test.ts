import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const files = [
  '../src/views/OrderDetail.vue',
  '../src/views/GuestOrderDetail.vue',
  '../src/templates/vault/OrderDetail.vue',
  '../src/templates/vault/GuestOrderDetail.vue',
]

test('order summary cards use compact mobile spacing while preserving desktop layout', () => {
  for (const path of files) {
    const source = readFileSync(new URL(path, import.meta.url), 'utf8')
    assert.match(source, /data-order-summary/)
    assert.match(source, /p-4[^\"]*md:p-6|p-4[^\"]*sm:p-\[22px\]/)
    assert.match(source, /gap-3[^\"]*md:gap-4|gap-3[^\"]*sm:gap-\[18px\]/)
  }
})
