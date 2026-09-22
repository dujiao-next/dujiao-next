import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const products = fs.readFileSync(new URL('../src/views/Products.vue', import.meta.url), 'utf8')

test('inline ad does not reuse a cached modal announcement after public config resolves', () => {
  assert.doesNotMatch(products, /readCachedAnnouncement|writeCachedAnnouncement|cachedAnnouncement/)
  assert.match(products, /config\?\.homepage_ad/)
  assert.match(products, /config\?\.announcement/)
})
