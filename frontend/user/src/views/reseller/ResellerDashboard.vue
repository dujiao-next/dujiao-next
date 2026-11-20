<template>
  <div class="space-y-6">
    <ResellerSectionHeader
      :title="t('resellerConsole.dashboard.title')"
      :description="t('resellerConsole.dashboard.description')"
    >
      <template #actions>
        <Button v-if="primaryDomainUrl" as-child variant="outline" size="sm">
          <a :href="primaryDomainUrl" target="_blank" rel="noopener noreferrer">
            <ExternalLink class="h-4 w-4" />
            {{ t('resellerConsole.dashboard.visitStore') }}
          </a>
        </Button>
        <Button type="button" variant="outline" size="sm" @click="initialize">
          <RotateCw class="h-4 w-4" />
          {{ t('orders.filters.refresh') }}
        </Button>
      </template>
    </ResellerSectionHeader>

    <ResellerPageState v-if="profileLoading || dashboardLoading || ordersLoading || setupLoading" loading :title="t('resellerConsole.common.loading')" />

    <template v-else-if="profileState.profileStatus !== 'active'">
      <Card class="overflow-hidden p-6 sm:p-8">
        <div class="mx-auto max-w-xl text-center">
          <span class="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-primary/10 text-primary">
            <Rocket class="h-7 w-7" />
          </span>
          <h2 class="mt-4 text-xl font-bold text-foreground">{{ t('resellerConsole.dashboard.inactiveTitle') }}</h2>
          <p class="mt-2 text-sm text-muted-foreground">{{ t('resellerConsole.dashboard.inactiveDescription') }}</p>
          <ul class="mx-auto mt-5 max-w-md space-y-2 text-left">
            <li v-for="(benefit, i) in benefits" :key="i" class="flex items-start gap-2 text-sm text-foreground">
              <CheckCircle2 class="mt-0.5 h-5 w-5 shrink-0 text-success" />
              <span>{{ benefit }}</span>
            </li>
          </ul>
          <Button as-child class="mt-6">
            <RouterLink to="/reseller/apply">{{ t('resellerConsole.nav.apply') }}</RouterLink>
          </Button>
        </div>
      </Card>
    </template>

    <template v-else>
      <!-- 概览头 -->
      <Card class="p-5">
        <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div class="min-w-0">
            <p class="text-xs font-semibold text-primary">{{ t('resellerConsole.dashboard.storeLabel') }}</p>
            <h2 class="mt-1 break-all tabular-nums text-lg font-bold text-foreground">{{ primaryDomain || t('resellerConsole.dashboard.noDomain') }}</h2>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <ResellerStatusBadge :label="settlementText" tone="info" dot />
            <ResellerStatusBadge :label="t('resellerConsole.dashboard.statusActive')" tone="success" dot />
          </div>
        </div>
      </Card>

      <!-- 指标卡 -->
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <ResellerMetricCard :label="t('resellerConsole.dashboard.orderTotal')" :value="stats?.total || 0" :icon="ShoppingBag" tone="accent" />
        <ResellerMetricCard :label="t('resellerConsole.orders.paidOrders')" :value="paidCount" :hint="paidRatioText" :icon="CheckCircle2" tone="success" />
        <ResellerMetricCard :label="t('personalCenter.reseller.primaryAvailable')" :value="primaryBalanceText" :hint="primaryBalanceHint" :icon="Banknote" tone="info" />
        <ResellerMetricCard :label="t('resellerConsole.dashboard.domainCount')" :value="profileSnapshot?.domains?.length || 0" :icon="Globe" tone="neutral" />
      </div>

      <!-- 开店清单 -->
      <Card class="p-5">
        <div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h3 class="text-sm font-bold text-foreground">{{ t('resellerConsole.dashboard.setup.title') }}</h3>
            <p class="mt-1 text-sm text-muted-foreground">
              {{ allSetupDone ? t('resellerConsole.dashboard.setup.allDone') : t('resellerConsole.dashboard.setup.description') }}
            </p>
          </div>
          <div class="flex items-center gap-2 sm:shrink-0">
            <ResellerStatusBadge :label="`${completedSetupCount}/${setupChecklist.length}`" :tone="allSetupDone ? 'success' : 'accent'" />
            <Button
              v-if="allSetupDone"
              type="button"
              variant="ghost"
              size="sm"
              class="gap-1"
              @click="setupCollapsed = !setupCollapsed"
            >
              {{ setupCollapsed ? t('resellerConsole.dashboard.setup.expand') : t('resellerConsole.dashboard.setup.collapse') }}
              <ChevronDown class="h-4 w-4 transition-transform" :class="setupCollapsed ? '' : 'rotate-180'" />
            </Button>
          </div>
        </div>
        <div v-show="!setupCollapsed" class="mt-4 grid gap-3 lg:grid-cols-5">
          <RouterLink
            v-for="item in setupChecklist"
            :key="item.to"
            :to="item.to"
            class="group flex min-h-[124px] flex-col justify-between rounded-xl border bg-card p-4 transition-shadow hover:shadow-md"
          >
            <div class="flex items-start justify-between gap-3">
              <span class="flex h-9 w-9 items-center justify-center rounded-xl" :class="item.done ? 'bg-success/10 text-success' : 'bg-primary/10 text-primary'">
                <component :is="item.icon" class="h-5 w-5" />
              </span>
              <ResellerStatusBadge :label="item.done ? t('resellerConsole.dashboard.setup.ready') : t('resellerConsole.dashboard.setup.pending')" :tone="item.done ? 'success' : 'warning'" />
            </div>
            <div>
              <div class="mt-4 text-sm font-semibold text-foreground">{{ item.label }}</div>
              <p class="mt-1 text-xs text-muted-foreground">{{ item.description }}</p>
            </div>
          </RouterLink>
        </div>
      </Card>

      <!-- 可视化:订单状态分布 + 多币种余额 -->
      <div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <Card class="p-5">
          <h3 class="mb-4 text-sm font-bold text-foreground">{{ t('resellerConsole.dashboard.orderDistribution') }}</h3>
          <ResellerDonut
            v-if="statusSegments.length"
            :segments="statusSegments"
            :center-value="stats?.total || 0"
            :center-label="t('resellerConsole.dashboard.orderTotal')"
          />
          <p v-else class="py-6 text-center text-sm text-muted-foreground">{{ t('resellerConsole.orders.empty') }}</p>
        </Card>

        <Card class="p-5">
          <h3 class="mb-4 text-sm font-bold text-foreground">{{ t('resellerConsole.dashboard.balanceOverview') }}</h3>
          <ResellerDonut
            v-if="balanceSegments.length"
            :segments="balanceSegments"
            :center-value="balances.length"
            :center-label="t('personalCenter.reseller.currencyCount')"
          />
          <p v-else class="py-6 text-center text-sm text-muted-foreground">{{ t('personalCenter.reseller.balanceEmpty') }}</p>
        </Card>
      </div>

      <!-- 快捷入口 -->
      <section>
        <h3 class="mb-3 text-sm font-bold text-foreground">{{ t('resellerConsole.dashboard.quickActions') }}</h3>
        <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
          <RouterLink
            v-for="action in quickActions"
            :key="action.to"
            :to="action.to"
            class="group flex items-center gap-3 rounded-xl border bg-card p-4 shadow-sm transition-shadow hover:shadow-md"
          >
            <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary transition-transform group-hover:scale-105">
              <component :is="action.icon" class="h-5 w-5" />
            </span>
            <span class="min-w-0 text-sm font-semibold text-foreground">{{ action.label }}</span>
          </RouterLink>
        </div>
      </section>

      <!-- 最近订单 -->
      <Card class="p-5">
        <div class="mb-4 flex items-center justify-between gap-3">
          <h3 class="text-base font-bold text-foreground">{{ t('resellerConsole.orders.title') }}</h3>
          <RouterLink to="/reseller/orders" class="inline-flex items-center gap-1 text-sm font-semibold text-primary hover:underline">
            {{ t('resellerConsole.dashboard.viewOrders') }}
            <ChevronRight class="h-4 w-4" />
          </RouterLink>
        </div>

        <ResellerPageState v-if="recentOrders.length === 0" :title="t('resellerConsole.orders.empty')" :icon="ShoppingBag" />

        <template v-else>
          <!-- 桌面表 -->
          <ResellerDataTable class="hidden lg:block">
            <TableHeader>
              <TableRow class="bg-muted/50">
                <TableHead class="px-4">{{ t('resellerConsole.orders.orderNo') }}</TableHead>
                <TableHead class="px-4">{{ t('resellerConsole.orders.status') }}</TableHead>
                <TableHead class="px-4 text-right">{{ t('resellerConsole.orders.totalAmount') }}</TableHead>
                <TableHead class="px-4 text-right">{{ t('resellerConsole.orders.profitAmount') }}</TableHead>
                <TableHead class="px-4">{{ t('resellerConsole.orders.createdAt') }}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="row in recentOrders" :key="row.order_no">
                <TableCell class="px-4 py-3 tabular-nums text-xs text-foreground">{{ row.order_no }}</TableCell>
                <TableCell class="px-4 py-3"><ResellerStatusBadge :label="statusLabel(row.status)" :tone="statusTone(row.status)" /></TableCell>
                <TableCell class="px-4 py-3 text-right tabular-nums text-xs">{{ formatResellerConsoleAmount(row.total_amount, row.currency) }}</TableCell>
                <TableCell class="px-4 py-3 text-right tabular-nums text-xs">{{ formatResellerConsoleAmount(row.profit_amount, row.currency) }}</TableCell>
                <TableCell class="px-4 py-3 text-xs text-muted-foreground">{{ formatResellerConsoleDate(row.created_at) }}</TableCell>
              </TableRow>
            </TableBody>
          </ResellerDataTable>

          <!-- 移动卡片 -->
          <div class="space-y-3 lg:hidden">
            <RouterLink
              v-for="row in recentOrders"
              :key="row.order_no"
              :to="`/reseller/orders/${encodeURIComponent(row.order_no)}`"
              class="block"
            >
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
              </ResellerRecordCard>
            </RouterLink>
          </div>
        </template>
      </Card>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Banknote,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  ExternalLink,
  Globe,
  Palette,
  Rocket,
  RotateCw,
  Settings,
  ShoppingBag,
  Tag,
  Upload,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { resellerAPI } from '../../api'
import type { ResellerDomainData, ResellerSiteConfigSnapshotData } from '../../api/types'
import ResellerDataTable from '../../components/reseller-console/ResellerDataTable.vue'
import ResellerDonut from '../../components/reseller-console/ResellerDonut.vue'
import ResellerMetricCard from '../../components/reseller-console/ResellerMetricCard.vue'
import ResellerPageState from '../../components/reseller-console/ResellerPageState.vue'
import ResellerRecordCard from '../../components/reseller-console/ResellerRecordCard.vue'
import ResellerSectionHeader from '../../components/reseller-console/ResellerSectionHeader.vue'
import ResellerStatusBadge, { type ResellerBadgeTone } from '../../components/reseller-console/ResellerStatusBadge.vue'
import { useResellerFinance } from '../../composables/reseller/useResellerFinance'
import { useResellerOrders } from '../../composables/reseller/useResellerOrders'
import { useResellerProfile } from '../../composables/reseller/useResellerProfile'
import {
  RESELLER_DOMAIN_STATUS_ACTIVE,
  RESELLER_DOMAIN_VERIFICATION_VERIFIED,
} from '../../constants/reseller'
import {
  formatResellerConsoleAmount,
  formatResellerConsoleDate,
  resellerCurrencyColor,
  resellerOrderStatusColor,
  resellerOrderStatusTone,
} from '../../utils/resellerConsole'
import { pickPrimaryResellerBalance } from '../../utils/resellerFinance'

const { t, te } = useI18n()
const { loading: profileLoading, snapshot: profileSnapshot, state: profileState, load: loadProfile } = useResellerProfile()
const finance = useResellerFinance()
const { dashboardLoading, dashboard, balances, loadDashboard, loadBalances } = finance
const { loading: ordersLoading, rows: recentOrders, stats, load: loadOrders, loadStats } = useResellerOrders()
const setupLoading = ref(false)
const siteSnapshot = ref<ResellerSiteConfigSnapshotData | null>(null)
const configuredProductCount = ref(0)

const isActiveVerifiedDomain = (domain: ResellerDomainData) =>
  domain.status === RESELLER_DOMAIN_STATUS_ACTIVE &&
  domain.verification_status === RESELLER_DOMAIN_VERIFICATION_VERIFIED

const primaryDomainObj = computed(() => {
  const domains = profileSnapshot.value?.domains || []
  const activeDomains = domains.filter(isActiveVerifiedDomain)
  return activeDomains.find((d) => d.is_primary) || activeDomains[0] || null
})
const primaryDomain = computed(() => primaryDomainObj.value?.domain || '')
const primaryDomainUrl = computed(() => (primaryDomain.value ? `https://${primaryDomain.value}` : ''))

const settlementText = computed(() => {
  const status = profileSnapshot.value?.profile?.settlement_status
  const key = `personalCenter.reseller.settlementStatusMap.${status}`
  return status && te(key) ? t(key) : t('personalCenter.reseller.settlementStatus')
})

const benefits = computed(() => [
  t('resellerConsole.dashboard.benefit1'),
  t('resellerConsole.dashboard.benefit2'),
  t('resellerConsole.dashboard.benefit3'),
])

const paidCount = computed(() => stats.value?.by_status?.paid || 0)
const paidRatioText = computed(() => {
  const total = stats.value?.total || 0
  if (total <= 0) return ''
  return `${((paidCount.value / total) * 100).toFixed(0)}%`
})

const balanceList = computed(() => (balances.value.length ? balances.value : dashboard.value?.balances || []))
const primaryBalance = computed(() => pickPrimaryResellerBalance(balanceList.value))
const primaryBalanceText = computed(() =>
  primaryBalance.value
    ? formatResellerConsoleAmount(primaryBalance.value.available_amount, primaryBalance.value.currency)
    : '-',
)
const primaryBalanceHint = computed(() =>
  balanceList.value.length > 1
    ? t('personalCenter.reseller.moreCurrencies', { count: balanceList.value.length - 1 })
    : '',
)

const statusSegments = computed(() => {
  const byStatus = stats.value?.by_status || {}
  return Object.entries(byStatus)
    .filter(([, count]) => count > 0)
    .map(([status, count]) => ({ value: count, color: resellerOrderStatusColor(status), label: statusLabel(status) }))
})

const balanceSegments = computed(() =>
  balances.value
    .map((b, i) => ({ value: Number(b.available_amount) || 0, color: resellerCurrencyColor(i), label: b.currency }))
    .filter((s) => s.value > 0),
)

const siteBrandConfigured = computed(() => {
  const cfg = siteSnapshot.value?.config
  return Boolean(cfg?.site_name || cfg?.logo || cfg?.favicon)
})

const quickActions = computed(() => [
  { to: '/reseller/withdraws', label: t('resellerConsole.nav.withdraws'), icon: Upload },
  { to: '/reseller/domains', label: t('resellerConsole.nav.domains'), icon: Globe },
  { to: '/reseller/products', label: t('resellerConsole.nav.products'), icon: Tag },
  { to: '/reseller/site', label: t('resellerConsole.nav.site'), icon: Settings },
])

const setupChecklist = computed(() => [
  {
    to: '/reseller/apply',
    label: t('resellerConsole.dashboard.setup.profileEnabled'),
    description: settlementText.value,
    icon: CheckCircle2,
    done: profileState.value.profileStatus === 'active',
  },
  {
    to: '/reseller/domains',
    label: t('resellerConsole.dashboard.setup.primaryDomain'),
    description: primaryDomain.value || t('resellerConsole.dashboard.setup.primaryDomainEmpty'),
    icon: Globe,
    done: Boolean(primaryDomain.value),
  },
  {
    to: '/reseller/site',
    label: t('resellerConsole.dashboard.setup.siteBrand'),
    description: t('resellerConsole.dashboard.setup.siteBrandDescription'),
    icon: Palette,
    done: siteBrandConfigured.value,
  },
  {
    to: '/reseller/products',
    label: t('resellerConsole.dashboard.setup.productRules'),
    description: t('resellerConsole.dashboard.setup.productRulesDescription'),
    icon: Tag,
    done: configuredProductCount.value > 0,
  },
  {
    to: '/reseller/orders',
    label: t('resellerConsole.dashboard.setup.firstOrder'),
    description: stats.value?.total
      ? t('resellerConsole.dashboard.setup.firstOrderCount', { count: stats.value.total })
      : t('resellerConsole.dashboard.setup.firstOrderDescription'),
    icon: ShoppingBag,
    done: (stats.value?.total || 0) > 0,
  },
])

const completedSetupCount = computed(() => setupChecklist.value.filter((item) => item.done).length)
const allSetupDone = computed(() => setupChecklist.value.length > 0 && completedSetupCount.value === setupChecklist.value.length)
const setupCollapsed = ref(false)

const statusLabel = (status?: string) => {
  if (!status) return '-'
  const key = `resellerConsole.orders.statusMap.${status}`
  return te(key) ? t(key) : status
}
const statusTone = (status?: string) => resellerOrderStatusTone(status) as ResellerBadgeTone

const loadSetupState = async () => {
  setupLoading.value = true
  try {
    const [siteResponse, productResponse] = await Promise.all([
      resellerAPI.siteConfig(),
      resellerAPI.productSettings({ configured: 'configured', page: 1, page_size: 1 }),
    ])
    siteSnapshot.value = siteResponse.data.data || null
    configuredProductCount.value = Number(productResponse.data.pagination?.total || 0)
  } catch {
    // 引导清单为辅助信息，失败时静默降级，不阻塞工作台其它区段。
    siteSnapshot.value = null
    configuredProductCount.value = 0
  } finally {
    setupLoading.value = false
  }
}

const initialize = async () => {
  await loadProfile()
  if (profileState.value.profileStatus !== 'active') return
  await Promise.all([loadDashboard(), loadBalances(), loadStats(), loadOrders({ page: 1, page_size: 5 }), loadSetupState()])
  setupCollapsed.value = allSetupDone.value
}

onMounted(initialize)
</script>
