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

test('product breadcrumb is absent from both product-detail themes', () => {
  assert.doesNotMatch(classic, /<BreadcrumbNav/)
  assert.doesNotMatch(vault, /<nav class="[^"]*text-muted-foreground[^"]*">[\s\S]*?nav\.products/)
})

test('mobile home navigation remains available', () => {
  assert.match(mobile, /path: '\/'/)
  assert.match(mobile, /bottomNav\.home/)
})
