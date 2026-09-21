import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const router = readFileSync(new URL('../src/router/index.ts', import.meta.url), 'utf8')

test('registration route uses a shared loader that is warmed immediately after login', () => {
  assert.match(router, /const registerViewLoader/)
  assert.match(router, /const authRouteWarmupLoaders:[\s\S]*?registerViewLoader/)
  assert.match(router, /path: '\/auth\/register'[\s\S]*?templateView\('auth\/Register', registerViewLoader\)/)
  assert.match(router, /router\.currentRoute\.value\.name === 'user-login'[\s\S]*?runRouteWarmupQueue\(\[\.\.\.authRouteWarmupLoaders\]\)/)
})
