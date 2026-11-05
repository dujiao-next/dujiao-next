<script setup lang="ts">
import { onMounted, reactive, ref, watch, computed } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Copy } from 'lucide-vue-next'
import { adminAPI } from '@/api/admin'
import type { AdminPayment } from '@/api/types'
import IdCell from '@/components/IdCell.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Dialog, DialogScrollContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Card, CardContent } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import TableSkeleton from '@/components/TableSkeleton.vue'
import ListPagination from '@/components/ListPagination.vue'
import { useListRefresh, type ListFetchOptions } from '@/composables/useListRefresh'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { paymentExceptionLabel, paymentStatusClass, paymentStatusLabel } from '@/utils/status'
import ComplianceGuardWrapper from '@/components/ComplianceGuardWrapper.vue'
import { copyText } from '@/utils/clipboard'
import { formatDate, toRFC3339 } from '@/utils/format'
import { adminUrl } from '@/utils/adminBase'

const loading = ref(true)
const { refreshing, refreshList } = useListRefresh()
const payments = ref<AdminPayment[]>([])
const pagination = ref({
  page: 1,
  page_size: 20,
  total: 0,
  total_page: 1,
})
const filters = reactive({
  userId: '',
  orderId: '',
  channelId: '',
  channelType: '__all__',
  providerType: '__all__',
  createdFrom: '',
  createdTo: '',
  status: '__all__',
})
const normalizeFilterValue = (value: string) => (value === '__all__' ? '' : value)
const { t } = useI18n()
const route = useRoute()
const showDetail = ref(false)
const detailLoading = ref(false)
const detailError = ref('')
const detailPayment = ref<AdminPayment | null>(null)
const exporting = ref(false)
const exportError = ref('')

// 动态获取的筛选选项：channelType 按 providerType 联动
const rawChannels = ref<Array<{ provider_type: string; channel_type: string }>>([])
const availableProviderTypes = ref<Array<{ value: string; label: string }>>([])

const fetchFilterOptions = async () => {
  try {
    const response = await adminAPI.getPaymentChannels({ page: 1, page_size: 200 })
    const channels = response.data.data || []
    rawChannels.value = channels.map((channel: any) => ({
      provider_type: channel.provider_type,
      channel_type: channel.channel_type,
    }))

    const providerTypeSet = new Set<string>()
    channels.forEach((channel: any) => {
      if (channel.provider_type) providerTypeSet.add(channel.provider_type)
    })
    // 钱包余额不在 channel 配置里，单独补充
    providerTypeSet.add('wallet')

    availableProviderTypes.value = Array.from(providerTypeSet)
      .sort()
      .map(type => ({
        value: type,
        label: providerTypeLabel(type)
      }))
  } catch (error) {
    console.error('Failed to fetch filter options:', error)
  }
}

const availableChannelTypes = computed(() => {
  const selectedProvider = normalizeFilterValue(filters.providerType)
  const channelTypeSet = new Set<string>()

  rawChannels.value.forEach((channel) => {
    if (!channel.channel_type) return
    if (selectedProvider && channel.provider_type !== selectedProvider) return
    channelTypeSet.add(channel.channel_type)
  })

  // wallet 特殊处理：钱包余额不在 channel 配置里
  if (!selectedProvider || selectedProvider === 'wallet') {
    channelTypeSet.add('balance')
  }

  return Array.from(channelTypeSet)
    .sort()
    .map(type => ({ value: type, label: channelTypeLabel(type) }))
})

const handleProviderTypeChange = () => {
  filters.channelType = '__all__'
  handleSearch()
}

const fetchPayments = async (page = 1, options: ListFetchOptions = {}) => {
  if (!options.preserveRows) loading.value = true
  try {
    const response = await adminAPI.getPayments({
      page,
      page_size: pagination.value.page_size,
      status: normalizeFilterValue(filters.status) || undefined,
      user_id: filters.userId || undefined,
      order_id: filters.orderId || undefined,
      channel_id: filters.channelId || undefined,
      provider_type: normalizeFilterValue(filters.providerType) || undefined,
      channel_type: normalizeFilterValue(filters.channelType) || undefined,
      created_from: toRFC3339(filters.createdFrom),
      created_to: toRFC3339(filters.createdTo),
    })
    payments.value = response.data.data || []
    pagination.value = response.data.pagination || pagination.value
  } catch (error) {
    if (!options.preserveRows) payments.value = []
  } finally {
    if (!options.preserveRows) loading.value = false
  }
}

const handleSearch = () => {
  fetchPayments(1)
}
const debouncedSearch = useDebounceFn(handleSearch, 300)

const refresh = () => {
  refreshList(() => fetchPayments(pagination.value.page, { preserveRows: true }))
}

const changePage = (page: number) => {
  if (page < 1 || page > pagination.value.total_page) return
  fetchPayments(page)
}

const pageSizeOptions = [10, 20, 50, 100]

const changePageSize = (size: number) => {
  if (size === pagination.value.page_size) return
  pagination.value.page_size = size
  fetchPayments(1)
}

const orderLink = (orderId: number) => adminUrl(`/orders?order_id=${orderId}`)
const userDetailLink = (userId: number) => adminUrl(`/users/${userId}`)
const channelLink = (channelId: number) => adminUrl(`/payment-channels?channel_id=${channelId}`)

const handleExport = async () => {
  exportError.value = ''
  exporting.value = true
  try {
    const response = await adminAPI.exportPayments({
      user_id: filters.userId || undefined,
      order_id: filters.orderId || undefined,
      channel_id: filters.channelId || undefined,
      provider_type: normalizeFilterValue(filters.providerType) || undefined,
      channel_type: normalizeFilterValue(filters.channelType) || undefined,
      created_from: toRFC3339(filters.createdFrom),
      created_to: toRFC3339(filters.createdTo),
      status: normalizeFilterValue(filters.status) || undefined,
    })
    const blob = new Blob([response.data], { type: 'text/csv;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    const timestamp = new Date().toISOString().replace(/[:.]/g, '-')
    link.href = url
    link.download = `payments_${timestamp}.csv`
    document.body.appendChild(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(url)
  } catch (error) {
    exportError.value = t('admin.payments.exportFailed')
  } finally {
    exporting.value = false
  }
}

const openDetail = async (payment: { id: number }) => {
  showDetail.value = true
  detailLoading.value = true
  detailError.value = ''
  detailPayment.value = null
  try {
    const response = await adminAPI.getPayment(payment.id)
    detailPayment.value = response.data.data
  } catch (err: any) {
    detailError.value = err?.message || t('admin.payments.detailFetchFailed')
  } finally {
    detailLoading.value = false
  }
}

const openDetailById = async (paymentId: number) => {
  if (!paymentId || paymentId <= 0) return
  await openDetail({ id: paymentId })
}

const closeDetail = () => {
  showDetail.value = false
  detailPayment.value = null
  detailError.value = ''
}

const statusClass = (status: string) => paymentStatusClass(status)

const statusLabel = (status: string) => paymentStatusLabel(t, status)

const feePolicyLabel = (policy?: string) => {
  if (!policy) return '-'
  const key = `admin.payments.feePolicies.${policy}`
  const translated = t(key)
  return translated === key ? policy : translated
}

const exceptionLabel = (code?: string) => paymentExceptionLabel(t, code)

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

const handleCopyOrderNo = async (orderNo?: string) => {
  if (!orderNo) return
  try { await copyText(orderNo) } catch {}
}

const interactionModeLabel = (value?: string) => {
  const map: Record<string, string> = {
    qr: t('admin.paymentChannels.interactionModes.qr'),
    redirect: t('admin.paymentChannels.interactionModes.redirect'),
    wap: t('admin.paymentChannels.interactionModes.wap'),
    page: t('admin.paymentChannels.interactionModes.page'),
    balance: t('admin.paymentChannels.interactionModes.balance'),
  }
  if (!value) return '-'
  return map[value] || value
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

const formatMoney = (value: string | number | null | undefined, currency?: string) => {
  if (value === undefined || value === null || value === '') return '-'
  const displayCurrency = currency ? ` ${currency}` : ''
  return `${value}${displayCurrency}`
}

const formatPayload = (payload: unknown) => {
  if (!payload) return '-'
  try {
    return JSON.stringify(payload, null, 2)
  } catch (error) {
    return String(payload)
  }
}

onMounted(() => {
  fetchFilterOptions()
  if (route.query.user_id) {
    filters.userId = String(route.query.user_id || '')
  }
  fetchPayments()
  const paymentId = Number(route.query.payment_id)
  if (Number.isFinite(paymentId) && paymentId > 0) {
    openDetailById(paymentId)
  }
})

watch(
  () => route.query.user_id,
  (value) => {
    if (value) {
      filters.userId = String(value)
      fetchPayments(1)
    }
  }
)

watch(
  () => route.query.payment_id,
  (value) => {
    const paymentId = Number(value)
    if (Number.isFinite(paymentId) && paymentId > 0) {
      openDetailById(paymentId)
    }
  }
)
</script>

<template>
  <ComplianceGuardWrapper>
  <div class="space-y-6">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <h1 class="text-2xl font-semibold">{{ t('admin.payments.title') }}</h1>
    </div>

    <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div class="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center">
        <div class="w-full md:w-32">
          <Input v-model="filters.userId" :placeholder="t('admin.payments.filterUserId')" @update:modelValue="debouncedSearch" />
        </div>
        <div class="w-full md:w-48">
          <Input v-model="filters.orderId" :placeholder="t('admin.payments.filterOrderId')" @update:modelValue="debouncedSearch" />
        </div>
        <div class="w-full md:w-48">
          <Input v-model="filters.channelId" :placeholder="t('admin.payments.filterChannelId')" @update:modelValue="debouncedSearch" />
        </div>
        <div class="w-full md:w-40">
          <Select v-model="filters.providerType" @update:modelValue="handleProviderTypeChange">
            <SelectTrigger class="h-9 w-full">
              <SelectValue :placeholder="t('admin.payments.filterProviderAll')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="__all__">{{ t('admin.payments.filterProviderAll') }}</SelectItem>
              <SelectItem v-for="opt in availableProviderTypes" :key="opt.value" :value="opt.value">
                {{ opt.label }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="w-full md:w-40">
          <Select v-model="filters.channelType" @update:modelValue="handleSearch">
            <SelectTrigger class="h-9 w-full">
              <SelectValue placeholder="全部支付方式" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="__all__">全部支付方式</SelectItem>
              <SelectItem v-for="opt in availableChannelTypes" :key="opt.value" :value="opt.value">
                {{ opt.label }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="flex w-full flex-col gap-2 md:w-auto md:flex-row md:flex-wrap md:items-center">
          <span class="text-xs text-muted-foreground whitespace-nowrap">{{ t('admin.payments.filterCreatedRange') }}</span>
          <Input
            v-model="filters.createdFrom"
            type="datetime-local"
            class="h-9 w-full md:w-auto"
            :placeholder="t('admin.payments.filterCreatedFrom')"
            @update:modelValue="handleSearch"
          />
          <span class="hidden text-muted-foreground md:inline">-</span>
          <Input
            v-model="filters.createdTo"
            type="datetime-local"
            class="h-9 w-full md:w-auto"
            :placeholder="t('admin.payments.filterCreatedTo')"
            @update:modelValue="handleSearch"
          />
        </div>
        <div class="w-full md:w-40">
          <Select v-model="filters.status" @update:modelValue="handleSearch">
            <SelectTrigger class="h-9 w-full">
            <SelectValue :placeholder="t('admin.payments.filterStatusAll')" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="__all__">{{ t('admin.payments.filterStatusAll') }}</SelectItem>
            <SelectItem value="initiated">{{ t('payment.status.initiated') }}</SelectItem>
            <SelectItem value="pending">{{ t('payment.status.pending') }}</SelectItem>
            <SelectItem value="success">{{ t('payment.status.success') }}</SelectItem>
            <SelectItem value="failed">{{ t('payment.status.failed') }}</SelectItem>
            <SelectItem value="expired">{{ t('payment.status.expired') }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="hidden flex-1 sm:block"></div>
        <Button size="sm" variant="outline" class="w-full sm:w-auto" :disabled="refreshing" @click="refresh">{{ t('admin.common.refresh') }}</Button>
        <Button size="sm" :disabled="exporting" @click="handleExport">
          {{ exporting ? t('admin.payments.exporting') : t('admin.payments.export') }}
        </Button>
      </div>
    </div>

    <div v-if="exportError" class="rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">
      {{ exportError }}
    </div>

    <div class="rounded-xl border border-border bg-card overflow-x-auto">
      <Table class="min-w-[900px]">
        <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
          <TableRow>
            <TableHead class="px-6 py-3">{{ t('admin.payments.table.paymentId') }}</TableHead>
            <TableHead class="px-6 py-3 min-w-[160px]">{{ t('admin.payments.table.orderId') }}</TableHead>
            <TableHead class="px-6 py-3 min-w-[180px]">{{ t('admin.payments.table.channel') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.payments.table.status') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.payments.table.amount') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.payments.table.feeRate') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.payments.table.feeAmount') }}</TableHead>
            <TableHead class="px-6 py-3">{{ t('admin.payments.table.createdAt') }}</TableHead>
            <TableHead class="px-6 py-3 min-w-[90px] text-right">{{ t('admin.payments.table.action') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody class="divide-y divide-border">
          <TableRow v-if="loading">
            <TableCell :colspan="9" class="p-0">
              <TableSkeleton :columns="9" :rows="5" />
            </TableCell>
          </TableRow>
          <TableRow v-else-if="payments.length === 0">
            <TableCell colspan="9" class="px-6 py-8 text-center text-muted-foreground">{{ t('admin.payments.empty') }}</TableCell>
          </TableRow>
          <TableRow v-for="payment in payments" :key="payment.id" class="hover:bg-muted/30">
            <TableCell class="px-6 py-4">
              <IdCell :value="payment.id" />
            </TableCell>
            <TableCell class="min-w-[160px] px-6 py-4 text-foreground">
              <a v-if="payment.order_id" :href="orderLink(payment.order_id)" target="_blank" rel="noopener" class="text-primary underline-offset-4 hover:underline">
                #{{ payment.order_id }}
              </a>
              <div v-else-if="payment.recharge_no" class="break-all text-foreground font-mono">
                {{ payment.recharge_no }}
              </div>
              <span v-else>-</span>
              <div v-if="payment.recharge_no" class="text-xs text-muted-foreground mt-1">
                {{ t('admin.payments.rechargeUser') }}:
                <a
                  v-if="payment.recharge_user_id"
                  :href="userDetailLink(payment.recharge_user_id)"
                  target="_blank"
                  rel="noopener"
                  class="font-mono text-primary underline-offset-4 hover:underline"
                >
                  #{{ payment.recharge_user_id }}
                </a>
                <span v-else>-</span>
              </div>
              <div v-if="payment.recharge_no && payment.recharge_status" class="text-xs text-muted-foreground mt-0.5">
                {{ t('admin.payments.rechargeStatus') }}: {{ statusLabel(payment.recharge_status) }}
              </div>
            </TableCell>
            <TableCell class="min-w-[180px] px-6 py-4 text-xs text-muted-foreground">
              <div class="break-words text-foreground">{{ payment.channel_name || '-' }}</div>
              <div class="break-words text-muted-foreground">{{ providerTypeLabel(payment.provider_type) }} / {{ paymentChannelTypeLabel(payment) }}</div>
              <div class="text-muted-foreground mt-0.5">
                {{ t('admin.payments.channelId') }}:
                <a v-if="payment.channel_id" :href="channelLink(payment.channel_id)" target="_blank" rel="noopener" class="text-primary underline-offset-4 hover:underline">
                  #{{ payment.channel_id }}
                </a>
                <span v-else>-</span>
              </div>
            </TableCell>
            <TableCell class="px-6 py-4 text-xs">
              <span class="inline-flex rounded-full border px-2.5 py-1 text-xs" :class="statusClass(payment.status)">
                {{ statusLabel(payment.status) }}
              </span>
            </TableCell>
            <TableCell class="px-6 py-4 font-mono text-foreground">{{ payment.amount }} {{ payment.currency }}</TableCell>
            <TableCell class="px-6 py-4 text-xs text-muted-foreground">{{ formatFeeRate(payment) }}</TableCell>
            <TableCell class="px-6 py-4 font-mono text-foreground">{{ formatMoney(payment.fee_amount, payment.currency) }}</TableCell>
            <TableCell class="px-6 py-4 text-xs text-muted-foreground">{{ formatDate(payment.created_at) }}</TableCell>
            <TableCell class="min-w-[90px] px-6 py-4 text-right">
              <Button size="sm" variant="outline" @click="openDetail(payment)">
                {{ t('admin.payments.view') }}
              </Button>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>

      <ListPagination
        :page="pagination.page"
        :total-page="pagination.total_page"
        :total="pagination.total"
        :page-size="pagination.page_size"
        :page-size-options="pageSizeOptions"
        @change-page="changePage"
        @change-page-size="changePageSize"
      />
    </div>

    <Dialog v-model:open="showDetail" @update:open="(value) => { if (!value) closeDetail() }">
      <DialogScrollContent class="w-[calc(100vw-1rem)] max-w-5xl p-4 sm:p-6">
        <DialogHeader>
          <DialogTitle>{{ t('admin.payments.detailTitle') }}</DialogTitle>
        </DialogHeader>
        <div class="space-y-6">
          <div v-if="detailLoading" class="h-32 rounded-lg border border-border bg-muted/40 animate-pulse"></div>
          <div v-else-if="detailError" class="rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">
            {{ detailError }}
          </div>
          <div v-else-if="detailPayment" class="space-y-6 text-sm text-muted-foreground">
            <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
              <Card class="rounded-lg border-border bg-background shadow-none">
                <CardContent class="p-4">
                  <div class="text-xs text-muted-foreground mb-2">{{ t('admin.payments.detailPaymentId') }}</div>
                  <div class="text-foreground font-mono text-sm break-all">
                    <IdCell :value="detailPayment.id" />
                  </div>
                </CardContent>
              </Card>
              <Card class="rounded-lg border-border bg-background shadow-none">
                <CardContent class="p-4">
                  <div class="text-xs text-muted-foreground mb-2">{{ t('admin.payments.detailOrderId') }}</div>
                  <div class="text-foreground font-mono text-sm break-all">
                    <div v-if="detailPayment.order_id" class="flex items-center gap-1.5">
                      <a :href="orderLink(detailPayment.order_id)" target="_blank" rel="noopener" class="break-all text-primary underline-offset-4 hover:underline">
                        {{ detailPayment.order_no }}
                      </a>
                      <button
                        type="button"
                        class="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-md border border-border/60 text-muted-foreground hover:text-foreground hover:border-border"
                        :title="t('admin.common.copy')"
                        @click="handleCopyOrderNo(detailPayment.order_no)"
                      >
                        <Copy class="h-3 w-3" />
                      </button>
                    </div>
                    <span v-else-if="detailPayment.recharge_no">{{ detailPayment.recharge_no }}</span>
                    <span v-else>-</span>
                    <div v-if="detailPayment.recharge_no" class="mt-2 space-y-1 text-xs text-muted-foreground">
                      <div>
                        {{ t('admin.payments.rechargeUser') }}:
                        <a
                          v-if="detailPayment.recharge_user_id"
                          :href="userDetailLink(detailPayment.recharge_user_id)"
                          target="_blank"
                          rel="noopener"
                          class="font-mono text-primary underline-offset-4 hover:underline"
                        >
                          #{{ detailPayment.recharge_user_id }}
                        </a>
                        <span v-else>-</span>
                      </div>
                      <div v-if="detailPayment.recharge_status">
                        {{ t('admin.payments.rechargeStatus') }}: {{ statusLabel(detailPayment.recharge_status) }}
                      </div>
                    </div>
                  </div>
                </CardContent>
              </Card>
              <Card class="rounded-lg border-border bg-background shadow-none">
                <CardContent class="p-4">
                  <div class="text-xs text-muted-foreground mb-2">{{ t('admin.payments.detailChannel') }}</div>
                  <div class="text-foreground text-sm break-words">{{ detailPayment.channel_name || '-' }}</div>
                  <div class="text-xs text-muted-foreground mt-1 break-words">{{ providerTypeLabel(detailPayment.provider_type) }} / {{ paymentChannelTypeLabel(detailPayment) }}</div>
                </CardContent>
              </Card>
              <Card class="rounded-lg border-border bg-background shadow-none">
                <CardContent class="p-4">
                  <div class="text-xs text-muted-foreground mb-2">{{ t('admin.payments.channelId') }}</div>
                  <div class="text-foreground font-mono text-sm break-all">
                    <a v-if="detailPayment.channel_id" :href="channelLink(detailPayment.channel_id)" target="_blank" rel="noopener" class="text-primary underline-offset-4 hover:underline">
                      #{{ detailPayment.channel_id }}
                    </a>
                    <span v-else>-</span>
                  </div>
                </CardContent>
              </Card>
              <Card class="rounded-lg border-border bg-background shadow-none">
                <CardContent class="p-4">
                  <div class="text-xs text-muted-foreground mb-2">{{ t('admin.payments.detailStatus') }}</div>
                  <div class="text-foreground text-sm">{{ statusLabel(detailPayment.status) }}</div>
                </CardContent>
              </Card>
              <Card class="rounded-lg border-border bg-background shadow-none">
                <CardContent class="p-4">
                  <div class="text-xs text-muted-foreground mb-2">{{ t('admin.payments.detailAmount') }}</div>
                  <div class="text-foreground font-mono text-sm">{{ detailPayment.amount }} {{ detailPayment.currency }}</div>
                </CardContent>
              </Card>
              <Card class="rounded-lg border-border bg-background shadow-none">
                <CardContent class="p-4">
                  <div class="text-xs text-muted-foreground mb-2">{{ t('admin.payments.detailFeeRate') }}</div>
                  <div class="text-foreground font-mono text-sm">{{ formatFeeRate(detailPayment) }}</div>
                </CardContent>
              </Card>
              <Card class="rounded-lg border-border bg-background shadow-none">
                <CardContent class="p-4">
                  <div class="text-xs text-muted-foreground mb-2">{{ t('admin.payments.detailFeeAmount') }}</div>
                  <div class="text-foreground font-mono text-sm">{{ formatMoney(detailPayment.fee_amount, detailPayment.currency) }}</div>
                </CardContent>
              </Card>
              <Card class="rounded-lg border-border bg-background shadow-none">
                <CardContent class="p-4">
                  <div class="text-xs text-muted-foreground mb-2">{{ t('admin.payments.detailFeePolicy') }}</div>
                  <div class="text-foreground text-sm">{{ feePolicyLabel(detailPayment.fee_policy) }}</div>
                </CardContent>
              </Card>
              <Card v-if="detailPayment.exception_code" class="rounded-lg border-warning/40 bg-warning/5 shadow-none">
                <CardContent class="p-4">
                  <div class="text-xs text-warning mb-2">{{ t('admin.payments.detailException') }}</div>
                  <div class="text-warning text-sm">{{ exceptionLabel(detailPayment.exception_code) }}</div>
                </CardContent>
              </Card>
              <Card class="rounded-lg border-border bg-background shadow-none">
                <CardContent class="p-4">
                  <div class="text-xs text-muted-foreground mb-2">{{ t('admin.payments.detailInteraction') }}</div>
                  <div class="text-foreground text-sm">{{ interactionModeLabel(detailPayment.interaction_mode) }}</div>
                </CardContent>
              </Card>
            </div>

            <div class="grid grid-cols-1 lg:grid-cols-3 gap-4">
              <Card class="rounded-lg border-border bg-background shadow-none">
                <CardContent class="p-4">
                  <div class="text-xs text-muted-foreground mb-2">{{ t('admin.payments.detailCreatedAt') }}</div>
                  <div class="text-foreground text-sm">{{ formatDate(detailPayment.created_at) }}</div>
                </CardContent>
              </Card>
              <Card class="rounded-lg border-border bg-background shadow-none">
                <CardContent class="p-4">
                  <div class="text-xs text-muted-foreground mb-2">{{ t('admin.payments.detailPaidAt') }}</div>
                  <div class="text-foreground text-sm">{{ formatDate(detailPayment.paid_at) }}</div>
                </CardContent>
              </Card>
              <Card class="rounded-lg border-border bg-background shadow-none">
                <CardContent class="p-4">
                  <div class="text-xs text-muted-foreground mb-2">{{ t('admin.payments.detailExpiredAt') }}</div>
                  <div class="text-foreground text-sm">{{ formatDate(detailPayment.expired_at) }}</div>
                </CardContent>
              </Card>
            </div>

            <Card class="rounded-lg border-border bg-background shadow-none">
              <CardContent class="p-4">
                <div class="text-xs text-muted-foreground mb-3">{{ t('admin.payments.detailProviderRef') }}</div>
                <div class="text-foreground break-all text-sm">{{ detailPayment.provider_ref || '-' }}</div>
              </CardContent>
            </Card>

            <Card v-if="detailPayment.pay_url" class="rounded-lg border-border bg-background shadow-none">
              <CardContent class="p-4">
                <div class="text-xs text-muted-foreground mb-3">{{ t('admin.payments.detailPayUrl') }}</div>
                <div class="text-foreground break-all text-sm">{{ detailPayment.pay_url }}</div>
              </CardContent>
            </Card>

            <Card v-if="detailPayment.qr_code" class="rounded-lg border-border bg-background shadow-none">
              <CardContent class="p-4">
                <div class="text-xs text-muted-foreground mb-3">{{ t('admin.payments.detailQrCode') }}</div>
                <div class="text-foreground break-all text-sm">{{ detailPayment.qr_code }}</div>
              </CardContent>
            </Card>

            <Card class="rounded-lg border-border bg-background shadow-none">
              <CardContent class="p-4">
                <div class="text-xs text-muted-foreground mb-3">{{ t('admin.payments.detailPayload') }}</div>
                <div class="text-xs text-muted-foreground whitespace-pre-wrap font-mono bg-muted/40 p-4 rounded-lg border border-border overflow-auto max-h-64 break-all word-break-break-all">
                  {{ formatPayload(detailPayment.provider_payload) }}
                </div>
              </CardContent>
            </Card>
          </div>
        </div>
      </DialogScrollContent>
    </Dialog>
  </div>
  </ComplianceGuardWrapper>
</template>

<style scoped>
.break-all {
  word-break: break-all;
  overflow-wrap: break-word;
}
</style>
