<template>
  <div class="mx-auto w-full max-w-[880px] px-4 pb-10 sm:px-6">
    <div class="my-5 flex flex-wrap items-start justify-between gap-4">
      <div class="flex flex-col gap-2">
        <h1 class="text-[clamp(1.75rem,1.3rem+1.9vw,2.6rem)] font-semibold tracking-[-0.022em]">{{ t('guestOrderDetail.title') }}</h1>
        <p class="text-[15px] text-muted-foreground">{{ t('guestOrderDetail.subtitle') }}</p>
      </div>
      <RouterLink class="text-[13px] text-muted-foreground transition-colors hover:text-foreground" to="/guest/orders">
        {{ t('guestOrderDetail.backSearch') }}
      </RouterLink>
    </div>

    <!-- 身份验证 -->
    <section v-if="viewState === 'auth'" class="rounded-[var(--r-lg)] border border-border bg-card p-5">
      <h2 class="mb-1.5 text-[15px] font-semibold">{{ t('guestOrderDetail.authTitle') }}</h2>
      <p class="mb-4 text-[13px] text-muted-foreground">{{ t('guestOrderDetail.authHint') }}</p>

      <div class="grid gap-3 sm:grid-cols-2">
        <input
          v-model="auth.email"
          type="email"
          class="h-11 rounded-[var(--r)] border border-border bg-[color:var(--surface-2)] px-4 text-[14px] outline-none transition-colors focus:border-[color:var(--accent)] focus:bg-card focus:ring-[var(--ring)]"
          :placeholder="t('guestOrders.emailPlaceholder')"
        />
        <input
          v-model="auth.order_password"
          type="password"
          class="h-11 rounded-[var(--r)] border border-border bg-[color:var(--surface-2)] px-4 text-[14px] outline-none transition-colors focus:border-[color:var(--accent)] focus:bg-card focus:ring-[var(--ring)]"
          :placeholder="t('guestOrders.passwordPlaceholder')"
        />
      </div>

      <p v-if="authError" class="mt-3.5 rounded-[var(--r)] bg-[color:var(--danger-soft)] px-3.5 py-2.5 text-[13px] text-[color:var(--danger-strong)]">
        {{ authError }}
      </p>

      <div class="mt-4 flex flex-wrap gap-3">
        <Button type="button" size="sm" class="rounded-full" @click="handleAuthSubmit">{{ t('guestOrderDetail.authSubmit') }}</Button>
        <Button type="button" variant="ghost" size="sm" class="rounded-full" @click="clearAuth">{{ t('guestOrderDetail.authClear') }}</Button>
      </div>
    </section>

    <!-- 加载 -->
    <div v-else-if="viewState === 'loading'" class="flex flex-col gap-4">
      <div class="h-[140px] rounded-[var(--r-lg)] border border-border bg-card"></div>
      <div class="h-[200px] rounded-[var(--r-lg)] border border-border bg-card"></div>
    </div>

    <!-- 不存在 -->
    <div v-else-if="viewState === 'empty'" class="flex flex-col items-center gap-3 rounded-[var(--r-lg)] border border-dashed border-border py-16 text-center">
      <AlertCircle class="h-11 w-11 text-muted-foreground" />
      <p class="text-[14px] text-muted-foreground">{{ t('guestOrderDetail.notFound') }}</p>
      <Button as-child size="sm" class="mt-2 rounded-full"><RouterLink to="/guest/orders">{{ t('guestOrderDetail.backSearch') }}</RouterLink></Button>
    </div>

    <!-- 详情 -->
    <template v-else-if="viewState === 'detail' && order">
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
            <RouterLink :to="`/pay?guest=1&order_no=${order.order_no}`">{{ t('orders.payNow') }}</RouterLink>
          </Button>
        </div>
      </div>

      <EmberOrderBody
        :order="order"
        variant="guest"
        :fulfillment-downloading="fulfillmentDownloading"
        @download="handleDownloadFulfillment"
      />
    </template>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { AlertCircle } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import EmberOrderBody from './components/EmberOrderBody.vue'
import { useGuestOrderDetail } from '../../composables/useGuestOrderDetail'

const { t } = useI18n()

const {
  order, authError, auth, viewState, handleAuthSubmit, clearAuth,
  fulfillmentDownloading, handleDownloadFulfillment,
  statusLabel, statusVariant, formatDate, formatMoney,
} = useGuestOrderDetail()

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
