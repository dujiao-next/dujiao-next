import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const html = fs.readFileSync(new URL('../index.html', import.meta.url), 'utf8')

test('Safari receives cache-safe raster favicon and apple touch icon metadata', () => {
  assert.match(html, /rel="icon" type="image\/png" sizes="32x32" href="\/favicon-32x32-v3\.png"/)
  assert.match(html, /rel="apple-touch-icon" sizes="180x180" href="\/apple-touch-icon-v3\.png"/)
  assert.match(html, /rel="shortcut icon" href="\/favicon-v3\.ico"/)
})
