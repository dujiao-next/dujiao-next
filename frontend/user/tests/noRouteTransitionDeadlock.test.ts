import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const app = fs.readFileSync(new URL('../src/App.vue', import.meta.url), 'utf8')

test('route animation cannot retain both old and new route roots', () => {
  assert.doesNotMatch(app, /<Transition\s+name="page-fade"/)
  assert.match(app, /class="route-page"/)
  assert.match(app, /@keyframes page-fade-in/)
  assert.match(app, /animation:\s*page-fade-in 200ms ease both/)
})
