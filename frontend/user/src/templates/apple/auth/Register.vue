<template>
  <div class="st-root st-page auth-page">
    <div class="st-container">
      <div class="auth-shell">
        <nav class="auth-top">
          <RouterLink to="/" class="auth-back">
            <ArrowLeft aria-hidden="true" /> {{ t('auth.login.backHome') }}
          </RouterLink>
        </nav>

        <header class="st-page-head auth-head">
          <p class="auth-brand">{{ brandSiteName }}</p>
          <h1 class="st-page-title">{{ t('auth.register.title') }}</h1>
          <p class="st-page-lead">{{ t('auth.register.subtitle') }}</p>
        </header>

        <div v-if="!registrationEnabled" class="st-panel auth-panel--lg">
          <p class="auth-panel-text">{{ t('auth.register.registrationDisabled') }}</p>
        </div>

        <form v-else class="auth-form" @submit.prevent="handleRegister">
          <!-- 邀请码 -->
          <div v-if="inviteOnlyEnabled" class="auth-field">
            <label for="register-invite" class="st-label">{{ t('auth.invite.codeLabel') }}</label>
            <input
              id="register-invite"
              v-model="inviteCode"
              type="text"
              required
              autocomplete="off"
              class="st-input"
              :placeholder="t('auth.invite.codePlaceholder')"
            />
            <p class="st-hint auth-field-hint">{{ t('auth.invite.registerHint') }}</p>
          </div>

          <!-- 邮箱 -->
          <div class="auth-field">
            <p class="st-label">{{ t('auth.register.emailLabel') }}</p>
            <div v-if="emailDomainSelectionRequired" class="auth-email-row">
              <input
                v-model="emailLocalPart"
                type="text"
                required
                autocomplete="username"
                class="st-input"
                :class="{ 'is-invalid': formValidation.hasError('email') }"
                :aria-invalid="formValidation.hasError('email')"
                :aria-describedby="formValidation.hasError('email') ? 'register-email-error' : undefined"
                :placeholder="t('auth.register.emailLocalPlaceholder')"
                @blur="touchRegistrationEmail"
              />
              <Select v-model="selectedEmailDomain" @update:model-value="touchRegistrationEmail">
                <SelectTrigger class="auth-select-trigger" :class="{ 'is-invalid': formValidation.hasError('email') }">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem v-for="domain in allowedEmailDomains" :key="domain" :value="domain">@{{ domain }}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <input
              v-else
              v-model="email"
              type="email"
              required
              class="st-input"
              :class="{ 'is-invalid': formValidation.hasError('email') }"
              :aria-invalid="formValidation.hasError('email')"
              :aria-describedby="formValidation.hasError('email') ? 'register-email-error' : undefined"
              :placeholder="t('auth.register.emailPlaceholder')"
              @blur="touchRegistrationEmail"
            />
            <p v-if="formValidation.hasError('email')" id="register-email-error" class="auth-error" role="alert">
              {{ formValidation.getError('email') }}
            </p>
            <p v-else-if="emailDomainSelectionRequired" class="st-hint auth-field-hint">
              {{ t('auth.register.emailDomainSelectHint') }}
            </p>
            <p v-else-if="emailDomainAllowlistEnabled" class="st-hint auth-field-hint">
              {{ allowedEmailDomains.length > 0
                ? t('auth.register.allowedEmailDomainsHint', { domains: allowedEmailDomainsText })
                : t('auth.register.noAllowedEmailDomainsHint') }}
            </p>
          </div>

          <!-- 密码 -->
          <div class="auth-field">
            <label for="register-password" class="st-label">{{ t('auth.register.passwordLabel') }}</label>
            <div class="auth-password">
              <input
                id="register-password"
                v-model="password"
                :type="showPassword ? 'text' : 'password'"
                required
                class="st-input"
                :class="{ 'is-invalid': formValidation.hasError('password') }"
                :aria-invalid="formValidation.hasError('password')"
                :aria-describedby="formValidation.hasError('password') ? 'register-password-error' : undefined"
                :placeholder="t('auth.register.passwordPlaceholder')"
                @blur="formValidation.touchField('password', password)"
              />
              <button
                type="button"
                class="auth-eye"
                :aria-label="showPassword ? t('auth.common.hidePassword') : t('auth.common.showPassword')"
                @click="showPassword = !showPassword"
              >
                <EyeOff v-if="showPassword" aria-hidden="true" /><Eye v-else aria-hidden="true" />
              </button>
            </div>
            <p v-if="formValidation.hasError('password')" id="register-password-error" class="auth-error" role="alert">
              {{ formValidation.getError('password') }}
            </p>
            <div v-if="password && !formValidation.hasError('password')" class="auth-strength">
              <div class="auth-strength-bars" aria-hidden="true">
                <span class="auth-strength-bar" :class="passwordStrength === 'weak' ? 'is-weak' : passwordStrength === 'medium' ? 'is-medium' : 'is-strong'"></span>
                <span class="auth-strength-bar" :class="passwordStrength === 'medium' ? 'is-medium' : passwordStrength === 'strong' ? 'is-strong' : ''"></span>
                <span class="auth-strength-bar" :class="passwordStrength === 'strong' ? 'is-strong' : ''"></span>
              </div>
              <span class="auth-strength-label" :class="`is-${passwordStrength}`">
                {{ t(`formValidation.passwordStrength.${passwordStrength}`) }}
              </span>
            </div>
          </div>

          <!-- 图形验证 -->
          <div v-if="emailVerificationEnabled && sendCodeCaptchaEnabled" class="auth-field">
            <p class="st-label">{{ t('auth.common.captchaLabel') }}</p>
            <div class="auth-captcha">
              <ImageCaptcha
                v-if="captchaProvider === 'image'"
                ref="imageCaptchaRef"
                v-model="captchaPayload"
                :disabled="sending || countdown > 0"
                @config-stale="handleCaptchaConfigStale"
              />
              <TurnstileCaptcha
                v-else-if="captchaProvider === 'turnstile'"
                ref="turnstileRef"
                v-model="turnstileToken"
                :site-key="turnstileSiteKey"
              />
            </div>
          </div>

          <!-- 邮箱验证码 -->
          <div v-if="emailVerificationEnabled" class="auth-field">
            <label for="register-code" class="st-label">{{ t('auth.register.codeLabel') }}</label>
            <div class="auth-code-row">
              <input
                id="register-code"
                v-model="code"
                type="text"
                required
                class="st-input"
                :placeholder="t('auth.register.codePlaceholder')"
              />
              <button
                type="button"
                class="st-btn st-btn--ghost auth-send"
                :disabled="sending || countdown > 0"
                @click="handleSendCode"
              >
                {{ countdown > 0 ? t('auth.common.countdown', { seconds: countdown }) : t('auth.common.sendCode') }}
              </button>
            </div>
          </div>

          <!-- 协议 -->
          <label class="st-panel auth-agree">
            <input v-model="agreed" type="checkbox" />
            <span>
              {{ t('auth.register.agreementPrefix') }}
              <RouterLink to="/privacy" target="_blank" rel="noopener noreferrer">{{ t('footer.privacy') }}</RouterLink>
              {{ t('auth.register.agreementAnd') }}
              <RouterLink to="/terms" target="_blank" rel="noopener noreferrer">{{ t('footer.terms') }}</RouterLink>
            </span>
          </label>

          <div v-if="error" class="auth-alert auth-alert--error" role="alert">{{ error }}</div>

          <button type="submit" class="st-btn st-btn--primary st-btn--block" :disabled="userAuthStore.loading || !agreed">
            {{ userAuthStore.loading ? t('auth.register.creating') : t('auth.register.create') }}
          </button>
        </form>

        <div class="auth-foot">
          <RouterLink :to="loginLink" class="auth-foot-link">{{ t('auth.register.hasAccount') }}</RouterLink>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ArrowLeft, Eye, EyeOff } from 'lucide-vue-next'
import ImageCaptcha from '../../../components/captcha/ImageCaptcha.vue'
import TurnstileCaptcha from '../../../components/captcha/TurnstileCaptcha.vue'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useRegister } from '../../../composables/useRegister'

const { t } = useI18n()

const {
  userAuthStore, brandSiteName,
  email, emailLocalPart, selectedEmailDomain, password, showPassword, code, agreed, inviteCode, inviteOnlyEnabled, loginLink,
  passwordStrength, error, sending, countdown,
  captchaPayload, turnstileToken, imageCaptchaRef, turnstileRef,
  captchaProvider, sendCodeCaptchaEnabled, turnstileSiteKey,
  registrationEnabled, emailVerificationEnabled,
  emailDomainAllowlistEnabled, allowedEmailDomains, allowedEmailDomainsText, emailDomainSelectionRequired,
  touchRegistrationEmail, formValidation, handleCaptchaConfigStale, handleSendCode, handleRegister,
} = useRegister()

// imageCaptchaRef / turnstileRef 仅通过字符串模板 ref 绑定，显式标记避免 noUnusedLocals 误报。
void imageCaptchaRef
void turnstileRef
</script>

<style scoped>
.auth-page {
  padding-top: 20px;
}

/* 居中窄栏 */
.auth-shell {
  width: 100%;
  max-width: 440px;
  margin-inline: auto;
}

/* 顶部返回 */
.auth-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 28px;
  font-size: 14px;
}
.auth-back {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--st-link);
  text-decoration: none;
}
.auth-back:hover {
  text-decoration: underline;
}
.auth-back svg {
  width: 15px;
  height: 15px;
}

/* 页头 */
.auth-shell .auth-head {
  padding-top: 36px;
  padding-bottom: 32px;
  text-align: center;
}
.auth-brand {
  margin: 0 0 12px;
  font-size: 13px;
  font-weight: 600;
  letter-spacing: 0.02em;
  color: var(--st-text-2);
}
.auth-head .st-page-lead {
  max-width: 340px;
  margin: 12px auto 0;
}

/* 表单 */
.auth-form {
  display: grid;
  gap: 20px;
}
.auth-field {
  min-width: 0;
}
.auth-field .st-input.is-invalid {
  border-color: #d70015;
}
.auth-field .st-input.is-invalid:focus {
  border-color: #d70015;
  box-shadow: 0 0 0 3px rgba(215, 0, 21, 0.16);
}
.auth-error {
  margin: 8px 0 0;
  font-size: 13px;
  line-height: 1.45;
  color: #d70015;
}
.dark .auth-error {
  color: #ff6961;
}
.auth-field-hint {
  margin: 8px 0 0;
}

/* 邮箱后缀选择 */
.auth-email-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(120px, 42%);
  gap: 8px;
}
.auth-email-row .auth-select-trigger {
  height: 48px;
  padding: 0 12px;
  border-color: color-mix(in srgb, var(--st-text) 16%, transparent);
  border-radius: 12px;
  background: var(--st-bg);
  font-size: 15px;
  color: var(--st-text);
  box-shadow: none;
}
.auth-email-row .auth-select-trigger:focus {
  border-color: var(--st-blue);
  outline: none;
  box-shadow: 0 0 0 3px rgba(0, 113, 227, 0.22);
}
.auth-email-row .auth-select-trigger.is-invalid {
  border-color: #d70015;
}
.auth-email-row .auth-select-trigger.is-invalid:focus {
  border-color: #d70015;
  box-shadow: 0 0 0 3px rgba(215, 0, 21, 0.16);
}

/* 密码可见切换 */
.auth-password {
  position: relative;
}
.auth-password .st-input {
  padding-right: 48px;
}
.auth-eye {
  position: absolute;
  top: 50%;
  right: 4px;
  transform: translateY(-50%);
  display: grid;
  place-items: center;
  width: 40px;
  height: 40px;
  padding: 0;
  border: 0;
  border-radius: 50%;
  background: none;
  color: var(--st-text-2);
  cursor: pointer;
}
.auth-eye:hover {
  background: var(--st-bg-alt);
  color: var(--st-text);
}
.auth-eye svg {
  width: 18px;
  height: 18px;
}

/* 密码强度 */
.auth-strength {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 2px;
}
.auth-strength-bars {
  display: flex;
  flex: 1;
  gap: 4px;
}
.auth-strength-bar {
  flex: 1;
  height: 4px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--st-text) 10%, transparent);
}
.dark .auth-strength-bar {
  background: rgba(255, 255, 255, 0.16);
}
.auth-strength-bar.is-weak {
  background: #d70015;
}
.auth-strength-bar.is-medium {
  background: var(--st-orange);
}
.auth-strength-bar.is-strong {
  background: var(--st-green);
}
.auth-strength-label {
  font-size: 12px;
  font-weight: 600;
}
.auth-strength-label.is-weak {
  color: #d70015;
}
.dark .auth-strength-label.is-weak {
  color: #ff6961;
}
.auth-strength-label.is-medium {
  color: var(--st-orange);
}
.auth-strength-label.is-strong {
  color: var(--st-green);
}

/* 验证码组件对齐表单尺寸 */
.auth-captcha :deep(input) {
  height: 48px;
  padding: 0 14px;
  border-radius: 12px;
  border-color: color-mix(in srgb, var(--st-text) 16%, transparent);
  background: var(--st-bg);
  font-size: 17px;
}
.auth-captcha :deep(input:focus) {
  border-color: var(--st-blue);
  outline: none;
  box-shadow: 0 0 0 3px rgba(0, 113, 227, 0.22);
}

/* 验证码输入 + 发送 */
.auth-code-row {
  display: flex;
  gap: 8px;
}
.auth-code-row .st-input {
  flex: 1;
  min-width: 0;
}
.auth-send {
  flex: none;
  height: 48px;
  padding-inline: 16px;
  font-size: 14px;
}

/* 协议面板 */
.auth-agree {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 14px 16px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--st-text-2);
  cursor: pointer;
}
.auth-agree input {
  width: 17px;
  height: 17px;
  margin: 2px 0 0;
  flex: none;
  accent-color: var(--st-blue);
}
.auth-agree a {
  color: var(--st-link);
  font-weight: 500;
  text-decoration: none;
}
.auth-agree a:hover {
  text-decoration: underline;
}

/* 反馈 */
.auth-alert {
  padding: 10px 14px;
  border-radius: 12px;
  font-size: 13px;
  line-height: 1.5;
  text-align: center;
}
.auth-alert--error {
  background: rgba(255, 59, 48, 0.08);
  color: #d70015;
}
.dark .auth-alert--error {
  background: rgba(255, 69, 58, 0.14);
  color: #ff6961;
}

/* 关闭注册提示面板 */
.auth-panel--lg {
  display: grid;
  gap: 14px;
  justify-items: center;
  padding: 32px 24px;
  text-align: center;
}
.auth-panel-text {
  margin: 0;
  font-size: 15px;
  line-height: 1.5;
  color: var(--st-text);
}

/* 底部链接 */
.auth-foot {
  margin-top: 36px;
  text-align: center;
}
.auth-foot-link {
  color: var(--st-link);
  font-size: 14px;
  text-decoration: none;
}
.auth-foot-link:hover {
  text-decoration: underline;
}
</style>
