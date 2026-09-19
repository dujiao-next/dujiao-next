import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const products = fs.readFileSync(new URL('../src/views/Products.vue', import.meta.url), 'utf8')

test('announcement uses a cached last-known value before public config resolves', () => {
  assert.match(products, /readCachedAnnouncement/)
  assert.match(products, /writeCachedAnnouncement/)
  assert.match(products, /cachedAnnouncement/)
  assert.doesNotMatch(products, /announcementContent = computed\(\(\) => sanitizeRichHtml\(getLocalizedText\(appStore\.config\?\.announcement\?\.content\) \|\|/)
})
