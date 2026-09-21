<template>
  <div class="mx-auto w-full max-w-[1180px] px-6 pb-8">
    <div class="my-5 flex items-start justify-between gap-4">
      <div>
        <h1 class="mb-1.5 text-3xl font-extrabold">{{ t('guestOrderDetail.title') }}</h1>
        <p class="text-muted-foreground">{{ t('guestOrderDetail.subtitle') }}</p>
      </div>
      <RouterLink class="text-[13px] font-semibold text-muted-foreground transition-colors hover:text-primary" to="/guest/orders">{{ t('guestOrderDetail.backSearch') }}</RouterLink>
    </div>

    <!-- 游客验证 -->
    <div v-if="viewState === 'auth'" class="mb-[18px] rounded-xl border bg-card p-[22px]">
      <h2 class="mb-1.5 text-lg font-bold">{{ t('guestOrderDetail.authTitle') }}</h2>
      <p class="mb-3.5 text-[13px] text-muted-foreground">{{ t('guestOrderDetail.authHint') }}</p>
      <div class="grid gap-3 sm:grid-cols-2">
        <Input v-model="auth.email" type="email" class="h-11" :placeholder="t('guestOrders.emailPlaceholder')" />
        <Input v-model="auth.order_password" type="password" class="h-11" :placeholder="t('guestOrders.passwordPlaceholder')" />
      </div>
      <div v-if="authError" class="mt-3.5 rounded-sm bg-destructive/10 px-3 py-2.5 text-[13px] font-semibold text-destructive">{{ authError }}</div>
      <div class="mt-3.5 flex flex-wrap gap-3">
        <Button type="button" size="sm" class="rounded-full" @click="handleAuthSubmit">{{ t('guestOrderDetail.authSubmit') }}</Button>
        <Button type="button" variant="ghost" size="sm" class="rounded-full" @click="clearAuth">{{ t('guestOrderDetail.authClear') }}</Button>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="viewState === 'loading'" class="rounded-xl border bg-card p-[22px]">
      <div class="mb-4 h-5 w-[35%] rounded bg-secondary"></div>
      <div class="h-[200px] rounded-md bg-secondary"></div>
    </div>

    <!-- 不存在 -->
    <div v-else-if="viewState === 'empty'" class="my-6 flex flex-col items-center gap-3 rounded-xl border border-dashed py-16 text-center text-muted-foreground">
      <AlertCircle class="h-10 w-10 opacity-60" />
      <p>{{ t('guestOrderDetail.notFound') }}</p>
    </div>

    <template v-else-if="viewState === 'detail' && order">
      <div class="mb-[18px] flex flex-wrap items-start justify-between gap-[18px] rounded-xl border bg-card p-[22px]">
        <div>
          <div class="text-[11px] uppercase tracking-[0.06em] text-muted-foreground">{{ t('orders.orderNo') }}</div>
          <div class="mt-1 font-bold">{{ order.order_no }}</div>
          <div class="mt-1.5 text-[13px] text-muted-foreground">{{ t('orderDetail.createdAtLabel') }}：{{ formatDate(order.created_at) }}</div>
        </div>
        <div>
          <div class="text-[11px] uppercase tracking-[0.06em] text-muted-foreground">{{ t('orderDetail.amountTotal') }}</div>
          <div class="mt-1 text-2xl font-extrabold tabular-nums">{{ formatMoney(order.total_amount, order.currency) }}</div>
        </div>
        <div class="flex flex-wrap items-center gap-2.5">
          <Badge :variant="statusVariant(order.status)" class="rounded-full">{{ statusLabel(order.status) }}</Badge>
          <Button v-if="order.status === 'pending_payment'" as-child size="sm" class="rounded-full">
            <RouterLink :to="`/pay?guest=1&order_no=${order.order_no}`">{{ t('orders.payNow') }}</RouterLink>
          </Button>
          <Button v-if="order.status === 'pending_payment'" as-child variant="outline" size="sm" class="rounded-full">
            <RouterLink :to="`/pay?guest=1&change_method=1&order_no=${order.order_no}`">{{ t('payment.changeMethod') }}</RouterLink>
          </Button>
          <Button v-if="order.status === 'pending_payment'" type="button" variant="outline" size="sm" class="rounded-full border-destructive text-destructive hover:bg-destructive/10" @click="cancelOrder">{{ t('orderDetail.cancel') }}</Button>
        </div>
      </div>

      <VaultOrderBody
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
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import VaultOrderBody from './components/VaultOrderBody.vue'
import { useGuestOrderDetail } from '../../composables/useGuestOrderDetail'

const { t } = useI18n()

const {
  order, authError, auth, viewState, handleAuthSubmit, clearAuth, cancelOrder,
  fulfillmentDownloading, handleDownloadFulfillment,
  statusLabel, statusVariant, formatDate, formatMoney,
} = useGuestOrderDetail()
</script>
