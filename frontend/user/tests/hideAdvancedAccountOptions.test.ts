import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const read = (path: string) => fs.readFileSync(new URL(path, import.meta.url), 'utf8')

test('shared personal center navigation hides promotion, reseller and API integration', () => {
  const source = read('../src/composables/usePersonalCenter.ts')
  const menu = source.split('const sectionItems: PersonalSectionItem[] = [')[1]?.split('\n  ]')[0]
  assert.ok(menu, 'personal-center menu should exist')
  for (const section of ['affiliate', 'reseller', 'api']) {
    assert.doesNotMatch(menu, new RegExp(`key: '${section}'`))
  }
})

test('both themes do not render hidden advanced panels', () => {
  for (const file of ['../src/views/PersonalCenter.vue', '../src/templates/vault/PersonalCenter.vue']) {
    const source = read(file)
    for (const panel of ['AffiliatePanel', 'ApiPanel']) assert.doesNotMatch(source, new RegExp(`<${panel}\\b`))
    assert.doesNotMatch(source, /currentSection === 'reseller'/)
  }
})
