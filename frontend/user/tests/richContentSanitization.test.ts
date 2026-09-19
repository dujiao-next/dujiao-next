import test from 'node:test'
import assert from 'node:assert/strict'
import { JSDOM } from 'jsdom'

const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://store.example.test/' })
Object.assign(globalThis, {
  window: dom.window,
  document: dom.window.document,
  Node: dom.window.Node,
  Element: dom.window.Element,
  HTMLElement: dom.window.HTMLElement,
  HTMLTemplateElement: dom.window.HTMLTemplateElement,
  DocumentFragment: dom.window.DocumentFragment,
  DOMParser: dom.window.DOMParser,
  NodeFilter: dom.window.NodeFilter,
})

const { sanitizeRichHtml } = await import('../src/utils/richContent.ts')

test('storefront rich HTML removes executable markup and dangerous URLs', () => {
  const container = document.createElement('div')
  container.innerHTML = sanitizeRichHtml(`
    <script>alert('xss')</script>
    <img src="https://cdn.example.test/product.png" onerror="alert('xss')">
    <img src="javascript:alert('xss')" alt="dangerous image">
    <a href="javascript:alert('xss')" onclick="alert('xss')">bad link</a>
    <a href="JaVaScRiPt:alert('xss')">mixed case</a>
  `)
  assert.equal(container.querySelector('script'), null)
  assert.equal(container.querySelectorAll('img')[0]?.hasAttribute('onerror'), false)
  assert.equal(container.querySelectorAll('img')[1]?.hasAttribute('src'), false)
  assert.equal(container.querySelector('a')?.hasAttribute('onclick'), false)
  assert.equal(container.querySelectorAll('a[href]').length, 0)
})

test('storefront rich HTML retains valid formatting, safe links, and images', () => {
  const container = document.createElement('div')
  container.innerHTML = sanitizeRichHtml(`
    <h2>Product details</h2><p><strong>Fast</strong> and <em>reliable</em></p>
    <ul><li>One</li><li>Two</li></ul>
    <a href="https://docs.example.test/guide" target="_blank" rel="noopener">Guide</a>
    <img src="/uploads/product.png" alt="Product">
  `)
  assert.equal(container.querySelector('h2')?.textContent, 'Product details')
  assert.equal(container.querySelector('strong')?.textContent, 'Fast')
  assert.equal(container.querySelector('em')?.textContent, 'reliable')
  assert.deepEqual([...container.querySelectorAll('li')].map((item) => item.textContent), ['One', 'Two'])
  assert.equal(container.querySelector('a')?.getAttribute('href'), 'https://docs.example.test/guide')
  assert.equal(container.querySelector('img')?.getAttribute('src'), '/uploads/product.png')
  assert.equal(container.querySelector('img')?.getAttribute('alt'), 'Product')
})
