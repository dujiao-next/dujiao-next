<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { adminAPI } from '@/api/admin'
import type { AdminResellerBalanceAccount } from '@/api/types'
import {
  RESELLER_BALANCE_STATUS_DISABLED,
  RESELLER_BALANCE_STATUS_FROZEN_REVIEW,
  RESELLER_BALANCE_STATUS_NEGATIVE,
  RESELLER_BALANCE_STATUS_NORMAL,
} from '@/constants/reseller'
import IdCell from '@/components/IdCell.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import TableSkeleton from '@/components/TableSkeleton.vue'
import ListPagination from '@/components/ListPagination.vue'
import { useListRefresh, type ListFetchOptions } from '@/composables/useListRefresh'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { formatDate } from '@/utils/format'
import ComplianceGuardWrapper from '@/components/ComplianceGuardWrapper.vue'
import { adminUrl } from '@/utils/adminBase'

const { t } = useI18n()
const route = useRoute()
const loading = ref(true)
const { refreshing, refreshList } = useListRefresh()
const rows = ref<AdminResellerBalanceAccount[]>([])
const pagination = ref({
  page: 1,
  page_size: 20,
  total: 0,
  total_page: 1,
})

const filters = reactive({
  keyword: '',
  resellerId: '',
  userId: '',
  status: '__all__',
})

const queryString = (value: unknown) => (Array.isArray(value) ? value[0] : value)

const initFiltersFromQuery = () => {
  const resellerId = String(queryString(route.query.reseller_id) || '').trim()
  if (resellerId) filters.resellerId = resellerId
}

const normalizeFilterValue = (value: string) => (value === '__all__' ? '' : value)
const pageSizeOptions = [10, 20, 50, 100]
const userDetailLink = (userId: number) => adminUrl(`/users/${userId}`)
const resolveResellerUserID = (item: AdminResellerBalanceAccount) => Number(item?.profile?.user_id || item?.profile?.user?.id || 0)

const fetchRows = async (page = 1, options: ListFetchOptions = {}) => {
  if (!options.preserveRows) loading.value = true
  try {
    const response = await adminAPI.getResellerBalanceAccounts({
      page,
      page_size: pagination.value.page_size,
      keyword: filters.keyword || undefined,
      reseller_id: filters.resellerId || undefined,
      user_id: filters.userId || undefined,
      status: normalizeFilterValue(filters.status) || undefined,
    })
    rows.value = response.data.data || []
    pagination.value = response.data.pagination || pagination.value
  } catch {
    if (!options.preserveRows) rows.value = []
  } finally {
    if (!options.preserveRows) loading.value = false
  }
}

const handleSearch = () => {
  fetchRows(1, { preserveRows: true })
}
const debouncedSearch = useDebounceFn(handleSearch, 300)

const refreshCurrentPage = () => {
  refreshList(() => fetchRows(pagination.value.page, { preserveRows: true }))
}

const changePage = (page: number) => {
  if (page < 1 || page > pagination.value.total_page) return
  fetchRows(page)
}

const changePageSize = (size: number) => {
  if (size === pagination.value.page_size) return
  pagination.value.page_size = size
  fetchRows(1)
}

const statusLabel = (status?: string) => {
  if (status === RESELLER_BALANCE_STATUS_NORMAL) return t('admin.resellerBalanceAccounts.status.normal')
  if (status === RESELLER_BALANCE_STATUS_NEGATIVE) return t('admin.resellerBalanceAccounts.status.negative_balance')
  if (status === RESELLER_BALANCE_STATUS_FROZEN_REVIEW) return t('admin.resellerBalanceAccounts.status.frozen_review')
  if (status === RESELLER_BALANCE_STATUS_DISABLED) return t('admin.resellerBalanceAccounts.status.disabled')
  return status || '-'
}

const statusClass = (status?: string) => {
  if (status === RESELLER_BALANCE_STATUS_NORMAL) return 'border-emerald-200 bg-emerald-50 text-emerald-700'
  if (status === RESELLER_BALANCE_STATUS_NEGATIVE) return 'border-red-200 bg-red-50 text-red-700'
  if (status === RESELLER_BALANCE_STATUS_FROZEN_REVIEW) return 'border-amber-200 bg-amber-50 text-amber-700'
  if (status === RESELLER_BALANCE_STATUS_DISABLED) return 'border-zinc-200 bg-zinc-50 text-zinc-700'
  return 'border-border bg-muted/30 text-muted-foreground'
}

const isNegative = (value: number | string | undefined) => Number(value || 0) > 0

onMounted(() => {
  initFiltersFromQuery()
  fetchRows()
})
</script>

<template>
  <ComplianceGuardWrapper>
    <div class="space-y-6">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <h1 class="text-2xl font-semibold">{{ t('admin.resellerBalanceAccounts.title') }}</h1>
      </div>

      <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
        <div class="flex flex-wrap items-center gap-3">
          <div class="w-full md:w-56">
            <Input v-model="filters.keyword" :placeholder="t('admin.resellerBalanceAccounts.filters.keyword')" @update:modelValue="debouncedSearch" />
          </div>
          <div class="w-full md:w-36">
            <Input v-model="filters.resellerId" :placeholder="t('admin.resellerBalanceAccounts.filters.resellerId')" @update:modelValue="debouncedSearch" />
          </div>
          <div class="w-full md:w-32">
            <Input v-model="filters.userId" :placeholder="t('admin.resellerBalanceAccounts.filters.userId')" @update:modelValue="debouncedSearch" />
          </div>
          <div class="w-full md:w-44">
            <Select v-model="filters.status" @update:modelValue="handleSearch">
              <SelectTrigger class="h-9 w-full">
                <SelectValue :placeholder="t('admin.resellerBalanceAccounts.filters.statusAll')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__all__">{{ t('admin.resellerBalanceAccounts.filters.statusAll') }}</SelectItem>
                <SelectItem :value="RESELLER_BALANCE_STATUS_NORMAL">{{ t('admin.resellerBalanceAccounts.status.normal') }}</SelectItem>
                <SelectItem :value="RESELLER_BALANCE_STATUS_NEGATIVE">{{ t('admin.resellerBalanceAccounts.status.negative_balance') }}</SelectItem>
                <SelectItem :value="RESELLER_BALANCE_STATUS_FROZEN_REVIEW">{{ t('admin.resellerBalanceAccounts.status.frozen_review') }}</SelectItem>
                <SelectItem :value="RESELLER_BALANCE_STATUS_DISABLED">{{ t('admin.resellerBalanceAccounts.status.disabled') }}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="flex-1"></div>
          <Button size="sm" variant="outline" class="w-full sm:w-auto" :disabled="refreshing" @click="refreshCurrentPage">{{ t('admin.common.refresh') }}</Button>
        </div>
      </div>

      <div class="rounded-xl border border-border bg-card overflow-x-auto">
        <Table class="min-w-[980px]">
          <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
            <TableRow>
              <TableHead class="px-6 py-3">{{ t('admin.resellerBalanceAccounts.table.id') }}</TableHead>
              <TableHead class="min-w-[180px] px-6 py-3">{{ t('admin.resellerBalanceAccounts.table.reseller') }}</TableHead>
              <TableHead class="px-6 py-3">{{ t('admin.resellerBalanceAccounts.table.currency') }}</TableHead>
              <TableHead class="px-6 py-3">{{ t('admin.resellerBalanceAccounts.table.available') }}</TableHead>
              <TableHead class="px-6 py-3">{{ t('admin.resellerBalanceAccounts.table.locked') }}</TableHead>
              <TableHead class="px-6 py-3">{{ t('admin.resellerBalanceAccounts.table.negative') }}</TableHead>
              <TableHead class="px-6 py-3">{{ t('admin.resellerBalanceAccounts.table.carryover') }}</TableHead>
              <TableHead class="min-w-[110px] px-6 py-3">{{ t('admin.resellerBalanceAccounts.table.status') }}</TableHead>
              <TableHead class="min-w-[100px] px-6 py-3">{{ t('admin.resellerBalanceAccounts.table.lastLedger') }}</TableHead>
              <TableHead class="min-w-[140px] px-6 py-3">{{ t('admin.resellerBalanceAccounts.table.updatedAt') }}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody class="divide-y divide-border">
            <TableRow v-if="loading">
              <TableCell :colspan="10" class="p-0">
                <TableSkeleton :columns="9" :rows="5" />
              </TableCell>
            </TableRow>
            <TableRow v-else-if="rows.length === 0">
              <TableCell colspan="10" class="px-6 py-8 text-center text-muted-foreground">{{ t('admin.resellerBalanceAccounts.empty') }}</TableCell>
            </TableRow>
            <TableRow v-for="item in rows" :key="item.id" class="hover:bg-muted/30">
              <TableCell class="px-6 py-4">
                <IdCell :value="item.id" />
              </TableCell>
              <TableCell class="min-w-[180px] px-6 py-4 text-xs text-muted-foreground">
                <div class="text-foreground break-words">{{ item?.profile?.user?.display_name || '-' }}</div>
                <div v-if="item?.profile?.user?.email" class="mt-0.5 break-all">{{ item.profile.user.email }}</div>
                <div class="mt-0.5 break-all">
                  <a
                    v-if="resolveResellerUserID(item) > 0"
                    :href="userDetailLink(resolveResellerUserID(item))"
                    target="_blank"
                    rel="noopener"
                    class="font-mono text-primary underline-offset-4 hover:underline"
                  >
                    #{{ resolveResellerUserID(item) }}
                  </a>
                  <span v-else class="font-mono text-foreground">-</span>
                  <span class="ml-1 font-mono text-muted-foreground">/ R#{{ item.reseller_id }}</span>
                </div>
              </TableCell>
              <TableCell class="px-6 py-4 font-mono text-xs text-muted-foreground">{{ item.currency || '-' }}</TableCell>
              <TableCell class="px-6 py-4 font-mono text-xs text-foreground">{{ item.available_amount_cache || '0.00' }}</TableCell>
              <TableCell class="px-6 py-4 font-mono text-xs text-foreground">{{ item.locked_amount_cache || '0.00' }}</TableCell>
              <TableCell class="px-6 py-4 font-mono text-xs" :class="isNegative(item.negative_amount_cache) ? 'text-destructive' : 'text-foreground'">
                {{ item.negative_amount_cache || '0.00' }}
              </TableCell>
              <TableCell class="px-6 py-4 font-mono text-xs text-foreground">
                {{ item.channel_fee_carryover || '0.00' }}
              </TableCell>
              <TableCell class="min-w-[110px] px-6 py-4 text-xs">
                <span class="inline-flex rounded-full border px-2.5 py-1 text-xs" :class="statusClass(item.status)">
                  {{ statusLabel(item.status) }}
                </span>
              </TableCell>
              <TableCell class="min-w-[100px] px-6 py-4 font-mono text-xs text-foreground">
                <span v-if="item.last_ledger_entry_id">#{{ item.last_ledger_entry_id }}</span>
                <span v-else>-</span>
              </TableCell>
              <TableCell class="min-w-[140px] px-6 py-4 text-xs text-muted-foreground">{{ formatDate(item.updated_at) }}</TableCell>
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
