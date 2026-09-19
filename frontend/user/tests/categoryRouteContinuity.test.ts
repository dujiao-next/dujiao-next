import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const app = fs.readFileSync(new URL('../src/App.vue', import.meta.url), 'utf8')

test('home and category URLs keep one mounted product-list instance', () => {
  assert.match(app, /const routeRenderKey/)
  assert.match(app, /products.*category-products|category-products.*products/)
  assert.equal((app.match(/:key="routeRenderKey\(route\)"/g) || []).length, 2)
  assert.doesNotMatch(app, /:key="route\.fullPath"/)
})
