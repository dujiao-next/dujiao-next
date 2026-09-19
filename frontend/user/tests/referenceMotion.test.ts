import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const app = fs.readFileSync(new URL('../src/App.vue', import.meta.url), 'utf8')
const style = fs.readFileSync(new URL('../src/style.css', import.meta.url), 'utf8')
const card = fs.readFileSync(new URL('../src/components/ProductCard.vue', import.meta.url), 'utf8')

test('route entry borrows subtle reference motion without transition lifecycle ownership', () => {
  assert.match(app, /animation: page-fade-in 260ms/)
  assert.match(app, /translateY\(8px\) scale\(0\.998\)/)
  assert.doesNotMatch(app, /<Transition|mode="out-in"/)
})

test('product cards use a short staggered upward reveal', () => {
  assert.match(style, /theme-slide-up[^}]*240ms/s)
  assert.match(style, /translateY\(12px\)/)
  assert.match(card, /animationStep: 50/)
})

test('motion respects reduced-motion preference', () => {
  assert.match(style, /prefers-reduced-motion: reduce/)
  assert.match(app, /prefers-reduced-motion: reduce/)
})
