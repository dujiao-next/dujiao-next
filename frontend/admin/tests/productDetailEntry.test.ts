import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const products = fs.readFileSync(new URL('../src/views/admin/Products.vue', import.meta.url), 'utf8')

test('product list exposes product edit from the name and action button', () => {
  assert.match(products, /class="font-medium[^\"]*cursor-pointer[^"]*"[\s\S]*?@click="openEditById\(product\.id\)"/)
  assert.match(products, /@click="openEditById\(product\.id\)"[^>]*>\{\{ t\('admin\.products\.actions\.edit'\) \}\}/)
})
