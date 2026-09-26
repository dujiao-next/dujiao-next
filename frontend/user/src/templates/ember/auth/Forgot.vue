<template>
  <div class="flex min-h-[70vh] items-center justify-center px-4 py-12">
    <div class="w-full max-w-[440px]">
      <div class="mb-3.5 flex items-center justify-between px-1">
        <RouterLink
          to="/auth/login"
          class="inline-flex items-center gap-1.5 text-[13px] text-muted-foreground transition-colors hover:text-foreground"
        >
          <ArrowLeft class="h-4 w-4" /> {{ t('auth.login.backHome') }}
        </RouterLink>
        <span class="ember-pill">{{ t('auth.forgot.title') }}</span>
      </div>

      <div class="rounded-[var(--r-lg)] border border-border bg-card p-7 shadow-[var(--shadow-lg)] sm:p-9">
        <div class="mb-7 flex flex-col items-center gap-2 text-center">
          <p class="text-[11px] font-semibold uppercase tracking-[0.2em] text-[color:var(--accent)]">{{ brandSiteName }}</p>
          <h1 class="text-[26px] font-semibold tracking-[-0.025em]">{{ t('auth.forgot.title') }}</h1>
          <p class="text-[13.5px] text-muted-foreground">{{ t('auth.forgot.subtitle') }}</p>
        </div>

        <form class="flex flex-col gap-4" @submit.prevent="handleReset">
          <!-- 邮箱验证未开启时不可自助重置 -->
          <div
            v-if="!emailVerificationEnabled"
            class="rounded-[var(--r)] bg-[color:var(--danger-soft)] px-4 py-3 text-center text-[13px] text-[color:var(--danger-strong)]"
          >
            <p class="font-medium">{{ t('auth.forgot.disabled') }}</p>
          </div>

          <!-- 邮箱 -->
          <div class="flex flex-col gap-2">
            <span class="flex items-center gap-1.5 text-[12px] font-medium text-muted-foreground">
              <Mail class="h-3.5 w-3.5" /> {{ t('auth.forgot.emailLabel') }}
            </span>
            <Input
              v-model="email"
              type="email"
              required
              class="h-12"
              :placeholder="t('auth.forgot.emailPlaceholder')"
            />
          </div>

          <!-- 图形验证码 -->
          <div v-if="sendCodeCaptchaEnabled" class="flex flex-col gap-2">
            <span class="flex items-center gap-1.5 text-[12px] font-medium text-muted-foreground">
              <ShieldCheck class="h-3.5 w-3.5" /> {{ t('auth.common.captchaLabel') }}
            </span>
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

          <!-- 邮箱验证码 -->
          <div v-if="emailVerificationEnabled" class="flex flex-col gap-2">
            <span class="flex items-center gap-1.5 text-[12px] font-medium text-muted-foreground">
              <ShieldCheck class="h-3.5 w-3.5" /> {{ t('auth.forgot.codeLabel') }}
            </span>
            <div class="flex gap-2.5">
              <Input
                v-model="code"
                type="text"
                required
                class="h-12 min-w-0 flex-1"
                :placeholder="t('auth.forgot.codePlaceholder')"
              />
              <Button
                type="button"
                variant="secondary"
                class="h-12 shrink-0 whitespace-nowrap rounded-full px-5 text-[13.5px]"
                :disabled="sending || countdown > 0"
                @click="handleSendCode"
              >
                {{ countdown > 0 ? t('auth.common.countdown', { seconds: countdown }) : t('auth.common.sendCode') }}
              </Button>
            </div>
          </div>

          <!-- 新密码 -->
          <div class="flex flex-col gap-2">
            <span class="flex items-center gap-1.5 text-[12px] font-medium text-muted-foreground">
              <Lock class="h-3.5 w-3.5" /> {{ t('auth.forgot.newPasswordLabel') }}
            </span>
            <Input
              v-model="newPassword"
              type="password"
              required
              autocomplete="new-password"
              class="h-12"
              :placeholder="t('auth.forgot.newPasswordPlaceholder')"
            />
          </div>

          <p v-if="error" class="rounded-[var(--r)] bg-[color:var(--danger-soft)] px-4 py-2.5 text-center text-[13px] text-[color:var(--danger-strong)]">
            {{ error }}
          </p>

          <Button type="submit" :disabled="userAuthStore.loading" class="h-12 w-full rounded-full text-[15px] font-medium">
            {{ userAuthStore.loading ? t('auth.forgot.submitting') : t('auth.forgot.submit') }}
          </Button>
        </form>
      </div>

      <div class="mt-4 text-center">
        <RouterLink to="/auth/login" class="text-[13px] text-muted-foreground transition-colors hover:text-foreground">
          {{ t('auth.forgot.backLogin') }}
        </RouterLink>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ArrowLeft, Lock, Mail, ShieldCheck } from 'lucide-vue-next'
import ImageCaptcha from '../../../components/captcha/ImageCaptcha.vue'
import TurnstileCaptcha from '../../../components/captcha/TurnstileCaptcha.vue'
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

// imageCaptchaRef / turnstileRef 仅通过字符串模板 ref 绑定，显式标记避免 noUnusedLocals 误报。
void imageCaptchaRef
void turnstileRef
</script>
