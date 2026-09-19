import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const products = fs.readFileSync(new URL('../src/views/Products.vue', import.meta.url), 'utf8')

test('category icon highlight follows the selected category', () => {
  assert.doesNotMatch(products, /category-icon bg-primary text-primary-foreground/)
  assert.doesNotMatch(products, /category-icon bg-secondary text-primary/)
  assert.match(products, /\.category-icon \{[^}]*background:/s)
  assert.match(products, /\.category-pill-active \.category-icon \{[^}]*background:/s)
})
