<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { notifyError, notifySuccess } from '@/utils/notify'

const { t } = useI18n()

interface OrderNotificationStatuses {
  paid: boolean
  delivered: boolean
  completed: boolean
  refunded: boolean
}

const orderStatusKeys = ['paid', 'delivered', 'completed', 'refunded'] as const

interface SMTPData {
  enabled: boolean
  host: string
  port: number
  username: string
  password: string
  has_password: boolean
  from: string
  from_name: string
  use_tls: boolean
  use_ssl: boolean
  order_notification_enabled: boolean
  order_notification_statuses: OrderNotificationStatuses
  verify_code: {
    expire_minutes: number
    send_interval_seconds: number
    max_attempts: number
    length: number
  }
}

const props = defineProps<{
  data: SMTPData
}>()

const emit = defineEmits<{
  saved: []
}>()

const submitting = ref(false)
const smtpTesting = ref(false)

const form = reactive({
  enabled: false,
  host: '',
  port: 587,
  username: '',
  password: '',
  has_password: false,
  from: '',
  from_name: '',
  use_tls: true,
  use_ssl: false,
  order_notification_enabled: true,
  order_notification_statuses: { paid: true, delivered: true, completed: true, refunded: true } as OrderNotificationStatuses,
  verify_code: {
    expire_minutes: 10,
    send_interval_seconds: 60,
    max_attempts: 5,
    length: 6,
  },
  test_email: '',
})

const syncFromProps = () => {
  form.enabled = props.data.enabled
  form.host = props.data.host
  form.port = props.data.port
  form.username = props.data.username
  form.password = props.data.password
  form.has_password = props.data.has_password
  form.from = props.data.from
  form.from_name = props.data.from_name
  form.use_tls = props.data.use_tls
  form.use_ssl = props.data.use_ssl
  form.order_notification_enabled = props.data.order_notification_enabled ?? true
  for (const key of orderStatusKeys) {
    form.order_notification_statuses[key] = props.data.order_notification_statuses?.[key] ?? true
  }
  form.verify_code.expire_minutes = props.data.verify_code.expire_minutes
  form.verify_code.send_interval_seconds = props.data.verify_code.send_interval_seconds
  form.verify_code.max_attempts = props.data.verify_code.max_attempts
  form.verify_code.length = props.data.verify_code.length
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
    const payload = {
      enabled: form.enabled,
      host: form.host,
      port: Number(form.port),
      username: form.username,
      password: form.password,
      from: form.from,
      from_name: form.from_name,
      use_tls: form.use_tls,
      use_ssl: form.use_ssl,
      order_notification_enabled: form.order_notification_enabled,
      order_notification_statuses: { ...form.order_notification_statuses },
      verify_code: {
        expire_minutes: Number(form.verify_code.expire_minutes),
        send_interval_seconds: Number(form.verify_code.send_interval_seconds),
        max_attempts: Number(form.verify_code.max_attempts),
        length: Number(form.verify_code.length),
      },
    }
    const res = await adminAPI.updateSMTPSettings(payload)
    const data = res.data?.data as Record<string, unknown> | undefined
    form.password = ''
    form.has_password = !!data?.has_password || form.has_password
    notifySuccess(t('admin.settings.alerts.saveSuccess'))
    emit('saved')
  } catch (err) {
    notifyErrorIfNeeded(err, t('admin.settings.alerts.saveFailed'))
  } finally {
    submitting.value = false
  }
}

const testSMTPSettings = async () => {
  if (!form.test_email || form.test_email.trim() === '') {
    notifyError(t('admin.settings.smtp.testEmailRequired'))
    return
  }
  smtpTesting.value = true
  try {
    await adminAPI.testSMTPSettings({ to_email: form.test_email.trim() })
    notifySuccess(t('admin.settings.smtp.testSuccess'))
  } catch (err) {
    notifyErrorIfNeeded(err, t('admin.settings.smtp.testFailed'))
  } finally {
    smtpTesting.value = false
  }
}

defineExpose({ save, submitting, smtpTesting })
</script>

<template>
  <div class="space-y-6">
    <div class="rounded-xl border border-border bg-card">
      <div class="border-b border-border bg-muted/40 px-6 py-4">
        <h2 class="text-lg font-semibold">{{ t('admin.settings.smtp.title') }}</h2>
        <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.smtp.subtitle') }}</p>
      </div>

      <div class="space-y-6 p-6">
        <div class="grid grid-cols-1 gap-6 md:grid-cols-2">
          <div class="flex items-center gap-3 rounded-lg border border-border bg-muted/20 px-4 py-3">
            <Switch id="smtp-enabled" v-model="form.enabled" />
            <Label for="smtp-enabled" class="text-sm font-medium">{{ t('admin.settings.smtp.enabled') }}</Label>
          </div>
          <div class="flex items-center gap-3 rounded-lg border border-border bg-muted/20 px-4 py-3">
            <Switch id="smtp-tls" v-model="form.use_tls" />
            <Label for="smtp-tls" class="text-sm font-medium">{{ t('admin.settings.smtp.useTLS') }}</Label>
            <Switch id="smtp-ssl" v-model="form.use_ssl" class="ml-4" />
            <Label for="smtp-ssl" class="text-sm font-medium">{{ t('admin.settings.smtp.useSSL') }}</Label>
          </div>
        </div>

        <div class="grid grid-cols-1 gap-6 md:grid-cols-2">
          <div class="flex items-center gap-3 rounded-lg border border-border bg-muted/20 px-4 py-3">
            <Switch id="smtp-order-notification" v-model="form.order_notification_enabled" :disabled="!form.enabled" />
            <div>
              <Label for="smtp-order-notification" class="text-sm font-medium">{{ t('admin.settings.smtp.orderNotificationEnabled') }}</Label>
              <p class="text-xs text-muted-foreground">{{ t('admin.settings.smtp.orderNotificationHint') }}</p>
            </div>
          </div>
        </div>

        <div class="rounded-xl border border-border">
          <div class="border-b border-border bg-muted/30 px-4 py-3">
            <h3 class="text-sm font-semibold">{{ t('admin.settings.smtp.orderStatuses.title') }}</h3>
            <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.smtp.orderStatuses.hint') }}</p>
          </div>
          <div class="grid grid-cols-1 gap-4 p-4 md:grid-cols-2">
            <div v-for="key in orderStatusKeys" :key="key" class="flex items-center gap-3 rounded-lg border border-border bg-muted/20 px-4 py-3">
              <Switch :id="`smtp-order-status-${key}`" v-model="form.order_notification_statuses[key]" :disabled="!form.enabled || !form.order_notification_enabled" />
              <div>
                <Label :for="`smtp-order-status-${key}`" class="text-sm font-medium">{{ t(`admin.settings.smtp.orderStatuses.${key}`) }}</Label>
                <p class="text-xs text-muted-foreground">{{ t(`admin.settings.smtp.orderStatuses.${key}Hint`) }}</p>
              </div>
            </div>
          </div>
        </div>

        <div class="grid grid-cols-1 gap-6 md:grid-cols-2">
          <div class="space-y-2">
            <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.smtp.host') }}</label>
            <Input v-model="form.host" :placeholder="t('admin.settings.smtp.hostPlaceholder')" />
          </div>
          <div class="space-y-2">
            <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.smtp.port') }}</label>
            <Input v-model.number="form.port" type="number" :placeholder="t('admin.settings.smtp.portPlaceholder')" />
          </div>
          <div class="space-y-2">
            <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.smtp.username') }}</label>
            <Input v-model="form.username" :placeholder="t('admin.settings.smtp.usernamePlaceholder')" />
          </div>
          <div class="space-y-2">
            <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.smtp.password') }}</label>
            <Input v-model="form.password" type="password" :placeholder="t('admin.settings.smtp.passwordPlaceholder')" />
            <p class="text-xs text-muted-foreground">
              {{ form.has_password ? t('admin.settings.smtp.passwordHintKeep') : t('admin.settings.smtp.passwordHintEmpty') }}
            </p>
          </div>
          <div class="space-y-2">
            <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.smtp.from') }}</label>
            <Input v-model="form.from" :placeholder="t('admin.settings.smtp.fromPlaceholder')" />
          </div>
          <div class="space-y-2">
            <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.smtp.fromName') }}</label>
            <Input v-model="form.from_name" :placeholder="t('admin.settings.smtp.fromNamePlaceholder')" />
          </div>
        </div>

        <div class="rounded-xl border border-border">
          <div class="border-b border-border bg-muted/30 px-4 py-3">
            <h3 class="text-sm font-semibold">{{ t('admin.settings.smtp.verifyCode.title') }}</h3>
          </div>
          <div class="grid grid-cols-1 gap-4 p-4 md:grid-cols-4">
            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.smtp.verifyCode.expireMinutes') }}</label>
              <Input v-model.number="form.verify_code.expire_minutes" type="number" min="1" />
            </div>
            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.smtp.verifyCode.sendIntervalSeconds') }}</label>
              <Input v-model.number="form.verify_code.send_interval_seconds" type="number" min="1" />
            </div>
            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.smtp.verifyCode.maxAttempts') }}</label>
              <Input v-model.number="form.verify_code.max_attempts" type="number" min="1" />
            </div>
            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.smtp.verifyCode.length') }}</label>
              <Input v-model.number="form.verify_code.length" type="number" min="4" max="10" />
            </div>
          </div>
        </div>

        <div class="rounded-xl border border-border bg-muted/20 p-4">
          <h3 class="text-sm font-semibold">{{ t('admin.settings.smtp.testTitle') }}</h3>
          <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.smtp.testSubtitle') }}</p>
          <div class="mt-3 flex flex-col gap-3 md:flex-row">
            <Input v-model="form.test_email" :placeholder="t('admin.settings.smtp.testEmailPlaceholder')" />
            <Button variant="secondary" :disabled="smtpTesting" @click="testSMTPSettings">
              {{ smtpTesting ? t('admin.settings.smtp.testing') : t('admin.settings.smtp.testButton') }}
            </Button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
