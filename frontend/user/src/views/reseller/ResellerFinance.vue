<template>
  <div class="space-y-5">
    <ResellerSectionHeader
      :title="t('resellerConsole.finance.title')"
      :description="t('resellerConsole.finance.description')"
    >
      <template #actions>
        <Button as-child size="sm">
          <RouterLink to="/reseller/withdraws">
            <Upload class="h-4 w-4" />
            {{ t('resellerConsole.nav.withdraws') }}
          </RouterLink>
        </Button>
      </template>
    </ResellerSectionHeader>

    <ResellerPageState v-if="dashboardLoading || balanceLoading" loading :title="t('resellerConsole.common.loading')" />

    <template v-else>
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <ResellerMetricCard :label="t('personalCenter.reseller.primaryAvailable')" :value="primaryBalanceText" :hint="primaryBalanceHint" :icon="Banknote" tone="success" />
        <ResellerMetricCard :label="t('personalCenter.reseller.currencyCount')" :value="balances.length" :icon="Wallet" tone="info" />
        <ResellerMetricCard :label="t('personalCenter.reseller.settlementStatus')" :value="statusText" :icon="BadgeCheck" tone="accent" />
      </div>

      <ResellerPageState v-if="balances.length === 0" :title="t('personalCenter.reseller.balanceEmpty')" :icon="Banknote">
        <Button as-child>
          <RouterLink to="/reseller/products">{{ t('resellerConsole.nav.products') }}</RouterLink>
        </Button>
      </ResellerPageState>

      <template v-else>
        <!-- 可用余额占比 -->
        <Card v-if="donutSegments.length" class="p-5">
          <h3 class="mb-4 text-sm font-bold text-foreground">{{ t('resellerConsole.finance.distribution') }}</h3>
          <ResellerDonut :segments="donutSegments" :center-value="balances.length" :center-label="t('personalCenter.reseller.currencyCount')" />
        </Card>

        <!-- 各币种明细 -->
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Card v-for="item in balances" :key="item.id" class="p-4 sm:p-5">
            <div class="flex items-start justify-between gap-3">
              <div>
                <div class="tabular-nums text-sm font-bold text-foreground">{{ item.currency }}</div>
                <div class="mt-2 text-xs text-muted-foreground">{{ t('personalCenter.reseller.availableAmount') }}</div>
                <div class="mt-1 tabular-nums text-lg font-bold text-foreground">{{ formatResellerConsoleAmount(item.available_amount, item.currency) }}</div>
              </div>
              <div class="flex items-center gap-1.5">
                <ResellerStatusBadge :label="balanceStatusLabel(item.status)" :tone="balanceTone(item.status)" />
                <ResellerHint :text="t('resellerConsole.finance.settlementHint')" />
              </div>
            </div>
            <div class="mt-4">
              <ResellerProgressBar :segments="balanceSegments(item)" />
            </div>
          </Card>
        </div>
      </template>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { BadgeCheck, Banknote, Upload, Wallet } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import ResellerDonut from '../../components/reseller-console/ResellerDonut.vue'
import ResellerHint from '../../components/reseller-console/ResellerHint.vue'
import ResellerMetricCard from '../../components/reseller-console/ResellerMetricCard.vue'
import ResellerPageState from '../../components/reseller-console/ResellerPageState.vue'
import ResellerProgressBar from '../../components/reseller-console/ResellerProgressBar.vue'
import ResellerSectionHeader from '../../components/reseller-console/ResellerSectionHeader.vue'
import ResellerStatusBadge, { type ResellerBadgeTone } from '../../components/reseller-console/ResellerStatusBadge.vue'
import { useResellerFinance } from '../../composables/reseller/useResellerFinance'
import type { ResellerBalanceData } from '../../api'
import {
  RESELLER_BALANCE_STATUS_DISABLED,
  RESELLER_BALANCE_STATUS_FROZEN_REVIEW,
  RESELLER_BALANCE_STATUS_NEGATIVE_BALANCE,
  RESELLER_BALANCE_STATUS_NORMAL,
} from '../../constants/reseller'
import { formatResellerConsoleAmount, resellerCurrencyColor } from '../../utils/resellerConsole'
import { getResellerFinanceStatusView, pickPrimaryResellerBalance } from '../../utils/resellerFinance'

const { t } = useI18n()
const { dashboardLoading, balanceLoading, dashboard, balances, loadDashboard, loadBalances } = useResellerFinance()

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

const statusView = computed(() => getResellerFinanceStatusView(dashboard.value?.profile))
const statusText = computed(() => t(`personalCenter.reseller.${statusView.value.namespace}.${statusView.value.key}`))

const donutSegments = computed(() =>
  balances.value
    .map((b, i) => ({ value: Number(b.available_amount) || 0, color: resellerCurrencyColor(i), label: b.currency }))
    .filter((s) => s.value > 0),
)

const balanceSegments = (item: ResellerBalanceData) => [
  {
    value: Number(item.available_amount) || 0,
    color: '#10b981',
    label: t('personalCenter.reseller.availableAmount'),
    display: formatResellerConsoleAmount(item.available_amount),
  },
  {
    value: Number(item.locked_amount) || 0,
    color: '#f59e0b',
    label: t('personalCenter.reseller.lockedAmount'),
    display: formatResellerConsoleAmount(item.locked_amount),
  },
  {
    value: Number(item.negative_amount) || 0,
    color: '#ef4444',
    label: t('personalCenter.reseller.negativeAmount'),
    display: formatResellerConsoleAmount(item.negative_amount),
  },
]

const balanceStatusLabel = (status?: string) => {
  if (status === RESELLER_BALANCE_STATUS_NORMAL) return t('personalCenter.reseller.balanceStatus.normal')
  if (status === RESELLER_BALANCE_STATUS_NEGATIVE_BALANCE) return t('personalCenter.reseller.balanceStatus.negativeBalance')
  if (status === RESELLER_BALANCE_STATUS_FROZEN_REVIEW) return t('personalCenter.reseller.balanceStatus.frozenReview')
  if (status === RESELLER_BALANCE_STATUS_DISABLED) return t('personalCenter.reseller.balanceStatus.disabled')
  return status || '-'
}

const balanceTone = (status?: string): ResellerBadgeTone => {
  if (status === RESELLER_BALANCE_STATUS_NORMAL) return 'success'
  if (status === RESELLER_BALANCE_STATUS_NEGATIVE_BALANCE || status === RESELLER_BALANCE_STATUS_FROZEN_REVIEW) return 'warning'
  return 'neutral'
}

onMounted(() => {
  void Promise.all([loadDashboard(), loadBalances()])
})
</script>
