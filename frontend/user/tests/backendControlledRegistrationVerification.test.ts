import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const read = (path: string) => fs.readFileSync(new URL(path, import.meta.url), 'utf8')
const logic = read('../src/composables/useRegister.ts')
const registrationViews = [
  read('../src/views/auth/Register.vue'),
  read('../src/templates/vault/auth/Register.vue'),
]

test('registration email verification follows public backend config', () => {
  assert.match(logic, /emailVerificationEnabled = computed\(\(\) => appStore\.config\?\.email_verification_enabled !== false\)/)
  assert.match(logic, /await userAuthStore\.sendVerifyCode\(/)
  assert.match(logic, /if \(emailVerificationEnabled\.value && !code\.value\) return/)
  assert.match(logic, /code: emailVerificationEnabled\.value \? code\.value : ''/)
})

test('both registration themes conditionally render verification code controls', () => {
  for (const source of registrationViews) {
    assert.match(source, /v-if="emailVerificationEnabled"/)
    assert.match(source, /@click="handleSendCode"/)
    assert.match(source, /auth\.register\.codeLabel/)
  }
})
