import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const navbar = fs.readFileSync(new URL('../src/components/Navbar.vue', import.meta.url), 'utf8')

test('mobile header uses only an inline language popover and no side drawer', () => {
  assert.doesNotMatch(navbar, /showMobileMenu|toggleMobileMenu|<Teleport|EllipsisVertical|translate-x-full|navbar\.more/)
  assert.doesNotMatch(navbar, /Mobile Drawer|Guest orders \(not in bottom nav\)/)
  assert.match(navbar, /Popover/)
  assert.match(navbar, /Languages/)
  assert.match(navbar, /class="inline-flex gap-1\.5 px-2 text-muted-foreground"/)
  assert.match(navbar, /currentLocale/)
})
