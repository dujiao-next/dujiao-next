import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const products = fs.readFileSync(new URL('../src/views/Products.vue', import.meta.url), 'utf8')

test('product-first homepage preserves the customized inline ad and horizontal category cards', () => {
  assert.match(products, /products-announcement/)
  assert.match(products, /sanitizeRichHtml/)
  assert.match(products, /config\?\.homepage_ad/)
  assert.match(products, /<AnnouncementModal/)
  assert.match(products, /category-card-grid/)
  assert.match(products, /category-pill/)
  assert.match(products, /categoryGroups/)
  assert.doesNotMatch(products, /<CategorySidebar/)
  assert.doesNotMatch(products, /products\.subtitle/)
})
