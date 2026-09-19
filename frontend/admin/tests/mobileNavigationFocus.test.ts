import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const layout = fs.readFileSync(new URL('../src/layouts/AdminLayout.vue', import.meta.url), 'utf8')

test('opening mobile navigation does not autofocus the search field', () => {
  const mobileBlock = layout.match(/<!-- Mobile sidebar \(Sheet\) -->([\s\S]*?)<\/Sheet>/)?.[1] ?? ''
  assert.match(mobileBlock, /<SheetContent[\s\S]*?@open-auto-focus="preventMobileNavAutoFocus"/)
  assert.match(layout, /const preventMobileNavAutoFocus = \(event: Event\) => \{[\s\S]*?event\.preventDefault\(\)/)
})
