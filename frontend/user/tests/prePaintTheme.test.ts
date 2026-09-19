import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const html = fs.readFileSync(new URL('../index.html', import.meta.url), 'utf8')

test('theme is applied in head before styles and module entry can paint', () => {
  const themeScript = html.indexOf("localStorage.getItem('dujiao_theme')")
  const moduleEntry = html.indexOf('type="module"')
  assert.ok(themeScript > -1)
  assert.ok(moduleEntry === -1 || themeScript < moduleEntry)
  assert.match(html, /document\.documentElement\.classList\.toggle\('dark'/)
  assert.match(html, /matchMedia\('\(prefers-color-scheme: dark\)'\)/)
})

test('document has a dark-aware pre-css background', () => {
  assert.match(html, /color-scheme:\s*light dark/)
  assert.match(html, /@media\s*\(prefers-color-scheme:\s*dark\)/)
  assert.match(html, /html\.dark[\s\S]*background:\s*#000/)
  assert.match(html, /html,\s*body,\s*#app/)
})
