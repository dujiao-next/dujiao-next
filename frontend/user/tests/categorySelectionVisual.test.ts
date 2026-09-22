import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const products = fs.readFileSync(new URL('../src/views/Products.vue', import.meta.url), 'utf8')

test('selected category has a stable visual and semantic indicator', () => {
  assert.match(products, /:aria-pressed="selectedCategory === null"/)
  assert.match(products, /:aria-pressed="selectedCategory === group\.id"/)
  assert.match(products, /:aria-pressed="selectedCategory === child\.id"/)
  assert.match(products, /<span class="category-active-indicator" aria-hidden="true"><\/span>/)
  assert.match(products, /\.category-pill \{[^}]*position:relative/s)
  assert.match(products, /\.category-pill-active \{[^}]*box-shadow:/s)
  assert.match(products, /\.category-active-indicator \{[^}]*opacity:0/s)
  assert.match(products, /\.category-pill-active \.category-active-indicator \{[^}]*opacity:1/s)
})

test('category icon highlight follows the selected category', () => {
  assert.doesNotMatch(products, /category-icon bg-primary text-primary-foreground/)
  assert.doesNotMatch(products, /category-icon bg-secondary text-primary/)
  assert.match(products, /\.category-icon \{[^}]*background:/s)
  assert.match(products, /\.category-pill-active \.category-icon \{[^}]*background:/s)
})
