<template>
  <div class="flex min-h-[70vh] items-center justify-center px-4 py-12">
    <div class="w-full max-w-[460px]">
      <div class="mb-3.5 flex items-center justify-between px-1">
        <RouterLink
          to="/"
          class="inline-flex items-center gap-1.5 text-[13px] text-muted-foreground transition-colors hover:text-foreground"
        >
          <ArrowLeft class="h-4 w-4" /> {{ t('auth.login.backHome') }}
        </RouterLink>
        <span class="ember-pill">{{ t('auth.register.title') }}</span>
      </div>

      <div class="rounded-[var(--r-lg)] border border-border bg-card p-7 shadow-[var(--shadow-lg)] sm:p-9">
        <!-- 注册关闭 -->
        <div v-if="!registrationEnabled" class="flex flex-col items-center gap-4 py-7 text-center">
          <p class="text-[13.5px] text-muted-foreground">{{ t('auth.register.registrationDisabled') }}</p>
          <RouterLink to="/auth/login" class="text-[13.5px] font-medium text-[color:var(--accent)] hover:underline">
            {{ t('auth.register.hasAccount') }}
          </RouterLink>
        </div>

        <template v-else>
          <div class="mb-7 flex flex-col items-center gap-2 text-center">
            <p class="text-[11px] font-semibold uppercase tracking-[0.2em] text-[color:var(--accent)]">{{ brandSiteName }}</p>
            <h1 class="text-[26px] font-semibold tracking-[-0.025em]">{{ t('auth.register.title') }}</h1>
            <p class="text-[13.5px] text-muted-foreground">{{ t('auth.register.subtitle') }}</p>
          </div>

          <form class="flex flex-col gap-4" @submit.prevent="handleRegister">
            <!-- 邮箱 -->
            <div class="flex flex-col gap-2">
              <span class="flex items-center gap-1.5 text-[12px] font-medium text-muted-foreground">
                <Mail class="h-3.5 w-3.5" /> {{ t('auth.register.emailLabel') }}
              </span>

              <div v-if="emailDomainSelectionRequired" class="grid gap-2 sm:grid-cols-[minmax(0,1fr)_minmax(9rem,auto)]">
                <Input
                  v-model="emailLocalPart"
                  type="text"
                  required
                  autocomplete="username"
                  class="h-12"
                  :class="{ 'ring-2 ring-[color:var(--danger)]/40': formValidation.hasError('email') }"
                  :placeholder="t('auth.register.emailLocalPlaceholder')"
                  @blur="touchRegistrationEmail"
                />
                <Select v-model="selectedEmailDomain" @update:model-value="touchRegistrationEmail">
                  <SelectTrigger class="h-12 w-full" :class="{ 'ring-2 ring-[color:var(--danger)]/40': formValidation.hasError('email') }">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem v-for="domain in allowedEmailDomains" :key="domain" :value="domain">@{{ domain }}</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <Input
                v-else
                v-model="email"
                type="email"
                required
                class="h-12"
                :class="{ 'ring-2 ring-[color:var(--danger)]/40': formValidation.hasError('email') }"
                :placeholder="t('auth.register.emailPlaceholder')"
                @blur="touchRegistrationEmail"
              />

              <p v-if="formValidation.hasError('email')" class="text-[12px] text-[color:var(--danger-strong)]">
                {{ formValidation.getError('email') }}
              </p>
              <p v-else-if="emailDomainSelectionRequired" class="text-[12px] text-muted-foreground">
                {{ t('auth.register.emailDomainSelectHint') }}
              </p>
              <p v-else-if="emailDomainAllowlistEnabled" class="text-[12px] text-muted-foreground">
                {{ allowedEmailDomains.length > 0
                  ? t('auth.register.allowedEmailDomainsHint', { domains: allowedEmailDomainsText })
                  : t('auth.register.noAllowedEmailDomainsHint') }}
              </p>
            </div>

            <!-- 密码 -->
            <div class="flex flex-col gap-2">
              <span class="flex items-center gap-1.5 text-[12px] font-medium text-muted-foreground">
                <Lock class="h-3.5 w-3.5" /> {{ t('auth.register.passwordLabel') }}
              </span>
              <div class="relative">
                <Input
                  v-model="password"
                  :type="showPassword ? 'text' : 'password'"
                  required
                  class="h-12 pr-11"
                  :class="{ 'ring-2 ring-[color:var(--danger)]/40': formValidation.hasError('password') }"
                  :placeholder="t('auth.register.passwordPlaceholder')"
                  @blur="formValidation.touchField('password', password)"
                />
                <button
                  type="button"
                  class="absolute right-3.5 top-1/2 -translate-y-1/2 text-muted-foreground transition-colors hover:text-foreground"
                  :aria-label="showPassword ? t('auth.common.hidePassword') : t('auth.common.showPassword')"
                  @click="showPassword = !showPassword"
                >
                  <EyeOff v-if="showPassword" class="h-4 w-4" />
                  <Eye v-else class="h-4 w-4" />
                </button>
              </div>

              <p v-if="formValidation.hasError('password')" class="text-[12px] text-[color:var(--danger-strong)]">
                {{ formValidation.getError('password') }}
              </p>

              <!-- 强度条 -->
              <div v-if="password && !formValidation.hasError('password')" class="flex items-center gap-2">
                <div class="flex flex-1 gap-1.5">
                  <div class="h-1 flex-1 rounded-full transition-colors" :class="strengthBarClass(0)" />
                  <div class="h-1 flex-1 rounded-full transition-colors" :class="strengthBarClass(1)" />
                  <div class="h-1 flex-1 rounded-full transition-colors" :class="strengthBarClass(2)" />
                </div>
                <span class="text-[11.5px] font-medium" :class="strengthTextClass">
                  {{ t(`formValidation.passwordStrength.${passwordStrength}`) }}
                </span>
              </div>
            </div>

            <!-- 图形验证码 -->
            <div v-if="emailVerificationEnabled && sendCodeCaptchaEnabled" class="flex flex-col gap-2">
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
                <ShieldCheck class="h-3.5 w-3.5" /> {{ t('auth.register.codeLabel') }}
              </span>
              <div class="flex gap-2.5">
                <Input
                  v-model="code"
                  type="text"
                  required
                  class="h-12 min-w-0 flex-1"
                  :placeholder="t('auth.register.codePlaceholder')"
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

            <!-- 协议 -->
            <label class="flex items-start gap-3 rounded-[var(--r)] border border-[color:var(--border-soft)] bg-[color:var(--surface-2)] px-4 py-3 text-[13px] leading-[1.6] text-muted-foreground">
              <input v-model="agreed" type="checkbox" class="mt-0.5 h-4 w-4 shrink-0 accent-[var(--ui-accent)]" />
              <span>
                {{ t('auth.register.agreementPrefix') }}
                <RouterLink to="/privacy" target="_blank" rel="noopener noreferrer" class="font-medium text-[color:var(--accent)] hover:underline">{{ t('footer.privacy') }}</RouterLink>
                {{ t('auth.register.agreementAnd') }}
                <RouterLink to="/terms" target="_blank" rel="noopener noreferrer" class="font-medium text-[color:var(--accent)] hover:underline">{{ t('footer.terms') }}</RouterLink>
              </span>
            </label>

            <p v-if="error" class="rounded-[var(--r)] bg-[color:var(--danger-soft)] px-4 py-2.5 text-center text-[13px] text-[color:var(--danger-strong)]">
              {{ error }}
            </p>

            <Button type="submit" :disabled="userAuthStore.loading || !agreed" class="h-12 w-full rounded-full text-[15px] font-medium">
              <UserPlus v-if="!userAuthStore.loading" class="mr-1.5 h-4 w-4" />
              {{ userAuthStore.loading ? t('auth.register.creating') : t('auth.register.create') }}
            </Button>
          </form>
        </template>
      </div>

      <div class="mt-4 text-center">
        <RouterLink to="/auth/login" class="text-[13px] text-muted-foreground transition-colors hover:text-foreground">
          {{ t('auth.register.hasAccount') }}
        </RouterLink>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowLeft, Eye, EyeOff, Lock, Mail, ShieldCheck, UserPlus } from 'lucide-vue-next'
import ImageCaptcha from '../../../components/captcha/ImageCaptcha.vue'
import TurnstileCaptcha from '../../../components/captcha/TurnstileCaptcha.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useRegister } from '../../../composables/useRegister'

const { t } = useI18n()

const {
  userAuthStore, brandSiteName,
  email, emailLocalPart, selectedEmailDomain, password, showPassword, code, agreed,
  passwordStrength, error, sending, countdown,
  captchaPayload, turnstileToken, imageCaptchaRef, turnstileRef,
  captchaProvider, sendCodeCaptchaEnabled, turnstileSiteKey,
  registrationEnabled, emailVerificationEnabled,
  emailDomainAllowlistEnabled, allowedEmailDomains, allowedEmailDomainsText, emailDomainSelectionRequired,
  touchRegistrationEmail, formValidation, handleCaptchaConfigStale, handleSendCode, handleRegister,
} = useRegister()

// 强度条：弱 → 中 → 强，用 Ember 的功能色
const strengthBarClass = (index: number) => {
  const level = passwordStrength.value
  const active = level === 'weak' ? 1 : level === 'medium' ? 2 : 3
  if (index >= active) return 'bg-[color:var(--surface-sunken)]'
  if (level === 'weak') return 'bg-[color:var(--danger)]'
  if (level === 'medium') return 'bg-[color:var(--warning)]'
  return 'bg-[color:var(--success)]'
}

const strengthTextClass = computed(() => {
  const level = passwordStrength.value
  if (level === 'weak') return 'text-[color:var(--danger-strong)]'
  if (level === 'medium') return 'text-[color:var(--warning-strong)]'
  return 'text-[color:var(--success-strong)]'
})

// imageCaptchaRef / turnstileRef 仅通过字符串模板 ref 绑定，显式标记避免 noUnusedLocals 误报。
void imageCaptchaRef
void turnstileRef
</script>
