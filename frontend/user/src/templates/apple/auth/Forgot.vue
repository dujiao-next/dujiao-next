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
          <h1 class="st-page-title">{{ t('auth.forgot.title') }}</h1>
          <p class="st-page-lead">{{ t('auth.forgot.subtitle') }}</p>
        </header>

        <div v-if="!emailVerificationEnabled" class="st-panel auth-panel--lg">
          <p class="auth-panel-text">{{ t('auth.forgot.disabled') }}</p>
        </div>

        <form v-else class="auth-form" @submit.prevent="handleReset">
          <!-- 邮箱 -->
          <div class="auth-field">
            <label for="forgot-email" class="st-label">{{ t('auth.forgot.emailLabel') }}</label>
            <input
              id="forgot-email"
              v-model="email"
              type="email"
              required
              class="st-input"
              :placeholder="t('auth.forgot.emailPlaceholder')"
            />
          </div>

          <!-- 图形验证 -->
          <div v-if="sendCodeCaptchaEnabled" class="auth-field">
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

          <!-- 验证码 -->
          <div class="auth-field">
            <label for="forgot-code" class="st-label">{{ t('auth.forgot.codeLabel') }}</label>
            <div class="auth-code-row">
              <input
                id="forgot-code"
                v-model="code"
                type="text"
                required
                class="st-input"
                :placeholder="t('auth.forgot.codePlaceholder')"
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

          <!-- 新密码 -->
          <div class="auth-field">
            <label for="forgot-password" class="st-label">{{ t('auth.forgot.newPasswordLabel') }}</label>
            <input
              id="forgot-password"
              v-model="newPassword"
              type="password"
              required
              class="st-input"
              :placeholder="t('auth.forgot.newPasswordPlaceholder')"
            />
          </div>

          <div v-if="error" class="auth-alert auth-alert--error" role="alert">{{ error }}</div>

          <button type="submit" class="st-btn st-btn--primary st-btn--block" :disabled="userAuthStore.loading">
            {{ userAuthStore.loading ? t('auth.forgot.submitting') : t('auth.forgot.submit') }}
          </button>
        </form>

        <div class="auth-foot">
          <RouterLink to="/auth/login" class="auth-foot-link">{{ t('auth.forgot.backLogin') }}</RouterLink>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ArrowLeft } from 'lucide-vue-next'
import ImageCaptcha from '../../../components/captcha/ImageCaptcha.vue'
import TurnstileCaptcha from '../../../components/captcha/TurnstileCaptcha.vue'
import { useForgot } from '../../../composables/useForgot'

const { t } = useI18n()

const {
  userAuthStore, brandSiteName, emailVerificationEnabled,
  email, code, newPassword, error, sending, countdown,
  captchaPayload, turnstileToken, imageCaptchaRef, turnstileRef,
  captchaProvider, sendCodeCaptchaEnabled, turnstileSiteKey,
  handleCaptchaConfigStale, handleSendCode, handleReset,
} = useForgot()

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

/* 关闭找回密码提示面板 */
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
