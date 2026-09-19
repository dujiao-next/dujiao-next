import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const read = (path: string) => fs.readFileSync(new URL(`../${path}`, import.meta.url), 'utf8')
const registerLogic = read('src/composables/useRegister.ts')
const views = [read('src/views/auth/Register.vue'), read('src/templates/vault/auth/Register.vue')]

test('registration card renders the configured storefront logo', () => {
  assert.match(registerLogic, /brandLogo/)
  assert.match(registerLogic, /site_logo/)
  for (const view of views) {
    assert.match(view, /<img\s+v-if="brandLogo"/)
    assert.match(view, /:src="brandLogo"/)
    assert.match(view, /userAuthStore, brandSiteName, brandLogo/)
  }
})
