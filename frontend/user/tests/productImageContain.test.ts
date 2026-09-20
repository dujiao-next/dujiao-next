import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const vaultSource = fs.readFileSync(new URL('../src/templates/vault/ProductDetail.vue', import.meta.url), 'utf8')
const classicSource = fs.readFileSync(new URL('../src/components/product/ProductImageGallery.vue', import.meta.url), 'utf8')

test('product detail contains product images without cropping them', () => {
  assert.match(
    vaultSource,
    /<img v-if="currentImage"[^>]*class="[^"]*object-contain[^"]*"/,
    'the vault main product image must use object-contain',
  )
  assert.match(
    classicSource,
    /<img v-if="currentImage"[^>]*class="[^"]*object-contain[^"]*"/,
    'the classic main product image must use object-contain',
  )
})
