import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const layout = fs.readFileSync(new URL('../src/layouts/AdminLayout.vue', import.meta.url), 'utf8')

test('mobile navigation uses a plain fixed drawer with in-app links', () => {
  const mobileBlock = layout.match(/<!-- Mobile sidebar \(Sheet\) -->([\s\S]*?)<!-- Mobile sidebar end -->/)?.[1] ?? ''

  assert.match(mobileBlock, /v-if="mobileNavOpen"/)
  assert.match(mobileBlock, /class="fixed inset-0 z-50 bg-black\/80"/)
  assert.match(mobileBlock, /class="fixed inset-y-0 left-0 z-\[51\][^"]*"/)
  assert.match(mobileBlock, /<RouterLink[\s\S]*?to="\/"[\s\S]*?@click="mobileNavOpen = false"/)
  assert.match(mobileBlock, /<RouterLink[\s\S]*?:to="item\.to"[\s\S]*?@click="mobileNavOpen = false"/)
  assert.doesNotMatch(mobileBlock, /:href="adminUrl/)
  assert.doesNotMatch(mobileBlock, /<Sheet/)
})
