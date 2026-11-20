<template>
  <div class="space-y-5">
    <ResellerSectionHeader :title="t('resellerConsole.orders.title')" :description="t('resellerConsole.orders.description')" />

    <div v-if="stats" class="grid grid-cols-1 gap-4 sm:grid-cols-3">
      <ResellerMetricCard :label="t('resellerConsole.dashboard.orderTotal')" :value="stats.total" :icon="ShoppingBag" tone="accent" />
      <ResellerMetricCard :label="t('resellerConsole.orders.paidOrders')" :value="stats.by_status?.paid || 0" :icon="CheckCircle2" tone="success" />
      <ResellerMetricCard :label="t('resellerConsole.orders.currencyKinds')" :value="Object.keys(stats.by_currency || {}).length" :icon="Banknote" tone="info" />
    </div>

    <ResellerFilterBar @search="reload" @reset="resetFilters">
      <template #fields>
        <Input v-model.trim="filters.order_no" :placeholder="t('resellerConsole.orders.search')" />
        <Select v-model="filters.status">
          <SelectTrigger>
            <SelectValue :placeholder="t('resellerConsole.orders.statusAll')" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{{ t('resellerConsole.orders.statusAll') }}</SelectItem>
            <SelectItem v-for="status in statusOptions" :key="status" :value="status">{{ statusLabel(status) }}</SelectItem>
          </SelectContent>
        </Select>
        <div class="flex flex-col gap-1">
          <Label class="text-xs text-muted-foreground">{{ t('resellerConsole.common.dateFrom') }}</Label>
          <Input v-model="filters.created_from" type="date" />
        </div>
        <div class="flex flex-col gap-1">
          <Label class="text-xs text-muted-foreground">{{ t('resellerConsole.common.dateTo') }}</Label>
          <Input v-model="filters.created_to" type="date" />
        </div>
      </template>
    </ResellerFilterBar>

    <ResellerPageState v-if="loading" loading :title="t('resellerConsole.common.loading')" />
    <ResellerPageState
      v-else-if="error"
      :title="t('resellerConsole.common.loadFailed')"
      :description="error !== 'error' ? error : undefined"
      :icon="AlertTriangle"
    >
      <Button type="button" variant="outline" size="sm" @click="reload">
        {{ t('resellerConsole.common.retry') }}
      </Button>
    </ResellerPageState>
    <ResellerPageState
      v-else-if="rows.length === 0 && hasActiveFilter"
      :title="t('resellerConsole.common.noFilterResult')"
      :icon="ShoppingBag"
    >
      <Button type="button" variant="outline" size="sm" @click="resetFilters">
        {{ t('resellerConsole.common.reset') }}
      </Button>
    </ResellerPageState>
    <ResellerPageState v-else-if="rows.length === 0" :title="t('resellerConsole.orders.empty')" :icon="ShoppingBag" />

    <template v-else>
      <!-- 桌面表 -->
      <ResellerDataTable class="hidden lg:block">
        <TableHeader>
          <TableRow class="bg-muted/50">
            <TableHead class="px-4">{{ t('resellerConsole.orders.orderNo') }}</TableHead>
            <TableHead class="px-4">{{ t('resellerConsole.orders.status') }}</TableHead>
            <TableHead class="px-4 text-right">{{ t('resellerConsole.orders.totalAmount') }}</TableHead>
            <TableHead class="px-4 text-right">{{ t('resellerConsole.orders.profitAmount') }}</TableHead>
            <TableHead class="px-4">
              <span class="inline-flex items-center gap-1">
                {{ t('resellerConsole.orders.profitStatus') }}
                <ResellerHint :text="t('resellerConsole.orders.profitHelp')" />
              </span>
            </TableHead>
            <TableHead class="px-4">{{ t('resellerConsole.orders.createdAt') }}</TableHead>
            <TableHead class="px-4"></TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow
            v-for="row in rows"
            :key="row.order_no"
            class="cursor-pointer"
            @click="goDetail(row.order_no)"
          >
            <TableCell class="px-4 py-3 tabular-nums text-xs text-foreground">{{ row.order_no }}</TableCell>
            <TableCell class="px-4 py-3"><ResellerStatusBadge :label="statusLabel(row.status)" :tone="statusTone(row.status)" /></TableCell>
            <TableCell class="px-4 py-3 text-right tabular-nums text-xs">{{ formatResellerConsoleAmount(row.total_amount, row.currency) }}</TableCell>
            <TableCell class="px-4 py-3 text-right tabular-nums text-xs">{{ formatResellerConsoleAmount(row.profit_amount, row.currency) }}</TableCell>
            <TableCell class="px-4 py-3"><ResellerStatusBadge :label="profitLabel(row.profit_status)" :tone="profitTone(row.profit_status)" /></TableCell>
            <TableCell class="px-4 py-3 text-xs text-muted-foreground">{{ formatResellerConsoleDate(row.created_at) }}</TableCell>
            <TableCell class="px-4 py-3 text-right">
              <ChevronRight class="ml-auto h-4 w-4 text-muted-foreground" />
            </TableCell>
          </TableRow>
        </TableBody>
      </ResellerDataTable>

      <!-- 移动卡片 -->
      <div class="space-y-3 lg:hidden">
        <RouterLink v-for="row in rows" :key="row.order_no" :to="`/reseller/orders/${encodeURIComponent(row.order_no)}`" class="block">
          <ResellerRecordCard>
            <template #header>
              <span class="min-w-0 truncate tabular-nums text-xs font-semibold text-foreground">{{ row.order_no }}</span>
              <ResellerStatusBadge :label="statusLabel(row.status)" :tone="statusTone(row.status)" />
            </template>
            <div>
              <dt class="text-muted-foreground">{{ t('resellerConsole.orders.totalAmount') }}</dt>
              <dd class="mt-0.5 tabular-nums text-foreground">{{ formatResellerConsoleAmount(row.total_amount, row.currency) }}</dd>
            </div>
            <div>
              <dt class="text-muted-foreground">{{ t('resellerConsole.orders.profitAmount') }}</dt>
              <dd class="mt-0.5 tabular-nums text-foreground">{{ formatResellerConsoleAmount(row.profit_amount, row.currency) }}</dd>
            </div>
            <div>
              <dt class="text-muted-foreground">{{ t('resellerConsole.orders.profitStatus') }}</dt>
              <dd class="mt-0.5"><ResellerStatusBadge :label="profitLabel(row.profit_status)" :tone="profitTone(row.profit_status)" /></dd>
            </div>
            <div>
              <dt class="text-muted-foreground">{{ t('resellerConsole.orders.createdAt') }}</dt>
              <dd class="mt-0.5 text-foreground">{{ formatResellerConsoleDate(row.created_at) }}</dd>
            </div>
          </ResellerRecordCard>
        </RouterLink>
      </div>

      <div v-if="pagination.total_page > 1" class="flex flex-wrap items-center justify-center gap-3">
        <Button variant="outline" size="sm" :disabled="pagination.page <= 1" @click="goPage(pagination.page - 1)">
          {{ t('orders.prevPage') }}
        </Button>
        <span class="rounded-full border bg-card px-4 py-1.5 text-sm text-muted-foreground">
          {{ t('orders.pageInfo', { page: pagination.page, total: pagination.total_page }) }}
        </span>
        <Button variant="outline" size="sm" :disabled="pagination.page >= pagination.total_page" @click="goPage(pagination.page + 1)">
          {{ t('orders.nextPage') }}
        </Button>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { AlertTriangle, Banknote, CheckCircle2, ChevronRight, ShoppingBag } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import ResellerDataTable from '../../components/reseller-console/ResellerDataTable.vue'
import ResellerFilterBar from '../../components/reseller-console/ResellerFilterBar.vue'
import ResellerHint from '../../components/reseller-console/ResellerHint.vue'
import ResellerMetricCard from '../../components/reseller-console/ResellerMetricCard.vue'
import ResellerPageState from '../../components/reseller-console/ResellerPageState.vue'
import ResellerRecordCard from '../../components/reseller-console/ResellerRecordCard.vue'
import ResellerSectionHeader from '../../components/reseller-console/ResellerSectionHeader.vue'
import ResellerStatusBadge, { type ResellerBadgeTone } from '../../components/reseller-console/ResellerStatusBadge.vue'
import { useResellerOrders } from '../../composables/reseller/useResellerOrders'
import {
  formatResellerConsoleAmount,
  formatResellerConsoleDate,
  resellerOrderStatusTone,
  resellerProfitStatusKey,
} from '../../utils/resellerConsole'

const { t, te } = useI18n()
const router = useRouter()
const { loading, error, rows, stats, pagination, load, loadStats } = useResellerOrders()
const filters = reactive({ order_no: '', status: 'all', created_from: '', created_to: '' })

const statusOptions = ['pending_payment', 'paid', 'completed', 'partially_refunded', 'refunded', 'canceled']

const hasActiveFilter = computed(() =>
  Boolean(filters.order_no || (filters.status && filters.status !== 'all') || filters.created_from || filters.created_to),
)

const statusLabel = (status?: string) => {
  if (!status) return '-'
  const key = `resellerConsole.orders.statusMap.${status}`
  return te(key) ? t(key) : status
}
const statusTone = (status?: string) => resellerOrderStatusTone(status) as ResellerBadgeTone

const profitLabel = (status?: string) => t(`resellerConsole.orders.profit.${resellerProfitStatusKey(status)}`)
const profitTone = (status?: string): ResellerBadgeTone => {
  if (status === 'credited') return 'success'
  if (status === 'pending') return 'warning'
  return 'neutral'
}

const currentParams = () => ({
  order_no: filters.order_no || undefined,
  status: filters.status && filters.status !== 'all' ? filters.status : undefined,
  created_from: filters.created_from || undefined,
  created_to: filters.created_to || undefined,
})

const reload = () => Promise.all([load({ ...currentParams(), page: 1 }), loadStats(currentParams())])
const goPage = (page: number) => load({ ...currentParams(), page })
const goDetail = (orderNo: string) => router.push(`/reseller/orders/${encodeURIComponent(orderNo)}`)

const resetFilters = () => {
  filters.order_no = ''
  filters.status = 'all'
  filters.created_from = ''
  filters.created_to = ''
  void reload()
}

onMounted(reload)
</script>
