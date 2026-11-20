<template>
  <div class="space-y-5">
    <ResellerSectionHeader :title="t('resellerConsole.ledger.title')" :description="t('resellerConsole.ledger.description')" />

    <ResellerFilterBar @search="reload" @reset="resetFilters">
      <template #fields>
        <Select v-model="filters.type">
          <SelectTrigger>
            <SelectValue :placeholder="t('resellerConsole.common.allTypes')" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{{ t('resellerConsole.common.allTypes') }}</SelectItem>
            <SelectItem value="order_profit">{{ t('personalCenter.reseller.ledgerType.orderProfit') }}</SelectItem>
            <SelectItem value="refund_deduct">{{ t('personalCenter.reseller.ledgerType.refundDeduct') }}</SelectItem>
            <SelectItem value="withdraw_lock">{{ t('personalCenter.reseller.ledgerType.withdrawLock') }}</SelectItem>
            <SelectItem value="manual_adjust">{{ t('personalCenter.reseller.ledgerType.manualAdjust') }}</SelectItem>
          </SelectContent>
        </Select>
        <Select v-model="filters.status">
          <SelectTrigger>
            <SelectValue :placeholder="t('resellerConsole.orders.statusAll')" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{{ t('resellerConsole.orders.statusAll') }}</SelectItem>
            <SelectItem value="pending_confirm">{{ t('personalCenter.reseller.ledgerStatus.pendingConfirm') }}</SelectItem>
            <SelectItem value="available">{{ t('personalCenter.reseller.ledgerStatus.available') }}</SelectItem>
            <SelectItem value="locked">{{ t('personalCenter.reseller.ledgerStatus.locked') }}</SelectItem>
            <SelectItem value="withdrawn">{{ t('personalCenter.reseller.ledgerStatus.withdrawn') }}</SelectItem>
          </SelectContent>
        </Select>
        <Input v-model.number="filters.order_id" type="number" min="0" :placeholder="t('resellerConsole.ledger.orderId')" />
      </template>
    </ResellerFilterBar>

    <ResellerPageState v-if="ledgerLoading" loading :title="t('resellerConsole.common.loading')" />
    <ResellerPageState
      v-else-if="ledgerEntries.length === 0 && hasActiveFilter"
      :title="t('resellerConsole.common.noFilterResult')"
      :icon="FileText"
    >
      <Button type="button" variant="outline" size="sm" @click="resetFilters">
        {{ t('resellerConsole.common.reset') }}
      </Button>
    </ResellerPageState>
    <ResellerPageState v-else-if="ledgerEntries.length === 0" :title="t('personalCenter.reseller.ledgerEmpty')" :icon="FileText" />

    <template v-else>
      <!-- 桌面表 -->
      <ResellerDataTable class="hidden lg:block">
        <TableHeader>
          <TableRow class="bg-muted/50">
            <TableHead class="px-4">{{ t('personalCenter.reseller.ledgerTable.type') }}</TableHead>
            <TableHead class="px-4 text-right">{{ t('personalCenter.reseller.ledgerTable.amount') }}</TableHead>
            <TableHead class="px-4">{{ t('personalCenter.reseller.ledgerTable.status') }}</TableHead>
            <TableHead class="px-4">{{ t('personalCenter.reseller.ledgerTable.availableAt') }}</TableHead>
            <TableHead class="px-4">{{ t('personalCenter.reseller.ledgerTable.createdAt') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-for="item in ledgerEntries" :key="item.id">
            <TableCell class="px-4 py-3 text-xs text-foreground">
              <span class="inline-flex items-center gap-2">
                <component :is="typeIcon(item.type)" class="h-4 w-4 text-muted-foreground" />
                {{ ledgerTypeLabel(item.type) }}
              </span>
            </TableCell>
            <TableCell class="px-4 py-3 text-right tabular-nums text-xs font-semibold" :class="amountClass(item)">{{ amountDisplay(item) }}</TableCell>
            <TableCell class="px-4 py-3"><ResellerStatusBadge :label="ledgerStatusLabel(item.status)" :tone="ledgerTone(item.status)" /></TableCell>
            <TableCell class="px-4 py-3 text-xs text-muted-foreground">{{ formatResellerConsoleDate(item.available_at) }}</TableCell>
            <TableCell class="px-4 py-3 text-xs text-muted-foreground">{{ formatResellerConsoleDate(item.created_at) }}</TableCell>
          </TableRow>
        </TableBody>
      </ResellerDataTable>

      <!-- 移动卡片 -->
      <div class="space-y-3 lg:hidden">
        <ResellerRecordCard v-for="item in ledgerEntries" :key="item.id">
          <template #header>
            <span class="inline-flex items-center gap-2 text-xs font-semibold text-foreground">
              <component :is="typeIcon(item.type)" class="h-4 w-4 text-muted-foreground" />
              {{ ledgerTypeLabel(item.type) }}
            </span>
            <span class="tabular-nums text-sm font-bold" :class="amountClass(item)">{{ amountDisplay(item) }}</span>
          </template>
          <div>
            <dt class="text-muted-foreground">{{ t('personalCenter.reseller.ledgerTable.status') }}</dt>
            <dd class="mt-0.5"><ResellerStatusBadge :label="ledgerStatusLabel(item.status)" :tone="ledgerTone(item.status)" /></dd>
          </div>
          <div>
            <dt class="text-muted-foreground">{{ t('personalCenter.reseller.ledgerTable.availableAt') }}</dt>
            <dd class="mt-0.5 text-foreground">{{ formatResellerConsoleDate(item.available_at) }}</dd>
          </div>
          <div class="col-span-2">
            <dt class="text-muted-foreground">{{ t('personalCenter.reseller.ledgerTable.createdAt') }}</dt>
            <dd class="mt-0.5 text-foreground">{{ formatResellerConsoleDate(item.created_at) }}</dd>
          </div>
        </ResellerRecordCard>
      </div>

      <div v-if="ledgerPagination.total_page > 1" class="flex flex-wrap items-center justify-center gap-3">
        <Button variant="outline" size="sm" :disabled="ledgerPagination.page <= 1" @click="goPage(ledgerPagination.page - 1)">
          {{ t('orders.prevPage') }}
        </Button>
        <span class="rounded-full border bg-card px-4 py-1.5 text-sm text-muted-foreground">
          {{ t('orders.pageInfo', { page: ledgerPagination.page, total: ledgerPagination.total_page }) }}
        </span>
        <Button variant="outline" size="sm" :disabled="ledgerPagination.page >= ledgerPagination.total_page" @click="goPage(ledgerPagination.page + 1)">
          {{ t('orders.nextPage') }}
        </Button>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import { FileText, Lock, SlidersHorizontal, TrendingUp, Undo2 } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import ResellerDataTable from '../../components/reseller-console/ResellerDataTable.vue'
import ResellerFilterBar from '../../components/reseller-console/ResellerFilterBar.vue'
import ResellerPageState from '../../components/reseller-console/ResellerPageState.vue'
import ResellerRecordCard from '../../components/reseller-console/ResellerRecordCard.vue'
import ResellerSectionHeader from '../../components/reseller-console/ResellerSectionHeader.vue'
import ResellerStatusBadge, { type ResellerBadgeTone } from '../../components/reseller-console/ResellerStatusBadge.vue'
import { useResellerFinance } from '../../composables/reseller/useResellerFinance'
import type { ResellerLedgerData } from '../../api'
import {
  RESELLER_LEDGER_STATUS_AVAILABLE,
  RESELLER_LEDGER_STATUS_CANCELED,
  RESELLER_LEDGER_STATUS_LOCKED,
  RESELLER_LEDGER_STATUS_PENDING_CONFIRM,
  RESELLER_LEDGER_STATUS_WITHDRAWN,
} from '../../constants/reseller'
import { formatResellerConsoleDate, resellerAmountSign } from '../../utils/resellerConsole'
import { getResellerLedgerTypeKey } from '../../utils/resellerFinance'

const { t } = useI18n()
const { ledgerLoading, ledgerEntries, ledgerPagination, loadLedgerEntries } = useResellerFinance()
const filters = reactive({ type: 'all', status: 'all', order_id: undefined as number | undefined })

const hasActiveFilter = computed(() =>
  Boolean((filters.type && filters.type !== 'all') || (filters.status && filters.status !== 'all') || filters.order_id),
)

const currentParams = () => ({
  type: filters.type && filters.type !== 'all' ? filters.type : undefined,
  status: filters.status && filters.status !== 'all' ? filters.status : undefined,
  order_id: filters.order_id || undefined,
})

const reload = () => loadLedgerEntries({ ...currentParams(), page: 1 })
const goPage = (page: number) => loadLedgerEntries({ ...currentParams(), page })

const resetFilters = () => {
  filters.type = 'all'
  filters.status = 'all'
  filters.order_id = undefined
  void reload()
}

const ledgerTypeLabel = (type?: string) => {
  const key = getResellerLedgerTypeKey(type)
  return key ? t(`personalCenter.reseller.ledgerType.${key}`) : type || '-'
}

const typeIcon = (type?: string): Component => {
  switch (type) {
    case 'order_profit':
      return TrendingUp
    case 'refund_deduct':
      return Undo2
    case 'withdraw_lock':
      return Lock
    default:
      return SlidersHorizontal
  }
}

const ledgerDirection = (item: ResellerLedgerData): 'income' | 'expense' | 'neutral' => {
  if (item.type === 'order_profit') return 'income'
  if (item.type === 'refund_deduct' || item.type === 'withdraw_lock') return 'expense'
  const sign = resellerAmountSign(item.amount)
  if (sign === 'negative') return 'expense'
  if (sign === 'positive') return 'income'
  return 'neutral'
}

const amountDisplay = (item: ResellerLedgerData) => {
  const dir = ledgerDirection(item)
  const abs = Math.abs(Number(item.amount) || 0)
  const prefix = dir === 'income' ? '+' : dir === 'expense' ? '−' : ''
  return `${prefix}${abs.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })} ${item.currency}`
}

const amountClass = (item: ResellerLedgerData) => {
  const dir = ledgerDirection(item)
  if (dir === 'income') return 'text-success'
  if (dir === 'expense') return 'text-destructive'
  return 'text-foreground'
}

const ledgerStatusLabel = (status?: string) => {
  if (status === RESELLER_LEDGER_STATUS_PENDING_CONFIRM) return t('personalCenter.reseller.ledgerStatus.pendingConfirm')
  if (status === RESELLER_LEDGER_STATUS_AVAILABLE) return t('personalCenter.reseller.ledgerStatus.available')
  if (status === RESELLER_LEDGER_STATUS_LOCKED) return t('personalCenter.reseller.ledgerStatus.locked')
  if (status === RESELLER_LEDGER_STATUS_WITHDRAWN) return t('personalCenter.reseller.ledgerStatus.withdrawn')
  if (status === RESELLER_LEDGER_STATUS_CANCELED) return t('personalCenter.reseller.ledgerStatus.canceled')
  return status || '-'
}

const ledgerTone = (status?: string): ResellerBadgeTone => {
  if (status === RESELLER_LEDGER_STATUS_AVAILABLE) return 'success'
  if (status === RESELLER_LEDGER_STATUS_PENDING_CONFIRM || status === RESELLER_LEDGER_STATUS_LOCKED) return 'warning'
  return 'neutral'
}

onMounted(reload)
</script>
