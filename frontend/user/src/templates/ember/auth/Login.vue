<template>
  <div class="flex min-h-[70vh] items-center justify-center px-4 py-12">
    <div class="w-full max-w-[440px]">
      <div class="mb-3.5 flex items-center justify-between px-1">
        <RouterLink
          to="/"
          class="inline-flex items-center gap-1.5 text-[13px] text-muted-foreground transition-colors hover:text-foreground"
        >
          <ArrowLeft class="h-4 w-4" /> {{ t('auth.login.backHome') }}
        </RouterLink>
        <span class="ember-pill">{{ t('navbar.personalCenter') }}</span>
      </div>

      <div class="rounded-[var(--r-lg)] border border-border bg-card p-7 shadow-[var(--shadow-lg)] sm:p-9">
        <div class="mb-7 flex flex-col items-center gap-2 text-center">
          <p class="text-[11px] font-semibold uppercase tracking-[0.2em] text-[color:var(--accent)]">{{ brandSiteName }}</p>
          <h1 class="text-[26px] font-semibold tracking-[-0.025em]">
            {{ step === 'totp' ? t('auth.login.totp.title') : t('auth.login.title') }}
          </h1>
          <p class="text-[13.5px] text-muted-foreground">
            {{ step === 'totp' ? t('auth.login.totp.subtitle') : t('auth.login.subtitle') }}
          </p>
        </div>

        <!-- ============ 两步验证 ============ -->
        <form v-if="step === 'totp'" class="flex flex-col gap-4" @submit.prevent="handleVerify2FA">
          <div class="rounded-[var(--r)] bg-[color:var(--surface-2)] px-4 py-2 text-center text-[12.5px] text-muted-foreground">
            {{ t('auth.login.totp.countdown', { seconds: challengeRemainingSeconds }) }}
          </div>

          <FormField v-if="totpMode === 'code'" :label="t('auth.login.totp.codeLabel')">
            <template #default="{ id }">
              <Input
                :id="id"
                v-model="totpCode"
                inputmode="numeric"
                autocomplete="one-time-code"
                maxlength="6"
                class="h-12 text-center tracking-[0.4em]"
                :placeholder="t('auth.login.totp.codePlaceholder')"
              />
            </template>
          </FormField>

          <FormField v-else :label="t('auth.login.totp.recoveryLabel')">
            <template #default="{ id }">
              <Input
                :id="id"
                v-model="recoveryCode"
                autocomplete="off"
                class="h-12"
                :placeholder="t('auth.login.totp.recoveryPlaceholder')"
              />
            </template>
          </FormField>

          <button
            type="button"
            class="text-center text-[12.5px] text-muted-foreground transition-colors hover:text-foreground"
            @click="totpMode = totpMode === 'code' ? 'recovery' : 'code'"
          >
            {{ totpMode === 'code' ? t('auth.login.totp.useRecovery') : t('auth.login.totp.useCode') }}
          </button>

          <p v-if="error" class="rounded-[var(--r)] bg-[color:var(--danger-soft)] px-4 py-2.5 text-center text-[13px] text-[color:var(--danger-strong)]">
            {{ error }}
          </p>

          <Button type="submit" :disabled="userAuthStore.loading" class="h-12 w-full rounded-full text-[15px] font-medium">
            {{ userAuthStore.loading ? t('auth.login.totp.verifying') : t('auth.login.totp.submit') }}
          </Button>

          <button
            type="button"
            class="text-center text-[12.5px] text-muted-foreground transition-colors hover:text-foreground"
            @click="cancel2FA"
          >{{ t('auth.login.totp.cancel') }}</button>
        </form>

        <!-- ============ 密码登录 ============ -->
        <form v-else class="flex flex-col gap-4" @submit.prevent="handleLogin">
          <FormField :label="t('auth.login.emailLabel')" :error="formValidation.getError('email')">
            <template #icon><Mail class="h-3.5 w-3.5" aria-hidden="true" /></template>
            <template #default="{ id, hasError, describedBy }">
              <Input
                :id="id"
                v-model="email"
                type="email"
                required
                class="h-12"
                :class="{ 'ring-2 ring-[color:var(--danger)]/40': hasError }"
                :aria-invalid="hasError"
                :aria-describedby="describedBy"
                :placeholder="t('auth.login.emailPlaceholder')"
                @blur="formValidation.touchField('email', email)"
              />
            </template>
          </FormField>

          <FormField :label="t('auth.login.passwordLabel')" :error="formValidation.getError('password')">
            <template #icon><Lock class="h-3.5 w-3.5" aria-hidden="true" /></template>
            <template #default="{ id, hasError, describedBy }">
              <div class="relative">
                <Input
                  :id="id"
                  v-model="password"
                  :type="showPassword ? 'text' : 'password'"
                  required
                  class="h-12 pr-11"
                  :class="{ 'ring-2 ring-[color:var(--danger)]/40': hasError }"
                  :aria-invalid="hasError"
                  :aria-describedby="describedBy"
                  :placeholder="t('auth.login.passwordPlaceholder')"
                  @blur="formValidation.touchField('password', password)"
                />
                <button
                  type="button"
                  class="absolute right-3.5 top-1/2 -translate-y-1/2 text-muted-foreground transition-colors hover:text-foreground"
                  :aria-label="showPassword ? t('auth.common.hidePassword') : t('auth.common.showPassword')"
                  @click="showPassword = !showPassword"
                >
                  <EyeOff v-if="showPassword" class="h-4 w-4" aria-hidden="true" />
                  <Eye v-else class="h-4 w-4" aria-hidden="true" />
                </button>
              </div>
            </template>
          </FormField>

          <!-- 验证码 -->
          <div v-if="loginCaptchaEnabled" class="flex flex-col gap-2">
            <span class="flex items-center gap-1.5 text-[12px] font-medium text-muted-foreground">
              <ShieldCheck class="h-3.5 w-3.5" /> {{ t('auth.common.captchaLabel') }}
            </span>
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

          <div class="flex flex-wrap items-center justify-between gap-2 text-[12.5px] text-muted-foreground">
            <label class="inline-flex items-center gap-2">
              <input v-model="rememberMe" type="checkbox" class="h-4 w-4 accent-[var(--ui-accent)]" />
              {{ t('auth.login.rememberMe') }}
            </label>
            <RouterLink
              v-if="emailVerificationEnabled"
              to="/auth/forgot"
              class="transition-colors hover:text-foreground"
            >{{ t('auth.login.forgot') }}</RouterLink>
          </div>

          <p v-if="info" class="rounded-[var(--r)] bg-[color:var(--success-soft)] px-4 py-2.5 text-center text-[13px] text-[color:var(--success-strong)]">
            {{ info }}
          </p>
          <p v-if="error" class="rounded-[var(--r)] bg-[color:var(--danger-soft)] px-4 py-2.5 text-center text-[13px] text-[color:var(--danger-strong)]">
            {{ error }}
          </p>

          <Button type="submit" :disabled="userAuthStore.loading" class="h-12 w-full rounded-full text-[15px] font-medium">
            <LogIn v-if="!userAuthStore.loading" class="mr-1.5 h-4 w-4" />
            {{ userAuthStore.loading ? t('auth.login.submitting') : t('auth.login.submit') }}
          </Button>

          <!-- 第三方登录 -->
          <div v-if="showThirdPartyLogin" class="flex flex-col gap-4 pt-1">
            <div class="flex items-center gap-3 text-[11px] uppercase tracking-[0.12em] text-muted-foreground">
              <span class="h-px flex-1 bg-border"></span>
              <span>{{ t('auth.login.socialOr') }}</span>
              <span class="h-px flex-1 bg-border"></span>
            </div>

            <div class="flex flex-col gap-3">
              <div v-if="showTelegramWidget" class="flex flex-col gap-2">
                <div ref="telegramWidgetRef" class="flex justify-center"></div>
                <p class="text-center text-[12px] text-muted-foreground">{{ t('auth.login.telegramHint') }}</p>
              </div>
              <div v-else-if="showTelegramOidc" class="flex flex-col gap-2">
                <Button type="button" variant="outline" class="h-12 w-full rounded-full border-[color:var(--border-strong)] text-[14px] font-medium" @click="startTelegramOidc">
                  {{ t('auth.login.telegramOidcButton') }}
                </Button>
                <p class="text-center text-[12px] text-muted-foreground">{{ t('auth.login.telegramOidcHint') }}</p>
              </div>
              <div v-else-if="showMiniAppLoginHint" class="flex flex-col gap-2">
                <p class="text-center text-[12px] text-muted-foreground">
                  {{ attemptingMiniAppLogin ? t('auth.login.telegramMiniAppLoggingIn') : t('auth.login.telegramMiniAppHint') }}
                </p>
              </div>

              <div v-if="showGoogleLogin" class="flex flex-col gap-2">
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
                <p class="text-center text-[12px] text-muted-foreground">{{ t('auth.login.googleHint') }}</p>
              </div>
            </div>
          </div>

          <div v-if="showTelegramMiniAppEntry" class="flex flex-col gap-2 pt-1">
            <p class="text-center text-[12px] text-muted-foreground">{{ t('auth.login.telegramMiniAppEntryHint') }}</p>
            <Button type="button" variant="outline" class="h-12 w-full rounded-full border-[color:var(--border-strong)] text-[14px] font-medium" @click="openTelegramMiniAppEntry">
              {{ t('auth.login.telegramMiniAppEntryAction') }}
            </Button>
          </div>
        </form>
      </div>

      <div v-if="registrationEnabled" class="mt-4 text-center">
        <RouterLink to="/auth/register" class="text-[13px] text-muted-foreground transition-colors hover:text-foreground">
          {{ t('auth.login.noAccount') }}
        </RouterLink>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ArrowLeft, Eye, EyeOff, Lock, LogIn, Mail, ShieldCheck } from 'lucide-vue-next'
import ImageCaptcha from '../../../components/captcha/ImageCaptcha.vue'
import TurnstileCaptcha from '../../../components/captcha/TurnstileCaptcha.vue'
import FormField from '../../../components/FormField.vue'
import GoogleIdentityButton from '../../../components/auth/GoogleIdentityButton.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useLogin } from '../../../composables/useLogin'

const { t } = useI18n()

const {
  userAuthStore, brandSiteName,
  email, password, showPassword, rememberMe,
  step, totpMode, totpCode, recoveryCode, challengeRemainingSeconds, handleVerify2FA, cancel2FA,
  error, info, formValidation,
  loginCaptchaEnabled, captchaProvider, captchaPayload, turnstileToken, turnstileSiteKey,
  imageCaptchaRef, turnstileRef, handleCaptchaConfigStale,
  registrationEnabled, emailVerificationEnabled,
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
