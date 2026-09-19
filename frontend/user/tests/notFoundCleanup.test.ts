import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const read = (path: string) => fs.readFileSync(new URL(`../${path}`, import.meta.url), 'utf8')
const pages = [read('src/views/NotFound.vue'), read('src/templates/vault/NotFound.vue')]

test('404 pages expose only the product-list recovery action', () => {
  for (const page of pages) {
    assert.doesNotMatch(page, /to="\/notice"|to="\/blog"|notFoundPage\.quickLinksTitle/)
    assert.doesNotMatch(page, /notFoundPage\.backPrevious|@click="goBack"/)
    assert.doesNotMatch(page, /Bell|BookOpen|ArrowLeft/)
    assert.match(page, /<RouterLink to="\/">/)
    assert.match(page, /notFoundPage\.backProducts/)
  }
})
