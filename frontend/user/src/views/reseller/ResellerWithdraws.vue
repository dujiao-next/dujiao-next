<template>
  <div class="space-y-5">
    <ResellerSectionHeader :title="t('resellerConsole.withdraws.title')" :description="t('resellerConsole.withdraws.description')" />

    <Alert
      v-if="alert"
      :variant="alert.level === 'error' ? 'destructive' : 'default'"
      :class="alert.level === 'success' ? 'border-success/40 text-success' : ''"
    >
      <AlertDescription>{{ alert.message }}</AlertDescription>
    </Alert>

    <Card class="p-4 sm:p-5">
      <Alert v-if="!withdrawEnabled" class="mb-4 border-warning/40 text-warning">
        <AlertDescription>{{ withdrawDisabledReasonText }}</AlertDescription>
      </Alert>

      <div v-if="selectedAvailable !== null" class="mb-4 flex items-center justify-between gap-3 rounded-xl bg-muted px-4 py-3">
        <span class="text-xs text-muted-foreground">{{ t('resellerConsole.withdraws.available') }}</span>
        <span class="tabular-nums text-sm font-bold text-foreground">{{ formatResellerConsoleAmount(selectedAvailable, form.currency) }}</span>
      </div>

      <form class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-5" @submit.prevent="onSubmit">
        <div class="flex flex-col gap-1">
          <Input
            v-model.trim="form.amount"
            inputmode="decimal"
            :class="amountError ? 'border-destructive ring-1 ring-destructive' : ''"
            :placeholder="t('personalCenter.reseller.withdrawAmountPlaceholder')"
            :disabled="!withdrawEnabled || submittingWithdraw"
          />
          <span v-if="amountError" class="text-xs text-destructive">{{ t('resellerConsole.withdraws.exceedAvailable') }}</span>
        </div>
        <Select v-if="balanceCurrencies.length > 0" v-model="form.currency" :disabled="!withdrawEnabled || submittingWithdraw">
          <SelectTrigger>
            <SelectValue :placeholder="t('personalCenter.reseller.withdrawCurrencyPlaceholder')" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem v-for="currency in balanceCurrencies" :key="currency" :value="currency">{{ currency }}</SelectItem>
          </SelectContent>
        </Select>
        <Input v-else model-value="" readonly disabled :placeholder="t('resellerConsole.withdraws.noCurrency')" />
        <Input v-model.trim="form.channel" :placeholder="t('personalCenter.reseller.withdrawChannelPlaceholder')" :disabled="!withdrawEnabled || submittingWithdraw" />
        <Input v-model.trim="form.account" :placeholder="t('personalCenter.reseller.withdrawAccountPlaceholder')" :disabled="!withdrawEnabled || submittingWithdraw" />
        <Button type="submit" :disabled="submittingWithdraw || !withdrawEnabled || amountError || balanceCurrencies.length === 0">
          {{ submittingWithdraw ? t('personalCenter.reseller.withdrawing') : t('personalCenter.reseller.withdrawSubmit') }}
        </Button>
      </form>
    </Card>

    <ResellerPageState v-if="withdrawsLoading" loading :title="t('resellerConsole.common.loading')" />
    <ResellerPageState v-else-if="withdraws.length === 0" :title="t('personalCenter.reseller.withdrawEmpty')" :icon="Upload" />

    <template v-else>
      <!-- 桌面表 -->
      <ResellerDataTable class="hidden lg:block">
        <TableHeader>
          <TableRow class="bg-muted/50">
            <TableHead class="px-4 text-right">{{ t('personalCenter.reseller.withdrawTable.amount') }}</TableHead>
            <TableHead class="px-4">{{ t('personalCenter.reseller.withdrawTable.channel') }}</TableHead>
            <TableHead class="px-4">{{ t('personalCenter.reseller.withdrawTable.status') }}</TableHead>
            <TableHead class="px-4">{{ t('personalCenter.reseller.withdrawTable.createdAt') }}</TableHead>
            <TableHead class="px-4">{{ t('personalCenter.reseller.withdrawTable.processedAt') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-for="item in withdraws" :key="item.id">
            <TableCell class="px-4 py-3 text-right tabular-nums text-xs text-foreground">{{ formatResellerConsoleAmount(item.amount, item.currency) }}</TableCell>
            <TableCell class="px-4 py-3 text-xs text-foreground">{{ item.channel }}</TableCell>
            <TableCell class="px-4 py-3">
              <ResellerStatusBadge :label="withdrawStatusLabel(item.status)" :tone="withdrawTone(item.status)" />
              <span
                v-if="item.status === RESELLER_WITHDRAW_STATUS_REJECTED && item.reject_reason"
                class="mt-1 block max-w-[16rem] break-all text-xs text-destructive"
              >
                {{ t('personalCenter.reseller.withdrawTable.rejectReason') }}：{{ item.reject_reason }}
              </span>
            </TableCell>
            <TableCell class="px-4 py-3 text-xs text-muted-foreground">{{ formatResellerConsoleDate(item.created_at) }}</TableCell>
            <TableCell class="px-4 py-3 text-xs text-muted-foreground">{{ formatResellerConsoleDate(item.processed_at) }}</TableCell>
          </TableRow>
        </TableBody>
      </ResellerDataTable>

      <!-- 移动卡片 -->
      <div class="space-y-3 lg:hidden">
        <ResellerRecordCard v-for="item in withdraws" :key="item.id">
          <template #header>
            <span class="tabular-nums text-sm font-bold text-foreground">{{ formatResellerConsoleAmount(item.amount, item.currency) }}</span>
            <ResellerStatusBadge :label="withdrawStatusLabel(item.status)" :tone="withdrawTone(item.status)" />
          </template>
          <div class="col-span-2">
            <dt class="text-muted-foreground">{{ t('personalCenter.reseller.withdrawTable.channel') }}</dt>
            <dd class="mt-0.5 break-all text-foreground">{{ item.channel }}</dd>
          </div>
          <div v-if="item.status === RESELLER_WITHDRAW_STATUS_REJECTED && item.reject_reason" class="col-span-2">
            <dt class="text-muted-foreground">{{ t('personalCenter.reseller.withdrawTable.rejectReason') }}</dt>
            <dd class="mt-0.5 break-all text-destructive">{{ item.reject_reason }}</dd>
          </div>
          <div>
            <dt class="text-muted-foreground">{{ t('personalCenter.reseller.withdrawTable.createdAt') }}</dt>
            <dd class="mt-0.5 text-foreground">{{ formatResellerConsoleDate(item.created_at) }}</dd>
          </div>
          <div>
            <dt class="text-muted-foreground">{{ t('personalCenter.reseller.withdrawTable.processedAt') }}</dt>
            <dd class="mt-0.5 text-foreground">{{ formatResellerConsoleDate(item.processed_at) }}</dd>
          </div>
        </ResellerRecordCard>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Upload } from 'lucide-vue-next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import ResellerDataTable from '../../components/reseller-console/ResellerDataTable.vue'
import ResellerPageState from '../../components/reseller-console/ResellerPageState.vue'
import ResellerRecordCard from '../../components/reseller-console/ResellerRecordCard.vue'
import ResellerSectionHeader from '../../components/reseller-console/ResellerSectionHeader.vue'
import ResellerStatusBadge, { type ResellerBadgeTone } from '../../components/reseller-console/ResellerStatusBadge.vue'
import { useResellerFinance } from '../../composables/reseller/useResellerFinance'
import { useConfirmDialog } from '../../composables/useConfirmDialog'
import { RESELLER_WITHDRAW_STATUS_PAID, RESELLER_WITHDRAW_STATUS_PENDING, RESELLER_WITHDRAW_STATUS_REJECTED } from '../../constants/reseller'
import { type PageAlert } from '../../utils/alerts'
import { formatResellerConsoleAmount, formatResellerConsoleDate } from '../../utils/resellerConsole'
import { getResellerWithdrawDisabledReasonKey, isResellerWithdrawEnabled } from '../../utils/resellerFinance'

const { t } = useI18n()
const { confirm } = useConfirmDialog()
const finance = useResellerFinance()
const { dashboard, balances, withdraws, withdrawsLoading, submittingWithdraw, loadDashboard, loadBalances, loadWithdraws, applyWithdraw } = finance
const alert = ref<PageAlert | null>(null)
const form = reactive({ amount: '', currency: '', channel: '', account: '' })

const balanceCurrencies = computed(() => Array.from(new Set(balances.value.map((item) => item.currency).filter(Boolean))))
const withdrawEnabled = computed(() => isResellerWithdrawEnabled(dashboard.value))
const withdrawDisabledReasonText = computed(() => {
  const key = getResellerWithdrawDisabledReasonKey(dashboard.value?.withdraw_disabled_reason)
  return t(`personalCenter.reseller.withdrawDisabledReason.${key}`)
})

const selectedAvailable = computed<number | null>(() => {
  if (!form.currency) return null
  const match = balances.value.find((b) => b.currency === form.currency)
  return match ? Number(match.available_amount) || 0 : null
})

const amountError = computed(() => {
  const amount = Number(form.amount)
  if (!form.amount || Number.isNaN(amount)) return false
  if (selectedAvailable.value === null) return false
  return amount > selectedAvailable.value
})

watch(balanceCurrencies, (values) => {
  if (!form.currency && values[0]) form.currency = values[0]
})

const onSubmit = async () => {
  if (amountError.value) return
  const ok = await confirm({
    title: t('resellerConsole.withdraws.confirmTitle'),
    message: t('resellerConsole.withdraws.confirmMessage', { amount: form.amount, currency: form.currency }),
    variant: 'default',
  })
  if (!ok) return
  alert.value = null
  try {
    await applyWithdraw({ ...form })
    form.amount = ''
    form.channel = ''
    form.account = ''
    alert.value = { level: 'success', message: t('personalCenter.common.saveSuccess') }
  } catch (err) {
    // 透传后端细分错误（余额不足/币种不可用/账户冻结等），失败兜底通用文案。
    const message = err instanceof Error && err.message ? err.message : t('personalCenter.common.saveFailed')
    alert.value = { level: 'error', message }
  }
}

const withdrawStatusLabel = (status?: string) => {
  if (status === RESELLER_WITHDRAW_STATUS_PENDING) return t('personalCenter.reseller.withdrawStatus.pending')
  if (status === RESELLER_WITHDRAW_STATUS_REJECTED) return t('personalCenter.reseller.withdrawStatus.rejected')
  if (status === RESELLER_WITHDRAW_STATUS_PAID) return t('personalCenter.reseller.withdrawStatus.paid')
  return status || '-'
}

const withdrawTone = (status?: string): ResellerBadgeTone => {
  if (status === RESELLER_WITHDRAW_STATUS_PAID) return 'success'
  if (status === RESELLER_WITHDRAW_STATUS_PENDING) return 'warning'
  return 'neutral'
}

onMounted(() => {
  void Promise.all([loadDashboard(), loadBalances(), loadWithdraws({ page: 1 })])
})
</script>
