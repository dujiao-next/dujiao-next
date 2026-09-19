import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const main = fs.readFileSync(new URL('../src/main.ts', import.meta.url), 'utf8')
const router = fs.readFileSync(new URL('../src/router/index.ts', import.meta.url), 'utf8')
const app = fs.readFileSync(new URL('../src/App.vue', import.meta.url), 'utf8')

test('Vue mounts immediately instead of waiting for Telegram promises', () => {
  assert.match(main, /app\.mount\('#app'\)/)
  assert.doesNotMatch(main, /Promise\.all\([\s\S]{0,300}app\.mount\('#app'\)/)
})

test('navigation waits for config before resolving a config-selected route component', () => {
  assert.match(router, /if \(!appStore\.config\) \{\s*await appStore\.loadConfig\(\)\s*\}/)
  assert.doesNotMatch(router, /void appStore\.loadConfig\(\)/)
})

test('global full-screen loading component is not mounted', () => {
  assert.doesNotMatch(app, /<Loading\s+:loading=/)
  assert.doesNotMatch(app, /import Loading from/)
})
