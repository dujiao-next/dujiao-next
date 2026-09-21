import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const html = readFileSync(new URL('../index.html', import.meta.url), 'utf8')
const manifest = JSON.parse(readFileSync(new URL('../public/site-v4.webmanifest', import.meta.url), 'utf8'))

test('Safari chrome follows neutral storefront backgrounds instead of brand blue', () => {
  assert.match(html, /name="theme-color" media="\(prefers-color-scheme: light\)" content="#f5f5f7"/)
  assert.match(html, /name="theme-color" media="\(prefers-color-scheme: dark\)" content="#000000"/)
  assert.equal(manifest.theme_color, '#000000')
  assert.notEqual(manifest.theme_color.toLowerCase(), '#2563eb')
})
