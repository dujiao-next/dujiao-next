<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI, type AdminWalletRecharge } from '@/api/admin'
import type { AdminPayment } from '@/api/types'
import IdCell from '@/components/IdCell.vue'
import { Dialog, DialogHeader, DialogTitle, DialogScrollContent } from '@/components/ui/dialog'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { Table, TableHeader, TableHead, TableRow, TableBody, TableCell } from '@/components/ui/table'
import { confirmAction } from '@/utils/confirm'
import { adminUrl } from '@/utils/adminBase'
import { formatDate, formatMoney, hasPositiveAmount } from '@/utils/format'
import { providerTypeLabel as providerTypeLabelMap, paymentChannelTypeLabel as paymentChannelTypeLabelMap, formatFeeRate } from '@/utils/paymentChannelDisplay'
import { paymentStatusClass, paymentStatusLabel } from '@/utils/status'

const props = defineProps<{ modelValue: boolean; recharge: AdminWalletRecharge | null }>()
const emit = defineEmits<{ 'update:modelValue': [value: boolean]; updated: [] }>()
const { t } = useI18n()
const item = ref<AdminWalletRecharge | null>(null)
const payment = ref<AdminPayment | null>(null)
const refundableAmount = ref('0.00')
const refundWindowExpired = ref(false)
const loading = ref(false)
const busy = ref(false)
const error = ref('')
const refundError = ref('')
const refundSuccess = ref('')
const confirmError = ref('')
const confirming = ref(false)
const amount = ref<string | number>('')
const remark = ref('')
const paymentFeeRefunded = ref(true)
const confirmRemark = ref('')
const confirmProviderRef = ref('')
const confirmPaymentID = ref<number | null>(null)
const requestID = ref('')
let loadVersion = 0
const canConfirm = computed(() => ['pending', 'expired', 'failed', 'canceled'].includes(item.value?.status || ''))
const showRefund = computed(() => !refundWindowExpired.value && item.value?.status === 'success')
const canRefund = computed(() => showRefund.value && Number(refundableAmount.value) > 0)
const money = (value?: string | number) => formatMoney(value, item.value?.currency)
const providerTypeLabel = (value?: string) => providerTypeLabelMap(t, value)
const paymentChannelTypeLabel = (payment: AdminPayment) => paymentChannelTypeLabelMap(t, payment)
const fetchDetail = async () => {
  const id = props.recharge?.id
  if (!id) return
  const version = ++loadVersion
  loading.value = true
  error.value = ''
  try {
    const response = await adminAPI.getWalletRecharge(id)
    if (version !== loadVersion) return
    item.value = { ...props.recharge, ...response.data.data.recharge }
    payment.value = response.data.data.payment
    refundableAmount.value = response.data.data.refundable_amount ?? '0.00'
    refundWindowExpired.value = Boolean(response.data.data.refund_window_expired)
  } catch (err: any) {
    if (version === loadVersion) error.value = err?.message || t('admin.walletRecharges.detail.loadFailed')
  } finally {
    if (version === loadVersion) loading.value = false
  }
}
watch(() => [props.modelValue, props.recharge?.id], () => {
  ++loadVersion
  if (!props.modelValue) return
  item.value = null
  payment.value = null
  refundableAmount.value = '0.00'
  refundWindowExpired.value = false
  confirmError.value = ''
  refundError.value = ''
  refundSuccess.value = ''
  paymentFeeRefunded.value = true; amount.value = ''; remark.value = ''; confirmRemark.value = ''; confirmProviderRef.value = ''; confirmPaymentID.value = null; requestID.value = ''
  fetchDetail()
}, { immediate: true })
const validateRefund = () => {
  const value = Number(amount.value)
  if (!Number.isFinite(value) || value <= 0 || value !== Number(value.toFixed(2))) {
    refundError.value = t('admin.walletRecharges.detail.refundInvalidAmount')
    return false
  }
  if (!canRefund.value || value > Number(refundableAmount.value)) {
    refundError.value = t('admin.walletRecharges.detail.refundExceeded')
    return false
  }
  return true
}
const reviewConfirm = async () => {
  if (!item.value || busy.value || confirming.value) return
  confirmError.value = ''
  if (!payment.value || confirmPaymentID.value !== payment.value.id) {
    confirmError.value = t('admin.orders.manualConfirmSelectRequired')
    return
  }
  const id = item.value.id
  const version = loadVersion
  const payload = {
    payment_id: confirmPaymentID.value,
    provider_ref: confirmProviderRef.value.trim() || undefined,
    remark: confirmRemark.value.trim() || undefined,
  }
  confirming.value = true
  try {
    const confirmed = await confirmAction({
      title: t('admin.orders.manualConfirmTitle'),
      description: t('admin.walletRecharges.detail.confirmPrompt'),
      variant: 'destructive',
    })
    if (!confirmed || !props.modelValue || version !== loadVersion) return
    busy.value = true
    await adminAPI.manualConfirmRecharge(id, payload)
    confirmRemark.value = ''; confirmProviderRef.value = ''; confirmPaymentID.value = null
    emit('updated')
    await fetchDetail()
  } catch (err: any) {
    if (version === loadVersion) confirmError.value = err?.message || t('admin.orders.manualConfirmFailed')
  } finally {
    busy.value = false
    confirming.value = false
  }
}
const submitManualRefund = async () => {
  if (!item.value || busy.value || confirming.value) return
  refundError.value = ''
  refundSuccess.value = ''
  if (!validateRefund()) return
  const id = item.value.id
  busy.value = true
  try {
    requestID.value ||= Array.from(crypto.getRandomValues(new Uint8Array(16)), value => value.toString(16).padStart(2, '0')).join('')
    await adminAPI.manualRefundRecharge(id, { amount: String(amount.value), payment_fee_refunded: paymentFeeRefunded.value, remark: remark.value.trim() || undefined, request_id: requestID.value })
    refundSuccess.value = t('admin.walletRecharges.detail.manualRefundSuccess')
    paymentFeeRefunded.value = true; amount.value = ''; remark.value = ''; confirmRemark.value = ''; confirmProviderRef.value = ''; confirmPaymentID.value = null; requestID.value = ''
    emit('updated')
    await fetchDetail()
  } catch (err: any) {
    refundError.value = err?.message || t('admin.walletRecharges.detail.actionFailed')
  } finally { busy.value = false }
}
</script>

<template>
  <Dialog :open="modelValue" @update:open="value => { if (!busy && !confirming) emit('update:modelValue', value) }">
    <DialogScrollContent class="w-[calc(100vw-1rem)] max-w-5xl p-4 sm:p-6">
      <DialogHeader><DialogTitle>{{ t('admin.walletRecharges.detail.title') }}</DialogTitle></DialogHeader>
      <div v-if="loading" class="h-32 animate-pulse rounded-lg bg-muted/40" />
      <div v-if="error" role="alert" class="rounded-lg border border-destructive/30 p-3 text-sm text-destructive">{{ error }}</div>
      <div v-if="item && !loading" class="space-y-6">
        <div class="grid grid-cols-1 gap-4 text-sm md:grid-cols-2">
          <Card v-for="field in ['id', 'rechargeNo', 'user', 'channel', 'status', 'createdAt', 'paidAt', 'clientIP']" :key="field" class="min-w-0 rounded-lg border-border bg-background shadow-none">
            <CardContent class="p-3">
              <div class="text-xs text-muted-foreground" :class="{ 'mb-1': field !== 'status' }">{{ t(`admin.walletRecharges.detail.${field}`) }}</div>
              <IdCell v-if="field === 'id'" :value="item.id" />
              <span v-else-if="field === 'rechargeNo'" class="break-all font-mono">{{ item.recharge_no }}</span>
              <div v-else-if="field === 'user'">
                <a :href="adminUrl(`/users/${item.user_id}`)" target="_blank" rel="noopener" class="text-primary hover:underline">#{{ item.user_id }}</a>
                <div class="break-all">{{ item.user?.display_name }} {{ item.user?.email }}</div>
              </div>
              <div v-else-if="field === 'status'" class="text-sm text-foreground">{{ paymentStatusLabel(t, item.status) }}</div>
              <a v-else-if="field === 'channel'" :href="adminUrl(`/payment-channels?channel_id=${item.channel_id}`)" target="_blank" rel="noopener" class="text-primary hover:underline">{{ item.channel_name || `#${item.channel_id}` }}</a>
              <span v-else-if="field === 'createdAt'">{{ formatDate(item.created_at) }}</span>
              <span v-else-if="field === 'paidAt'">{{ formatDate(item.paid_at) || '-' }}</span>
              <span v-else class="break-all font-mono">{{ item.client_ip || '-' }}</span>
            </CardContent>
          </Card>
        </div>
        <div class="rounded-xl border border-border bg-muted/20 p-4">
          <h3 class="mb-3 text-sm font-semibold text-foreground">{{ t('admin.walletRecharges.detail.amounts') }}</h3>
          <div class="grid grid-cols-1 gap-4 text-sm md:grid-cols-3 [&>*]:min-w-0">
            <Card v-for="field in ['amount', 'payable_amount', 'fee_amount'] as const" :key="field" class="min-w-0 rounded-lg border-border bg-background shadow-none">
              <CardContent class="p-3">
                <div class="text-xs text-muted-foreground">{{ t(`admin.walletRecharges.detail.${field}`) }}</div>
                <div class="mt-1 font-mono text-foreground">{{ money(item[field]) }}</div>
              </CardContent>
            </Card>
            <Card v-if="hasPositiveAmount(item.refunded_amount)" class="min-w-0 rounded-lg border-border bg-background shadow-none">
              <CardContent class="p-3">
                <div class="text-xs text-muted-foreground">{{ t('admin.walletRecharges.detail.refunded_amount') }}</div>
                <div class="mt-1 font-mono text-foreground">{{ money(item.refunded_amount) }}</div>
              </CardContent>
            </Card>
          </div>
        </div>
        <div class="rounded-xl border border-border bg-muted/20 p-4">
          <h3 class="mb-3 text-sm font-semibold text-foreground">{{ t('admin.walletRecharges.detail.payments') }}</h3>
          <div v-if="payment" class="overflow-x-auto">
            <Table class="min-w-[760px]">
              <TableHeader class="bg-muted/40 text-xs uppercase text-muted-foreground">
                <TableRow>
                  <TableHead v-for="field in ['paymentId', 'channel', 'status', 'feeRate', 'amount', 'createdAt']" :key="field" class="px-4 py-3">{{ t(`admin.payments.table.${field}`) }}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody class="divide-y divide-border">
                <TableRow class="hover:bg-muted/30">
                  <TableCell class="px-4 py-3">
                    <div class="flex items-center gap-2">
                      <IdCell :value="payment.id" />
                      <a :href="adminUrl(`/payments?payment_id=${payment.id}`)" target="_blank" rel="noopener" class="text-xs text-primary underline-offset-4 hover:underline">{{ t('admin.payments.view') }}</a>
                    </div>
                  </TableCell>
                  <TableCell class="px-4 py-3 text-xs">
                    <div class="text-foreground">{{ item.channel_name || `#${payment.channel_id}` }}</div>
                    <div class="text-muted-foreground">{{ providerTypeLabel(payment.provider_type) }} / {{ paymentChannelTypeLabel(payment) }}</div>
                  </TableCell>
                  <TableCell class="px-4 py-3 text-xs">
                    <span class="inline-flex rounded-full border px-2.5 py-1 text-xs" :class="paymentStatusClass(payment.status)">{{ paymentStatusLabel(t, payment.status) }}</span>
                  </TableCell>
                  <TableCell class="px-4 py-3 text-xs text-muted-foreground">{{ formatFeeRate(payment) }}</TableCell>
                  <TableCell class="px-4 py-3 font-mono text-foreground">{{ formatMoney(payment.amount, payment.currency || item.currency) }}</TableCell>
                  <TableCell class="px-4 py-3 text-xs text-muted-foreground">{{ formatDate(payment.created_at) }}</TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </div>
          <div v-else class="text-xs text-muted-foreground">{{ t('admin.payments.empty') }}</div>
        </div>
        <div v-if="showRefund" class="rounded-xl border border-border bg-muted/20 p-4">
          <h3 class="text-sm font-semibold text-foreground mb-3">{{ t('admin.walletRecharges.detail.refund') }}</h3>
          <div class="mb-3 text-xs text-muted-foreground">
            {{ t('admin.walletRecharges.detail.refundableAmount') }}：{{ money(refundableAmount) }}
          </div>
          <div class="mb-4 inline-flex rounded-lg border border-border bg-background p-1">
            <button type="button" class="rounded-md px-3 py-1.5 text-xs transition-colors bg-primary text-primary-foreground">
              {{ t('admin.walletRecharges.detail.refundTabManual') }}
            </button>
          </div>
          <form v-if="showRefund" class="space-y-3" @submit.prevent="submitManualRefund">
            <div class="grid grid-cols-1 gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)_auto]">
              <Label for="recharge-refund-amount" class="sr-only">{{ t('admin.walletRecharges.detail.refundAmountPlaceholder') }}</Label>
              <Input id="recharge-refund-amount" v-model="amount" class="min-w-0" type="number" min="0.01" step="0.01" :max="refundableAmount" :placeholder="t('admin.walletRecharges.detail.refundAmountPlaceholder')" required :disabled="busy || !canRefund" @update:model-value="requestID = ''" />
              <Label for="recharge-refund-remark" class="sr-only">{{ t('admin.walletRecharges.detail.manualRefundReasonPlaceholder') }}</Label>
              <Input id="recharge-refund-remark" v-model="remark" class="min-w-0" :placeholder="t('admin.walletRecharges.detail.manualRefundReasonPlaceholder')" maxlength="255" :disabled="busy || !canRefund" @update:model-value="requestID = ''" />
              <Button type="submit" class="w-full md:w-auto" :disabled="busy || !canRefund">{{ t(busy ? 'admin.walletRecharges.detail.refunding' : 'admin.walletRecharges.detail.manualRefundSubmit') }}</Button>
            </div>
            <label class="flex cursor-pointer items-start gap-2 rounded-lg border border-border bg-background px-3 py-2">
              <Checkbox v-model="paymentFeeRefunded" class="mt-0.5" :disabled="busy || !canRefund" @update:model-value="requestID = ''" />
              <span class="space-y-0.5">
                <span class="block text-sm font-medium text-foreground">{{ t('admin.walletRecharges.detail.paymentFeeRefunded') }}</span>
                <span class="block text-xs text-muted-foreground">{{ t('admin.walletRecharges.detail.paymentFeeRefundedHint') }}</span>
              </span>
            </label>
          </form>
          <div v-if="refundError" role="alert" class="mt-3 rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">{{ refundError }}</div>
          <div v-if="refundSuccess" role="status" class="mt-3 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700">{{ refundSuccess }}</div>
        </div>
        <div v-if="canConfirm" class="rounded-xl border border-warning/40 bg-warning/5 p-4">
          <h3 class="mb-2 text-sm font-semibold text-foreground">{{ t('admin.orders.manualConfirmTitle') }}</h3>
          <p class="mb-3 text-xs text-muted-foreground">{{ t('admin.walletRecharges.detail.confirmHint') }}</p>
          <div class="mb-3">
            <div class="mb-2 text-xs font-medium text-foreground">{{ t('admin.orders.manualConfirmSelectPayment') }}</div>
            <div v-if="payment" class="space-y-2" role="radiogroup">
              <label class="flex cursor-pointer items-start gap-3 rounded-lg border p-3 text-xs" :class="confirmPaymentID === payment.id ? 'border-primary bg-primary/5' : 'border-border'">
                <input v-model="confirmPaymentID" type="radio" name="recharge-confirm-payment" class="mt-0.5" :value="payment.id" :disabled="busy || confirming" />
                <div class="min-w-0 flex-1 space-y-1">
                  <div class="flex flex-wrap items-center gap-2">
                    <span class="font-mono text-foreground">#{{ payment.id }}</span>
                    <span class="text-foreground">{{ item.channel_name || '-' }}</span>
                    <span class="inline-flex rounded-full border px-2 py-0.5" :class="paymentStatusClass(payment.status)">{{ paymentStatusLabel(t, payment.status) }}</span>
                    <span class="font-mono text-foreground">{{ formatMoney(payment.amount, payment.currency || item.currency) }}</span>
                  </div>
                </div>
              </label>
            </div>
            <div v-else class="text-xs text-muted-foreground">{{ t('admin.orders.manualConfirmNoPayments') }}</div>
          </div>
          <form class="grid grid-cols-1 gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]" @submit.prevent="reviewConfirm">
            <Label for="recharge-confirm-provider-ref" class="sr-only">{{ t('admin.orders.manualConfirmProviderRefPlaceholder') }}</Label>
            <Input id="recharge-confirm-provider-ref" v-model="confirmProviderRef" class="min-w-0" maxlength="255" :placeholder="t('admin.orders.manualConfirmProviderRefPlaceholder')" :disabled="busy || confirming" />
            <Label for="recharge-confirm-remark" class="sr-only">{{ t('admin.orders.manualConfirmRemarkPlaceholder') }}</Label>
            <Input id="recharge-confirm-remark" v-model="confirmRemark" class="min-w-0" maxlength="500" :placeholder="t('admin.orders.manualConfirmRemarkPlaceholder')" :disabled="busy || confirming" />
            <Button type="submit" class="w-full md:w-auto" :disabled="busy || confirming || !confirmPaymentID">{{ t(busy ? 'admin.orders.manualConfirmSubmitting' : 'admin.orders.manualConfirmSubmit') }}</Button>
          </form>
          <div v-if="confirmError" role="alert" class="mt-3 rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">{{ confirmError }}</div>
        </div>
      </div>
    </DialogScrollContent>
  </Dialog>
</template>
