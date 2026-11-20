<template>
  <div class="space-y-5">
    <ResellerSectionHeader
      :title="detail?.order_no || t('resellerConsole.orderDetail.title')"
      :description="t('resellerConsole.orderDetail.description')"
    >
      <template #actions>
        <ResellerCopyButton v-if="detail" :value="detail.order_no" />
        <Button as-child variant="outline" size="sm">
          <RouterLink to="/reseller/orders">
            <ArrowLeft class="h-4 w-4" />
            {{ t('resellerConsole.orderDetail.back') }}
          </RouterLink>
        </Button>
      </template>
    </ResellerSectionHeader>

    <ResellerPageState v-if="detailLoading" loading :title="t('resellerConsole.common.loading')" />
    <ResellerPageState
      v-else-if="detailError"
      :title="t('resellerConsole.common.loadFailed')"
      :description="detailError !== 'error' ? detailError : undefined"
      :icon="AlertTriangle"
    >
      <Button type="button" variant="outline" size="sm" @click="reload">
        {{ t('resellerConsole.common.retry') }}
      </Button>
    </ResellerPageState>
    <ResellerPageState v-else-if="!detail" :title="t('resellerConsole.orderDetail.notFound')" :icon="ShoppingBag" />

    <div v-else class="grid grid-cols-1 gap-5 lg:grid-cols-3">
      <!-- 左:订单信息 + 商品明细 -->
      <div class="space-y-5 lg:col-span-2">
        <Card class="p-5">
          <dl class="grid grid-cols-1 gap-4 text-sm sm:grid-cols-2">
            <div>
              <dt class="text-muted-foreground">{{ t('resellerConsole.orders.status') }}</dt>
              <dd class="mt-1"><ResellerStatusBadge :label="statusLabel(detail.status)" :tone="statusTone(detail.status)" /></dd>
            </div>
            <div>
              <dt class="text-muted-foreground">{{ t('resellerConsole.orders.domain') }}</dt>
              <dd class="mt-1 break-all tabular-nums text-foreground">{{ detail.domain || '-' }}</dd>
            </div>
            <div>
              <dt class="text-muted-foreground">{{ t('resellerConsole.orders.buyer') }}</dt>
              <dd class="mt-1 text-foreground">{{ detail.buyer_label || '-' }}</dd>
            </div>
            <div>
              <dt class="text-muted-foreground">{{ t('resellerConsole.orders.createdAt') }}</dt>
              <dd class="mt-1 text-foreground">{{ formatResellerConsoleDate(detail.created_at) }}</dd>
            </div>
            <div>
              <dt class="text-muted-foreground">{{ t('resellerConsole.orders.paidAt') }}</dt>
              <dd class="mt-1 text-foreground">{{ formatResellerConsoleDate(detail.paid_at) }}</dd>
            </div>
          </dl>
        </Card>

        <section>
          <h2 class="mb-3 text-base font-bold text-foreground">{{ t('resellerConsole.orderDetail.items') }}</h2>
          <!-- 桌面表 -->
          <ResellerDataTable class="hidden lg:block">
            <TableHeader>
              <TableRow class="bg-muted/50">
                <TableHead class="px-4">{{ t('resellerConsole.orderDetail.product') }}</TableHead>
                <TableHead class="px-4 text-right">{{ t('resellerConsole.orderDetail.quantity') }}</TableHead>
                <TableHead class="px-4 text-right">{{ t('resellerConsole.orderDetail.unitPrice') }}</TableHead>
                <TableHead class="px-4 text-right">{{ t('resellerConsole.orderDetail.baseUnit') }}</TableHead>
                <TableHead class="px-4 text-right">{{ t('resellerConsole.orders.profitAmount') }}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="(item, index) in detail.items" :key="index">
                <TableCell class="px-4 py-3 text-sm text-foreground">
                  <div class="font-semibold">{{ localized(item.title) }}</div>
                  <div class="mt-1 text-xs text-muted-foreground">{{ skuText(item) }}</div>
                </TableCell>
                <TableCell class="px-4 py-3 text-right tabular-nums text-xs">{{ item.quantity }}</TableCell>
                <TableCell class="px-4 py-3 text-right tabular-nums text-xs">{{ formatResellerConsoleAmount(item.unit_price, detail.currency) }}</TableCell>
                <TableCell class="px-4 py-3 text-right tabular-nums text-xs">{{ formatResellerConsoleAmount(item.base_unit_amount, detail.currency) }}</TableCell>
                <TableCell class="px-4 py-3 text-right tabular-nums text-xs">{{ formatResellerConsoleAmount(item.profit_amount, detail.currency) }}</TableCell>
              </TableRow>
            </TableBody>
          </ResellerDataTable>

          <!-- 移动卡片 -->
          <div class="space-y-3 lg:hidden">
            <ResellerRecordCard v-for="(item, index) in detail.items" :key="index">
              <template #header>
                <div class="min-w-0">
                  <div class="truncate text-sm font-semibold text-foreground">{{ localized(item.title) }}</div>
                  <div class="mt-0.5 truncate text-xs text-muted-foreground">{{ skuText(item) }}</div>
                </div>
              </template>
              <div>
                <dt class="text-muted-foreground">{{ t('resellerConsole.orderDetail.quantity') }}</dt>
                <dd class="mt-0.5 tabular-nums text-foreground">{{ item.quantity }}</dd>
              </div>
              <div>
                <dt class="text-muted-foreground">{{ t('resellerConsole.orderDetail.unitPrice') }}</dt>
                <dd class="mt-0.5 tabular-nums text-foreground">{{ formatResellerConsoleAmount(item.unit_price, detail.currency) }}</dd>
              </div>
              <div>
                <dt class="text-muted-foreground">{{ t('resellerConsole.orderDetail.baseUnit') }}</dt>
                <dd class="mt-0.5 tabular-nums text-foreground">{{ formatResellerConsoleAmount(item.base_unit_amount, detail.currency) }}</dd>
              </div>
              <div>
                <dt class="text-muted-foreground">{{ t('resellerConsole.orders.profitAmount') }}</dt>
                <dd class="mt-0.5 tabular-nums text-foreground">{{ formatResellerConsoleAmount(item.profit_amount, detail.currency) }}</dd>
              </div>
            </ResellerRecordCard>
          </div>
        </section>
      </div>

      <!-- 右:利润快照 + 进度 -->
      <div class="space-y-5">
        <Card class="p-5">
          <div class="mb-4 flex items-center gap-1.5">
            <h2 class="text-base font-bold text-foreground">{{ t('resellerConsole.orderDetail.profitSnapshot') }}</h2>
            <ResellerHint :text="t('resellerConsole.orders.profitHelp')" />
          </div>
          <dl class="space-y-3 text-sm">
            <div class="flex items-center justify-between gap-3">
              <dt class="text-muted-foreground">{{ t('resellerConsole.orders.totalAmount') }}</dt>
              <dd class="tabular-nums font-semibold text-foreground">{{ formatResellerConsoleAmount(detail.total_amount, detail.currency) }}</dd>
            </div>
            <div class="flex items-center justify-between gap-3">
              <dt class="text-muted-foreground">{{ t('resellerConsole.orders.baseAmount') }}</dt>
              <dd class="tabular-nums text-foreground/80">{{ formatResellerConsoleAmount(detail.base_amount, detail.currency) }}</dd>
            </div>
            <div class="flex items-center justify-between gap-3 border-t pt-3">
              <dt class="font-semibold text-foreground">{{ t('resellerConsole.orders.profitAmount') }}</dt>
              <dd class="tabular-nums text-base font-bold text-success">{{ formatResellerConsoleAmount(detail.profit_amount, detail.currency) }}</dd>
            </div>
            <div class="flex items-center justify-between gap-3">
              <dt class="text-muted-foreground">{{ t('resellerConsole.orders.profitStatus') }}</dt>
              <dd><ResellerStatusBadge :label="profitLabel(detail.profit_status)" :tone="profitTone(detail.profit_status)" /></dd>
            </div>
          </dl>
        </Card>

        <Card class="p-5">
          <h2 class="mb-4 text-base font-bold text-foreground">{{ t('resellerConsole.orderDetail.timeline') }}</h2>
          <ol class="space-y-4">
            <li v-for="(step, i) in timeline" :key="i" class="flex gap-3">
              <div class="flex flex-col items-center">
                <span
                  class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-xs"
                  :class="step.done ? 'bg-success text-white' : 'bg-muted text-muted-foreground'"
                >
                  <Check v-if="step.done" class="h-3.5 w-3.5" />
                  <span v-else class="h-1.5 w-1.5 rounded-full bg-current"></span>
                </span>
                <span v-if="i < timeline.length - 1" class="mt-1 h-6 w-px" :class="step.done ? 'bg-success/40' : 'bg-border'"></span>
              </div>
              <div class="min-w-0 pb-1">
                <div class="text-sm font-semibold" :class="step.done ? 'text-foreground' : 'text-muted-foreground'">{{ step.label }}</div>
                <div v-if="step.time" class="mt-0.5 text-xs text-muted-foreground">{{ formatResellerConsoleDate(step.time) }}</div>
              </div>
            </li>
          </ol>
        </Card>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { AlertTriangle, ArrowLeft, Check, ShoppingBag } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import ResellerCopyButton from '../../components/reseller-console/ResellerCopyButton.vue'
import ResellerDataTable from '../../components/reseller-console/ResellerDataTable.vue'
import ResellerHint from '../../components/reseller-console/ResellerHint.vue'
import ResellerPageState from '../../components/reseller-console/ResellerPageState.vue'
import ResellerRecordCard from '../../components/reseller-console/ResellerRecordCard.vue'
import ResellerSectionHeader from '../../components/reseller-console/ResellerSectionHeader.vue'
import ResellerStatusBadge, { type ResellerBadgeTone } from '../../components/reseller-console/ResellerStatusBadge.vue'
import { useResellerOrders } from '../../composables/reseller/useResellerOrders'
import { formatResellerOrderItemSkuText } from '../../utils/resellerOrderDisplay'
import {
  formatResellerConsoleAmount,
  formatResellerConsoleDate,
  resellerOrderStatusTone,
  resellerProfitStatusKey,
} from '../../utils/resellerConsole'

const { t, te, locale } = useI18n()
const route = useRoute()
const { detailLoading, detailError, detail, loadDetail } = useResellerOrders()
const orderNo = computed(() => String(route.params.order_no || ''))

const reload = () => {
  if (orderNo.value) void loadDetail(orderNo.value)
}

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

const timeline = computed(() => {
  const d = detail.value
  if (!d) return []
  const refunded = d.status === 'refunded' || d.status === 'partially_refunded' || d.status === 'canceled'
  const completed = d.status === 'completed' || d.status === 'delivered'
  return [
    { label: t('resellerConsole.orderDetail.tlCreated'), time: d.created_at, done: true },
    { label: t('resellerConsole.orderDetail.tlPaid'), time: d.paid_at || '', done: Boolean(d.paid_at) },
    refunded
      ? { label: t('resellerConsole.orderDetail.tlRefunded'), time: '', done: true }
      : { label: t('resellerConsole.orderDetail.tlCompleted'), time: '', done: completed },
  ]
})

const localized = (value?: Record<string, string>) => {
  if (!value) return '-'
  const loc = locale.value as string
  return value[loc] || value['zh-CN'] || value['zh-TW'] || value['en-US'] || Object.values(value)[0] || '-'
}

const skuText = (item: unknown) => formatResellerOrderItemSkuText(item, {
  locale: locale.value as string,
  fallback: t('productDetail.skuFallback'),
})

onMounted(() => {
  if (orderNo.value) {
    void loadDetail(orderNo.value)
  }
})
</script>
