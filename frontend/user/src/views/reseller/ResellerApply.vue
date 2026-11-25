<template>
  <div class="space-y-5">
    <ResellerSectionHeader
      :title="t('resellerConsole.apply.title')"
      :description="t('resellerConsole.apply.description')"
    />

    <Alert
      v-if="alert"
      :variant="alert.level === 'error' ? 'destructive' : 'default'"
      :class="alert.level === 'success' ? 'border-success/40 text-success' : ''"
    >
      <AlertDescription>{{ alert.message }}</AlertDescription>
    </Alert>

    <ResellerPageState
      v-if="loading"
      loading
      :title="t('resellerConsole.common.loading')"
      :description="t('resellerConsole.common.loadingDescription')"
    />

    <template v-else>
      <!-- 状态 stepper -->
      <Card class="p-5">
        <p class="mb-4 text-xs font-semibold text-muted-foreground">{{ t('resellerConsole.apply.currentStatus') }}: {{ statusText }}</p>
        <div class="flex items-start">
          <template v-for="(step, i) in steps" :key="i">
            <div class="flex w-20 shrink-0 flex-col items-center gap-1.5 text-center">
              <span class="flex h-8 w-8 items-center justify-center rounded-full text-sm font-bold" :class="nodeClass(step.state)">
                <Check v-if="step.state === 'done'" class="h-4 w-4" />
                <X v-else-if="step.state === 'failed'" class="h-4 w-4" />
                <span v-else>{{ i + 1 }}</span>
              </span>
              <span class="text-xs" :class="step.state === 'pending' ? 'text-muted-foreground' : 'text-foreground'">{{ step.label }}</span>
            </div>
            <span v-if="i < steps.length - 1" class="mt-4 h-0.5 flex-1 rounded-full" :class="step.state === 'done' ? 'bg-success/50' : 'bg-muted'"></span>
          </template>
        </div>
      </Card>

      <!-- 拒绝原因 -->
      <Alert v-if="snapshot?.profile?.reject_reason" class="border-warning/40 text-warning">
        <AlertDescription>{{ t('personalCenter.reseller.rejectReason', { reason: snapshot.profile.reject_reason }) }}</AlertDescription>
      </Alert>

      <div class="grid grid-cols-1 gap-5 lg:grid-cols-3">
        <!-- 权益 -->
        <Card class="p-5">
          <h2 class="text-sm font-bold text-foreground">{{ t('resellerConsole.apply.benefitsTitle') }}</h2>
          <ul class="mt-3 space-y-2.5">
            <li v-for="(benefit, i) in benefits" :key="i" class="flex items-start gap-2 text-sm text-foreground">
              <CheckCircle2 class="mt-0.5 h-5 w-5 shrink-0 text-success" />
              <span>{{ benefit }}</span>
            </li>
          </ul>
        </Card>

        <!-- 申请表单 / 不可申请 -->
        <Card class="p-5 lg:col-span-2">
          <form v-if="state.modules.apply.enabled" class="space-y-4" @submit.prevent="submit">
            <Label class="block text-sm font-semibold text-foreground">
              {{ t('personalCenter.reseller.applyReasonPlaceholder') }}
            </Label>
            <Textarea v-model.trim="reason" rows="6" maxlength="500" :disabled="submitting" />
            <div class="flex items-center justify-between gap-3">
              <span class="text-xs text-muted-foreground">{{ reason.length }}/500</span>
              <Button type="submit" :disabled="submitting">
                {{ submitting ? t('personalCenter.reseller.applying') : t('personalCenter.reseller.applySubmit') }}
              </Button>
            </div>
          </form>

          <div v-else class="flex h-full flex-col items-center justify-center py-6 text-center">
            <span class="flex h-12 w-12 items-center justify-center rounded-2xl bg-muted text-muted-foreground">
              <BadgeCheck class="h-6 w-6" />
            </span>
            <h3 class="mt-3 text-base font-bold text-foreground">{{ t('resellerConsole.apply.unavailableTitle') }}</h3>
            <p class="mx-auto mt-2 max-w-md text-sm text-muted-foreground">{{ t('resellerConsole.apply.unavailableDescription') }}</p>
            <Button as-child class="mt-5">
              <RouterLink to="/reseller">{{ t('resellerConsole.nav.dashboard') }}</RouterLink>
            </Button>
          </div>
        </Card>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { BadgeCheck, Check, CheckCircle2, X } from 'lucide-vue-next'
import { resellerAPI } from '../../api'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import ResellerPageState from '../../components/reseller-console/ResellerPageState.vue'
import ResellerSectionHeader from '../../components/reseller-console/ResellerSectionHeader.vue'
import { useResellerProfile } from '../../composables/reseller/useResellerProfile'
import { type PageAlert } from '../../utils/alerts'
import { getResellerProfileStatusKey } from '../../utils/resellerManagement'

const { t } = useI18n()
const { loading, snapshot, state, load } = useResellerProfile()
const submitting = ref(false)
const reason = ref('')
const alert = ref<PageAlert | null>(null)

const statusText = computed(() => {
  const status = snapshot.value?.profile?.status
  if (!snapshot.value?.opened) return t('personalCenter.reseller.managementStatus.notOpened')
  return t(`personalCenter.reseller.profileStatusMap.${getResellerProfileStatusKey(status)}`)
})

const benefits = computed(() => [
  t('resellerConsole.dashboard.benefit1'),
  t('resellerConsole.dashboard.benefit2'),
  t('resellerConsole.dashboard.benefit3'),
])

type StepState = 'done' | 'current' | 'pending' | 'failed'
const steps = computed<{ label: string; state: StepState }[]>(() => {
  const status = state.value.profileStatus
  const submitted = status !== 'not_opened'
  const reviewing = status === 'pending_review'
  const active = status === 'active'
  const rejected = status === 'rejected'
  return [
    { label: t('resellerConsole.apply.stepSubmit'), state: submitted ? 'done' : 'current' },
    {
      label: rejected ? t('resellerConsole.apply.stepRejected') : t('resellerConsole.apply.stepReview'),
      state: active ? 'done' : reviewing ? 'current' : rejected ? 'failed' : 'pending',
    },
    { label: t('resellerConsole.apply.stepActive'), state: active ? 'done' : 'pending' },
  ]
})

const nodeClass = (s: StepState) => {
  if (s === 'done') return 'bg-success text-white'
  if (s === 'failed') return 'bg-destructive text-white'
  if (s === 'current') return 'bg-primary/10 text-primary'
  return 'bg-muted text-muted-foreground'
}

const submit = async () => {
  alert.value = null
  submitting.value = true
  try {
    await resellerAPI.apply({ reason: reason.value })
    reason.value = ''
    await load()
    alert.value = { level: 'success', message: t('personalCenter.common.saveSuccess') }
  } catch (err) {
    // 透传后端细分错误（申请已关闭 / 账号被禁用等），失败兜底通用文案。
    const message = err instanceof Error && err.message ? err.message : t('personalCenter.common.saveFailed')
    alert.value = { level: 'error', message }
  } finally {
    submitting.value = false
  }
}

onMounted(load)
</script>
