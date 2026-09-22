import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const products = fs.readFileSync(new URL('../src/views/Products.vue', import.meta.url), 'utf8')

test('homepage ad owns the inline card independently of modal announcement', () => {
  assert.match(products, /products-announcement[^>]*v-if="homepageAd"|v-if="homepageAd"[^>]*products-announcement/)
  assert.match(products, /config\?\.homepage_ad/)
  assert.match(products, /homepageAdContent = computed\(\(\) => sanitizeRichHtml\(getLocalizedText\(homepageAd\.value\?\.content\)\)\)/)
  assert.doesNotMatch(products, /readCachedAnnouncement|writeCachedAnnouncement/)
  assert.doesNotMatch(products, /欢迎访问雪糕数卡/)
})

test('existing homepage announcement is shown via dismissible versioned modal', () => {
  assert.match(products, /<AnnouncementModal/)
  assert.match(products, /useAnnouncement\(\)/)
  assert.match(products, /shouldShow\(announcement\)/)
  assert.match(products, /config\?\.announcement/)
})
