import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const read = (path: string) => fs.readFileSync(new URL(`../${path}`, import.meta.url), 'utf8')
const navbar = read('src/components/Navbar.vue')
const classic = read('src/views/ProductDetail.vue')
const vault = read('src/templates/vault/ProductDetail.vue')
const mobile = read('src/components/MobileBottomNav.vue')

test('desktop primary navigation omits the redundant home item', () => {
  assert.match(navbar, /primaryNavItems\.value\.filter\(\(item\) => item\.path !== '\/'\)/)
})

test('product breadcrumb is hidden only at the desktop breakpoint in both themes', () => {
  assert.match(classic, /<BreadcrumbNav[\s\S]*?class="mb-8 lg:hidden"/)
  assert.match(vault, /<nav class="[^"]*lg:hidden[^"]*"/)
})

test('mobile home navigation remains available', () => {
  assert.match(mobile, /path: '\/'/)
  assert.match(mobile, /bottomNav\.home/)
})
