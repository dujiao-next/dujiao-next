<script setup lang="ts">
import { computed, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Dialog, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { DialogContent, DialogOverlay, DialogPortal } from 'reka-ui'
import ImageCaptcha from '@/components/captcha/ImageCaptcha.vue'
import TurnstileCaptcha from '@/components/captcha/TurnstileCaptcha.vue'
import { adminAPI, type CaptchaPayload } from '@/api/admin'
import { useAdminAuthStore } from '@/stores/auth'
import { registerReloginHandler } from '@/api/session'

const { t } = useI18n()
const authStore = useAdminAuthStore()

type Step = 'password' | 'totp'

const open = ref(false)
const step = ref<Step>('password')
const submitting = ref(false)
const error = ref('')
const username = ref('')
const password = ref('')
const totpCode = ref('')
const recoveryCode = ref('')
const useRecovery = ref(false)

const captchaProvider = ref('none')
const captchaEnabled = ref(false)
const captchaPayload = ref<CaptchaPayload>({})
const turnstileToken = ref('')
const turnstileSiteKey = ref('')
const imageCaptchaRef = ref<InstanceType<typeof ImageCaptcha> | null>(null)
const turnstileRef = ref<InstanceType<typeof TurnstileCaptcha> | null>(null)

let pendingResolve: ((ok: boolean) => void) | null = null

const title = computed(() => (step.value === 'totp' ? t('relogin.totpTitle') : t('relogin.title')))
const subtitle = computed(() => (step.value === 'totp' ? t('relogin.totpHint') : t('relogin.subtitle')))

const reset = () => {
  step.value = 'password'
  submitting.value = false
  error.value = ''
  password.value = ''
  totpCode.value = ''
  recoveryCode.value = ''
  useRecovery.value = false
}

const finish = (ok: boolean) => {
  open.value = false
  const resolve = pendingResolve
  pendingResolve = null
  reset()
  resolve?.(ok)
}

const handleOpenChange = (value: boolean) => {
  if (!value && pendingResolve) {
    finish(false)
  }
}

const loadCaptcha = async () => {
  try {
    const res = await adminAPI.getPublicConfig()
    const payload = (res.data?.data || {}) as any
    const captcha = payload?.captcha || {}
    captchaProvider.value = String(captcha?.provider || 'none')
    captchaEnabled.value = Boolean(captcha?.scenes?.login) && captchaProvider.value !== 'none'
    turnstileSiteKey.value = String(captcha?.turnstile?.site_key || '')
  } catch {
    captchaEnabled.value = false
  }
}

registerReloginHandler(() => {
  open.value = true
  reset()
  loadCaptcha()
  return new Promise<boolean>((resolve) => {
    pendingResolve = resolve
  })
})

onUnmounted(() => {
  registerReloginHandler(null)
})

const getCaptchaPayload = (): CaptchaPayload | undefined => {
  if (!captchaEnabled.value) return undefined
  if (captchaProvider.value === 'image') {
    return {
      captcha_id: captchaPayload.value.captcha_id || '',
      captcha_code: captchaPayload.value.captcha_code || '',
    }
  }
  if (captchaProvider.value === 'turnstile') {
    return { turnstile_token: turnstileToken.value }
  }
  return undefined
}

const submitPassword = async () => {
  error.value = ''
  if (!username.value.trim() || !password.value) {
    error.value = t('relogin.errors.empty')
    return
  }
  submitting.value = true
  try {
    const res = await authStore.login({
      username: username.value.trim(),
      password: password.value,
      captcha_payload: getCaptchaPayload(),
    })
    if (res.requiresTotp) {
      step.value = 'totp'
      totpCode.value = ''
      recoveryCode.value = ''
      useRecovery.value = false
      return
    }
    finish(true)
  } catch (err: any) {
    error.value = err?.message || t('relogin.errors.failed')
    if (captchaProvider.value === 'image') imageCaptchaRef.value?.refresh()
    if (captchaProvider.value === 'turnstile') {
      turnstileRef.value?.reset()
      turnstileToken.value = ''
    }
  } finally {
    submitting.value = false
  }
}

const submitTotp = async () => {
  error.value = ''
  if (useRecovery.value) {
    if (!recoveryCode.value.trim()) {
      error.value = t('relogin.errors.recoveryEmpty')
      return
    }
  } else if (totpCode.value.length !== 6) {
    error.value = t('relogin.errors.totpEmpty')
    return
  }

  submitting.value = true
  try {
    await authStore.verify2FA(
      useRecovery.value ? { recovery_code: recoveryCode.value.trim() } : { code: totpCode.value },
    )
    finish(true)
  } catch (err: any) {
    error.value = err?.message || t('relogin.errors.totpFailed')
  } finally {
    submitting.value = false
  }
}

const backToPassword = () => {
  authStore.clearChallenge()
  step.value = 'password'
  password.value = ''
  totpCode.value = ''
  recoveryCode.value = ''
  useRecovery.value = false
  error.value = ''
}
</script>

<template>
  <Dialog :open="open" @update:open="handleOpenChange">
    <DialogPortal>
      <DialogOverlay
        class="fixed inset-0 z-[190] bg-black/80 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0"
      />
      <DialogContent
        class="fixed left-1/2 top-1/2 z-[191] grid w-full max-w-md -translate-x-1/2 -translate-y-1/2 gap-4 border bg-background p-6 shadow-lg duration-200 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95 sm:rounded-lg"
      >
        <DialogHeader>
          <DialogTitle>{{ title }}</DialogTitle>
          <DialogDescription>{{ subtitle }}</DialogDescription>
        </DialogHeader>

        <form v-if="step === 'password'" class="space-y-4" @submit.prevent="submitPassword">
          <div class="space-y-2">
            <Label for="relogin-username">{{ t('relogin.username') }}</Label>
            <Input
              id="relogin-username"
              v-model="username"
              autocomplete="username"
              :placeholder="t('relogin.username')"
            />
          </div>
          <div class="space-y-2">
            <Label for="relogin-password">{{ t('relogin.password') }}</Label>
            <Input
              id="relogin-password"
              v-model="password"
              type="password"
              autocomplete="current-password"
              :placeholder="t('relogin.password')"
            />
          </div>

          <div v-if="captchaEnabled" class="space-y-2">
            <Label>{{ t('relogin.captcha') }}</Label>
            <ImageCaptcha
              v-if="captchaProvider === 'image'"
              ref="imageCaptchaRef"
              v-model="captchaPayload"
              :disabled="submitting"
            />
            <TurnstileCaptcha
              v-else-if="captchaProvider === 'turnstile'"
              ref="turnstileRef"
              v-model="turnstileToken"
              :site-key="turnstileSiteKey"
            />
          </div>

          <div v-if="error" class="text-sm text-destructive">{{ error }}</div>

          <DialogFooter>
            <Button type="button" variant="outline" @click="finish(false)">{{ t('relogin.later') }}</Button>
            <Button type="submit" :disabled="submitting">
              {{ submitting ? t('relogin.submitting') : t('relogin.submit') }}
            </Button>
          </DialogFooter>
        </form>

        <form v-else class="space-y-4" @submit.prevent="submitTotp">
          <div class="space-y-2">
            <Label :for="useRecovery ? 'relogin-recovery' : 'relogin-totp'">
              {{ useRecovery ? t('relogin.recoveryCode') : t('relogin.totpCode') }}
            </Label>
            <Input
              v-if="useRecovery"
              id="relogin-recovery"
              v-model="recoveryCode"
              autocomplete="one-time-code"
            />
            <Input
              v-else
              id="relogin-totp"
              v-model="totpCode"
              inputmode="numeric"
              maxlength="6"
              autocomplete="one-time-code"
            />
          </div>

          <button
            type="button"
            class="text-sm text-muted-foreground underline-offset-4 hover:underline"
            @click="useRecovery = !useRecovery"
          >
            {{ useRecovery ? t('relogin.useTotp') : t('relogin.useRecovery') }}
          </button>

          <div v-if="error" class="text-sm text-destructive">{{ error }}</div>

          <DialogFooter>
            <Button type="button" variant="outline" @click="backToPassword">{{ t('relogin.back') }}</Button>
            <Button type="submit" :disabled="submitting">
              {{ submitting ? t('relogin.verifying') : t('relogin.verify') }}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </DialogPortal>
  </Dialog>
</template>
