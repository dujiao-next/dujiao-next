<template>
  <div class="relative flex min-h-screen items-center justify-center bg-background text-foreground px-4 py-16 sm:px-6">
    <div class="relative z-10 w-full max-w-lg">
      <div class="mb-4">
        <router-link to="/" class="theme-pill-link pl-2.5 pr-3.5">
          <ChevronLeft class="h-3.5 w-3.5" aria-hidden="true" />{{ t('auth.login.backHome') }}
        </router-link>
      </div>

      <div class="theme-float rounded-[22px] bg-card p-6 sm:rounded-[28px] sm:p-9">
        <div class="mb-7">
          <p class="text-[13px] font-semibold text-muted-foreground">{{ brandSiteName }}</p>
          <h1 class="mt-2 text-[28px] font-bold leading-tight tracking-tight">
            {{ step === 'totp' ? t('auth.login.totp.title') : t('auth.login.title') }}
          </h1>
          <p class="mt-2 text-sm text-muted-foreground">
            {{ step === 'totp' ? t('auth.login.totp.subtitle') : t('auth.login.subtitle') }}
          </p>
        </div>

        <form
          v-if="step === 'totp'"
          class="space-y-6"
          @submit.prevent="handleVerify2FA"
        >
          <div class="rounded-2xl bg-secondary/60 px-4 py-2 text-center text-xs text-muted-foreground">
            {{ t('auth.login.totp.countdown', { seconds: challengeRemainingSeconds }) }}
          </div>

          <FormField
            v-if="totpMode === 'code'"
            :label="t('auth.login.totp.codeLabel')"
          >
            <template #default="{ id }">
              <Input
                :id="id"
                v-model="totpCode"
                inputmode="numeric"
                autocomplete="one-time-code"
                maxlength="6"
                class="h-11 text-center tracking-[0.4em]"
                :placeholder="t('auth.login.totp.codePlaceholder')"
              />
            </template>
          </FormField>

          <FormField
            v-else
            :label="t('auth.login.totp.recoveryLabel')"
          >
            <template #default="{ id }">
              <Input
                :id="id"
                v-model="recoveryCode"
                autocomplete="off"
                class="h-11"
                :placeholder="t('auth.login.totp.recoveryPlaceholder')"
              />
            </template>
          </FormField>

          <div class="text-center">
            <Button
              type="button"
              variant="link"
              class="h-auto p-0 text-xs font-normal text-muted-foreground hover:text-foreground hover:no-underline"
              @click="totpMode = totpMode === 'code' ? 'recovery' : 'code'"
            >
              {{ totpMode === 'code' ? t('auth.login.totp.useRecovery') : t('auth.login.totp.useCode') }}
            </Button>
          </div>

          <Alert v-if="error" variant="destructive" class="text-center">
            <AlertDescription>{{ error }}</AlertDescription>
          </Alert>

          <Button type="submit" :disabled="userAuthStore.loading" class="h-12 w-full rounded-full bg-foreground font-semibold text-background hover:bg-foreground/85">
            {{ userAuthStore.loading ? t('auth.login.totp.verifying') : t('auth.login.totp.submit') }}
          </Button>

          <div class="text-center">
            <Button
              type="button"
              variant="link"
              class="h-auto p-0 text-xs font-normal text-muted-foreground hover:text-foreground hover:no-underline"
              @click="cancel2FA"
            >
              {{ t('auth.login.totp.cancel') }}
            </Button>
          </div>
        </form>

        <form
          v-else
          class="space-y-6"
          @submit.prevent="handleLogin"
        >
          <FormField
            :label="t('auth.login.emailLabel')"
            :error="formValidation.getError('email')"
          >
            <template #icon>
              <Mail class="h-3.5 w-3.5" aria-hidden="true" />
            </template>
            <template #default="{ id, hasError, describedBy }">
              <Input
                :id="id"
                v-model="email"
                type="email"
                required
                class="h-11"
                :class="{ 'ring-2 ring-destructive/50': hasError }"
                :aria-invalid="hasError"
                :aria-describedby="describedBy"
                :placeholder="t('auth.login.emailPlaceholder')"
                @blur="formValidation.touchField('email', email)"
              />
            </template>
          </FormField>

          <FormField
            :label="t('auth.login.passwordLabel')"
            :error="formValidation.getError('password')"
          >
            <template #icon>
              <Lock class="h-3.5 w-3.5" aria-hidden="true" />
            </template>
            <template #default="{ id, hasError, describedBy }">
              <div class="relative">
                <Input
                  :id="id"
                  v-model="password"
                  :type="showPassword ? 'text' : 'password'"
                  required
                  class="h-11 pr-10"
                  :class="{ 'ring-2 ring-destructive/50': hasError }"
                  :aria-invalid="hasError"
                  :aria-describedby="describedBy"
                  :placeholder="t('auth.login.passwordPlaceholder')"
                  @blur="formValidation.touchField('password', password)"
                />
                <button
                  type="button"
                  class="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
                  :aria-label="showPassword ? t('auth.common.hidePassword') : t('auth.common.showPassword')"
                  @click="showPassword = !showPassword"
                >
                  <EyeOff v-if="showPassword" class="h-4 w-4" aria-hidden="true" />
                  <Eye v-else class="h-4 w-4" aria-hidden="true" />
                </button>
              </div>
            </template>
          </FormField>

          <div v-if="loginCaptchaEnabled">
            <label class="mb-2 flex items-center gap-1.5 text-[13px] font-semibold text-muted-foreground">
              <ShieldCheck class="h-3.5 w-3.5 opacity-60" />
              {{ t('auth.common.captchaLabel') }}
            </label>
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

          <div class="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
            <label class="inline-flex items-center gap-2">
              <input v-model="rememberMe" type="checkbox" class="h-4 w-4 accent-foreground" />
              {{ t('auth.login.rememberMe') }}
            </label>
            <router-link
              v-if="emailVerificationEnabled"
              to="/auth/forgot"
              class="font-medium text-muted-foreground transition-colors hover:text-foreground"
            >
              {{ t('auth.login.forgot') }}
            </router-link>
          </div>

          <Alert v-if="info" class="text-center border-success/40 text-success">
            <AlertDescription>{{ info }}</AlertDescription>
          </Alert>

          <Alert v-if="error" variant="destructive" class="text-center">
            <AlertDescription>{{ error }}</AlertDescription>
          </Alert>

          <Button type="submit" :disabled="userAuthStore.loading" class="h-12 w-full rounded-full bg-foreground font-semibold text-background hover:bg-foreground/85">
            <LogIn v-if="!userAuthStore.loading" class="h-4 w-4" />
            {{ userAuthStore.loading ? t('auth.login.submitting') : t('auth.login.submit') }}
          </Button>

          <div v-if="showThirdPartyLogin" class="space-y-4 pt-1">
            <div class="flex items-center gap-3 text-xs text-muted-foreground">
              <span class="h-px flex-1 border-t border-gray-200/80 dark:border-white/10"></span>
              <span>{{ t('auth.login.socialOr') }}</span>
              <span class="h-px flex-1 border-t border-gray-200/80 dark:border-white/10"></span>
            </div>
            <div class="space-y-3">
              <div v-if="showTelegramWidget" class="space-y-2">
                <div ref="telegramWidgetRef" class="flex justify-center"></div>
                <p class="text-center text-xs text-muted-foreground">
                  {{ t('auth.login.telegramHint') }}
                </p>
              </div>
              <div v-else-if="showTelegramOidc" class="space-y-2">
                <Button type="button" variant="secondary" class="h-11 w-full rounded-full font-semibold" @click="startTelegramOidc">
                  {{ t('auth.login.telegramOidcButton') }}
                </Button>
                <p class="text-center text-xs text-muted-foreground">
                  {{ t('auth.login.telegramOidcHint') }}
                </p>
              </div>
              <div v-else-if="showMiniAppLoginHint" class="space-y-2">
                <p class="text-center text-xs text-muted-foreground">
                  {{ attemptingMiniAppLogin ? t('auth.login.telegramMiniAppLoggingIn') : t('auth.login.telegramMiniAppHint') }}
                </p>
              </div>
              <div v-if="showGoogleLogin" class="space-y-2">
                <GoogleIdentityButton
                  :client-id="googleClientID"
                  :locale="googleButtonLocale"
                  :ux-mode="googleIdentityUXMode"
                  :login-uri="googleRedirectLoginURI"
                  :prepare-redirect="prepareGoogleRedirectLogin"
                  :disabled="userAuthStore.loading"
                  :loading-label="t('auth.login.googleLoading')"
                  @credential="handleGoogleCredential"
                  @error="handleGoogleScriptError"
                />
                <p class="text-center text-xs text-muted-foreground">
                  {{ t('auth.login.googleHint') }}
                </p>
              </div>
            </div>
          </div>
          <div v-if="showTelegramMiniAppEntry" class="space-y-2 pt-1">
            <p class="text-center text-xs text-muted-foreground">
              {{ t('auth.login.telegramMiniAppEntryHint') }}
            </p>
            <Button type="button" variant="secondary" class="h-11 w-full rounded-full font-semibold" @click="openTelegramMiniAppEntry">
              {{ t('auth.login.telegramMiniAppEntryAction') }}
            </Button>
          </div>
        </form>
      </div>

      <div v-if="registrationEnabled" class="mt-4 text-center">
        <router-link
          :to="registerLink"
          class="text-sm font-medium text-foreground underline-offset-4 hover:underline"
        >
          {{ t('auth.login.noAccount') }}
        </router-link>
        <p v-if="inviteOnlyEnabled" class="mt-1 text-xs text-muted-foreground">{{ t('auth.login.inviteOnlyHint') }}</p>
      </div>
    </div>
    <InviteCodeDialog @success="continueAfterLogin" />
  </div>
</template>

<script setup lang="ts">
import ImageCaptcha from '../../components/captcha/ImageCaptcha.vue'
import TurnstileCaptcha from '../../components/captcha/TurnstileCaptcha.vue'
import FormField from '../../components/FormField.vue'
import GoogleIdentityButton from '../../components/auth/GoogleIdentityButton.vue'
import InviteCodeDialog from '../../components/InviteCodeDialog.vue'
import { useI18n } from 'vue-i18n'
import { ChevronLeft, Mail, Lock, ShieldCheck, Eye, EyeOff, LogIn } from 'lucide-vue-next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useLogin } from '../../composables/useLogin'

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

// 以下三个引用仅通过模板字符串 ref 绑定（相关逻辑在 composable 内），
// vue-tsc 不将字符串模板 ref 计为使用，这里显式标记避免 noUnusedLocals 误报。
void imageCaptchaRef
void turnstileRef
void telegramWidgetRef
</script>

<style scoped>
/* FormField 与 vault 模板共用，这里只把标签收成和注册页一致的样子 */
:deep(.form-field > label) {
  font-size: 13px;
  letter-spacing: 0;
  text-transform: none;
}
</style>
