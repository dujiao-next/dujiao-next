<template>
  <div class="flex min-h-[70vh] items-center justify-center px-4 py-12">
    <div class="w-full max-w-[480px]">
      <div class="mb-3.5 flex items-center justify-between px-1">
        <RouterLink to="/" class="inline-flex items-center gap-1.5 text-sm font-semibold text-muted-foreground transition-colors hover:text-primary">
          <ArrowLeft class="h-4 w-4" /> {{ t('auth.login.backHome') }}
        </RouterLink>
        <Badge variant="neutral" size="sm" class="rounded-full">{{ t('auth.register.title') }}</Badge>
      </div>

      <Card class="p-7 shadow-[var(--shadow-lg)] sm:p-9">
        <div v-if="!registrationEnabled" class="py-7 text-center">
          <p class="text-sm text-muted-foreground">{{ t('auth.register.registrationDisabled') }}</p>
          <RouterLink to="/auth/login" class="mt-4 inline-block text-sm font-semibold text-primary hover:underline">
            {{ t('auth.register.hasAccount') }}
          </RouterLink>
        </div>

        <template v-else>
          <div class="mb-7 text-center">
            <img v-if="brandLogo" :src="brandLogo" :alt="brandSiteName" class="mx-auto h-16 w-16 object-contain" />
            <p class="mt-3 text-[11px] font-bold uppercase tracking-[0.2em] text-primary">{{ brandSiteName }}</p>
            <h1 class="mt-3 text-3xl font-extrabold">{{ t('auth.register.title') }}</h1>
            <p class="mt-2 text-sm text-muted-foreground">{{ t('auth.register.subtitle') }}</p>
          </div>

          <form class="grid gap-[18px]" @submit.prevent="handleRegister">
            <!-- 邮箱 -->
            <div>
              <label class="mb-2 flex items-center gap-1.5 text-[11px] font-bold uppercase tracking-[0.12em] text-muted-foreground">
                <Mail class="h-3.5 w-3.5 opacity-70" /> {{ t('auth.register.emailLabel') }}
              </label>
              <div v-if="emailDomainSelectionRequired" class="grid gap-2 sm:grid-cols-[minmax(0,1fr)_minmax(9rem,auto)]">
                <Input
                  v-model="emailLocalPart"
                  type="text"
                  required
                  autocomplete="username"
                  class="h-11"
                  :class="{ 'ring-2 ring-destructive/50': formValidation.hasError('email') }"
                  :placeholder="t('auth.register.emailLocalPlaceholder')"
                  @blur="touchRegistrationEmail"
                />
                <Select v-model="selectedEmailDomain" @update:model-value="touchRegistrationEmail">
                  <SelectTrigger class="h-11 w-full" :class="{ 'ring-2 ring-destructive/50': formValidation.hasError('email') }">
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
                class="h-11"
                :class="{ 'ring-2 ring-destructive/50': formValidation.hasError('email') }"
                :placeholder="t('auth.register.emailPlaceholder')"
                @blur="touchRegistrationEmail"
              />
              <p v-if="formValidation.hasError('email')" class="mt-1.5 text-xs text-destructive">{{ formValidation.getError('email') }}</p>
              <p v-else-if="emailDomainSelectionRequired" class="mt-1.5 text-xs text-muted-foreground">{{ t('auth.register.emailDomainSelectHint') }}</p>
              <p v-else-if="emailDomainAllowlistEnabled" class="mt-1.5 text-xs text-muted-foreground">
                {{ allowedEmailDomains.length > 0
                  ? t('auth.register.allowedEmailDomainsHint', { domains: allowedEmailDomainsText })
                  : t('auth.register.noAllowedEmailDomainsHint') }}
              </p>
            </div>

            <!-- 密码 -->
            <div>
              <label class="mb-2 flex items-center gap-1.5 text-[11px] font-bold uppercase tracking-[0.12em] text-muted-foreground">
                <Lock class="h-3.5 w-3.5 opacity-70" /> {{ t('auth.register.passwordLabel') }}
              </label>
              <div class="relative">
                <Input
                  v-model="password"
                  :type="showPassword ? 'text' : 'password'"
                  required
                  class="h-11 pr-10"
                  :class="{ 'ring-2 ring-destructive/50': formValidation.hasError('password') }"
                  :placeholder="t('auth.register.passwordPlaceholder')"
                  @blur="formValidation.touchField('password', password)"
                />
                <button
                  type="button"
                  class="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground transition-colors hover:text-foreground"
                  :aria-label="showPassword ? t('auth.common.hidePassword') : t('auth.common.showPassword')"
                  @click="showPassword = !showPassword"
                >
                  <EyeOff v-if="showPassword" class="h-4 w-4" /><Eye v-else class="h-4 w-4" />
                </button>
              </div>
              <p v-if="formValidation.hasError('password')" class="mt-1.5 text-xs text-destructive">{{ formValidation.getError('password') }}</p>
              <div v-if="password && !formValidation.hasError('password')" class="mt-2 flex items-center gap-2">
                <div class="flex flex-1 gap-1.5">
                  <div class="h-1 flex-1 rounded-full transition-colors" :class="passwordStrength === 'weak' ? 'bg-destructive' : passwordStrength === 'medium' ? 'bg-warm' : 'bg-success'" />
                  <div class="h-1 flex-1 rounded-full transition-colors" :class="passwordStrength === 'medium' ? 'bg-warm' : passwordStrength === 'strong' ? 'bg-success' : 'bg-muted'" />
                  <div class="h-1 flex-1 rounded-full transition-colors" :class="passwordStrength === 'strong' ? 'bg-success' : 'bg-muted'" />
                </div>
                <span class="text-[11px] font-bold" :class="passwordStrength === 'weak' ? 'text-destructive' : passwordStrength === 'medium' ? 'text-warm' : 'text-success'">
                  {{ t(`formValidation.passwordStrength.${passwordStrength}`) }}
                </span>
              </div>
            </div>

            <!-- 图形验证 -->
            <div v-if="emailVerificationEnabled && sendCodeCaptchaEnabled">
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

            <!-- 邮箱验证码 -->
            <div v-if="emailVerificationEnabled">
              <label class="mb-2 flex items-center gap-1.5 text-[11px] font-bold uppercase tracking-[0.12em] text-muted-foreground">
                <ShieldCheck class="h-3.5 w-3.5 opacity-70" /> {{ t('auth.register.codeLabel') }}
              </label>
              <div class="flex gap-2.5">
                <Input
                  v-model="code"
                  type="text"
                  required
                  class="h-11 min-w-0 flex-1"
                  :placeholder="t('auth.register.codePlaceholder')"
                />
                <Button type="button" variant="secondary" class="h-11 shrink-0 whitespace-nowrap rounded-full px-5" :disabled="sending || countdown > 0" @click="handleSendCode">
                  {{ countdown > 0 ? t('auth.common.countdown', { seconds: countdown }) : t('auth.common.sendCode') }}
                </Button>
              </div>
            </div>

            <!-- 注册验证 -->
            <div v-if="registerCaptchaEnabled">
              <label class="mb-2 flex items-center gap-1.5 text-[11px] font-bold uppercase tracking-[0.12em] text-muted-foreground">
                <ShieldCheck class="h-3.5 w-3.5 opacity-70" /> {{ t('auth.common.captchaLabel') }}
              </label>
              <ImageCaptcha
                v-if="captchaProvider === 'image'"
                ref="registerImageCaptchaRef"
                v-model="registerCaptchaPayload"
                :disabled="userAuthStore.loading"
                @config-stale="handleCaptchaConfigStale"
              />
              <TurnstileCaptcha
                v-else-if="captchaProvider === 'turnstile'"
                ref="registerTurnstileRef"
                v-model="registerTurnstileToken"
                :site-key="turnstileSiteKey"
              />
            </div>

            <!-- 协议 -->
            <label class="flex items-start gap-3 rounded-2xl border bg-muted px-4 py-3 text-sm leading-6 text-muted-foreground">
              <input v-model="agreed" type="checkbox" class="mt-0.5 h-4 w-4 shrink-0 accent-[var(--ui-accent)]" />
              <span>
                {{ t('auth.register.agreementPrefix') }}
                <RouterLink to="/privacy" target="_blank" rel="noopener noreferrer" class="font-semibold text-primary hover:underline">{{ t('footer.privacy') }}</RouterLink>
                {{ t('auth.register.agreementAnd') }}
                <RouterLink to="/terms" target="_blank" rel="noopener noreferrer" class="font-semibold text-primary hover:underline">{{ t('footer.terms') }}</RouterLink>
              </span>
            </label>

            <Alert v-if="error" variant="destructive" class="text-center">
              <AlertDescription>{{ error }}</AlertDescription>
            </Alert>

            <Button type="submit" :disabled="userAuthStore.loading || !agreed" class="h-11 w-full rounded-full font-bold">
              <UserPlus v-if="!userAuthStore.loading" class="h-4 w-4" />
              {{ userAuthStore.loading ? t('auth.register.creating') : t('auth.register.create') }}
            </Button>
          </form>
        </template>
      </Card>

      <div class="mt-4 text-center">
        <RouterLink to="/auth/login" class="text-sm text-muted-foreground transition-colors hover:text-foreground">{{ t('auth.register.hasAccount') }}</RouterLink>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ArrowLeft, Mail, Lock, ShieldCheck, Eye, EyeOff, UserPlus } from 'lucide-vue-next'
import ImageCaptcha from '../../../components/captcha/ImageCaptcha.vue'
import TurnstileCaptcha from '../../../components/captcha/TurnstileCaptcha.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useRegister } from '../../../composables/useRegister'

const { t } = useI18n()

const {
  userAuthStore, brandSiteName, brandLogo,
  email, emailLocalPart, selectedEmailDomain, password, showPassword, code, agreed,
  passwordStrength, error, sending, countdown,
  captchaPayload, turnstileToken, imageCaptchaRef, turnstileRef,
  registerCaptchaPayload, registerTurnstileToken, registerImageCaptchaRef, registerTurnstileRef,
  captchaProvider, registerCaptchaEnabled, sendCodeCaptchaEnabled, turnstileSiteKey,
  registrationEnabled, emailVerificationEnabled,
  emailDomainAllowlistEnabled, allowedEmailDomains, allowedEmailDomainsText, emailDomainSelectionRequired,
  touchRegistrationEmail, formValidation, handleCaptchaConfigStale, handleSendCode, handleRegister,
} = useRegister()

// imageCaptchaRef / turnstileRef 仅通过字符串模板 ref 绑定，显式标记避免 noUnusedLocals 误报。
void imageCaptchaRef
void turnstileRef
void registerImageCaptchaRef
void registerTurnstileRef
</script>
