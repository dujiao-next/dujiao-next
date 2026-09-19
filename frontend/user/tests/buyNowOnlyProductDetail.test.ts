import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const read = (path: string) => fs.readFileSync(new URL(`../${path}`, import.meta.url), 'utf8')
const files = [
  'src/views/ProductDetail.vue',
  'src/templates/vault/ProductDetail.vue',
  'src/components/product/ProductMobileBar.vue',
  'src/templates/vault/components/VaultProductMobileBar.vue',
]

test('all product purchase surfaces are buy-now only', () => {
  for (const path of files) {
    const source = read(path)
    assert.doesNotMatch(source, /addToCart|add-to-cart|ShoppingCart|productDetail\.addToCart/, path)
    assert.match(source, /buyNow|buy-now/, path)
  }
})

test('the surviving primary buy button fills the former action row', () => {
  const classic = read('src/views/ProductDetail.vue')
  const vault = read('src/templates/vault/ProductDetail.vue')
  assert.match(classic, /<Button class="[^"]*w-full[^"]*"[^>]*@click="buyNow"/)
  assert.match(vault, /<Button class="[^"]*flex-1[^"]*"[^>]*@click="buyNow"/)
})
