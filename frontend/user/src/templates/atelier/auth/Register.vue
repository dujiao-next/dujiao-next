<template>
  <div class="relative flex min-h-screen items-center justify-center px-4 pb-16 pt-[96px] text-foreground sm:px-6">
    <div class="w-full max-w-[460px]">
      <div class="mb-4">
        <RouterLink to="/" class="theme-pill-link pl-2.5 pr-3.5">
          <ChevronLeft class="h-3.5 w-3.5" aria-hidden="true" />{{ t('auth.login.backHome') }}
        </RouterLink>
      </div>

      <div class="atelier-card rounded-[16px] bg-card p-6 sm:p-8">
        <div v-if="!registrationEnabled" class="py-8 text-center">
          <p class="text-sm text-muted-foreground">{{ t('auth.register.registrationDisabled') }}</p>
          <RouterLink to="/auth/login" class="mt-4 inline-block text-sm font-semibold text-foreground underline underline-offset-4">
            {{ t('auth.register.hasAccount') }}
          </RouterLink>
        </div>

        <template v-else>
          <div class="mb-6 border-b pb-5">
            <span class="inline-flex h-5 items-center rounded-[6px] border bg-background px-2 text-[11px] font-semibold tracking-[0.03em] text-muted-foreground">
              {{ brandSiteName }}
            </span>
            <h1 class="mt-2.5 text-[26px] font-bold leading-tight tracking-[-0.025em]">{{ t('auth.register.title') }}</h1>
            <p class="mt-1.5 text-sm text-muted-foreground">{{ t('auth.register.subtitle') }}</p>
          </div>

          <form class="space-y-5" @submit.prevent="handleRegister">
            <div v-if="inviteOnlyEnabled">
              <label class="mb-2 flex items-center gap-1.5 text-[12.5px] font-semibold text-muted-foreground">
                <Ticket class="h-3.5 w-3.5 opacity-60" />
                {{ t('auth.invite.codeLabel') }}
              </label>
              <Input v-model="inviteCode" type="text" required autocomplete="off" class="h-11 rounded-[12px]" :placeholder="t('auth.invite.codePlaceholder')" />
              <p class="mt-1.5 text-xs text-muted-foreground">{{ t('auth.invite.registerHint') }}</p>
            </div>

            <div>
              <label class="mb-2 flex items-center gap-1.5 text-[12.5px] font-semibold text-muted-foreground">
                <Mail class="h-3.5 w-3.5 opacity-60" />
                {{ t('auth.register.emailLabel') }}
              </label>
              <div v-if="emailDomainSelectionRequired" class="grid gap-2 sm:grid-cols-[minmax(0,1fr)_minmax(9rem,auto)]">
                <Input
                  v-model="emailLocalPart"
                  type="text"
                  required
                  autocomplete="username"
                  class="h-11 rounded-[12px]"
                  :class="{ 'ring-2 ring-destructive/50': formValidation.hasError('email') }"
                  :placeholder="t('auth.register.emailLocalPlaceholder')"
                  @blur="touchRegistrationEmail"
                />
                <Select v-model="selectedEmailDomain" @update:model-value="touchRegistrationEmail">
                  <SelectTrigger class="h-11 w-full rounded-[12px]" :class="{ 'ring-2 ring-destructive/50': formValidation.hasError('email') }">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem v-for="domain in allowedEmailDomains" :key="domain" :value="domain">
                      @{{ domain }}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <Input
                v-else
                v-model="email"
                type="email"
                required
                class="h-11 rounded-[12px]"
                :class="{ 'ring-2 ring-destructive/50': formValidation.hasError('email') }"
                :placeholder="t('auth.register.emailPlaceholder')"
                @blur="touchRegistrationEmail"
              />
              <p v-if="formValidation.hasError('email')" class="mt-1.5 text-xs text-destructive">
                {{ formValidation.getError('email') }}
              </p>
              <p v-else-if="emailDomainSelectionRequired" class="mt-1.5 text-xs text-muted-foreground">
                {{ t('auth.register.emailDomainSelectHint') }}
              </p>
              <p v-else-if="emailDomainAllowlistEnabled" class="mt-1.5 text-xs text-muted-foreground">
                {{ allowedEmailDomains.length > 0
                  ? t('auth.register.allowedEmailDomainsHint', { domains: allowedEmailDomainsText })
                  : t('auth.register.noAllowedEmailDomainsHint') }}
              </p>
            </div>

            <div>
              <label class="mb-2 flex items-center gap-1.5 text-[12.5px] font-semibold text-muted-foreground">
                <Lock class="h-3.5 w-3.5 opacity-60" />
                {{ t('auth.register.passwordLabel') }}
              </label>
              <div class="relative">
                <Input
                  v-model="password"
                  :type="showPassword ? 'text' : 'password'"
                  required
                  class="h-11 rounded-[12px] pr-10"
                  :class="{ 'ring-2 ring-destructive/50': formValidation.hasError('password') }"
                  :placeholder="t('auth.register.passwordPlaceholder')"
                  @blur="formValidation.touchField('password', password)"
                />
                <button
                  type="button"
                  class="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground transition-colors hover:text-foreground"
                  @click="showPassword = !showPassword"
                >
                  <EyeOff v-if="showPassword" class="h-4 w-4" />
                  <Eye v-else class="h-4 w-4" />
                </button>
              </div>
              <p v-if="formValidation.hasError('password')" class="mt-1.5 text-xs text-destructive">
                {{ formValidation.getError('password') }}
              </p>
              <div v-if="password && !formValidation.hasError('password')" class="mt-2 flex items-center gap-2">
                <div class="flex flex-1 gap-1">
                  <div class="h-1 flex-1 rounded-full transition-colors" :class="passwordStrength === 'weak' ? 'bg-danger' : passwordStrength === 'medium' ? 'bg-warning' : 'bg-success'" />
                  <div class="h-1 flex-1 rounded-full transition-colors" :class="passwordStrength === 'medium' ? 'bg-warning' : passwordStrength === 'strong' ? 'bg-success' : 'bg-secondary'" />
                  <div class="h-1 flex-1 rounded-full transition-colors" :class="passwordStrength === 'strong' ? 'bg-success' : 'bg-secondary'" />
                </div>
                <span class="text-[11px] font-semibold" :class="passwordStrength === 'weak' ? 'text-danger' : passwordStrength === 'medium' ? 'text-warning' : 'text-success'">
                  {{ t(`formValidation.passwordStrength.${passwordStrength}`) }}
                </span>
              </div>
            </div>

            <div v-if="emailVerificationEnabled && sendCodeCaptchaEnabled">
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

            <div v-if="emailVerificationEnabled">
              <label class="mb-2 flex items-center gap-1.5 text-[12.5px] font-semibold text-muted-foreground">
                <ShieldCheck class="h-3.5 w-3.5 opacity-60" />
                {{ t('auth.register.codeLabel') }}
              </label>
              <div class="flex flex-col gap-2 sm:flex-row">
                <Input v-model="code" type="text" required class="h-11 min-w-0 flex-1 rounded-[12px]" :placeholder="t('auth.register.codePlaceholder')" />
                <Button type="button" variant="secondary" class="h-11 whitespace-nowrap rounded-[11px] px-4" :disabled="sending || countdown > 0" @click="handleSendCode">
                  {{ countdown > 0 ? t('auth.common.countdown', { seconds: countdown }) : t('auth.common.sendCode') }}
                </Button>
              </div>
            </div>

            <label class="flex cursor-pointer items-start gap-3 rounded-[14px] border bg-background px-4 py-3 text-xs text-muted-foreground">
              <Checkbox v-model="agreed" class="mt-0.5" />
              <span class="leading-5">
                {{ t('auth.register.agreementPrefix') }}
                <RouterLink to="/privacy" target="_blank" rel="noopener noreferrer" class="font-semibold text-foreground underline underline-offset-2">
                  {{ t('footer.privacy') }}
                </RouterLink>
                {{ t('auth.register.agreementAnd') }}
                <RouterLink to="/terms" target="_blank" rel="noopener noreferrer" class="font-semibold text-foreground underline underline-offset-2">
                  {{ t('footer.terms') }}
                </RouterLink>
              </span>
            </label>

            <Alert v-if="error" variant="destructive" class="text-center">
              <AlertDescription>{{ error }}</AlertDescription>
            </Alert>

            <Button type="submit" :disabled="userAuthStore.loading || !agreed" class="h-11.5 w-full rounded-[11px] bg-primary font-semibold text-primary-foreground shadow-xs hover:bg-primary/90">
              <UserPlus v-if="!userAuthStore.loading" class="h-4 w-4" />
              {{ userAuthStore.loading ? t('auth.register.creating') : t('auth.register.create') }}
            </Button>
          </form>
        </template>
      </div>

      <div class="mt-4 text-center">
        <RouterLink :to="loginLink" class="text-sm font-medium text-foreground underline-offset-4 hover:underline">
          {{ t('auth.register.hasAccount') }}
        </RouterLink>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ChevronLeft, Eye, EyeOff, Lock, Mail, ShieldCheck, Ticket, UserPlus } from 'lucide-vue-next'
import ImageCaptcha from '../../../components/captcha/ImageCaptcha.vue'
import TurnstileCaptcha from '../../../components/captcha/TurnstileCaptcha.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useRegister } from '../../../composables/useRegister'

const { t } = useI18n()

const {
  userAuthStore, brandSiteName,
  email, emailLocalPart, selectedEmailDomain, password, showPassword, code, agreed, inviteCode, inviteOnlyEnabled, loginLink,
  passwordStrength, error, sending, countdown,
  captchaPayload, turnstileToken, imageCaptchaRef, turnstileRef,
  captchaProvider, sendCodeCaptchaEnabled, turnstileSiteKey,
  registrationEnabled, emailVerificationEnabled,
  emailDomainAllowlistEnabled, allowedEmailDomains, allowedEmailDomainsText, emailDomainSelectionRequired,
  touchRegistrationEmail, formValidation, handleCaptchaConfigStale, handleSendCode, handleRegister,
} = useRegister()

void imageCaptchaRef
void turnstileRef
</script>
