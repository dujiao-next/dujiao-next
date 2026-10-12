<template>
  <div class="flex min-h-[70vh] items-center justify-center px-4 py-12">
    <div class="w-full max-w-[460px]">
      <div class="mb-3.5 flex items-center justify-between px-1">
        <RouterLink to="/" class="inline-flex items-center gap-1.5 text-sm font-semibold text-muted-foreground transition-colors hover:text-primary">
          <ArrowLeft class="h-4 w-4" /> {{ t('auth.login.backHome') }}
        </RouterLink>
        <Badge variant="neutral" size="sm" class="rounded-full">{{ t('auth.forgot.title') }}</Badge>
      </div>

      <Card class="p-7 shadow-[var(--luna-shadow-lg)] sm:p-9">
        <div class="mb-7 text-center">
          <p class="text-[11px] font-bold uppercase tracking-[0.2em] text-primary">{{ brandSiteName }}</p>
          <h1 class="mt-3 text-3xl font-extrabold">{{ t('auth.forgot.title') }}</h1>
          <p class="mt-2 text-sm text-muted-foreground">{{ t('auth.forgot.subtitle') }}</p>
        </div>

        <Alert v-if="!emailVerificationEnabled" variant="destructive" class="text-center">
          <AlertDescription class="block">
            <p class="text-sm font-medium">{{ t('auth.forgot.disabled') }}</p>
            <RouterLink to="/auth/login" class="mt-3 inline-block text-sm text-muted-foreground transition-colors hover:text-foreground">{{ t('auth.forgot.backLogin') }}</RouterLink>
          </AlertDescription>
        </Alert>

        <form v-else class="grid grid-cols-1 gap-[18px]" @submit.prevent="handleReset">
          <!-- 邮箱 -->
          <div>
            <label class="mb-2 flex items-center gap-1.5 text-[11px] font-bold uppercase tracking-[0.12em] text-muted-foreground">
              <Mail class="h-3.5 w-3.5 opacity-70" /> {{ t('auth.forgot.emailLabel') }}
            </label>
            <Input v-model="email" type="email" required class="h-11" :placeholder="t('auth.forgot.emailPlaceholder')" />
          </div>

          <!-- 图形验证 -->
          <div v-if="sendCodeCaptchaEnabled">
            <label class="mb-2 flex items-center gap-1.5 text-[11px] font-bold uppercase tracking-[0.12em] text-muted-foreground">
              <ShieldCheck class="h-3.5 w-3.5 opacity-70" /> {{ t('auth.common.captchaLabel') }}
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

          <!-- 验证码 -->
          <div>
            <label class="mb-2 flex items-center gap-1.5 text-[11px] font-bold uppercase tracking-[0.12em] text-muted-foreground">
              <ShieldCheck class="h-3.5 w-3.5 opacity-70" /> {{ t('auth.forgot.codeLabel') }}
            </label>
            <div class="flex gap-2.5">
              <Input v-model="code" type="text" required class="h-11 min-w-0 flex-1" :placeholder="t('auth.forgot.codePlaceholder')" />
              <Button type="button" variant="secondary" class="h-11 shrink-0 whitespace-nowrap rounded-full px-5" :disabled="sending || countdown > 0" @click="handleSendCode">
                {{ countdown > 0 ? t('auth.common.countdown', { seconds: countdown }) : t('auth.common.sendCode') }}
              </Button>
            </div>
          </div>

          <!-- 新密码 -->
          <div>
            <label class="mb-2 flex items-center gap-1.5 text-[11px] font-bold uppercase tracking-[0.12em] text-muted-foreground">
              <KeyRound class="h-3.5 w-3.5 opacity-70" /> {{ t('auth.forgot.newPasswordLabel') }}
            </label>
            <Input v-model="newPassword" type="password" required class="h-11" :placeholder="t('auth.forgot.newPasswordPlaceholder')" />
          </div>

          <Alert v-if="error" variant="destructive" class="text-center">
            <AlertDescription>{{ error }}</AlertDescription>
          </Alert>

          <Button type="submit" :disabled="userAuthStore.loading" class="luna-gradient h-11 w-full rounded-full font-bold">
            <RotateCw v-if="!userAuthStore.loading" class="h-4 w-4" />
            {{ userAuthStore.loading ? t('auth.forgot.submitting') : t('auth.forgot.submit') }}
          </Button>
        </form>
      </Card>

      <div class="mt-4 text-center">
        <RouterLink to="/auth/login" class="text-sm text-muted-foreground transition-colors hover:text-foreground">{{ t('auth.forgot.backLogin') }}</RouterLink>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ArrowLeft, Mail, ShieldCheck, KeyRound, RotateCw } from 'lucide-vue-next'
import ImageCaptcha from '../../../components/captcha/ImageCaptcha.vue'
import TurnstileCaptcha from '../../../components/captcha/TurnstileCaptcha.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
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

// imageCaptchaRef / turnstileRef 仅通过字符串模板 ref 绑定，显式标记避免 noUnusedLocals 误报。
void imageCaptchaRef
void turnstileRef
</script>
