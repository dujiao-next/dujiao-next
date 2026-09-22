import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const html = fs.readFileSync(new URL('../index.html', import.meta.url), 'utf8')

test('Safari receives a sharp, simplified v5 browser icon set', () => {
  assert.match(html, /rel="icon" type="image\/svg\+xml" href="\/favicon-v5\.svg"/)
  assert.match(html, /rel="icon" type="image\/png" sizes="32x32" href="\/favicon-32x32-v5\.png"/)
  assert.match(html, /rel="icon" type="image\/png" sizes="192x192" href="\/icon-192-v5\.png"/)
  assert.match(html, /rel="apple-touch-icon" sizes="180x180" href="\/apple-touch-icon-v5\.png"/)
  assert.match(html, /rel="shortcut icon" href="\/favicon-v5\.ico"/)
  assert.match(html, /rel="manifest" href="\/site-v5\.webmanifest"/)
  assert.doesNotMatch(html, /favicon-32x32-v4|favicon-v4|apple-touch-icon-v4|site-v4/)
})
