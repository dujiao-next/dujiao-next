<template>
  <div class="mx-auto w-full max-w-[880px] px-4 pb-10 sm:px-6">
    <nav class="flex flex-wrap items-center gap-1.5 py-5 text-[13px] text-muted-foreground">
      <RouterLink class="transition-colors hover:text-foreground" to="/">{{ t('nav.home') }}</RouterLink>
      <ChevronRight class="h-3.5 w-3.5 flex-none opacity-60" />
      <RouterLink class="transition-colors hover:text-foreground" to="/me/orders">{{ t('orders.title') }}</RouterLink>
      <ChevronRight class="h-3.5 w-3.5 flex-none opacity-60" />
      <span class="text-foreground">{{ t('orderDetail.title') }}</span>
    </nav>

    <header class="mb-6 flex flex-col gap-2">
      <h1 class="text-[clamp(1.75rem,1.3rem+1.9vw,2.6rem)] font-semibold tracking-[-0.022em]">{{ t('orderDetail.title') }}</h1>
      <p class="text-[15px] text-muted-foreground">{{ t('orderDetail.subtitle') }}</p>
    </header>

    <!-- 加载 -->
    <div v-if="loading" class="flex flex-col gap-4">
      <div class="h-[140px] rounded-[var(--r-lg)] border border-border bg-card"></div>
      <div class="h-[200px] rounded-[var(--r-lg)] border border-border bg-card"></div>
    </div>

    <!-- 不存在 -->
    <div v-else-if="!order" class="flex flex-col items-center gap-3 rounded-[var(--r-lg)] border border-dashed border-border py-16 text-center">
      <AlertCircle class="h-11 w-11 text-muted-foreground" />
      <p class="text-[14px] text-muted-foreground">{{ t('orderDetail.notFound') }}</p>
      <Button size="sm" class="mt-2 rounded-full" @click="debouncedLoadOrder()">{{ t('errorBoundary.retry') }}</Button>
    </div>

    <!-- 详情 -->
    <template v-else>
      <div class="mb-4 flex flex-wrap items-start justify-between gap-5 rounded-[var(--r-lg)] border border-border bg-card p-5">
        <div>
          <div class="text-[12px] text-muted-foreground">{{ t('orders.orderNo') }}</div>
          <div class="mt-1 font-mono text-[14px] font-medium">{{ order.order_no }}</div>
          <div class="mt-1.5 text-[12.5px] text-muted-foreground">{{ t('orderDetail.createdAtLabel') }}：{{ formatDate(order.created_at) }}</div>
        </div>

        <div>
          <div class="text-[12px] text-muted-foreground">{{ t('orderDetail.amountTotal') }}</div>
          <div class="mt-1 text-[24px] font-semibold tabular-nums tracking-[-0.03em]">
            {{ formatMoney(order.total_amount, order.currency) }}
          </div>
        </div>

        <div class="flex flex-wrap items-center gap-2.5">
          <span class="ember-pill" :class="pillClass(order.status)">{{ statusLabel(order.status) }}</span>

          <Button v-if="order.status === 'pending_payment'" as-child size="sm" class="rounded-full">
            <RouterLink :to="`/pay?order_no=${order.order_no}`">{{ t('orderDetail.payNow') }}</RouterLink>
          </Button>
          <Button
            v-if="order.status === 'pending_payment'"
            type="button"
            variant="outline"
            size="sm"
            class="rounded-full border-[color:var(--danger)] text-[color:var(--danger-strong)] hover:bg-[color:var(--danger-soft)]"
            @click="cancelOrder"
          >{{ t('orderDetail.cancel') }}</Button>
        </div>
      </div>

      <EmberOrderBody
        :order="order"
        variant="member"
        :fulfillment-downloading="fulfillmentDownloading"
        @download="handleDownloadFulfillment"
      />
    </template>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { AlertCircle, ChevronRight } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import EmberOrderBody from './components/EmberOrderBody.vue'
import { useOrderDetail } from '../../composables/useOrderDetail'

const { t } = useI18n()

const {
  loading, order, debouncedLoadOrder, cancelOrder,
  fulfillmentDownloading, handleDownloadFulfillment,
  statusLabel, statusVariant, formatDate, formatMoney,
} = useOrderDetail()

/** 状态徽标配色 */
const pillClass = (status?: string) => {
  const map: Record<string, string> = {
    success: 'ember-pill--success',
    warning: 'ember-pill--warning',
    danger: 'ember-pill--danger',
    info: 'ember-pill--accent',
    accent: 'ember-pill--accent',
    neutral: '',
  }
  return map[statusVariant(status ?? '')] || ''
}
</script>
