import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const layout = fs.readFileSync(new URL('../src/layouts/AdminLayout.vue', import.meta.url), 'utf8')

test('mobile admin navigation closes explicitly when a page link is selected', () => {
  const mobileBlock = layout.match(/<!-- Mobile sidebar \(Sheet\) -->([\s\S]*?)<\/Sheet>/)?.[1] ?? ''

  assert.match(mobileBlock, /<RouterLink[\s\S]*?@click="mobileNavOpen = false"/)
  assert.match(mobileBlock, /<SheetContent[\s\S]*?v-if="mobileNavOpen"/)
})
