import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const read = (path: string) => fs.readFileSync(new URL(`../${path}`, import.meta.url), 'utf8')
const navbar = read('src/components/Navbar.vue')
const router = read('src/router/index.ts')
const productList = read('src/composables/useProductList.ts')
const productDetail = read('src/composables/useProductDetail.ts')

test('cold shell renders a simple sharp vector brand mark inline before config resolves', () => {
  assert.doesNotMatch(navbar, /Dujiao-Next/)
  assert.match(navbar, /雪糕数卡/)
  assert.match(navbar, /<svg[^>]+viewBox="0 0 32 32"[^>]+aria-label="雪糕数卡"/)
  assert.match(navbar, /shape-rendering="geometricPrecision"/)
  assert.doesNotMatch(navbar, /linearGradient|radialGradient|filter=|opacity=/)
  assert.doesNotMatch(navbar, /<img[\s\S]*?:src="brandLogo"/)
  assert.doesNotMatch(navbar, /const brandLogo/)
})

test('root product route is eagerly available without config-selected loader', () => {
  assert.match(router, /import Products from '\.\.\/views\/Products\.vue'/)
  const root = router.match(/\{\s*path: '\/',[\s\S]*?\n\s*\},/)
  assert.ok(root)
  assert.match(root[0], /component: Products/)
  assert.doesNotMatch(root[0], /templateView|productsViewLoader/)
})

test('home categories and products start concurrently', () => {
  const initialize = productList.match(/const initialize = async \(\) => \{[\s\S]*?\n  \}/)
  assert.ok(initialize)
  assert.match(initialize[0], /const categoriesRequest = loadCategories\(\)/)
  assert.match(initialize[0], /const productsRequest = loadProducts\(\)/)
  assert.match(initialize[0], /Promise\.all\(\[categoriesRequest, productsRequest\]\)/)
  assert.doesNotMatch(initialize[0], /await loadCategories\(\)[\s\S]*await loadProducts\(\)/)
})

test('product detail request starts in the route guard before the lazy view resolves', () => {
  assert.match(router, /void prefetchProductDetail\(String\(to\.params\.slug \|\| ''\)\)/)
  assert.match(productDetail, /takeProductDetailRequest\(slug\)/)
  assert.match(productDetail, /const initialProductRequest = loadProduct\(\)/)
  assert.doesNotMatch(productDetail, /onMounted\(\(\) => \{\s*loadProduct\(\)/)
  assert.match(productDetail, /void initialProductRequest/)
})
