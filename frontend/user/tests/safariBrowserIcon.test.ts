import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const html = fs.readFileSync(new URL('../index.html', import.meta.url), 'utf8')

test('Safari receives one complete cache-safe icon set', () => {
  assert.match(html, /rel="icon" type="image\/png" sizes="32x32" href="\/favicon-32x32-v4\.png"/)
  assert.match(html, /rel="icon" type="image\/png" sizes="192x192" href="\/icon-192-v4\.png"/)
  assert.match(html, /rel="apple-touch-icon" sizes="180x180" href="\/apple-touch-icon-v4\.png"/)
  assert.match(html, /rel="shortcut icon" href="\/favicon-v4\.ico"/)
  assert.match(html, /rel="manifest" href="\/site-v4\.webmanifest"/)
  assert.doesNotMatch(html, /favicon-32x32-v3|favicon-v3|apple-touch-icon-v3/)
})
