<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { notifyError, notifySuccess } from '@/utils/notify'

const { t } = useI18n()

interface CaptchaData {
  provider: string
  scenes: {
    login: boolean
    register: boolean
    register_send_code: boolean
    reset_send_code: boolean
    guest_create_order: boolean
    gift_card_redeem: boolean
  }
  image: {
    length: number
    width: number
    height: number
    noise_count: number
    show_line: number
    expire_seconds: number
    max_store: number
  }
  turnstile: {
    site_key: string
    secret_key: string
    has_secret: boolean
    verify_url: string
    timeout_ms: number
  }
}

const props = defineProps<{
  data: CaptchaData
}>()

const emit = defineEmits<{
  saved: []
}>()

const submitting = ref(false)

const form = reactive({
  provider: 'none',
  scenes: {
    login: false,
    register: false,
    register_send_code: false,
    reset_send_code: false,
    guest_create_order: false,
    gift_card_redeem: false,
  },
  image: {
    length: 5,
    width: 240,
    height: 80,
    noise_count: 2,
    show_line: 2,
    expire_seconds: 300,
    max_store: 10240,
  },
  turnstile: {
    site_key: '',
    secret_key: '',
    has_secret: false,
    verify_url: 'https://challenges.cloudflare.com/turnstile/v0/siteverify',
    timeout_ms: 2000,
  },
})

const syncFromProps = () => {
  form.provider = props.data.provider
  Object.assign(form.scenes, props.data.scenes)
  Object.assign(form.image, props.data.image)
  form.turnstile.site_key = props.data.turnstile.site_key
  form.turnstile.secret_key = props.data.turnstile.secret_key
  form.turnstile.has_secret = props.data.turnstile.has_secret
  form.turnstile.verify_url = props.data.turnstile.verify_url
  form.turnstile.timeout_ms = props.data.turnstile.timeout_ms
}

syncFromProps()

watch(() => props.data, () => {
  syncFromProps()
}, { deep: true })

const notifyErrorIfNeeded = (err: unknown, fallback: string) => {
  const known = err as Error & { __notified?: boolean }
  if (known?.__notified) return
  notifyError(known?.message || fallback)
}

const save = async () => {
  submitting.value = true
  try {
    const payload: Record<string, unknown> = {
      provider: form.provider,
      scenes: {
        login: form.scenes.login,
        register: form.scenes.register,
        register_send_code: form.scenes.register_send_code,
        reset_send_code: form.scenes.reset_send_code,
        guest_create_order: form.scenes.guest_create_order,
        gift_card_redeem: form.scenes.gift_card_redeem,
      },
      image: {
        length: Number(form.image.length),
        width: Number(form.image.width),
        height: Number(form.image.height),
        noise_count: Number(form.image.noise_count),
        show_line: Number(form.image.show_line),
        expire_seconds: Number(form.image.expire_seconds),
        max_store: Number(form.image.max_store),
      },
      turnstile: {
        site_key: form.turnstile.site_key,
        verify_url: form.turnstile.verify_url,
        timeout_ms: Number(form.turnstile.timeout_ms),
      },
    }

    if (form.turnstile.secret_key.trim() !== '') {
      payload.turnstile = {
        ...(payload.turnstile as Record<string, unknown>),
        secret_key: form.turnstile.secret_key.trim(),
      }
    }

    const res = await adminAPI.updateCaptchaSettings(payload)
    const data = res.data?.data as Record<string, unknown> | undefined
    form.turnstile.secret_key = ''
    const resTurnstile = data?.turnstile as Record<string, unknown> | undefined
    form.turnstile.has_secret = !!resTurnstile?.has_secret || form.turnstile.has_secret
    notifySuccess(t('admin.settings.alerts.saveSuccess'))
    emit('saved')
  } catch (err) {
    notifyErrorIfNeeded(err, t('admin.settings.alerts.saveFailed'))
  } finally {
    submitting.value = false
  }
}

defineExpose({ save, submitting })
</script>

<template>
  <div class="space-y-6">
    <div class="rounded-xl border border-border bg-card">
      <div class="border-b border-border bg-muted/40 px-6 py-4">
        <h2 class="text-lg font-semibold">{{ t('admin.settings.captcha.title') }}</h2>
        <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.captcha.subtitle') }}</p>
      </div>

      <div class="space-y-6 p-6">
        <div class="space-y-2">
          <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.captcha.provider') }}</label>
          <Select v-model="form.provider">
            <SelectTrigger class="h-10">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="none">{{ t('admin.settings.captcha.providerNone') }}</SelectItem>
              <SelectItem value="image">{{ t('admin.settings.captcha.providerImage') }}</SelectItem>
              <SelectItem value="turnstile">{{ t('admin.settings.captcha.providerTurnstile') }}</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div class="rounded-xl border border-border bg-muted/20 p-4">
          <h3 class="text-sm font-semibold">{{ t('admin.settings.captcha.scenesTitle') }}</h3>
          <div class="mt-3 grid grid-cols-1 gap-3 md:grid-cols-2">
            <div class="flex items-center gap-2 text-sm">
              <Switch v-model="form.scenes.login" />
              <Label class="text-sm">{{ t('admin.settings.captcha.scenes.login') }}</Label>
            </div>
            <div class="flex items-center gap-2 text-sm">
              <Switch v-model="form.scenes.register" />
              <Label class="text-sm">{{ t('admin.settings.captcha.scenes.register') }}</Label>
            </div>
            <div class="flex items-center gap-2 text-sm">
              <Switch v-model="form.scenes.register_send_code" />
              <Label class="text-sm">{{ t('admin.settings.captcha.scenes.registerSendCode') }}</Label>
            </div>
            <div class="flex items-center gap-2 text-sm">
              <Switch v-model="form.scenes.reset_send_code" />
              <Label class="text-sm">{{ t('admin.settings.captcha.scenes.resetSendCode') }}</Label>
            </div>
            <div class="flex items-center gap-2 text-sm">
              <Switch v-model="form.scenes.guest_create_order" />
              <Label class="text-sm">{{ t('admin.settings.captcha.scenes.guestCreateOrder') }}</Label>
            </div>
            <div class="flex items-center gap-2 text-sm">
              <Switch v-model="form.scenes.gift_card_redeem" />
              <Label class="text-sm">{{ t('admin.settings.captcha.scenes.giftCardRedeem') }}</Label>
            </div>
          </div>
        </div>

        <div v-if="form.provider === 'image'" class="rounded-xl border border-border">
          <div class="border-b border-border bg-muted/30 px-4 py-3">
            <h3 class="text-sm font-semibold">{{ t('admin.settings.captcha.image.title') }}</h3>
          </div>
          <div class="grid grid-cols-1 gap-4 p-4 md:grid-cols-4">
            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.captcha.image.length') }}</label>
              <Input v-model.number="form.image.length" type="number" min="4" max="8" />
            </div>
            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.captcha.image.width') }}</label>
              <Input v-model.number="form.image.width" type="number" min="100" />
            </div>
            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.captcha.image.height') }}</label>
              <Input v-model.number="form.image.height" type="number" min="40" />
            </div>
            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.captcha.image.expireSeconds') }}</label>
              <Input v-model.number="form.image.expire_seconds" type="number" min="30" max="3600" />
            </div>
            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.captcha.image.noiseCount') }}</label>
              <Input v-model.number="form.image.noise_count" type="number" min="0" />
            </div>
            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.captcha.image.showLine') }}</label>
              <Input v-model.number="form.image.show_line" type="number" min="0" />
            </div>
            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.captcha.image.maxStore') }}</label>
              <Input v-model.number="form.image.max_store" type="number" min="100" />
            </div>
          </div>
        </div>

        <div v-if="form.provider === 'turnstile'" class="rounded-xl border border-border">
          <div class="border-b border-border bg-muted/30 px-4 py-3">
            <h3 class="text-sm font-semibold">{{ t('admin.settings.captcha.turnstile.title') }}</h3>
          </div>
          <div class="grid grid-cols-1 gap-4 p-4 md:grid-cols-2">
            <div class="space-y-2 md:col-span-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.captcha.turnstile.siteKey') }}</label>
              <Input v-model="form.turnstile.site_key" />
            </div>
            <div class="space-y-2 md:col-span-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.captcha.turnstile.secretKey') }}</label>
              <Input v-model="form.turnstile.secret_key" type="password" :placeholder="t('admin.settings.captcha.turnstile.secretKeyPlaceholder')" />
              <p class="text-xs text-muted-foreground">
                {{ form.turnstile.has_secret ? t('admin.settings.captcha.turnstile.secretHintKeep') : t('admin.settings.captcha.turnstile.secretHintEmpty') }}
              </p>
            </div>
            <div class="space-y-2 md:col-span-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.captcha.turnstile.verifyURL') }}</label>
              <Input v-model="form.turnstile.verify_url" />
            </div>
            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.captcha.turnstile.timeoutMS') }}</label>
              <Input v-model.number="form.turnstile.timeout_ms" type="number" min="500" max="10000" />
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
