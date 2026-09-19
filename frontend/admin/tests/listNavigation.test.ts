import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const cardSecrets = fs.readFileSync(new URL('../src/views/admin/CardSecrets.vue', import.meta.url), 'utf8')
const router = fs.readFileSync(new URL('../src/router/index.ts', import.meta.url), 'utf8')
const layout = fs.readFileSync(new URL('../src/layouts/AdminLayout.vue', import.meta.url), 'utf8')

test('card-secret import action and navigation target the registered route', () => {
  assert.match(cardSecrets, /<RouterLink to="\/card-secret-imports">/)
  assert.match(router, /path: 'card-secret-imports'[\s\S]*name: 'card-secret-imports'/)
  assert.match(layout, /to: '\/card-secret-imports'/)
})

test('list navigation is owned by RouterLink and not a nested button primitive', () => {
  assert.doesNotMatch(cardSecrets, /<Button[^>]*as-child>[\s\S]*?<RouterLink to="\/card-secret-imports">/)
})
