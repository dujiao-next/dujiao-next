<template>
  <div class="space-y-4">
    <PanelHeading :title="t('orders.title')" :description="t('orders.subtitle')" :icon="ShoppingBag">
      <template #actions>
        <Badge variant="neutral" size="sm" class="rounded-full">
          {{ t('orders.pageInfo', { page: activePagination.page, total: activePagination.total_page }) }}
        </Badge>
        <Button as-child variant="ghost" size="sm" class="rounded-full">
          <router-link to="/products">{{ t('orders.continueShopping') }}</router-link>
        </Button>
      </template>
    </PanelHeading>

    <!-- Tab 切换 -->
    <div class="flex rounded-xl border bg-card overflow-hidden">
      <button
        type="button"
        class="flex-1 py-3 text-sm font-semibold text-center transition-colors"
        :class="activeTab === 'product' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground'"
        @click="switchTab('product')"
      >
        {{ t('orders.tabs.product') }}
      </button>
      <button
        type="button"
        class="flex-1 py-3 text-sm font-semibold text-center transition-colors"
        :class="activeTab === 'recharge' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground'"
        @click="switchTab('recharge')"
      >
        {{ t('orders.tabs.recharge') }}
      </button>
    </div>

    <!-- 普通订单 Tab -->
    <template v-if="activeTab === 'product'">
      <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <StatCard :label="t('orders.stats.totalMatched')" :value="orderPagination.total" :icon="ShoppingBag" tone="info" mono />
        <StatCard :label="t('orders.stats.currentPage')" :value="orders.length" :icon="Layers" tone="neutral" mono />
        <StatCard :label="t('orders.stats.pendingPayment')" :value="pendingPaymentCount" :icon="Clock" tone="warning" mono />
        <StatCard :label="t('orders.stats.finished')" :value="finishedCount" :icon="CheckCircle2" tone="success" mono />
      </div>

      <div class="rounded-2xl border bg-card p-4 shadow-sm">
        <div class="flex flex-col gap-3 lg:flex-row lg:items-end">
          <div class="w-full lg:max-w-sm">
            <Label class="mb-1 block text-xs font-semibold text-muted-foreground">{{ t('orders.filters.keyword') }}</Label>
            <Input
              v-model="orderFilters.orderNo"
              type="text"
              :placeholder="t('orders.filters.orderNoPlaceholder')"
              class="h-11"
              @input="handleOrderNoInput"
              @keyup.enter="applyOrderFilters"
            />
          </div>

          <div class="w-full lg:w-56">
            <Label class="mb-1 block text-xs font-semibold text-muted-foreground">{{ t('orders.filters.status') }}</Label>
            <Select v-model="orderStatusProxy">
              <SelectTrigger class="h-11 w-full"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem v-for="item in orderStatusOptions" :key="item.value || 'all'" :value="item.value || 'all'">
                  {{ item.label }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div class="flex w-full flex-wrap items-center gap-2 lg:w-auto">
            <Button type="button" class="h-11 font-bold" @click="applyOrderFilters">
              {{ t('orders.filters.search') }}
            </Button>
            <Button type="button" variant="outline" class="h-11 font-semibold" @click="resetOrderFilters">
              {{ t('orders.filters.reset') }}
            </Button>
            <Button type="button" variant="outline" class="h-11 font-semibold" @click="refreshOrdersCurrentPage">
              {{ t('orders.filters.refresh') }}
            </Button>
          </div>
        </div>

        <div v-if="hasOrderActiveFilters" class="mt-3 text-xs text-muted-foreground">
          {{ t('orders.filters.current', { orderNo: orderFilters.orderNo || t('orders.filters.any'), status: currentOrderStatusLabel }) }}
        </div>
      </div>

      <div v-if="orderLoading" class="space-y-4">
        <div v-for="i in 3" :key="i" class="h-24 animate-pulse rounded-2xl border bg-muted"></div>
      </div>

      <EmptyState
        v-else-if="orders.length === 0"
        icon="order"
        :description="hasOrderActiveFilters ? t('orders.emptyFiltered') : t('orders.empty')"
        :action-label="t('orders.emptyAction')"
        action-to="/products"
      />

      <div v-else class="space-y-4">
        <div
          v-for="order in orders"
          :key="order.order_no"
          class="rounded-2xl border bg-card p-6 shadow-sm transition-all transition hover:-translate-y-0.5 hover:border-primary/30 hover:shadow-md"
        >
          <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <div class="text-xs uppercase tracking-[0.16em] text-muted-foreground">{{ t('orders.orderNo') }}：{{ order.order_no }}</div>
              <div class="mt-2 text-lg font-bold text-foreground">{{ formatMoney(order.total_amount, order.currency) }}</div>
              <div v-if="hasDiscount(order)" class="mt-2 flex flex-wrap gap-2 text-xs text-muted-foreground">
                <Badge v-if="hasDiscountAmount(order.discount_amount)" variant="success" size="sm">
                  {{ t('orderDetail.couponDiscountLabel') }}：{{ formatDiscountMoney(order.discount_amount, order.currency) }}
                </Badge>
                <Badge v-if="hasDiscountAmount(order.promotion_discount_amount)" variant="danger" size="sm">
                  {{ t('orderDetail.promotionDiscountLabel') }}：{{ formatDiscountMoney(order.promotion_discount_amount, order.currency) }}
                </Badge>
              </div>
              <div class="mt-2 text-xs text-muted-foreground">{{ formatDate(order.created_at) }}</div>
            </div>

            <div class="flex flex-wrap items-center gap-3">
              <Badge :variant="statusVariant(order.status)" size="sm">
                {{ statusLabel(order.status) }}
              </Badge>
              <Button as-child variant="outline" size="sm">
                <router-link :to="`/orders/${order.order_no}`">{{ t('orders.viewDetails') }}</router-link>
              </Button>
              <Button v-if="order.status === 'pending_payment'" as-child size="sm">
                <router-link :to="`/pay?order_no=${order.order_no}`">{{ t('orders.payNow') }}</router-link>
              </Button>
            </div>
          </div>
        </div>
      </div>

      <PaginationNav
        :current-page="orderPagination.page"
        :total-pages="orderPagination.total_page"
        :loading="orderLoading"
        :scroll-top="false"
        @change-page="changeOrderPage"
      />
    </template>

    <!-- 充值订单 Tab -->
    <template v-if="activeTab === 'recharge'">
      <div class="grid grid-cols-2 gap-3 lg:grid-cols-3">
        <StatCard :label="t('orders.stats.totalMatched')" :value="rechargePagination.total" :icon="Wallet" tone="info" mono />
        <StatCard :label="t('orders.stats.currentPage')" :value="rechargeOrders.length" :icon="Layers" tone="neutral" mono />
        <StatCard :label="t('orders.stats.pendingPayment')" :value="rechargePendingCount" :icon="Clock" tone="warning" mono />
      </div>

      <div class="rounded-2xl border bg-card p-4 shadow-sm">
        <div class="flex flex-col gap-3 lg:flex-row lg:items-end">
          <div class="w-full lg:max-w-sm">
            <Label class="mb-1 block text-xs font-semibold text-muted-foreground">{{ t('orders.rechargeFilters.keyword') }}</Label>
            <Input
              v-model="rechargeFilters.rechargeNo"
              type="text"
              :placeholder="t('orders.rechargeFilters.rechargeNoPlaceholder')"
              class="h-11"
              @input="handleRechargeNoInput"
              @keyup.enter="applyRechargeFilters"
            />
          </div>

          <div class="w-full lg:w-56">
            <Label class="mb-1 block text-xs font-semibold text-muted-foreground">{{ t('orders.filters.status') }}</Label>
            <Select v-model="rechargeStatusProxy">
              <SelectTrigger class="h-11 w-full"><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem v-for="item in rechargeStatusOptions" :key="item.value || 'all'" :value="item.value || 'all'">
                  {{ item.label }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div class="flex w-full flex-wrap items-center gap-2 lg:w-auto">
            <Button type="button" class="h-11 font-bold" @click="applyRechargeFilters">
              {{ t('orders.filters.search') }}
            </Button>
            <Button type="button" variant="outline" class="h-11 font-semibold" @click="resetRechargeFilters">
              {{ t('orders.filters.reset') }}
            </Button>
            <Button type="button" variant="outline" class="h-11 font-semibold" @click="refreshRechargeCurrentPage">
              {{ t('orders.filters.refresh') }}
            </Button>
          </div>
        </div>
      </div>

      <div v-if="rechargeLoading" class="space-y-4">
        <div v-for="i in 3" :key="i" class="h-24 animate-pulse rounded-2xl border bg-muted"></div>
      </div>

      <EmptyState
        v-else-if="rechargeOrders.length === 0"
        icon="order"
        :description="t('orders.rechargeEmpty')"
        :action-label="t('orders.rechargeEmptyAction')"
        action-to="/me/wallet"
      />

      <div v-else class="space-y-4">
        <div
          v-for="ro in rechargeOrders"
          :key="ro.recharge_no"
          class="rounded-2xl border bg-card p-6 shadow-sm transition-all transition hover:-translate-y-0.5 hover:border-primary/30 hover:shadow-md"
        >
          <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <div class="text-xs uppercase tracking-[0.16em] text-muted-foreground">{{ t('personalCenter.wallet.rechargeNoLabel') }}：{{ ro.recharge_no }}</div>
              <div class="mt-2 text-lg font-bold text-foreground">{{ formatMoney(ro.amount, ro.currency) }}</div>
              <div class="mt-2 text-xs text-muted-foreground">{{ formatDate(ro.created_at) }}</div>
            </div>

            <div class="flex flex-wrap items-center gap-3">
              <Badge :variant="rechargeStatusVariant(ro.status)" size="sm">
                {{ rechargeStatusText(ro.status) }}
              </Badge>
              <Button as-child variant="outline" size="sm">
                <router-link :to="`/recharge-orders/${ro.recharge_no}`">{{ t('orders.viewDetails') }}</router-link>
              </Button>
              <Button v-if="ro.status === 'pending'" as-child size="sm">
                <router-link :to="`/recharge-orders/${ro.recharge_no}`">{{ t('orders.payNow') }}</router-link>
              </Button>
            </div>
          </div>
        </div>
      </div>

      <PaginationNav
        :current-page="rechargePagination.page"
        :total-pages="rechargePagination.total_page"
        :loading="rechargeLoading"
        :scroll-top="false"
        @change-page="changeRechargePage"
      />
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ShoppingBag, Layers, Clock, CheckCircle2, Wallet } from 'lucide-vue-next'
import { userOrderAPI } from '../../api'
import { walletAPI } from '../../api/wallet'
import { orderStatusVariant, orderStatusLabel, type BadgeTone } from '../../utils/status'
import { debounceAsync } from '../../utils/debounce'
import { amountToCents } from '../../utils/money'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Input } from '@/components/ui/input'
import EmptyState from '../../components/EmptyState.vue'
import PaginationNav from '../../components/PaginationNav.vue'
import PanelHeading from '../../components/shared/PanelHeading.vue'
import StatCard from '../../components/shared/StatCard.vue'

const { t } = useI18n()


// ========== Tab 状态 ==========
const activeTab = ref<'product' | 'recharge'>('product')

const switchTab = (tab: 'product' | 'recharge') => {
  if (activeTab.value === tab) return
  activeTab.value = tab
  if (tab === 'product' && !orderLoaded.value) {
    loadOrders(1)
  }
  if (tab === 'recharge' && !rechargeLoaded.value) {
    loadRechargeOrders(1)
  }
}

const activePagination = computed(() =>
  activeTab.value === 'product' ? orderPagination.value : rechargePagination.value,
)

// ========== 普通订单 ==========
const orderLoading = ref(true)
const orderLoaded = ref(false)
const orders = ref<any[]>([])
const orderPagination = ref({ page: 1, page_size: 20, total: 0, total_page: 1 })
const orderFilters = reactive({ orderNo: '', status: '' })
const orderStats = ref<Record<string, number>>({})

const orderStatusOptions = computed(() => [
  { value: '', label: t('orders.filters.statusAll') },
  { value: 'pending_payment', label: t('order.status.pending_payment') },
  { value: 'paid', label: t('order.status.paid') },
  { value: 'fulfilling', label: t('order.status.fulfilling') },
  { value: 'partially_delivered', label: t('order.status.partially_delivered') },
  { value: 'partially_refunded', label: t('order.status.partially_refunded') },
  { value: 'delivered', label: t('order.status.delivered') },
  { value: 'completed', label: t('order.status.completed') },
  { value: 'expired', label: t('order.status.expired') },
  { value: 'canceled', label: t('order.status.canceled') },
  { value: 'refunded', label: t('order.status.refunded') },
])

const hasOrderActiveFilters = computed(() => Boolean(orderFilters.orderNo || orderFilters.status))
const currentOrderStatusLabel = computed(() => {
  const selected = orderStatusOptions.value.find((item) => item.value === orderFilters.status)
  return selected?.label || t('orders.filters.statusAll')
})
const pendingPaymentCount = computed(() => orderStats.value['pending_payment'] || 0)
const finishedCount = computed(
  () =>
    (orderStats.value['delivered'] || 0) +
    (orderStats.value['completed'] || 0) +
    (orderStats.value['partially_refunded'] || 0) +
    (orderStats.value['refunded'] || 0),
)

const loadOrders = async (page = 1) => {
  orderLoading.value = true
  try {
    const response = await userOrderAPI.list({
      page,
      page_size: orderPagination.value.page_size,
      status: orderFilters.status || undefined,
      order_no: orderFilters.orderNo || undefined,
    })
    orders.value = response.data.data || []
    orderPagination.value = response.data.pagination || orderPagination.value
    orderLoaded.value = true
  } catch {
    orders.value = []
  } finally {
    orderLoading.value = false
  }
  loadOrderStats()
}

// 按状态聚合的全量统计（不受分页与状态筛选影响，仅复用关键词筛选）
const loadOrderStats = async () => {
  try {
    const response = await userOrderAPI.stats({ order_no: orderFilters.orderNo || undefined })
    orderStats.value = response.data.data?.by_status || {}
  } catch {
    orderStats.value = {}
  }
}

const debouncedLoadOrders = debounceAsync(loadOrders, 300)

const changeOrderPage = (page: number) => {
  if (page < 1 || page > orderPagination.value.total_page) return
  debouncedLoadOrders(page)
}
const applyOrderFilters = () => loadOrders(1)
const handleOrderNoInput = () => debouncedLoadOrders(1)
const handleOrderStatusChange = () => loadOrders(1)
const orderStatusProxy = computed({
  get: () => orderFilters.status || 'all',
  set: (v: string) => {
    orderFilters.status = v === 'all' ? '' : v
    handleOrderStatusChange()
  },
})
const resetOrderFilters = () => {
  orderFilters.orderNo = ''
  orderFilters.status = ''
  loadOrders(1)
}
const refreshOrdersCurrentPage = () => loadOrders(orderPagination.value.page)

const statusLabel = (status: string) => orderStatusLabel(t, status)
const statusVariant = (status: string) => orderStatusVariant(status)

// ========== 充值订单 ==========
const rechargeLoading = ref(false)
const rechargeLoaded = ref(false)
const rechargeOrders = ref<any[]>([])
const rechargePagination = ref({ page: 1, page_size: 20, total: 0, total_page: 1 })
const rechargeFilters = reactive({ rechargeNo: '', status: '' })
const rechargeStats = ref<Record<string, number>>({})

const rechargeStatusOptions = computed(() => [
  { value: '', label: t('orders.filters.statusAll') },
  { value: 'pending', label: t('personalCenter.wallet.rechargeStatus.pending') },
  { value: 'success', label: t('personalCenter.wallet.rechargeStatus.success') },
  { value: 'partially_refunded', label: t('personalCenter.wallet.rechargeStatus.partially_refunded') },
  { value: 'refunded', label: t('personalCenter.wallet.rechargeStatus.refunded') },
  { value: 'failed', label: t('personalCenter.wallet.rechargeStatus.failed') },
  { value: 'expired', label: t('personalCenter.wallet.rechargeStatus.expired') },
])

const rechargePendingCount = computed(() => rechargeStats.value['pending'] || 0)

const loadRechargeOrders = async (page = 1) => {
  rechargeLoading.value = true
  try {
    const response = await walletAPI.rechargeOrders({
      page,
      page_size: rechargePagination.value.page_size,
      status: rechargeFilters.status || undefined,
      recharge_no: rechargeFilters.rechargeNo || undefined,
    })
    rechargeOrders.value = response.data.data || []
    rechargePagination.value = response.data.pagination || rechargePagination.value
    rechargeLoaded.value = true
  } catch {
    rechargeOrders.value = []
  } finally {
    rechargeLoading.value = false
  }
  loadRechargeStats()
}

// 按状态聚合的全量统计（不受分页与状态筛选影响，仅复用关键词筛选）
const loadRechargeStats = async () => {
  try {
    const response = await walletAPI.rechargeStats({ recharge_no: rechargeFilters.rechargeNo || undefined })
    rechargeStats.value = response.data.data?.by_status || {}
  } catch {
    rechargeStats.value = {}
  }
}

const debouncedLoadRechargeOrders = debounceAsync(loadRechargeOrders, 300)

const changeRechargePage = (page: number) => {
  if (page < 1 || page > rechargePagination.value.total_page) return
  debouncedLoadRechargeOrders(page)
}
const applyRechargeFilters = () => loadRechargeOrders(1)
const handleRechargeNoInput = () => debouncedLoadRechargeOrders(1)
const handleRechargeStatusChange = () => loadRechargeOrders(1)
const rechargeStatusProxy = computed({
  get: () => rechargeFilters.status || 'all',
  set: (v: string) => {
    rechargeFilters.status = v === 'all' ? '' : v
    handleRechargeStatusChange()
  },
})
const resetRechargeFilters = () => {
  rechargeFilters.rechargeNo = ''
  rechargeFilters.status = ''
  loadRechargeOrders(1)
}
const refreshRechargeCurrentPage = () => loadRechargeOrders(rechargePagination.value.page)

const rechargeStatusText = (status?: string) => {
  const normalized = String(status || '').toLowerCase()
  const key = `personalCenter.wallet.rechargeStatus.${normalized}`
  const translated = t(key)
  if (translated === key) return normalized || '-'
  return translated
}

const rechargeStatusVariant = (status?: string): BadgeTone => {
  const normalized = String(status || '').toLowerCase()
  if (normalized === 'success') return 'success'
  if (normalized === 'refunded') return 'info'
  if (normalized === 'failed' || normalized === 'expired') return 'danger'
  return 'warning'
}

// ========== 共用工具 ==========
const formatMoney = (amount?: string, currency?: string) => {
  if (amount === null || amount === undefined || amount === '') return '-'
  if (currency === null || currency === undefined || currency === '') return String(amount)
  return `${amount} ${currency}`
}

const formatDiscountMoney = (amount?: string, currency?: string) => {
  return hasDiscountAmount(amount) ? `-${formatMoney(amount, currency)}` : formatMoney(amount, currency)
}

const hasDiscountAmount = (amount?: string) => {
  if (amount === null || amount === undefined || amount === '') return false
  const valueCents = amountToCents(amount)
  return valueCents !== null && valueCents > 0
}

const hasDiscount = (order: any) => {
  if (!order) return false
  return hasDiscountAmount(order.discount_amount) || hasDiscountAmount(order.promotion_discount_amount)
}

const formatDate = (raw?: string) => {
  if (!raw) return ''
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return raw
  return date.toLocaleString()
}

onMounted(() => {
  debouncedLoadOrders(1)
})

onUnmounted(() => {
  debouncedLoadOrders.cancel()
  debouncedLoadRechargeOrders.cancel()
})
</script>
