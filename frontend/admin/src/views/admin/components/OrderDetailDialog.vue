<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Package } from 'lucide-vue-next'
import { adminAPI } from '@/api/admin'
import type { AdminOrder, AdminOrderItem, AdminFulfillment, AdminProcurementOrder, AdminPayment } from '@/api/types'
import IdCell from '@/components/IdCell.vue'
import { Button } from '@/components/ui/button'
import { Dialog, DialogScrollContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import { Card, CardContent } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import {
  orderStatusClass,
  orderStatusLabel,
  paymentExceptionLabel as paymentExceptionLabelMap,
  paymentStatusClass as paymentStatusClassMap,
  paymentStatusLabel as paymentStatusLabelMap,
} from '@/utils/status'
import {
  fulfillmentStatusLabel as fulfillmentStatusLabelMap,
  fulfillmentTypeLabel as fulfillmentTypeLabelMap,
} from '@/utils/fulfillment'
import { formatDate, formatMoney, getLocalizedText, hasPositiveAmount } from '@/utils/format'
import { resolveSkuCodeFromSnapshot, resolveSkuSpecFromSnapshot } from '@/utils/sku'
import { adminUrl } from '@/utils/adminBase'
import { confirmAction } from '@/utils/confirm'

const props = defineProps<{
  modelValue: boolean
  order: AdminOrder | null
  siteCurrency: string
  maxRefundDays: number
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'refresh'): void
  (e: 'openFulfillment', order: AdminOrder, parentId?: number): void
}>()

const { t, locale } = useI18n()

const detailLoading = ref(false)
const detailError = ref('')
const selectedOrder = ref<AdminOrder | null>(null)
const procurementOrder = ref<AdminProcurementOrder | null | undefined>(null)

const refundSubmitting = ref(false)
const refundError = ref('')
const refundSuccess = ref('')
const refundTab = ref<'manual' | 'wallet'>('wallet')
const refundForm = reactive({
  amount: '',
  remark: '',
})
const manualRefundSubmitting = ref(false)
const manualRefundError = ref('')
const manualRefundSuccess = ref('')
const manualRefundForm = reactive({
  amount: '',
  reason: '',
  paymentFeeRefunded: true,
})
const manualConfirmSubmitting = ref(false)
const manualConfirmError = ref('')
const manualConfirmSuccess = ref('')
const manualConfirmForm = reactive({
  providerRef: '',
  remark: '',
})
// 管理员必须明确选择要确认的那一条支付记录，不再默认取最新一条。
const manualConfirmPaymentId = ref<number | null>(null)


const userDetailLink = (userId: number) => adminUrl(`/users/${userId}`)
const productLink = (productId: number) => adminUrl(`/products?product_id=${productId}`)
const couponCodeLink = (code: string) => adminUrl(`/coupons?code=${encodeURIComponent(code)}`)
const promotionLink = (promotionId: number) => adminUrl(`/promotions?id=${promotionId}`)
const paymentLink = (paymentId: number) => adminUrl(`/payments?payment_id=${paymentId}`)

const statusLabel = (status: string) => orderStatusLabel(t, status)
const statusClass = (status: string) => orderStatusClass(status)
const fulfillmentTypeLabel = (type: string) => fulfillmentTypeLabelMap(t, type, 'admin.orders')
const fulfillmentStatusLabel = (status: string) => fulfillmentStatusLabelMap(t, status, 'admin.orders')
const paymentStatusLabel = (status: string) => paymentStatusLabelMap(t, status)
const paymentStatusClass = (status: string) => paymentStatusClassMap(status)

const providerTypeLabel = (value?: string) => {
  const map: Record<string, string> = {
    official: t('admin.paymentChannels.providerTypes.official'),
    epay: t('admin.paymentChannels.providerTypes.epay'),
    bepusdt: t('admin.paymentChannels.providerTypes.bepusdt'),
    epusdt: t('admin.paymentChannels.providerTypes.epusdt'),
    tokenpay: t('admin.paymentChannels.providerTypes.tokenpay'),
    wallet: t('admin.paymentChannels.providerTypes.wallet'),
  }
  if (!value) return '-'
  return map[value] || value
}

const channelTypeLabel = (value?: string) => {
  const map: Record<string, string> = {
    wechat: t('admin.paymentChannels.channelTypes.wechat'),
    alipay: t('admin.paymentChannels.channelTypes.alipay'),
    qqpay: t('admin.paymentChannels.channelTypes.qqpay'),
    paypal: t('admin.paymentChannels.channelTypes.paypal'),
    stripe: t('admin.paymentChannels.channelTypes.stripe'),
    usdt: t('admin.paymentChannels.channelTypes.usdt'),
    'usdt-trc20': t('admin.paymentChannels.channelTypes.usdtTrc20'),
    'usdc-trc20': t('admin.paymentChannels.channelTypes.usdcTrc20'),
    trx: t('admin.paymentChannels.channelTypes.trx'),
    bepusdt: t('admin.paymentChannels.channelTypes.bepusdtCashier'),
    epusdt: t('admin.paymentChannels.channelTypes.epusdtCashier'),
    balance: t('admin.paymentChannels.channelTypes.balance'),
  }
  if (!value) return '-'
  return map[value] || value
}

const paymentChannelTypeLabel = (payment: AdminPayment) => {
  const value = String(payment.display_channel_type || payment.channel_type || '').trim()
  return channelTypeLabel(value)
}

const itemRevenueAmount = (item: AdminOrderItem) => {
  return parseMoneyValue(item.total_price) - parseMoneyValue(item.coupon_discount_amount)
}

const itemBaseProfit = (item: AdminOrderItem) => {
  const cost = parseMoneyValue(item.cost_price) * (item.quantity || 1)
  return itemRevenueAmount(item) - cost
}

const refundedAmount = (order: AdminOrder | null) => {
  if (!order) return 0
  return Math.max(parseMoneyValue(order.refunded_amount), 0)
}

const toCents = (amount: number) => Math.round(amount * 100)
const fromCents = (cents: number) => Number((cents / 100).toFixed(2))

const refundAllocationCache = new WeakMap<AdminOrder, Map<AdminOrderItem, number>>()

const buildRefundAllocation = (order: AdminOrder | null) => {
  const allocation = new Map<AdminOrderItem, number>()
  if (!order || !Array.isArray(order.items) || order.items.length === 0) return allocation

  const refundedCents = toCents(refundedAmount(order))
  if (refundedCents <= 0) return allocation

  const items = order.items
  const weights = items.map((item) => Math.max(itemRevenueAmount(item), 0))
  const totalWeight = weights.reduce((sum, value) => sum + value, 0)

  if (totalWeight <= 0) {
    const base = Math.floor(refundedCents / items.length)
    let remainder = refundedCents - base * items.length
    items.forEach((item) => {
      const extra = remainder > 0 ? 1 : 0
      if (remainder > 0) remainder -= 1
      allocation.set(item, fromCents(base + extra))
    })
    return allocation
  }

  let assigned = 0
  items.forEach((item, index) => {
    let shareCents = 0
    if (index === items.length - 1) {
      shareCents = Math.max(refundedCents - assigned, 0)
    } else {
      shareCents = Math.floor((refundedCents * (weights[index] ?? 0)) / totalWeight)
      assigned += shareCents
    }
    allocation.set(item, fromCents(shareCents))
  })

  return allocation
}

const itemRefundAmount = (order: AdminOrder | null, item: AdminOrderItem) => {
  if (!order) return 0
  const cached = refundAllocationCache.get(order)
  if (cached) {
    return cached.get(item) || 0
  }
  const allocation = buildRefundAllocation(order)
  refundAllocationCache.set(order, allocation)
  return allocation.get(item) || 0
}

const itemProfit = (order: AdminOrder | null, item: AdminOrderItem) => {
  const profit = itemBaseProfit(item) - itemRefundAmount(order, item)
  return Number(profit.toFixed(2))
}

const orderPaymentFee = (order: AdminOrder | null) => {
  if (!order?.payments?.length) return 0
  const fee = order.payments.reduce((sum, payment) => {
    if (payment.status !== 'success' || payment.fee_policy !== 'merchant_absorbed') return sum
    return sum + Math.max(parseMoneyValue(payment.fee_amount), 0)
  }, 0)
  return Number(fee.toFixed(2))
}

const orderProfit = (order: AdminOrder | null) => {
  if (!order?.items?.length) return 0
  const itemTotal = order.items.reduce((sum, item) => sum + itemProfit(order, item), 0)
  return Number((itemTotal - orderPaymentFee(order)).toFixed(2))
}

const hasWalletPayment = (order: AdminOrder) => hasPositiveAmount(order?.wallet_paid_amount)
const hasOnlinePayment = (order: AdminOrder) => hasPositiveAmount(order?.online_paid_amount)
const formatDiscountMoney = (amount?: string | number, currency?: string) => {
  return hasPositiveAmount(amount) ? `-${formatMoney(amount, currency)}` : formatMoney(amount, currency)
}

const positiveAmountValue = (amount?: string | number) => {
  const value = Number(amount)
  return Number.isFinite(value) && value > 0 ? value : 0
}

const itemDiscountTotal = (item: AdminOrderItem) => {
  return Number((
    positiveAmountValue(item.promotion_discount_amount)
    + positiveAmountValue(item.member_discount_amount)
    + positiveAmountValue(item.wholesale_discount_amount)
    + positiveAmountValue(item.coupon_discount_amount)
  ).toFixed(2))
}

const hasItemDiscount = (item: AdminOrderItem) => itemDiscountTotal(item) > 0

const formatItemDiscountTotal = (item: AdminOrderItem, currency?: string) => {
  return formatMoney(itemDiscountTotal(item).toFixed(2), currency)
}

const itemPaidAmount = (item: AdminOrderItem) => {
  const paid = positiveAmountValue(item.total_price) - positiveAmountValue(item.coupon_discount_amount)
  return Number(Math.max(0, paid).toFixed(2))
}

const formatItemPaidAmount = (item: AdminOrderItem, currency?: string) => {
  return formatMoney(itemPaidAmount(item).toFixed(2), currency)
}

const orderPaymentMethodLabel = (order: AdminOrder) => {
  if (hasWalletPayment(order) && hasOnlinePayment(order)) {
    return t('admin.orders.paymentMethodMixed')
  }
  if (hasWalletPayment(order)) {
    return t('admin.orders.paymentMethodWalletOnly')
  }
  if (hasOnlinePayment(order)) {
    return t('admin.orders.paymentMethodOnlineOnly')
  }
  return t('admin.orders.paymentMethodUnknown')
}

const canCreateFulfillment = (order: AdminOrder | null) => {
  if (!order) return false
  if (order.fulfillment) return false
  if (order.parent_id == null && Array.isArray(order.children) && order.children.length > 0) return false
  if (Array.isArray(order.items) && order.items.length > 0 && !order.items.every((item: AdminOrderItem) => item.fulfillment_type === 'manual')) {
    return false
  }
  return order.status === 'paid' || order.status === 'fulfilling'
}

const canCreateChildFulfillment = (order: AdminOrder | null) => {
  if (!canCreateFulfillment(order)) return false
  if (!order || !order.items || !order.items.length) return false
  return order.items.every((item: AdminOrderItem) => item.fulfillment_type === 'manual')
}

const formatManualValue = (value: unknown) => {
  if (Array.isArray(value)) {
    return value.map((item) => String(item)).join(', ')
  }
  if (value === null || value === undefined) {
    return '-'
  }
  if (typeof value === 'object') {
    try {
      return JSON.stringify(value)
    } catch {
      return String(value)
    }
  }
  return String(value)
}

type ManualFormSnapshotField = {
  key: string
  label?: Record<string, unknown> | string
}

const normalizeManualSnapshotFields = (schemaSnapshot: Record<string, unknown> | null | undefined): ManualFormSnapshotField[] => {
  if (!schemaSnapshot || typeof schemaSnapshot !== 'object') return []
  const rawFields = Array.isArray(schemaSnapshot.fields) ? schemaSnapshot.fields : []
  return rawFields
    .map((field) => {
      if (!field || typeof field !== 'object') return null
      const key = String((field as Record<string, unknown>).key || '').trim()
      if (!key) return null
      return {
        key,
        label: (field as Record<string, unknown>).label as Record<string, unknown> | string | undefined,
      } satisfies ManualFormSnapshotField
    })
    .filter(Boolean) as ManualFormSnapshotField[]
}

const resolveManualFieldLabel = (field: ManualFormSnapshotField) => {
  const localized = getLocalizedText(field.label)
  if (localized) return String(localized)
  return field.key
}

const manualSubmissionRows = (
  submission: Record<string, unknown> | null | undefined,
  schemaSnapshot?: Record<string, unknown> | null
) => {
  if (!submission || typeof submission !== 'object') return []

  const entries = Object.entries(submission).filter(([key]) => String(key).trim() !== '')
  if (entries.length === 0) return []

  const valueMap = new Map(entries.map(([key, value]) => [String(key), value] as const))
  const rows: Array<{ key: string; label: string; value: string }> = []

  normalizeManualSnapshotFields(schemaSnapshot).forEach((field) => {
    if (!valueMap.has(field.key)) return
    rows.push({
      key: field.key,
      label: resolveManualFieldLabel(field),
      value: formatManualValue(valueMap.get(field.key)),
    })
    valueMap.delete(field.key)
  })

  valueMap.forEach((value, key) => {
    rows.push({
      key,
      label: key,
      value: formatManualValue(value),
    })
  })

  return rows
}

const parseOrderItemSkuId = (item: AdminOrderItem & Record<string, unknown>) => {
  const snapshot = item?.sku_snapshot as Record<string, unknown> | undefined
  const value = Number(item?.sku_id || snapshot?.sku_id || 0)
  if (!Number.isFinite(value)) return 0
  const normalized = Math.trunc(value)
  return normalized > 0 ? normalized : 0
}

const orderItemSkuCodeText = (item: AdminOrderItem & Record<string, unknown>) => {
  const code = resolveSkuCodeFromSnapshot(item?.sku_snapshot, {
    defaultLabel: t('admin.orders.itemSkuDefaultCode'),
  })
  if (code) return code
  const skuId = parseOrderItemSkuId(item)
  if (skuId > 0) return `#${skuId}`
  return t('admin.orders.itemSkuUnknown')
}

const orderItemSkuSpecText = (item: AdminOrderItem & Record<string, unknown>) => {
  const specText = resolveSkuSpecFromSnapshot(item?.sku_snapshot, locale.value)
  if (specText) return specText
  return t('admin.orders.itemSkuSpecEmpty')
}

const fulfillmentDeliveryLines = (fulfillment: AdminFulfillment & Record<string, unknown>) => {
  const lines: string[] = []
  const logistics = (fulfillment?.delivery_data || fulfillment?.logistics) as Record<string, unknown> | undefined
  if (logistics && typeof logistics === 'object') {
    const note = String(logistics.note || '').trim()
    if (note) {
      lines.push(note)
    }
    const entries = Array.isArray(logistics.entries) ? logistics.entries : []
    entries.forEach((item: Record<string, unknown>) => {
      const key = String(item?.key || '').trim()
      const value = String(item?.value || '').trim()
      if (!key && !value) {
        return
      }
      if (!key) {
        lines.push(value)
      } else if (!value) {
        lines.push(key)
      } else {
        lines.push(`${key}: ${value}`)
      }
    })

    Object.entries(logistics).forEach(([key, value]) => {
      if (key === 'note' || key === 'entries') return
      const text = String(value ?? '').trim()
      if (!text) return
      lines.push(`${key}: ${text}`)
    })
  }
  return lines
}

const fulfillmentDownloading = ref(false)

const isFulfillmentTruncated = (fulfillment: any) => {
  return fulfillment?.payload_line_count > 100
}

const handleDownloadFulfillment = async (orderId: number, orderNo: string) => {
  if (fulfillmentDownloading.value) return
  fulfillmentDownloading.value = true
  try {
    const res = await adminAPI.downloadFulfillment(orderId)
    const blob = new Blob([res.data], { type: 'text/plain; charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `fulfillment-${orderNo}.txt`
    a.click()
    URL.revokeObjectURL(url)
  } catch {} finally {
    fulfillmentDownloading.value = false
  }
}

const parseMoneyValue = (value?: string | number) => {
  if (value === null || value === undefined || value === '') return 0
  const parsed = Number(value)
  if (Number.isNaN(parsed)) return 0
  return parsed
}

const isPaidOrder = (order: AdminOrder | null) => {
  if (!order) return false
  return Boolean(order.paid_at)
}

const normalizedMaxRefundDays = () => {
  const parsed = Number(props.maxRefundDays)
  if (!Number.isFinite(parsed)) return 30
  const normalized = Math.trunc(parsed)
  if (normalized < 0) return 30
  if (normalized > 3650) return 3650
  return normalized
}

const isOrderRefundWindowExpired = (order: AdminOrder | null) => {
  if (!order || !isPaidOrder(order)) return false
  if (normalizedMaxRefundDays() === 0) return false
  const baseAtRaw = order.paid_at || order.created_at
  if (!baseAtRaw) return false
  const baseAtMs = Date.parse(baseAtRaw)
  if (Number.isNaN(baseAtMs)) return false
  const nowMs = Date.now()
  if (!Number.isFinite(nowMs) || nowMs <= baseAtMs) return false
  const diffDays = (nowMs - baseAtMs) / (1000 * 60 * 60 * 24)
  return diffDays > normalizedMaxRefundDays()
}

const refundableAmountValue = (order: AdminOrder | null) => {
  if (!order) return 0
  if (!isPaidOrder(order)) return 0
  const total = parseMoneyValue(order.total_amount)
  const refunded = parseMoneyValue(order.refunded_amount)
  const value = total - refunded
  if (!Number.isFinite(value)) return 0
  return Number(Math.max(value, 0).toFixed(2))
}

const refundableAmountDisplay = (order: AdminOrder | null) => formatMoney(refundableAmountValue(order).toFixed(2), order?.currency)

const canRefundToWallet = (order: AdminOrder | null) => {
  if (!order) return false
  if (!order.user_id) return false
  if (!isPaidOrder(order)) return false
  if (isOrderRefundWindowExpired(order)) return false
  return refundableAmountValue(order) > 0
}

const canManualRefund = (order: AdminOrder | null) => {
  if (!order) return false
  if (!isPaidOrder(order)) return false
  if (isOrderRefundWindowExpired(order)) return false
  return refundableAmountValue(order) > 0
}

const shouldShowRefundCard = (order: AdminOrder | null) => {
  if (!order) return false
  if (!isPaidOrder(order)) return false
  if (order.status === 'canceled') return false
  if (isOrderRefundWindowExpired(order)) return false
  return true
}

const canSelectWalletRefundTab = (order: AdminOrder | null) => Boolean(order?.user_id)

const defaultRefundTab = (order: AdminOrder | null): 'manual' | 'wallet' => (canSelectWalletRefundTab(order) ? 'wallet' : 'manual')

const ensureRefundTabAvailable = (order: AdminOrder | null) => {
  refundTab.value = defaultRefundTab(order)
}

const resetRefundForm = () => {
  refundForm.amount = ''
  refundForm.remark = ''
  refundError.value = ''
  refundSuccess.value = ''
}

const resetManualRefundForm = () => {
  manualRefundForm.amount = ''
  manualRefundForm.reason = ''
  manualRefundForm.paymentFeeRefunded = true
  manualRefundError.value = ''
  manualRefundSuccess.value = ''
}

const resetManualConfirmForm = () => {
  manualConfirmForm.providerRef = ''
  manualConfirmForm.remark = ''
  manualConfirmPaymentId.value = null
  manualConfirmError.value = ''
  manualConfirmSuccess.value = ''
}

const paymentExceptionLabel = (code?: string) => paymentExceptionLabelMap(t, code)

// 欠付款项已转入用户余额，不能再拿来确认履约（后端同样会拒绝）。
const isManualConfirmSelectable = (payment: AdminPayment) =>
  payment.exception_code !== 'underpaid_payment_succeeded'

// 仅未支付的父订单（待支付，或已被超时取消）才允许人工确认支付。
const canManualConfirmPayment = (order: AdminOrder | null) => {
  if (!order || order.parent_id) return false
  if (order.paid_at) return false
  return order.status === 'pending_payment' || order.status === 'canceled'
}

const submitManualConfirm = async () => {
  if (!selectedOrder.value || !canManualConfirmPayment(selectedOrder.value)) return
  manualConfirmError.value = ''
  manualConfirmSuccess.value = ''
  if (!manualConfirmPaymentId.value) {
    manualConfirmError.value = t('admin.orders.manualConfirmSelectRequired')
    return
  }
  const confirmed = await confirmAction({
    title: t('admin.orders.manualConfirmTitle'),
    description: t('admin.orders.manualConfirmPrompt'),
    variant: 'destructive',
  })
  if (!confirmed) return

  manualConfirmSubmitting.value = true
  try {
    await adminAPI.manualConfirmOrderPayment(Number(selectedOrder.value.id), {
      payment_id: manualConfirmPaymentId.value,
      provider_ref: manualConfirmForm.providerRef.trim() || undefined,
      remark: manualConfirmForm.remark.trim() || undefined,
    })
    manualConfirmSuccess.value = t('admin.orders.manualConfirmSuccess')
    manualConfirmForm.providerRef = ''
    manualConfirmForm.remark = ''
    manualConfirmPaymentId.value = null
    await fetchOrderDetail(Number(selectedOrder.value.id))
    emit('refresh')
  } catch (err: any) {
    manualConfirmError.value = err?.message || t('admin.orders.manualConfirmFailed')
  } finally {
    manualConfirmSubmitting.value = false
  }
}

const formatFeeRate = (channel: AdminPayment | { fee_rate: number | string; fixed_fee?: number | string }) => {
  const feeRate = channel.fee_rate
  const fixedFee = channel.fixed_fee

  let display = '-'
  if (feeRate !== undefined && feeRate !== null && feeRate !== '') {
    const rateParsed = Number(feeRate)
    if (!Number.isNaN(rateParsed)) {
      display = `${rateParsed.toFixed(2)}%`
    }
  }

  if (fixedFee !== undefined && fixedFee !== null && fixedFee !== '') {
    const fixedParsed = Number(fixedFee)
    if (!Number.isNaN(fixedParsed) && fixedParsed > 0) {
      if (display === '-') {
        display = fixedParsed.toFixed(2)
      } else {
        display += ` + ${fixedParsed.toFixed(2)}`
      }
    }
  }

  return display
}

const fetchOrderDetail = async (orderId: number) => {
  detailLoading.value = true
  detailError.value = ''
  selectedOrder.value = null
  procurementOrder.value = null
  try {
    const response = await adminAPI.getOrder(orderId)
    selectedOrder.value = response.data.data
    ensureRefundTabAvailable(selectedOrder.value)
    try {
      const procRes = await adminAPI.getProcurementOrders({ order_no: selectedOrder.value?.order_no, page_size: 1 })
      const procList = procRes.data.data
      if (Array.isArray(procList) && procList.length > 0) {
        procurementOrder.value = procList[0]
      }
    } catch {
      // silently ignore - procurement order may not exist
    }
  } catch (err: any) {
    detailError.value = err?.message || t('admin.orders.detailFetchFailed')
  } finally {
    detailLoading.value = false
  }
}

const submitRefundToWallet = async () => {
  if (!selectedOrder.value) return
  refundError.value = ''
  refundSuccess.value = ''
  if (!canRefundToWallet(selectedOrder.value)) {
    refundError.value = t('admin.orders.refundNoRemaining')
    return
  }
  const amount = refundForm.amount.trim()
  const value = Number(amount)
  if (!amount || Number.isNaN(value) || value <= 0) {
    refundError.value = t('admin.orders.refundInvalidAmount')
    return
  }
  if (value > refundableAmountValue(selectedOrder.value)) {
    refundError.value = t('admin.orders.refundExceeded')
    return
  }

  refundSubmitting.value = true
  try {
    await adminAPI.refundOrderToWallet(Number(selectedOrder.value.id), {
      amount,
      remark: refundForm.remark.trim() || undefined,
    })
    refundSuccess.value = t('admin.orders.refundSuccess')
    refundForm.amount = ''
    refundForm.remark = ''
    await fetchOrderDetail(Number(selectedOrder.value.id))
    emit('refresh')
  } catch (err: any) {
    refundError.value = err?.message || t('admin.orders.refundFailed')
  } finally {
    refundSubmitting.value = false
  }
}

const submitManualRefund = async () => {
  if (!selectedOrder.value) return
  manualRefundError.value = ''
  manualRefundSuccess.value = ''
  if (!canManualRefund(selectedOrder.value)) {
    manualRefundError.value = t('admin.orders.refundNoRemaining')
    return
  }
  const amount = manualRefundForm.amount.trim()
  const value = Number(amount)
  if (!amount || Number.isNaN(value) || value <= 0) {
    manualRefundError.value = t('admin.orders.refundInvalidAmount')
    return
  }
  if (value > refundableAmountValue(selectedOrder.value)) {
    manualRefundError.value = t('admin.orders.refundExceeded')
    return
  }

  manualRefundSubmitting.value = true
  try {
    await adminAPI.manualRefundOrder(Number(selectedOrder.value.id), {
      amount,
      remark: manualRefundForm.reason.trim() || undefined,
      payment_fee_refunded: manualRefundForm.paymentFeeRefunded,
    })
    manualRefundSuccess.value = t('admin.orders.manualRefundSuccess')
    manualRefundForm.amount = ''
    manualRefundForm.reason = ''
    manualRefundForm.paymentFeeRefunded = true
    await fetchOrderDetail(Number(selectedOrder.value.id))
    emit('refresh')
  } catch (err: any) {
    manualRefundError.value = err?.message || t('admin.orders.refundFailed')
  } finally {
    manualRefundSubmitting.value = false
  }
}

const handleClose = () => {
  emit('update:modelValue', false)
  selectedOrder.value = null
  detailError.value = ''
  refundTab.value = 'wallet'
  resetRefundForm()
  resetManualRefundForm()
  resetManualConfirmForm()
}

const handleOpenFulfillment = (order: AdminOrder, parentId?: number) => {
  handleClose()
  emit('openFulfillment', order, parentId)
}

// Watch for the order prop to trigger detail fetch
watch(
  () => props.order,
  (newOrder) => {
    if (newOrder && props.modelValue) {
      refundTab.value = defaultRefundTab(newOrder)
      resetRefundForm()
      resetManualRefundForm()
      resetManualConfirmForm()
      fetchOrderDetail(newOrder.id)
    }
  }
)

watch(
  () => props.modelValue,
  (open) => {
    if (open && props.order) {
      refundTab.value = defaultRefundTab(props.order)
      resetRefundForm()
      resetManualRefundForm()
      resetManualConfirmForm()
      fetchOrderDetail(props.order.id)
    }
    if (!open) {
      handleClose()
    }
  }
)
</script>

<template>
  <Dialog :open="modelValue" @update:open="(value) => { if (!value) handleClose() }">
    <DialogScrollContent class="w-[calc(100vw-1rem)] max-w-5xl p-4 sm:p-6">
      <DialogHeader>
        <DialogTitle>{{ t('admin.orders.detailTitle') }}</DialogTitle>
      </DialogHeader>
      <div class="space-y-6">
        <div v-if="detailLoading" class="h-32 rounded-lg border border-border bg-muted/40 animate-pulse"></div>
        <div v-else-if="detailError" class="rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">
          {{ detailError }}
        </div>
        <div v-else-if="selectedOrder" class="space-y-6">
          <div class="grid grid-cols-1 gap-4 text-sm md:grid-cols-2 [&>*]:min-w-0">
            <Card class="min-w-0 rounded-lg border-border bg-background shadow-none">
              <CardContent class="p-3">
                <div class="text-xs text-muted-foreground">{{ t('admin.orders.table.id') }}</div>
                <div class="text-foreground font-mono mt-1">
                  <IdCell :value="selectedOrder.id" />
                </div>
              </CardContent>
            </Card>
            <Card class="min-w-0 rounded-lg border-border bg-background shadow-none">
              <CardContent class="p-3">
                <div class="text-xs text-muted-foreground">{{ t('admin.orders.detailOrderNo') }}</div>
                <div class="mt-1 break-all font-mono text-foreground">{{ selectedOrder.order_no }}</div>
              </CardContent>
            </Card>
            <Card class="min-w-0 rounded-lg border-border bg-background shadow-none">
              <CardContent class="p-3">
                <div class="text-xs text-muted-foreground">{{ t('admin.orders.detailUser') }}</div>
                <div class="text-sm text-foreground break-words">
                  <span v-if="selectedOrder.user_id">
                    {{ t('admin.orders.userLabel') }}:
                    <a :href="userDetailLink(selectedOrder.user_id)" target="_blank" rel="noopener" class="text-primary underline-offset-4 hover:underline">
                      #{{ selectedOrder.user_id }}
                    </a>
                  </span>
                  <span v-else class="break-all">{{ t('admin.orders.guestLabel') }}: {{ selectedOrder.guest_email || '-' }}</span>
                </div>
              </CardContent>
            </Card>
            <Card class="min-w-0 rounded-lg border-border bg-background shadow-none">
              <CardContent class="p-3">
                <div class="text-xs text-muted-foreground">{{ t('admin.orders.detailStatus') }}</div>
                <div class="text-sm text-foreground">{{ statusLabel(selectedOrder.status) }}</div>
              </CardContent>
            </Card>
            <Card class="min-w-0 rounded-lg border-border bg-background shadow-none">
              <CardContent class="p-3">
                <div class="text-xs text-muted-foreground">{{ t('admin.orders.detailCreatedAt') }}</div>
                <div class="text-sm text-foreground">{{ formatDate(selectedOrder.created_at) }}</div>
              </CardContent>
            </Card>
            <Card class="min-w-0 rounded-lg border-border bg-background shadow-none">
              <CardContent class="p-3">
                <div class="text-xs text-muted-foreground">{{ t('admin.orders.detailClientIp') }}</div>
                <div class="break-all text-sm text-foreground">{{ selectedOrder.client_ip || '-' }}</div>
              </CardContent>
            </Card>
          </div>

          <div class="rounded-xl border border-border bg-muted/20 p-4">
            <h3 class="text-sm font-semibold text-foreground mb-3">{{ t('orderDetail.amountTitle') }}</h3>
            <div class="grid grid-cols-1 gap-4 text-sm md:grid-cols-3 [&>*]:min-w-0">
              <Card class="min-w-0 rounded-lg border-border bg-background shadow-none">
                <CardContent class="p-3">
                  <div class="text-xs text-muted-foreground">{{ t('orderDetail.amountOriginal') }}</div>
                  <div class="text-foreground font-mono mt-1">{{ formatMoney(selectedOrder.original_amount, selectedOrder.currency) }}</div>
                </CardContent>
              </Card>
              <Card class="min-w-0 rounded-lg border-border bg-background shadow-none">
                <CardContent class="p-3">
                  <div class="text-xs text-muted-foreground">{{ t('orderDetail.amountDiscount') }}</div>
                  <div
                    class="font-mono mt-1"
                    :class="hasPositiveAmount(selectedOrder.discount_amount) ? 'text-red-600' : 'text-foreground'"
                  >
                    {{ formatDiscountMoney(selectedOrder.discount_amount, selectedOrder.currency) }}
                  </div>
                </CardContent>
              </Card>
              <Card class="min-w-0 rounded-lg border-border bg-background shadow-none">
                <CardContent class="p-3">
                  <div class="text-xs text-muted-foreground">{{ t('orderDetail.amountTotal') }}</div>
                  <div class="text-foreground font-mono mt-1">{{ formatMoney(selectedOrder.total_amount, selectedOrder.currency) }}</div>
                </CardContent>
              </Card>
              <Card v-if="hasPositiveAmount(selectedOrder.wallet_paid_amount)" class="min-w-0 rounded-lg border-border bg-background shadow-none">
                <CardContent class="p-3">
                  <div class="text-xs text-muted-foreground">{{ t('admin.orders.detailWalletPaid') }}</div>
                  <div class="text-foreground font-mono mt-1">{{ formatMoney(selectedOrder.wallet_paid_amount, selectedOrder.currency) }}</div>
                </CardContent>
              </Card>
              <Card v-if="hasPositiveAmount(selectedOrder.online_paid_amount)" class="min-w-0 rounded-lg border-border bg-background shadow-none">
                <CardContent class="p-3">
                  <div class="text-xs text-muted-foreground">{{ t('admin.orders.detailOnlinePaid') }}</div>
                  <div class="text-foreground font-mono mt-1">{{ formatMoney(selectedOrder.online_paid_amount, selectedOrder.currency) }}</div>
                </CardContent>
              </Card>
              <Card v-if="hasPositiveAmount(selectedOrder.refunded_amount)" class="min-w-0 rounded-lg border-border bg-background shadow-none">
                <CardContent class="p-3">
                  <div class="text-xs text-muted-foreground">{{ t('admin.orders.detailRefunded') }}</div>
                  <div class="text-foreground font-mono mt-1">{{ formatMoney(selectedOrder.refunded_amount, selectedOrder.currency) }}</div>
                </CardContent>
              </Card>
              <Card class="min-w-0 rounded-lg border-border bg-background shadow-none">
                <CardContent class="p-3">
                  <div class="text-xs text-muted-foreground">{{ t('admin.orders.detailPaymentMethod') }}</div>
                  <div class="text-foreground mt-1">{{ orderPaymentMethodLabel(selectedOrder) }}</div>
                </CardContent>
              </Card>
            </div>
          </div>

          <div class="rounded-xl border border-border bg-muted/20 p-4">
            <h3 class="text-sm font-semibold text-foreground mb-3">{{ t('admin.orders.detailDiscountTitle') }}</h3>
            <div class="grid grid-cols-1 gap-4 text-sm md:grid-cols-2 [&>*]:min-w-0">
              <Card class="min-w-0 rounded-lg border-border bg-background shadow-none">
                <CardContent class="p-3">
                  <div class="text-xs text-muted-foreground">{{ t('admin.orders.detailCouponDiscount') }}</div>
                  <div
                    class="font-mono mt-1"
                    :class="hasPositiveAmount(selectedOrder.discount_amount) ? 'text-red-600' : 'text-foreground'"
                  >
                    {{ formatDiscountMoney(selectedOrder.discount_amount, selectedOrder.currency) }}
                  </div>
                </CardContent>
              </Card>
              <Card class="min-w-0 rounded-lg border-border bg-background shadow-none">
                <CardContent class="p-3">
                  <div class="text-xs text-muted-foreground">{{ t('admin.orders.detailPromotionDiscount') }}</div>
                  <div
                    class="font-mono mt-1"
                    :class="hasPositiveAmount(selectedOrder.promotion_discount_amount) ? 'text-red-600' : 'text-foreground'"
                  >
                    {{ formatDiscountMoney(selectedOrder.promotion_discount_amount, selectedOrder.currency) }}
                  </div>
                </CardContent>
              </Card>
              <Card v-if="hasPositiveAmount(selectedOrder.member_discount_amount)" class="min-w-0 rounded-lg border-amber-200 bg-amber-50/50 shadow-none">
                <CardContent class="p-3">
                  <div class="text-xs text-amber-700">{{ t('admin.orders.detailMemberDiscount') }}</div>
                  <div class="text-amber-700 font-mono mt-1">{{ formatDiscountMoney(selectedOrder.member_discount_amount, selectedOrder.currency) }}</div>
                </CardContent>
              </Card>
              <Card v-if="hasPositiveAmount(selectedOrder.wholesale_discount_amount)" class="min-w-0 rounded-lg border-emerald-200 bg-emerald-50/50 shadow-none">
                <CardContent class="p-3">
                  <div class="text-xs text-emerald-700">{{ t('admin.orders.detailWholesaleDiscount') }}</div>
                  <div class="text-emerald-700 font-mono mt-1">{{ formatDiscountMoney(selectedOrder.wholesale_discount_amount, selectedOrder.currency) }}</div>
                </CardContent>
              </Card>
            </div>
          </div>

          <div class="rounded-xl border border-border bg-muted/20 p-4">
            <h3 class="text-sm font-semibold text-foreground mb-3">{{ t('orderDetail.itemsTitle') }}</h3>
            <div v-if="selectedOrder.items && selectedOrder.items.length" class="space-y-3">
              <div v-for="item in selectedOrder.items" :key="item.id" class="flex flex-col gap-3 border-b border-border/60 pb-3 text-sm text-muted-foreground">
                <div class="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                  <div class="min-w-0">
                    <a :href="productLink(item.product_id)" target="_blank" rel="noopener" class="break-words font-medium text-primary underline-offset-4 hover:underline">
                      {{ getLocalizedText(item.title) }}
                    </a>
                    <div class="text-xs text-muted-foreground mt-1">#{{ item.product_id }}</div>
                    <div class="text-xs text-muted-foreground">{{ t('orderDetail.quantityLabel') }}：{{ item.quantity }}</div>
                    <div class="mt-2 rounded-lg border border-border/70 bg-background/90 px-3 py-2">
                      <div class="flex flex-wrap items-center gap-2">
                        <span class="inline-flex items-center gap-1 rounded-full border border-emerald-200 bg-emerald-50 px-2 py-0.5 text-[11px] font-medium text-emerald-700">
                          <Package class="h-3.5 w-3.5" />
                          {{ t('admin.orders.itemSkuLabel') }}
                        </span>
                        <span class="break-all font-mono text-xs text-foreground">{{ orderItemSkuCodeText(item) }}</span>
                      </div>
                      <div class="mt-1 break-words text-xs text-muted-foreground">{{ t('admin.orders.itemSkuSpec') }}：{{ orderItemSkuSpecText(item) }}</div>
                    </div>
                    <div class="text-xs text-muted-foreground mt-1">
                      {{ t('admin.orders.itemCouponCode') }}：
                      <a v-if="selectedOrder.coupon_code" :href="couponCodeLink(selectedOrder.coupon_code)" target="_blank" rel="noopener" class="text-primary underline-offset-4 hover:underline">
                          <span class="break-all">{{ selectedOrder.coupon_code }}</span>
                        </a>
                      <span v-else>-</span>
                    </div>
                    <div class="text-xs text-muted-foreground mt-1">
                      {{ t('admin.orders.itemPromotionName') }}：
                      <a v-if="item.promotion_id" :href="promotionLink(item.promotion_id)" target="_blank" rel="noopener" class="text-primary underline-offset-4 hover:underline">
                        <span class="break-words">{{ item.promotion_name || `#${item.promotion_id}` }}</span>
                      </a>
                      <span v-else>-</span>
                    </div>
                    <div v-if="item.tags && item.tags.length" class="mt-2 flex flex-wrap gap-2">
                      <span
                        v-for="(tag, index) in item.tags"
                        :key="index"
                        class="rounded-full border border-border/70 bg-background px-2 py-0.5 text-[11px] text-muted-foreground"
                      >
                        {{ tag }}
                      </span>
                    </div>
                    <div v-if="manualSubmissionRows(item.manual_form_submission, item.manual_form_schema_snapshot).length" class="mt-3 rounded-lg border border-border bg-background p-3">
                      <div class="text-xs font-semibold text-muted-foreground mb-2">{{ t('admin.orders.manualSubmissionTitle') }}</div>
                      <div class="space-y-1 text-xs text-muted-foreground">
                        <div v-for="row in manualSubmissionRows(item.manual_form_submission, item.manual_form_schema_snapshot)" :key="`${item.id}-${row.key}`" class="break-words">
                          <span class="text-foreground">{{ row.label }}</span>：<span class="break-all">{{ row.value }}</span>
                        </div>
                      </div>
                    </div>
                  </div>
                  <div class="space-y-1 text-left md:text-right">
                    <div>{{ t('orderDetail.unitPriceLabel') }}：{{ formatMoney(item.original_unit_price, selectedOrder.currency) }}</div>
                    <div>{{ t('admin.orders.costPrice') }}：{{ formatMoney(item.cost_price, selectedOrder.currency) }}</div>
                    <div>{{ t('orderDetail.totalPriceLabel') }}：{{ formatMoney(item.original_total_price, selectedOrder.currency) }}</div>
                    <div v-if="hasPositiveAmount(item.coupon_discount_amount)">
                      {{ t('orderDetail.couponDiscountLabel') }}：{{ formatDiscountMoney(item.coupon_discount_amount, selectedOrder.currency) }}
                    </div>
                    <div v-if="hasPositiveAmount(item.promotion_discount_amount)">
                      {{ t('orderDetail.promotionDiscountLabel') }}：{{ formatDiscountMoney(item.promotion_discount_amount, selectedOrder.currency) }}
                    </div>
                    <div v-if="hasPositiveAmount(item.wholesale_discount_amount)">
                      {{ t('orderDetail.wholesaleDiscountLabel') }}：{{ formatDiscountMoney(item.wholesale_discount_amount, selectedOrder.currency) }}
                    </div>
                    <div v-if="hasPositiveAmount(item.member_discount_amount)">
                      {{ t('orderDetail.memberDiscountLabel') }}：{{ formatDiscountMoney(item.member_discount_amount, selectedOrder.currency) }}
                    </div>
                    <div v-if="hasItemDiscount(item)" class="font-medium text-red-600">
                      {{ t('orderDetail.itemDiscountTotalLabel') }}：{{ formatItemDiscountTotal(item, selectedOrder.currency) }}
                    </div>
                    <div class="font-medium text-foreground">
                      {{ t('orderDetail.itemPaidAmountLabel') }}：{{ formatItemPaidAmount(item, selectedOrder.currency) }}
                    </div>
                    <div v-if="hasPositiveAmount(itemRefundAmount(selectedOrder, item))" class="text-blue-700">
                      {{ t('admin.orders.itemRefund') }}：{{ formatMoney(itemRefundAmount(selectedOrder, item).toFixed(2), selectedOrder.currency) }}
                    </div>
                    <div class="font-medium text-emerald-700">
                      {{ t('admin.orders.itemProfit') }}：{{ formatMoney(itemProfit(selectedOrder, item).toFixed(2), selectedOrder.currency) }}
                    </div>
                  </div>
                </div>
              </div>
            </div>
            <div v-if="selectedOrder.items && selectedOrder.items.length" class="mt-3 flex flex-wrap justify-end gap-2">
              <div v-if="hasPositiveAmount(selectedOrder.refunded_amount)" class="rounded-lg border border-blue-200 bg-blue-50/60 px-4 py-2 text-sm font-semibold text-blue-700">
                {{ t('admin.orders.itemRefund') }}：{{ formatMoney(selectedOrder.refunded_amount, selectedOrder.currency) }}
              </div>
              <div v-if="hasPositiveAmount(orderPaymentFee(selectedOrder))" class="rounded-lg border border-amber-200 bg-amber-50/60 px-4 py-2 text-sm font-semibold text-amber-700">
                {{ t('admin.payments.table.feeAmount') }}：{{ formatMoney(orderPaymentFee(selectedOrder).toFixed(2), selectedOrder.currency) }}
              </div>
              <div class="rounded-lg border border-emerald-200 bg-emerald-50/50 px-4 py-2 text-sm font-semibold text-emerald-700">
                {{ t('admin.orders.orderProfit') }}：{{ formatMoney(orderProfit(selectedOrder).toFixed(2), selectedOrder.currency) }}
              </div>
            </div>
            <div v-else class="text-xs text-muted-foreground">{{ t('orderDetail.noItems') }}</div>
          </div>

          <div v-if="selectedOrder.children && selectedOrder.children.length" class="rounded-xl border border-border bg-muted/20 p-4">
            <h3 class="text-sm font-semibold text-foreground mb-3">{{ t('orderDetail.childOrdersTitle') }}</h3>
            <div class="space-y-4">
              <div v-for="child in selectedOrder.children" :key="child.id" class="rounded-xl border border-border bg-background p-4">
                <div class="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                  <div class="min-w-0">
                    <div class="break-all text-xs text-muted-foreground">{{ t('orderDetail.childOrderNo') }}：{{ child.order_no }}</div>
                    <div class="text-xs text-muted-foreground mt-1">{{ t('orderDetail.childOrderAmount') }}：{{ formatMoney(child.total_amount, child.currency || selectedOrder.currency) }}</div>
                  </div>
                  <div class="flex w-full flex-wrap items-center gap-2 md:w-auto md:justify-end">
                    <span class="inline-flex rounded-full border px-2.5 py-1 text-xs" :class="statusClass(child.status)">
                      {{ statusLabel(child.status) }}
                    </span>
                    <Button v-if="canCreateChildFulfillment(child)" size="sm" variant="secondary" class="w-full sm:w-auto" @click="handleOpenFulfillment(child, selectedOrder.id)">
                      {{ t('admin.orders.fulfillmentCreate') }}
                    </Button>
                  </div>
                </div>
                <div class="mt-4">
                  <h4 class="text-xs font-semibold text-muted-foreground mb-2">{{ t('orderDetail.childItemsTitle') }}</h4>
                  <div v-if="child.items && child.items.length" class="space-y-3">
                    <div v-for="item in child.items" :key="item.id" class="flex flex-col gap-3 border-b border-border/60 pb-3 text-sm text-muted-foreground md:flex-row md:items-center md:justify-between">
                      <div class="min-w-0">
                        <a :href="productLink(item.product_id)" target="_blank" rel="noopener" class="break-words font-medium text-primary underline-offset-4 hover:underline">
                          {{ getLocalizedText(item.title) }}
                        </a>
                        <div class="text-xs text-muted-foreground mt-1">#{{ item.product_id }}</div>
                        <div class="text-xs text-muted-foreground">{{ t('orderDetail.quantityLabel') }}：{{ item.quantity }}</div>
                        <div class="mt-2 rounded-lg border border-border/70 bg-muted/20 px-3 py-2">
                          <div class="flex flex-wrap items-center gap-2">
                            <span class="inline-flex items-center gap-1 rounded-full border border-emerald-200 bg-emerald-50 px-2 py-0.5 text-[11px] font-medium text-emerald-700">
                              <Package class="h-3.5 w-3.5" />
                              {{ t('admin.orders.itemSkuLabel') }}
                            </span>
                            <span class="break-all font-mono text-xs text-foreground">{{ orderItemSkuCodeText(item) }}</span>
                          </div>
                          <div class="mt-1 break-words text-xs text-muted-foreground">{{ t('admin.orders.itemSkuSpec') }}：{{ orderItemSkuSpecText(item) }}</div>
                        </div>
                        <div v-if="manualSubmissionRows(item.manual_form_submission, item.manual_form_schema_snapshot).length" class="mt-3 rounded-lg border border-border bg-muted/20 p-3">
                          <div class="text-xs font-semibold text-muted-foreground mb-2">{{ t('admin.orders.manualSubmissionTitle') }}</div>
                          <div class="space-y-1 text-xs text-muted-foreground">
                            <div v-for="row in manualSubmissionRows(item.manual_form_submission, item.manual_form_schema_snapshot)" :key="`${item.id}-${row.key}`" class="break-words">
                              <span class="text-foreground">{{ row.label }}</span>：<span class="break-all">{{ row.value }}</span>
                            </div>
                          </div>
                        </div>
                      </div>
                      <div class="space-y-1 text-left md:text-right">
                        <div>{{ t('orderDetail.unitPriceLabel') }}：{{ formatMoney(item.original_unit_price, selectedOrder.currency) }}</div>
                        <div>{{ t('admin.orders.costPrice') }}：{{ formatMoney(item.cost_price, selectedOrder.currency) }}</div>
                        <div>{{ t('orderDetail.totalPriceLabel') }}：{{ formatMoney(item.original_total_price, selectedOrder.currency) }}</div>
                        <div v-if="hasPositiveAmount(item.coupon_discount_amount)">
                          {{ t('orderDetail.couponDiscountLabel') }}：{{ formatDiscountMoney(item.coupon_discount_amount, selectedOrder.currency) }}
                        </div>
                        <div v-if="hasPositiveAmount(item.promotion_discount_amount)">
                          {{ t('orderDetail.promotionDiscountLabel') }}：{{ formatDiscountMoney(item.promotion_discount_amount, selectedOrder.currency) }}
                        </div>
                        <div v-if="hasPositiveAmount(item.member_discount_amount)">
                          {{ t('orderDetail.memberDiscountLabel') }}：{{ formatDiscountMoney(item.member_discount_amount, selectedOrder.currency) }}
                        </div>
                        <div v-if="hasItemDiscount(item)" class="font-medium text-red-600">
                          {{ t('orderDetail.itemDiscountTotalLabel') }}：{{ formatItemDiscountTotal(item, selectedOrder.currency) }}
                        </div>
                        <div class="font-medium text-foreground">
                          {{ t('orderDetail.itemPaidAmountLabel') }}：{{ formatItemPaidAmount(item, selectedOrder.currency) }}
                        </div>
                        <div v-if="hasPositiveAmount(itemRefundAmount(child, item))" class="text-blue-700">
                          {{ t('admin.orders.itemRefund') }}：{{ formatMoney(itemRefundAmount(child, item).toFixed(2), selectedOrder.currency) }}
                        </div>
                        <div class="font-medium text-emerald-700">
                          {{ t('admin.orders.itemProfit') }}：{{ formatMoney(itemProfit(child, item).toFixed(2), selectedOrder.currency) }}
                        </div>
                      </div>
                    </div>
                  </div>
                  <div v-else class="text-xs text-muted-foreground">{{ t('orderDetail.noItems') }}</div>
                </div>
                <div class="mt-4">
                  <h4 class="text-xs font-semibold text-muted-foreground mb-2">{{ t('orderDetail.childFulfillmentTitle') }}</h4>
                  <div v-if="child.fulfillment">
                    <div class="text-xs text-muted-foreground">{{ t('admin.orders.detailFulfillmentType') }}：{{ fulfillmentTypeLabel(child.fulfillment.type) }}</div>
                    <div class="text-xs text-muted-foreground">{{ t('admin.orders.detailFulfillmentStatus') }}：{{ fulfillmentStatusLabel(child.fulfillment.status) }}</div>
                    <div v-if="isFulfillmentTruncated(child.fulfillment)" class="mt-3">
                      <div class="flex items-center justify-between mb-2">
                        <span class="text-xs text-muted-foreground">{{ t('admin.orders.fulfillmentTotalLines', { count: child.fulfillment.payload_line_count }) }}</span>
                        <Button size="xs" variant="outline" :disabled="fulfillmentDownloading" @click="handleDownloadFulfillment(child.id, child.order_no || selectedOrder?.order_no || '')">
                          {{ fulfillmentDownloading ? t('admin.orders.fulfillmentDownloading') : t('admin.orders.fulfillmentDownload') }}
                        </Button>
                      </div>
                      <div class="mb-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-400">
                        {{ t('admin.orders.fulfillmentTruncatedHint') }}
                      </div>
                      <div class="rounded-lg border border-border bg-muted/30 p-3 text-xs text-foreground whitespace-pre-wrap break-all max-h-48 overflow-y-auto">{{ child.fulfillment.payload }}</div>
                    </div>
                    <div v-else-if="fulfillmentDeliveryLines(child.fulfillment).length" class="mt-3 rounded-lg border border-border bg-muted/30 p-3 text-xs text-foreground space-y-1">
                      <div v-for="(line, lineIndex) in fulfillmentDeliveryLines(child.fulfillment)" :key="`child-fulfillment-${child.id}-${lineIndex}`" class="break-all">{{ line }}</div>
                    </div>
                    <div v-else-if="child.fulfillment.payload" class="mt-3 rounded-lg border border-border bg-muted/30 p-3 text-xs text-foreground whitespace-pre-wrap break-all">
                      {{ child.fulfillment.payload }}
                    </div>
                  </div>
                  <div v-else class="text-xs text-muted-foreground">{{ t('orderDetail.childFulfillmentEmpty') }}</div>
                </div>
              </div>
            </div>
          </div>

          <div v-if="selectedOrder.fulfillment" class="rounded-xl border border-border bg-muted/20 p-4">
            <div class="flex items-center justify-between mb-3">
              <h3 class="text-sm font-semibold text-foreground">{{ t('admin.orders.detailFulfillmentTitle') }}</h3>
              <Button v-if="isFulfillmentTruncated(selectedOrder.fulfillment)" size="xs" variant="outline" :disabled="fulfillmentDownloading" @click="handleDownloadFulfillment(selectedOrder.id, selectedOrder.order_no)">
                {{ fulfillmentDownloading ? t('admin.orders.fulfillmentDownloading') : t('admin.orders.fulfillmentDownload') }}
              </Button>
            </div>
            <div class="text-sm text-muted-foreground">{{ t('admin.orders.detailFulfillmentType') }}：{{ fulfillmentTypeLabel(selectedOrder.fulfillment.type) }}</div>
            <div class="text-sm text-muted-foreground">{{ t('admin.orders.detailFulfillmentStatus') }}：{{ fulfillmentStatusLabel(selectedOrder.fulfillment.status) }}</div>
            <div v-if="isFulfillmentTruncated(selectedOrder.fulfillment)" class="mt-3">
              <div class="text-xs text-muted-foreground mb-2">{{ t('admin.orders.fulfillmentTotalLines', { count: selectedOrder.fulfillment.payload_line_count }) }}</div>
              <div class="mb-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-400">
                {{ t('admin.orders.fulfillmentTruncatedHint') }}
              </div>
              <div class="rounded-lg border border-border bg-background p-3 text-xs text-muted-foreground whitespace-pre-wrap break-all max-h-64 overflow-y-auto">{{ selectedOrder.fulfillment.payload }}</div>
            </div>
            <div v-else-if="fulfillmentDeliveryLines(selectedOrder.fulfillment).length" class="mt-3 rounded-lg border border-border bg-background p-3 text-xs text-muted-foreground space-y-1">
              <div v-for="(line, lineIndex) in fulfillmentDeliveryLines(selectedOrder.fulfillment)" :key="`fulfillment-${selectedOrder.id}-${lineIndex}`" class="break-all">{{ line }}</div>
            </div>
            <div v-else class="mt-3 rounded-lg border border-border bg-background p-3 text-xs text-muted-foreground whitespace-pre-wrap break-all">
              {{ selectedOrder.fulfillment.payload }}
            </div>
          </div>

          <div v-if="procurementOrder" class="rounded-xl border border-indigo-200 bg-indigo-50/50 p-4">
            <h3 class="text-sm font-semibold text-indigo-800 mb-3">{{ t('orderDetail.procurementTitle') }}</h3>
            <div class="grid grid-cols-1 gap-3 text-sm sm:grid-cols-2">
              <div>
                <span class="text-xs text-muted-foreground">{{ t('procurement.columns.id') }}:</span>
                <span class="ml-1 font-mono">{{ procurementOrder.id }}</span>
              </div>
              <div>
                <span class="text-xs text-muted-foreground">{{ t('procurement.columns.status') }}:</span>
                <span
                  class="ml-1 inline-flex rounded-full border px-2 py-0.5 text-xs"
                  :class="{
                    'text-yellow-700 border-yellow-200 bg-yellow-50': procurementOrder.status === 'pending',
                    'text-blue-700 border-blue-200 bg-blue-50': ['submitted', 'accepted', 'refunded'].includes(procurementOrder.status),
                    'text-emerald-700 border-emerald-200 bg-emerald-50': ['fulfilled', 'completed'].includes(procurementOrder.status),
                    'text-orange-700 border-orange-200 bg-orange-50': procurementOrder.status === 'partially_refunded',
                    'text-red-700 border-red-200 bg-red-50': ['failed', 'rejected'].includes(procurementOrder.status),
                    'text-muted-foreground border-border bg-muted/30': procurementOrder.status === 'canceled',
                  }"
                >
                  {{ t('procurement.status.' + procurementOrder.status) }}
                </span>
              </div>
              <div>
                <span class="text-xs text-muted-foreground">{{ t('procurement.columns.connection') }}:</span>
                <span class="ml-1 break-words">{{ procurementOrder.connection?.name || procurementOrder.connection_id || '-' }}</span>
              </div>
              <div>
                <span class="text-xs text-muted-foreground">{{ t('procurement.columns.upstreamOrderNo') }}:</span>
                <span class="ml-1 break-all font-mono">{{ procurementOrder.upstream_order_no || '-' }}</span>
              </div>
              <div>
                <span class="text-xs text-muted-foreground">{{ t('procurement.columns.upstreamAmount') }}:</span>
                <span class="ml-1">{{ procurementOrder.upstream_amount || '-' }}</span>
              </div>
              <div v-if="procurementOrder.error_message">
                <span class="text-xs text-muted-foreground">{{ t('procurement.columns.errorMessage') }}:</span>
                <span class="ml-1 break-words text-xs text-red-600">{{ procurementOrder.error_message }}</span>
              </div>
            </div>
          </div>

          <div class="rounded-xl border border-border bg-muted/20 p-4">
            <h3 class="text-sm font-semibold text-foreground mb-3">{{ t('admin.orders.detailPayments') }}</h3>
            <div
              v-if="hasWalletPayment(selectedOrder) && (!selectedOrder.payments || selectedOrder.payments.length === 0)"
              class="mb-3 rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-700"
            >
              {{ t('admin.orders.detailWalletOnlyNoOnlinePayments') }}
            </div>
            <div v-if="selectedOrder.payments && selectedOrder.payments.length">
              <div class="overflow-x-auto">
                <Table class="min-w-[760px]">
                <TableHeader class="bg-muted/40 text-xs uppercase text-muted-foreground">
                  <TableRow>
                    <TableHead class="px-4 py-3">{{ t('admin.payments.table.paymentId') }}</TableHead>
                    <TableHead class="px-4 py-3">{{ t('admin.payments.table.channel') }}</TableHead>
                    <TableHead class="px-4 py-3">{{ t('admin.payments.table.status') }}</TableHead>
                    <TableHead class="px-4 py-3">{{ t('admin.payments.table.feeRate') }}</TableHead>
                    <TableHead class="px-4 py-3">{{ t('admin.payments.table.amount') }}</TableHead>
                    <TableHead class="px-4 py-3">{{ t('admin.payments.table.createdAt') }}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody class="divide-y divide-border">
                  <TableRow v-for="payment in selectedOrder.payments" :key="payment.id" class="hover:bg-muted/30">
                    <TableCell class="px-4 py-3">
                      <div class="flex items-center gap-2">
                        <IdCell :value="payment.id" />
                        <a :href="paymentLink(payment.id)" target="_blank" rel="noopener" class="text-primary underline-offset-4 hover:underline text-xs">
                          {{ t('admin.payments.view') }}
                        </a>
                      </div>
                    </TableCell>
                    <TableCell class="px-4 py-3 text-xs">
                      <div class="text-foreground">{{ payment.channel_name || '-' }}</div>
                      <div class="text-muted-foreground">{{ providerTypeLabel(payment.provider_type) }} / {{ paymentChannelTypeLabel(payment) }}</div>
                    </TableCell>
                    <TableCell class="px-4 py-3 text-xs">
                      <span class="inline-flex rounded-full border px-2.5 py-1 text-xs" :class="paymentStatusClass(payment.status)">
                        {{ paymentStatusLabel(payment.status) }}
                      </span>
                    </TableCell>
                    <TableCell class="px-4 py-3 text-xs text-muted-foreground">{{ formatFeeRate(payment) }}</TableCell>
                    <TableCell class="px-4 py-3 font-mono text-foreground">{{ formatMoney(payment.amount, payment.currency) }}</TableCell>
                    <TableCell class="px-4 py-3 text-xs text-muted-foreground">{{ formatDate(payment.created_at) }}</TableCell>
                  </TableRow>
                </TableBody>
                </Table>
              </div>
            </div>
            <div v-else class="text-xs text-muted-foreground">{{ t('admin.payments.empty') }}</div>
          </div>

          <div
            v-if="canManualConfirmPayment(selectedOrder)"
            class="rounded-xl border border-warning/40 bg-warning/5 p-4"
          >
            <h3 class="mb-2 text-sm font-semibold text-foreground">{{ t('admin.orders.manualConfirmTitle') }}</h3>
            <p class="mb-3 text-xs text-muted-foreground">{{ t('admin.orders.manualConfirmHint') }}</p>
            <div class="mb-3">
              <div class="mb-2 text-xs font-medium text-foreground">{{ t('admin.orders.manualConfirmSelectPayment') }}</div>
              <div v-if="selectedOrder.payments && selectedOrder.payments.length" class="space-y-2" role="radiogroup">
                <label
                  v-for="payment in selectedOrder.payments"
                  :key="payment.id"
                  class="flex items-start gap-3 rounded-lg border p-3 text-xs"
                  :class="[
                    manualConfirmPaymentId === payment.id ? 'border-primary bg-primary/5' : 'border-border',
                    isManualConfirmSelectable(payment) ? 'cursor-pointer' : 'cursor-not-allowed opacity-60',
                  ]"
                >
                  <input
                    v-model="manualConfirmPaymentId"
                    type="radio"
                    class="mt-0.5"
                    name="manual-confirm-payment"
                    :value="payment.id"
                    :disabled="!isManualConfirmSelectable(payment) || manualConfirmSubmitting"
                  />
                  <div class="min-w-0 flex-1 space-y-1">
                    <div class="flex flex-wrap items-center gap-2">
                      <span class="font-mono text-foreground">#{{ payment.id }}</span>
                      <span class="text-foreground">{{ payment.channel_name || '-' }}</span>
                      <span class="inline-flex rounded-full border px-2 py-0.5" :class="paymentStatusClass(payment.status)">
                        {{ paymentStatusLabel(payment.status) }}
                      </span>
                      <span class="font-mono text-foreground">{{ formatMoney(payment.amount, payment.currency) }}</span>
                    </div>
                    <div v-if="payment.exception_code" class="text-warning">
                      {{ paymentExceptionLabel(payment.exception_code) }}
                    </div>
                    <div v-if="!isManualConfirmSelectable(payment)" class="text-muted-foreground">
                      {{ t('admin.orders.manualConfirmUnderpaidDisabled') }}
                    </div>
                  </div>
                </label>
              </div>
              <div v-else class="text-xs text-muted-foreground">{{ t('admin.orders.manualConfirmNoPayments') }}</div>
            </div>
            <form
              class="grid grid-cols-1 gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]"
              @submit.prevent="submitManualConfirm"
            >
              <Input
                class="min-w-0"
                v-model="manualConfirmForm.providerRef"
                maxlength="255"
                :placeholder="t('admin.orders.manualConfirmProviderRefPlaceholder')"
                :disabled="manualConfirmSubmitting"
              />
              <Input
                class="min-w-0"
                v-model="manualConfirmForm.remark"
                maxlength="500"
                :placeholder="t('admin.orders.manualConfirmRemarkPlaceholder')"
                :disabled="manualConfirmSubmitting"
              />
              <Button class="w-full md:w-auto" type="submit" :disabled="manualConfirmSubmitting || !manualConfirmPaymentId">
                {{ manualConfirmSubmitting ? t('admin.orders.manualConfirmSubmitting') : t('admin.orders.manualConfirmSubmit') }}
              </Button>
            </form>
            <div
              v-if="manualConfirmError"
              class="mt-3 rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive"
            >
              {{ manualConfirmError }}
            </div>
            <div
              v-if="manualConfirmSuccess"
              class="mt-3 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700"
            >
              {{ manualConfirmSuccess }}
            </div>
          </div>

          <div v-if="shouldShowRefundCard(selectedOrder)" class="rounded-xl border border-border bg-muted/20 p-4">
            <h3 class="text-sm font-semibold text-foreground mb-3">{{ t('admin.orders.refundCardTitle') }}</h3>
            <div class="mb-3 text-xs text-muted-foreground">
              {{ t('admin.orders.refundableAmount') }}：{{ refundableAmountDisplay(selectedOrder) }}
            </div>
            <div class="mb-4 inline-flex rounded-lg border border-border bg-background p-1">
              <button
                v-if="canSelectWalletRefundTab(selectedOrder)"
                type="button"
                class="rounded-md px-3 py-1.5 text-xs transition-colors"
                :class="refundTab === 'wallet' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground'"
                @click="refundTab = 'wallet'"
              >
                {{ t('admin.orders.refundTabWallet') }}
              </button>
              <button
                type="button"
                class="rounded-md px-3 py-1.5 text-xs transition-colors"
                :class="refundTab === 'manual' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground'"
                @click="refundTab = 'manual'"
              >
                {{ t('admin.orders.refundTabManual') }}
              </button>
            </div>

            <form
              v-if="refundTab === 'wallet' && canSelectWalletRefundTab(selectedOrder)"
              class="grid grid-cols-1 gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)_auto]"
              @submit.prevent="submitRefundToWallet"
            >
              <Input
                class="min-w-0"
                v-model="refundForm.amount"
                :placeholder="t('admin.orders.refundAmountPlaceholder')"
                :disabled="refundSubmitting || !canRefundToWallet(selectedOrder)"
              />
              <Input
                class="min-w-0"
                v-model="refundForm.remark"
                :placeholder="t('admin.orders.refundRemarkPlaceholder')"
                :disabled="refundSubmitting || !canRefundToWallet(selectedOrder)"
              />
              <Button
                class="w-full md:w-auto"
                type="submit"
                :disabled="refundSubmitting || !canRefundToWallet(selectedOrder)"
              >
                {{ refundSubmitting ? t('admin.orders.refunding') : t('admin.orders.refundSubmit') }}
              </Button>
            </form>
            <form
              v-else
              class="space-y-3"
              @submit.prevent="submitManualRefund"
            >
              <div class="grid grid-cols-1 gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)_auto]">
                <Input
                  class="min-w-0"
                  v-model="manualRefundForm.amount"
                  :placeholder="t('admin.orders.refundAmountPlaceholder')"
                  :disabled="manualRefundSubmitting || !canManualRefund(selectedOrder)"
                />
                <Input
                  class="min-w-0"
                  v-model="manualRefundForm.reason"
                  :placeholder="t('admin.orders.manualRefundReasonPlaceholder')"
                  :disabled="manualRefundSubmitting || !canManualRefund(selectedOrder)"
                />
                <Button
                  class="w-full md:w-auto"
                  type="submit"
                  :disabled="manualRefundSubmitting || !canManualRefund(selectedOrder)"
                >
                  {{ manualRefundSubmitting ? t('admin.orders.refunding') : t('admin.orders.manualRefundSubmit') }}
                </Button>
              </div>
              <label class="flex cursor-pointer items-start gap-2 rounded-lg border border-border bg-background px-3 py-2">
                <Checkbox
                  v-model="manualRefundForm.paymentFeeRefunded"
                  class="mt-0.5"
                  :disabled="manualRefundSubmitting || !canManualRefund(selectedOrder)"
                />
                <span class="space-y-0.5">
                  <span class="block text-sm font-medium text-foreground">{{ t('admin.orders.paymentFeeRefunded') }}</span>
                  <span class="block text-xs text-muted-foreground">{{ t('admin.orders.paymentFeeRefundedHint') }}</span>
                </span>
              </label>
            </form>
            <div
              v-if="refundTab === 'wallet' && canSelectWalletRefundTab(selectedOrder) && !canRefundToWallet(selectedOrder)"
              class="mt-3 text-xs text-muted-foreground"
            >
              {{ t('admin.orders.refundNoRemaining') }}
            </div>
            <div
              v-if="(refundTab !== 'wallet' || !canSelectWalletRefundTab(selectedOrder)) && !canManualRefund(selectedOrder)"
              class="mt-3 text-xs text-muted-foreground"
            >
              {{ t('admin.orders.refundNoRemaining') }}
            </div>
            <div
              v-if="refundTab === 'wallet' && canSelectWalletRefundTab(selectedOrder) && refundError"
              class="mt-3 rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive"
            >
              {{ refundError }}
            </div>
            <div
              v-if="refundTab === 'wallet' && canSelectWalletRefundTab(selectedOrder) && refundSuccess"
              class="mt-3 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700"
            >
              {{ refundSuccess }}
            </div>
            <div
              v-if="(refundTab !== 'wallet' || !canSelectWalletRefundTab(selectedOrder)) && manualRefundError"
              class="mt-3 rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive"
            >
              {{ manualRefundError }}
            </div>
            <div
              v-if="(refundTab !== 'wallet' || !canSelectWalletRefundTab(selectedOrder)) && manualRefundSuccess"
              class="mt-3 rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-700"
            >
              {{ manualRefundSuccess }}
            </div>
          </div>
        </div>
      </div>
    </DialogScrollContent>
  </Dialog>
</template>
