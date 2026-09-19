import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const main = fs.readFileSync(new URL('../src/main.ts', import.meta.url), 'utf8')
const router = fs.readFileSync(new URL('../src/router/index.ts', import.meta.url), 'utf8')
const i18n = fs.readFileSync(new URL('../src/i18n/index.ts', import.meta.url), 'utf8')

test('cold navigation loads config before template-sensitive route resolution', () => {
  assert.match(router, /if \(!appStore\.config\) \{\s*await appStore\.loadConfig\(\)\s*\}/)
  assert.doesNotMatch(router, /void appStore\.loadConfig\(\)/)
})

test('detected locale and every supported message set exist before mount', () => {
  assert.match(i18n, /import zhTW from '\.\/locales\/zh-TW\.json'/)
  assert.match(i18n, /import enUS from '\.\/locales\/en-US\.json'/)
  assert.match(i18n, /const initialLocale = detectLocale\(\)/)
  assert.match(i18n, /locale: initialLocale/)
  assert.match(i18n, /'zh-TW': zhTW/)
  assert.match(i18n, /'en-US': enUS/)
  assert.doesNotMatch(main, /setI18nLocale\(detectLocale\(\)\)/)
})

test('initialization retains immediate mount without a promise or loading gate', () => {
  assert.match(main, /app\.mount\('#app'\)/)
  assert.doesNotMatch(main, /Promise\.all\([\s\S]{0,300}app\.mount\('#app'\)/)
})
