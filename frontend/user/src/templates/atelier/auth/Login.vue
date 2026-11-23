<template>
  <div class="relative flex min-h-screen items-center justify-center px-4 pb-16 pt-[96px] text-foreground sm:px-6">
    <div class="w-full max-w-[460px]">
      <div class="mb-4">
        <RouterLink to="/" class="theme-pill-link pl-2.5 pr-3.5">
          <ChevronLeft class="h-3.5 w-3.5" aria-hidden="true" />{{ t('auth.login.backHome') }}
        </RouterLink>
      </div>

      <div class="atelier-card rounded-[16px] bg-card p-6 sm:p-8">
        <div class="mb-6 border-b pb-5">
          <span class="inline-flex h-5 items-center rounded-[6px] border bg-background px-2 text-[11px] font-semibold tracking-[0.03em] text-muted-foreground">
            {{ brandSiteName }}
          </span>
          <h1 class="mt-2.5 text-[26px] font-bold leading-tight tracking-[-0.025em]">
            {{ step === 'totp' ? t('auth.login.totp.title') : t('auth.login.title') }}
          </h1>
          <p class="mt-1.5 text-sm text-muted-foreground">
            {{ step === 'totp' ? t('auth.login.totp.subtitle') : t('auth.login.subtitle') }}
          </p>
        </div>

        <!-- 两步验证表单 -->
        <form v-if="step === 'totp'" class="space-y-5" @submit.prevent="handleVerify2FA">
          <div class="rounded-[12px] border bg-background px-4 py-2.5 text-center font-mono text-xs text-muted-foreground">
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
                class="h-11 rounded-[12px] text-center font-mono tracking-[0.4em]"
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
                class="h-11 rounded-[12px]"
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

          <Button type="submit" :disabled="userAuthStore.loading" class="h-11.5 w-full rounded-[11px] bg-primary font-semibold text-primary-foreground shadow-xs hover:bg-primary/90">
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

        <!-- 常规登录表单 -->
        <form v-else class="space-y-5" @submit.prevent="handleLogin">
          <FormField :label="t('auth.login.emailLabel')" :error="formValidation.getError('email')">
            <template #icon><Mail class="h-3.5 w-3.5" aria-hidden="true" /></template>
            <template #default="{ id, hasError, describedBy }">
              <Input
                :id="id"
                v-model="email"
                type="email"
                required
                class="h-11 rounded-[12px]"
                :class="{ 'ring-2 ring-destructive/50': hasError }"
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
                  class="h-11 rounded-[12px] pr-10"
                  :class="{ 'ring-2 ring-destructive/50': hasError }"
                  :aria-invalid="hasError"
                  :aria-describedby="describedBy"
                  :placeholder="t('auth.login.passwordPlaceholder')"
                  @blur="formValidation.touchField('password', password)"
                />
                <button
                  type="button"
                  class="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground transition-colors hover:text-foreground"
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
            <label class="mb-2 flex items-center gap-1.5 text-[12.5px] font-semibold text-muted-foreground">
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
            <label class="inline-flex cursor-pointer items-center gap-2">
              <Checkbox v-model="rememberMe" />
              {{ t('auth.login.rememberMe') }}
            </label>
            <RouterLink
              v-if="emailVerificationEnabled"
              to="/auth/forgot"
              class="font-medium text-muted-foreground transition-colors hover:text-foreground"
            >
              {{ t('auth.login.forgot') }}
            </RouterLink>
          </div>

          <Alert v-if="info" class="border-success/40 text-center text-success">
            <AlertDescription>{{ info }}</AlertDescription>
          </Alert>
          <Alert v-if="error" variant="destructive" class="text-center">
            <AlertDescription>{{ error }}</AlertDescription>
          </Alert>

          <Button type="submit" :disabled="userAuthStore.loading" class="h-11.5 w-full rounded-[11px] bg-primary font-semibold text-primary-foreground shadow-xs hover:bg-primary/90">
            <LogIn v-if="!userAuthStore.loading" class="h-4 w-4" />
            {{ userAuthStore.loading ? t('auth.login.submitting') : t('auth.login.submit') }}
          </Button>

          <!-- 第三方登录 -->
          <div v-if="showThirdPartyLogin" class="space-y-4 pt-1">
            <div class="flex items-center gap-3 text-xs text-muted-foreground">
              <span class="h-px flex-1 bg-border"></span>
              <span>{{ t('auth.login.socialOr') }}</span>
              <span class="h-px flex-1 bg-border"></span>
            </div>
            <div class="space-y-3">
              <div v-if="showTelegramWidget" class="space-y-2">
                <div ref="telegramWidgetRef" class="flex justify-center"></div>
                <p class="text-center text-xs text-muted-foreground">{{ t('auth.login.telegramHint') }}</p>
              </div>
              <div v-else-if="showTelegramOidc" class="space-y-2">
                <Button type="button" variant="secondary" class="h-11 w-full rounded-[11px] font-semibold" @click="startTelegramOidc">
                  {{ t('auth.login.telegramOidcButton') }}
                </Button>
                <p class="text-center text-xs text-muted-foreground">{{ t('auth.login.telegramOidcHint') }}</p>
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
                <p class="text-center text-xs text-muted-foreground">{{ t('auth.login.googleHint') }}</p>
              </div>
            </div>
          </div>

          <div v-if="showTelegramMiniAppEntry" class="space-y-2 pt-1">
            <p class="text-center text-xs text-muted-foreground">{{ t('auth.login.telegramMiniAppEntryHint') }}</p>
            <Button type="button" variant="secondary" class="h-11 w-full rounded-[11px] font-semibold" @click="openTelegramMiniAppEntry">
              {{ t('auth.login.telegramMiniAppEntryAction') }}
            </Button>
          </div>
        </form>
      </div>

      <div v-if="registrationEnabled" class="mt-4 text-center">
        <RouterLink :to="registerLink" class="text-sm font-medium text-foreground underline-offset-4 hover:underline">
          {{ t('auth.login.noAccount') }}
        </RouterLink>
        <p v-if="inviteOnlyEnabled" class="mt-1 text-xs text-muted-foreground">{{ t('auth.login.inviteOnlyHint') }}</p>
      </div>
    </div>
    <InviteCodeDialog @success="continueAfterLogin" />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ChevronLeft, Eye, EyeOff, Lock, LogIn, Mail, ShieldCheck } from 'lucide-vue-next'
import ImageCaptcha from '../../../components/captcha/ImageCaptcha.vue'
import TurnstileCaptcha from '../../../components/captcha/TurnstileCaptcha.vue'
import FormField from '../../../components/FormField.vue'
import GoogleIdentityButton from '../../../components/auth/GoogleIdentityButton.vue'
import InviteCodeDialog from '../../../components/InviteCodeDialog.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
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
  registrationEnabled, emailVerificationEnabled, inviteOnlyEnabled, registerLink, continueAfterLogin,
  showTelegramWidget, telegramWidgetRef, showTelegramOidc, startTelegramOidc,
  showMiniAppLoginHint, attemptingMiniAppLogin, showTelegramMiniAppEntry, openTelegramMiniAppEntry,
  googleClientID, googleButtonLocale, googleIdentityUXMode, googleRedirectLoginURI,
  prepareGoogleRedirectLogin, showGoogleLogin, showThirdPartyLogin,
  handleGoogleCredential, handleGoogleScriptError,
  handleLogin,
} = useLogin()

void imageCaptchaRef
void turnstileRef
void telegramWidgetRef
</script>

<style scoped>
:deep(.form-field > label) {
  font-size: 12.5px;
  letter-spacing: 0;
  text-transform: none;
}
</style>
