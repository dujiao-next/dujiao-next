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
          <h1 class="mt-2.5 text-[26px] font-bold leading-tight tracking-[-0.025em]">{{ t('auth.forgot.title') }}</h1>
          <p class="mt-1.5 text-sm text-muted-foreground">{{ t('auth.forgot.subtitle') }}</p>
        </div>

        <Alert v-if="!emailVerificationEnabled" variant="destructive" class="text-center">
          <AlertDescription class="block">
            <p class="text-sm font-medium">{{ t('auth.forgot.disabled') }}</p>
            <RouterLink to="/auth/login" class="mt-3 inline-block text-sm text-muted-foreground transition-colors hover:text-foreground">
              {{ t('auth.forgot.backLogin') }}
            </RouterLink>
          </AlertDescription>
        </Alert>

        <form v-else class="space-y-5" @submit.prevent="handleReset">
          <div>
            <label class="mb-2 flex items-center gap-1.5 text-[12.5px] font-semibold text-muted-foreground">
              <Mail class="h-3.5 w-3.5 opacity-60" />
              {{ t('auth.forgot.emailLabel') }}
            </label>
            <Input v-model="email" type="email" required class="h-11 rounded-[12px]" :placeholder="t('auth.forgot.emailPlaceholder')" />
          </div>

          <div v-if="sendCodeCaptchaEnabled">
            <label class="mb-2 flex items-center gap-1.5 text-[12.5px] font-semibold text-muted-foreground">
              <ShieldCheck class="h-3.5 w-3.5 opacity-60" />
              {{ t('auth.common.captchaLabel') }}
            </label>
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

          <div>
            <label class="mb-2 flex items-center gap-1.5 text-[12.5px] font-semibold text-muted-foreground">
              <ShieldCheck class="h-3.5 w-3.5 opacity-60" />
              {{ t('auth.forgot.codeLabel') }}
            </label>
            <div class="flex flex-col gap-2 sm:flex-row">
              <Input v-model="code" type="text" required class="h-11 min-w-0 flex-1 rounded-[12px]" :placeholder="t('auth.forgot.codePlaceholder')" />
              <Button type="button" variant="secondary" class="h-11 whitespace-nowrap rounded-[11px] px-4" :disabled="sending || countdown > 0" @click="handleSendCode">
                {{ countdown > 0 ? t('auth.common.countdown', { seconds: countdown }) : t('auth.common.sendCode') }}
              </Button>
            </div>
          </div>

          <div>
            <label class="mb-2 flex items-center gap-1.5 text-[12.5px] font-semibold text-muted-foreground">
              <KeyRound class="h-3.5 w-3.5 opacity-60" />
              {{ t('auth.forgot.newPasswordLabel') }}
            </label>
            <Input v-model="newPassword" type="password" required class="h-11 rounded-[12px]" :placeholder="t('auth.forgot.newPasswordPlaceholder')" />
          </div>

          <Alert v-if="error" variant="destructive" class="text-center">
            <AlertDescription>{{ error }}</AlertDescription>
          </Alert>

          <Button type="submit" :disabled="userAuthStore.loading" class="h-11.5 w-full rounded-[11px] bg-primary font-semibold text-primary-foreground shadow-xs hover:bg-primary/90">
            <RotateCw v-if="!userAuthStore.loading" class="h-4 w-4" />
            {{ userAuthStore.loading ? t('auth.forgot.submitting') : t('auth.forgot.submit') }}
          </Button>
        </form>
      </div>

      <div class="mt-4 text-center">
        <RouterLink to="/auth/login" class="text-sm font-medium text-foreground underline-offset-4 hover:underline">
          {{ t('auth.forgot.backLogin') }}
        </RouterLink>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ChevronLeft, KeyRound, Mail, RotateCw, ShieldCheck } from 'lucide-vue-next'
import ImageCaptcha from '../../../components/captcha/ImageCaptcha.vue'
import TurnstileCaptcha from '../../../components/captcha/TurnstileCaptcha.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useForgot } from '../../../composables/useForgot'

const { t } = useI18n()

const {
  userAuthStore, brandSiteName, emailVerificationEnabled,
  email, code, newPassword, error, sending, countdown,
  captchaPayload, turnstileToken, imageCaptchaRef, turnstileRef,
  captchaProvider, sendCodeCaptchaEnabled, turnstileSiteKey,
  handleCaptchaConfigStale, handleSendCode, handleReset,
} = useForgot()

void imageCaptchaRef
void turnstileRef
</script>
