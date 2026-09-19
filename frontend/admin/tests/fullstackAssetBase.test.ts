import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const html = fs.readFileSync(new URL('../dist/index.html', import.meta.url), 'utf8')

test('fullstack admin build uses relative assets behind randomized base path', () => {
  assert.match(html, /<base href="__DJ_ADMIN_BASE__\/">/)
  assert.match(html, /src="\.\/assets\/index-[^"]+\.js"/)
  assert.match(html, /href="\.\/assets\/vendor-vue-[^"]+\.js"/)
  assert.doesNotMatch(html, /src="\/assets\//)
})
