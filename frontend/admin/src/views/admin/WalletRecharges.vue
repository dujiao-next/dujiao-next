<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { adminAPI, type AdminWalletRecharge } from '@/api/admin'
import IdCell from '@/components/IdCell.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import TableSkeleton from '@/components/TableSkeleton.vue'
import ListPagination from '@/components/ListPagination.vue'
import { useListRefresh, type ListFetchOptions } from '@/composables/useListRefresh'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { paymentStatusClass, paymentStatusLabel } from '@/utils/status'
import { formatDate, toRFC3339 } from '@/utils/format'
import { providerTypeLabel as providerTypeLabelMap, channelTypeLabel as channelTypeLabelMap } from '@/utils/paymentChannelDisplay'
import ComplianceGuardWrapper from '@/components/ComplianceGuardWrapper.vue'
import { adminUrl } from '@/utils/adminBase'

const { t } = useI18n()
const loading = ref(true)
const { refreshing, refreshList } = useListRefresh()
const recharges = ref<AdminWalletRecharge[]>([])
const pagination = ref({
  page: 1,
  page_size: 20,
  total: 0,
  total_page: 1,
})

const filters = reactive({
  rechargeNo: '',
  userId: '',
  userKeyword: '',
  paymentId: '',
  channelId: '',
  providerType: '__all__',
  status: '__all__',
  createdFrom: '',
  createdTo: '',
  paidFrom: '',
  paidTo: '',
})

const normalizeFilterValue = (value: string) => (value === '__all__' ? '' : value)

const fetchRecharges = async (page = 1, options: ListFetchOptions = {}) => {
  if (!options.preserveRows) loading.value = true
  try {
    const response = await adminAPI.getWalletRecharges({
      page,
      page_size: pagination.value.page_size,
      recharge_no: filters.rechargeNo || undefined,
      user_id: filters.userId || undefined,
      user_keyword: filters.userKeyword || undefined,
      payment_id: filters.paymentId || undefined,
      channel_id: filters.channelId || undefined,
      provider_type: normalizeFilterValue(filters.providerType) || undefined,
      status: normalizeFilterValue(filters.status) || undefined,
      created_from: toRFC3339(filters.createdFrom),
      created_to: toRFC3339(filters.createdTo),
      paid_from: toRFC3339(filters.paidFrom),
      paid_to: toRFC3339(filters.paidTo),
    })
    recharges.value = response.data.data || []
    pagination.value = response.data.pagination || pagination.value
  } catch (error) {
    if (!options.preserveRows) recharges.value = []
  } finally {
    if (!options.preserveRows) loading.value = false
  }
}

const handleSearch = () => {
  fetchRecharges(1)
}
const debouncedSearch = useDebounceFn(handleSearch, 300)

const refresh = () => {
  refreshList(() => fetchRecharges(pagination.value.page, { preserveRows: true }))
}

const changePage = (page: number) => {
  if (page < 1 || page > pagination.value.total_page) return
  fetchRecharges(page)
}

const pageSizeOptions = [10, 20, 50, 100]

const changePageSize = (size: number) => {
  if (size === pagination.value.page_size) return
  pagination.value.page_size = size
  fetchRecharges(1)
}

const userLink = (userID: number) => adminUrl(`/users/${userID}`)
const paymentLink = (paymentID: number) => adminUrl(`/payments?payment_id=${paymentID}`)
const channelLink = (channelID: number) => adminUrl(`/payment-channels?channel_id=${channelID}`)

const statusClass = (status?: string) => paymentStatusClass(status)
const statusLabel = (status?: string) => paymentStatusLabel(t, status)

const providerTypeLabel = (value?: string) => providerTypeLabelMap(t, value)
const channelTypeLabel = (value?: string) => channelTypeLabelMap(t, value)

onMounted(() => {
  fetchRecharges()
})
</script>

<template>
  <ComplianceGuardWrapper>
  <div class="space-y-6">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <h1 class="text-2xl font-semibold">{{ t('admin.walletRecharges.title') }}</h1>
    </div>

    <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div class="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center">
        <div class="w-full md:w-44">
          <Input v-model="filters.rechargeNo" :placeholder="t('admin.walletRecharges.filterRechargeNo')" @update:modelValue="debouncedSearch" />
        </div>
        <div class="w-full md:w-32">
          <Input v-model="filters.userId" :placeholder="t('admin.walletRecharges.filterUserId')" @update:modelValue="debouncedSearch" />
        </div>
        <div class="w-full md:w-48">
          <Input v-model="filters.userKeyword" :placeholder="t('admin.walletRecharges.filterUserKeyword')" @update:modelValue="debouncedSearch" />
        </div>
        <div class="w-full md:w-40">
          <Input v-model="filters.paymentId" :placeholder="t('admin.walletRecharges.filterPaymentId')" @update:modelValue="debouncedSearch" />
        </div>
        <div class="w-full md:w-40">
          <Input v-model="filters.channelId" :placeholder="t('admin.walletRecharges.filterChannelId')" @update:modelValue="debouncedSearch" />
        </div>
        <div class="w-full md:w-40">
          <Select v-model="filters.providerType" @update:modelValue="handleSearch">
            <SelectTrigger class="h-9 w-full">
              <SelectValue :placeholder="t('admin.walletRecharges.filterProviderAll')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="__all__">{{ t('admin.walletRecharges.filterProviderAll') }}</SelectItem>
              <SelectItem value="official">{{ t('admin.paymentChannels.providerTypes.official') }}</SelectItem>
              <SelectItem value="epay">{{ t('admin.paymentChannels.providerTypes.epay') }}</SelectItem>
              <SelectItem value="epusdt">{{ t('admin.paymentChannels.providerTypes.epusdt') }}</SelectItem>
              <SelectItem value="tokenpay">{{ t('admin.paymentChannels.providerTypes.tokenpay') }}</SelectItem>
              <SelectItem value="wallet">{{ t('admin.paymentChannels.providerTypes.wallet') }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="w-full md:w-40">
          <Select v-model="filters.status" @update:modelValue="handleSearch">
            <SelectTrigger class="h-9 w-full">
              <SelectValue :placeholder="t('admin.walletRecharges.filterStatusAll')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="__all__">{{ t('admin.walletRecharges.filterStatusAll') }}</SelectItem>
              <SelectItem value="pending">{{ t('payment.status.pending') }}</SelectItem>
              <SelectItem value="success">{{ t('payment.status.success') }}</SelectItem>
              <SelectItem value="failed">{{ t('payment.status.failed') }}</SelectItem>
              <SelectItem value="expired">{{ t('payment.status.expired') }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="flex w-full flex-col gap-2 md:w-auto md:flex-row md:flex-wrap md:items-center">
          <span class="text-xs text-muted-foreground whitespace-nowrap">{{ t('admin.walletRecharges.filterCreatedRange') }}</span>
          <Input
            v-model="filters.createdFrom"
            type="datetime-local"
            class="h-9 w-full md:w-auto"
            :placeholder="t('admin.walletRecharges.filterCreatedFrom')"
            @update:modelValue="handleSearch"
          />
          <span class="hidden text-muted-foreground md:inline">-</span>
          <Input
            v-model="filters.createdTo"
            type="datetime-local"
            class="h-9 w-full md:w-auto"
            :placeholder="t('admin.walletRecharges.filterCreatedTo')"
            @update:modelValue="handleSearch"
          />
        </div>
        <div class="flex w-full flex-col gap-2 md:w-auto md:flex-row md:flex-wrap md:items-center">
          <span class="text-xs text-muted-foreground whitespace-nowrap">{{ t('admin.walletRecharges.filterPaidRange') }}</span>
          <Input
            v-model="filters.paidFrom"
            type="datetime-local"
            class="h-9 w-full md:w-auto"
            :placeholder="t('admin.walletRecharges.filterPaidFrom')"
            @update:modelValue="handleSearch"
          />
          <span class="hidden text-muted-foreground md:inline">-</span>
          <Input
            v-model="filters.paidTo"
            type="datetime-local"
            class="h-9 w-full md:w-auto"
            :placeholder="t('admin.walletRecharges.filterPaidTo')"
            @update:modelValue="handleSearch"
          />
        </div>
        <div class="hidden flex-1 sm:block"></div>
        <Button size="sm" variant="outline" class="w-full sm:w-auto" :disabled="refreshing" @click="refresh">{{ t('admin.common.refresh') }}</Button>
      </div>
    </div>

    <div class="rounded-xl border border-border bg-card overflow-x-auto">
      <Table class="min-w-[960px]">
        <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
          <TableRow>
            <TableHead class="px-6 py-3">{{ t('admin.walletRecharges.table.id') }}</TableHead>
            <TableHead class="min-w-[160px] px-6 py-3">{{ t('admin.walletRecharges.table.rechargeNo') }}</TableHead>
            <TableHead class="min-w-[160px] px-6 py-3">{{ t('admin.walletRecharges.table.user') }}</TableHead>
            <TableHead class="min-w-[140px] px-6 py-3">{{ t('admin.walletRecharges.table.payment') }}</TableHead>
            <TableHead class="min-w-[160px] px-6 py-3">{{ t('admin.walletRecharges.table.channel') }}</TableHead>
            <TableHead class="min-w-[90px] px-6 py-3">{{ t('admin.walletRecharges.table.status') }}</TableHead>
            <TableHead class="min-w-[140px] px-6 py-3">{{ t('admin.walletRecharges.table.amount') }}</TableHead>
            <TableHead class="min-w-[140px] px-6 py-3">{{ t('admin.walletRecharges.table.paidAt') }}</TableHead>
            <TableHead class="min-w-[140px] px-6 py-3">{{ t('admin.walletRecharges.table.createdAt') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody class="divide-y divide-border">
          <TableRow v-if="loading">
            <TableCell :colspan="9" class="p-0">
              <TableSkeleton :columns="9" :rows="5" />
            </TableCell>
          </TableRow>
          <TableRow v-else-if="recharges.length === 0">
            <TableCell colspan="9" class="px-6 py-8 text-center text-muted-foreground">{{ t('admin.walletRecharges.empty') }}</TableCell>
          </TableRow>
          <TableRow v-for="item in recharges" :key="item.id" class="hover:bg-muted/30">
            <TableCell class="px-6 py-4">
              <IdCell :value="item.id" />
            </TableCell>
            <TableCell class="min-w-[160px] px-6 py-4 text-foreground font-mono">
              <div class="break-all">{{ item.recharge_no }}</div>
            </TableCell>
            <TableCell class="min-w-[160px] px-6 py-4 text-xs text-muted-foreground">
              <div class="text-foreground">
                <a v-if="item.user_id" :href="userLink(item.user_id)" target="_blank" rel="noopener" class="text-primary underline-offset-4 hover:underline">
                  #{{ item.user_id }}
                </a>
                <span v-else>-</span>
              </div>
              <div v-if="item.user?.display_name" class="mt-0.5 break-words text-foreground">{{ item.user.display_name }}</div>
              <div v-if="item.user?.email" class="mt-0.5 break-all">{{ item.user.email }}</div>
            </TableCell>
            <TableCell class="min-w-[140px] px-6 py-4 text-xs text-muted-foreground">
              <div class="text-foreground">
                <a v-if="item.payment_id" :href="paymentLink(item.payment_id)" target="_blank" rel="noopener" class="text-primary underline-offset-4 hover:underline">
                  #{{ item.payment_id }}
                </a>
                <span v-else>-</span>
              </div>
              <div v-if="item.payment_status" class="mt-1">
                <span class="inline-flex rounded-full border px-2.5 py-1 text-xs" :class="statusClass(item.payment_status)">
                  {{ statusLabel(item.payment_status) }}
                </span>
              </div>
            </TableCell>
            <TableCell class="min-w-[160px] px-6 py-4 text-xs text-muted-foreground">
              <div class="break-words text-foreground">{{ item.channel_name || '-' }}</div>
              <div class="break-words text-muted-foreground">{{ providerTypeLabel(item.provider_type) }} / {{ channelTypeLabel(item.channel_type) }}</div>
              <div class="mt-1">
                {{ t('admin.walletRecharges.channelId') }}:
                <a v-if="item.channel_id" :href="channelLink(item.channel_id)" target="_blank" rel="noopener" class="text-primary underline-offset-4 hover:underline">
                  #{{ item.channel_id }}
                </a>
                <span v-else>-</span>
              </div>
            </TableCell>
            <TableCell class="min-w-[90px] px-6 py-4 text-xs">
              <span class="inline-flex rounded-full border px-2.5 py-1 text-xs" :class="statusClass(item.status)">
                {{ statusLabel(item.status) }}
              </span>
            </TableCell>
            <TableCell class="min-w-[140px] px-6 py-4 text-xs text-muted-foreground">
              <div class="font-mono text-foreground">{{ item.amount }} {{ item.currency }}</div>
              <div class="mt-1">{{ t('admin.walletRecharges.payableAmount') }}: <span class="font-mono text-foreground">{{ item.payable_amount }} {{ item.currency }}</span></div>
              <div class="mt-1">{{ t('admin.walletRecharges.feeAmount') }}: <span class="font-mono text-foreground">{{ item.fee_amount }} {{ item.currency }}</span></div>
            </TableCell>
            <TableCell class="min-w-[140px] px-6 py-4 text-xs text-muted-foreground">{{ formatDate(item.paid_at) || '-' }}</TableCell>
            <TableCell class="min-w-[140px] px-6 py-4 text-xs text-muted-foreground">{{ formatDate(item.created_at) }}</TableCell>
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
  </div>
  </ComplianceGuardWrapper>
</template>
