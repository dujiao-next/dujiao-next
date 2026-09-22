import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

const read = (path: string) => fs.readFileSync(new URL(`../${path}`, import.meta.url), 'utf8')
const registerLogic = read('src/composables/useRegister.ts')
const registrationViews = [
  read('src/views/auth/Register.vue'),
  read('src/templates/vault/auth/Register.vue'),
]

test('direct registration captcha is independent from email verification and sent with registration', () => {
  assert.match(registerLogic, /registerCaptchaEnabled = computed\(\(\) => !!captchaConfig\.value\?\.scenes\?\.register/)
  assert.match(registerLogic, /captcha_payload: getCaptchaPayload\(registerCaptchaEnabled\.value, registerCaptchaPayload\.value, registerTurnstileToken\.value\)/)
  assert.match(registerLogic, /if \(registerCaptchaEnabled\.value && captchaProvider\.value === 'turnstile'\)/)
})

test('both registration themes render captcha for direct registration when email verification is disabled', () => {
  for (const source of registrationViews) {
    assert.match(source, /v-if="registerCaptchaEnabled"/)
    assert.match(source, /registerCaptchaEnabled/)
    assert.match(source, /<TurnstileCaptcha/)
  }
})
