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
          <h1 class="st-page-title">{{ step === 'totp' ? t('auth.login.totp.title') : t('auth.login.title') }}</h1>
          <p class="st-page-lead">{{ step === 'totp' ? t('auth.login.totp.subtitle') : t('auth.login.subtitle') }}</p>
        </header>

        <!-- 两步验证 -->
        <form v-if="step === 'totp'" class="auth-form" @submit.prevent="handleVerify2FA">
          <p class="st-panel auth-panel">{{ t('auth.login.totp.countdown', { seconds: challengeRemainingSeconds }) }}</p>

          <div v-if="totpMode === 'code'" class="auth-field">
            <label for="login-totp-code" class="st-label">{{ t('auth.login.totp.codeLabel') }}</label>
            <input
              id="login-totp-code"
              v-model="totpCode"
              type="text"
              inputmode="numeric"
              autocomplete="one-time-code"
              maxlength="6"
              class="st-input auth-code"
              :placeholder="t('auth.login.totp.codePlaceholder')"
            />
          </div>
          <div v-else class="auth-field">
            <label for="login-totp-recovery" class="st-label">{{ t('auth.login.totp.recoveryLabel') }}</label>
            <input
              id="login-totp-recovery"
              v-model="recoveryCode"
              type="text"
              autocomplete="off"
              class="st-input"
              :placeholder="t('auth.login.totp.recoveryPlaceholder')"
            />
          </div>

          <div class="auth-row auth-row--center">
            <button
              type="button"
              class="auth-text-btn"
              @click="totpMode = totpMode === 'code' ? 'recovery' : 'code'"
            >
              {{ totpMode === 'code' ? t('auth.login.totp.useRecovery') : t('auth.login.totp.useCode') }}
            </button>
          </div>

          <div v-if="error" class="auth-alert auth-alert--error" role="alert">{{ error }}</div>

          <button type="submit" class="st-btn st-btn--primary st-btn--block" :disabled="userAuthStore.loading">
            {{ userAuthStore.loading ? t('auth.login.totp.verifying') : t('auth.login.totp.submit') }}
          </button>

          <div class="auth-row auth-row--center">
            <button type="button" class="auth-text-btn" @click="cancel2FA">{{ t('auth.login.totp.cancel') }}</button>
          </div>
        </form>

        <!-- 密码登录 -->
        <form v-else class="auth-form" @submit.prevent="handleLogin">
          <div class="auth-field">
            <label for="login-email" class="st-label">{{ t('auth.login.emailLabel') }}</label>
            <input
              id="login-email"
              v-model="email"
              type="email"
              required
              class="st-input"
              :class="{ 'is-invalid': formValidation.hasError('email') }"
              :aria-invalid="formValidation.hasError('email')"
              :aria-describedby="formValidation.hasError('email') ? 'login-email-error' : undefined"
              :placeholder="t('auth.login.emailPlaceholder')"
              @blur="formValidation.touchField('email', email)"
            />
            <p v-if="formValidation.hasError('email')" id="login-email-error" class="auth-error" role="alert">
              {{ formValidation.getError('email') }}
            </p>
          </div>

          <div class="auth-field">
            <label for="login-password" class="st-label">{{ t('auth.login.passwordLabel') }}</label>
            <div class="auth-password">
              <input
                id="login-password"
                v-model="password"
                :type="showPassword ? 'text' : 'password'"
                required
                class="st-input"
                :class="{ 'is-invalid': formValidation.hasError('password') }"
                :aria-invalid="formValidation.hasError('password')"
                :aria-describedby="formValidation.hasError('password') ? 'login-password-error' : undefined"
                :placeholder="t('auth.login.passwordPlaceholder')"
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
            <p v-if="formValidation.hasError('password')" id="login-password-error" class="auth-error" role="alert">
              {{ formValidation.getError('password') }}
            </p>
          </div>

          <div v-if="loginCaptchaEnabled" class="auth-field">
            <p class="st-label">{{ t('auth.common.captchaLabel') }}</p>
            <div class="auth-captcha">
              <ImageCaptcha
                v-if="captchaProvider === 'image'"
                ref="imageCaptchaRef"
                v-model="captchaPayload"
                :disabled="userAuthStore.loading"
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

          <div class="auth-row">
            <label class="auth-check">
              <input v-model="rememberMe" type="checkbox" />
              {{ t('auth.login.rememberMe') }}
            </label>
            <RouterLink v-if="emailVerificationEnabled" to="/auth/forgot" class="auth-link">
              {{ t('auth.login.forgot') }}
            </RouterLink>
          </div>

          <div v-if="info" class="auth-alert auth-alert--info" role="status">{{ info }}</div>
          <div v-if="error" class="auth-alert auth-alert--error" role="alert">{{ error }}</div>

          <button type="submit" class="st-btn st-btn--primary st-btn--block" :disabled="userAuthStore.loading">
            {{ userAuthStore.loading ? t('auth.login.submitting') : t('auth.login.submit') }}
          </button>

          <!-- 第三方登录 -->
          <div v-if="showThirdPartyLogin" class="auth-third">
            <div class="auth-or"><span>{{ t('auth.login.socialOr') }}</span></div>

            <div v-if="showTelegramWidget" class="auth-third-item">
              <div ref="telegramWidgetRef" class="auth-telegram"></div>
              <p class="st-hint">{{ t('auth.login.telegramHint') }}</p>
            </div>
            <div v-else-if="showTelegramOidc" class="auth-third-item">
              <button type="button" class="st-btn st-btn--ghost st-btn--block" @click="startTelegramOidc">
                {{ t('auth.login.telegramOidcButton') }}
              </button>
              <p class="st-hint">{{ t('auth.login.telegramOidcHint') }}</p>
            </div>
            <div v-else-if="showMiniAppLoginHint" class="auth-third-item">
              <p class="st-hint">
                {{ attemptingMiniAppLogin ? t('auth.login.telegramMiniAppLoggingIn') : t('auth.login.telegramMiniAppHint') }}
              </p>
            </div>

            <div v-if="showGoogleLogin" class="auth-third-item">
              <GoogleIdentityButton
                :client-id="googleClientID"
                :locale="googleButtonLocale"
                shape="pill"
                :ux-mode="googleIdentityUXMode"
                :login-uri="googleRedirectLoginURI"
                :prepare-redirect="prepareGoogleRedirectLogin"
                :disabled="userAuthStore.loading"
                :loading-label="t('auth.login.googleLoading')"
                @credential="handleGoogleCredential"
                @error="handleGoogleScriptError"
              />
              <p class="st-hint">{{ t('auth.login.googleHint') }}</p>
            </div>
          </div>

          <div v-if="showTelegramMiniAppEntry" class="auth-third">
            <p class="st-hint">{{ t('auth.login.telegramMiniAppEntryHint') }}</p>
            <button type="button" class="st-btn st-btn--ghost st-btn--block" @click="openTelegramMiniAppEntry">
              {{ t('auth.login.telegramMiniAppEntryAction') }}
            </button>
          </div>
        </form>

        <div v-if="registrationEnabled" class="auth-foot">
          <RouterLink :to="registerLink" class="auth-foot-link">{{ t('auth.login.noAccount') }}</RouterLink>
          <p v-if="inviteOnlyEnabled" class="st-hint auth-foot-hint">{{ t('auth.login.inviteOnlyHint') }}</p>
        </div>
      </div>
    </div>
    <InviteCodeDialog @success="continueAfterLogin" />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ArrowLeft, Eye, EyeOff } from 'lucide-vue-next'
import ImageCaptcha from '../../../components/captcha/ImageCaptcha.vue'
import TurnstileCaptcha from '../../../components/captcha/TurnstileCaptcha.vue'
import GoogleIdentityButton from '../../../components/auth/GoogleIdentityButton.vue'
import InviteCodeDialog from '../../../components/InviteCodeDialog.vue'
import { useLogin } from '../../../composables/useLogin'

const { t } = useI18n()

const {
  userAuthStore, brandSiteName,
  email, password, showPassword, rememberMe,
  step, totpMode, totpCode, recoveryCode, challengeRemainingSeconds, handleVerify2FA, cancel2FA,
  error, info, formValidation,
  loginCaptchaEnabled, captchaProvider, captchaPayload, turnstileToken, turnstileSiteKey,
  imageCaptchaRef, turnstileRef, handleCaptchaConfigStale,
  registrationEnabled, emailVerificationEnabled, inviteOnlyEnabled, registerLink, continueAfterLogin,
  showTelegramWidget, telegramWidgetRef, showTelegramOidc, startTelegramOidc,
  showMiniAppLoginHint, attemptingMiniAppLogin, showTelegramMiniAppEntry, openTelegramMiniAppEntry,
  googleClientID, googleButtonLocale, googleIdentityUXMode, googleRedirectLoginURI,
  prepareGoogleRedirectLogin, showGoogleLogin, showThirdPartyLogin,
  handleGoogleCredential, handleGoogleScriptError,
  handleLogin,
} = useLogin()

// 以下三个引用仅通过模板字符串 ref 绑定（相关逻辑在 composable 内），显式标记避免 noUnusedLocals 误报。
void imageCaptchaRef
void turnstileRef
void telegramWidgetRef
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

/* 6 位动态码 */
.auth-code {
  padding-inline: 0.4em;
  text-align: center;
  letter-spacing: 0.4em;
  font-variant-numeric: tabular-nums;
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

/* 行内辅助链接与勾选 */
.auth-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px 16px;
  font-size: 14px;
  color: var(--st-text-2);
}
.auth-row--center {
  justify-content: center;
}
.auth-check {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}
.auth-check input {
  width: 16px;
  height: 16px;
  margin: 0;
  accent-color: var(--st-blue);
}
.auth-link {
  color: var(--st-link);
  font-size: 14px;
  text-decoration: none;
}
.auth-link:hover {
  text-decoration: underline;
}
.auth-text-btn {
  padding: 0;
  border: 0;
  background: none;
  color: var(--st-link);
  font-size: 13px;
  line-height: 1.5;
  cursor: pointer;
}
.auth-text-btn:hover {
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
.auth-alert--info {
  background: rgba(0, 113, 227, 0.07);
  color: var(--st-link);
}
.dark .auth-alert--error {
  background: rgba(255, 69, 58, 0.14);
  color: #ff6961;
}

/* 2FA 倒计时 */
.auth-panel {
  margin: 0;
  padding: 12px 16px;
  font-size: 13px;
  line-height: 1.5;
  color: var(--st-text-2);
  text-align: center;
}

/* 第三方登录 */
.auth-third {
  display: grid;
  gap: 16px;
  margin-top: 4px;
}
.auth-or {
  display: flex;
  align-items: center;
  gap: 14px;
  font-size: 12px;
  color: var(--st-text-3);
}
.auth-or::before,
.auth-or::after {
  content: '';
  flex: 1;
  height: 1px;
  background: color-mix(in srgb, var(--st-text) 10%, transparent);
}
.dark .auth-or::before,
.dark .auth-or::after {
  background: rgba(255, 255, 255, 0.14);
}
.auth-third-item {
  display: grid;
  gap: 8px;
}
.auth-telegram {
  display: flex;
  justify-content: center;
}
.auth-third .st-hint {
  text-align: center;
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
.auth-foot-hint {
  margin: 8px 0 0;
}
</style>
