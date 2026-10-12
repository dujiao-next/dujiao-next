<template>
  <div class="mx-auto w-full max-w-[1100px] px-6 pb-8">
    <div class="my-6">
      <span class="inline-flex items-center gap-2 rounded-full bg-primary/10 px-3 py-1.5 text-xs font-bold uppercase tracking-[0.04em] text-primary"><ClipboardList class="h-4 w-4" /> {{ t('guestOrders.title') }}</span>
      <h1 class="mb-1.5 mt-3 text-[32px] font-extrabold">{{ t('guestOrders.title') }}</h1>
      <p class="text-muted-foreground">{{ t('guestOrders.subtitle') }}</p>
    </div>

    <!-- 查询表单 -->
    <div class="mb-5 luna-panel p-[22px]">
      <div v-if="hasSavedAuth" class="mb-4 flex flex-wrap items-center justify-between gap-3 rounded-md border px-3.5 py-2.5 text-[12.5px] text-muted-foreground">
        <span>{{ t('guestOrders.savedHint', { email: savedAuth.email || '-' }) }}</span>
        <button type="button" class="text-[12.5px] text-muted-foreground transition-colors hover:text-primary" @click="clearSaved">{{ t('guestOrders.clearSaved') }}</button>
      </div>
      <div class="grid gap-3 grid-cols-1 sm:grid-cols-2 lg:grid-cols-[1fr_1fr_1fr_auto]">
        <Input v-model="email" type="email" class="h-11" :placeholder="t('guestOrders.emailPlaceholder')" />
        <Input v-model="orderPassword" type="password" class="h-11" :placeholder="t('guestOrders.passwordPlaceholder')" />
        <Input v-model="orderNo" type="text" class="h-11" :placeholder="t('guestOrders.orderNoPlaceholder')" />
        <Button type="button" class="luna-gradient h-11 rounded-full" :disabled="loading" @click="handleSearch">
          <Search /> {{ loading ? t('guestOrders.searching') : t('guestOrders.search') }}
        </Button>
      </div>
      <p class="mt-3 text-[13px] text-muted-foreground">{{ t('guestOrders.tip') }}</p>
      <div v-if="error" class="mt-3.5 rounded-sm bg-destructive/10 px-3 py-2.5 text-[13px] font-semibold text-destructive">{{ error }}</div>
    </div>

    <!-- 空态 -->
    <div v-if="orders.length === 0 && !loading" class="flex flex-col items-center gap-3 rounded-xl border border-dashed py-16 text-center text-muted-foreground">
      <ClipboardList class="h-10 w-10 opacity-60" />
      <p>{{ emptyMessage }}</p>
    </div>

    <!-- 列表 -->
    <div v-else class="grid grid-cols-1 gap-3.5">
      <div v-for="order in orders" :key="order.order_no" class="flex flex-wrap items-center justify-between gap-4 rounded-lg border bg-card px-5 py-[18px] transition hover:-translate-y-0.5 hover:border-hairline-strong hover:shadow-[var(--luna-shadow)]">
        <div class="grid grid-cols-1 min-w-0 gap-1.5">
          <div class="text-[11px] uppercase tracking-[0.06em] text-muted-foreground">{{ t('orders.orderNo') }}：{{ order.order_no }}</div>
          <div class="text-xl font-extrabold tabular-nums">{{ formatMoney(order.total_amount, order.currency) }}</div>
          <div v-if="hasDiscount(order)" class="flex flex-wrap gap-2.5 text-xs">
            <span v-if="hasDiscountAmount(order.discount_amount)" class="text-destructive">{{ t('orderDetail.couponDiscountLabel') }}：{{ formatDiscountMoney(order.discount_amount, order.currency) }}</span>
            <span v-if="hasDiscountAmount(order.promotion_discount_amount)" class="text-destructive">{{ t('orderDetail.promotionDiscountLabel') }}：{{ formatDiscountMoney(order.promotion_discount_amount, order.currency) }}</span>
          </div>
          <div class="text-[13px] text-muted-foreground">{{ formatDate(order.created_at) }}</div>
        </div>
        <div class="flex flex-wrap items-center gap-2.5">
          <Badge :variant="statusVariant(order.status)" class="rounded-full">{{ statusLabel(order.status) }}</Badge>
          <RouterLink
            :to="{ name: 'guest-order-detail', params: { order_no: order.order_no } }"
            :class="buttonVariants({ variant: 'outline', size: 'sm' })"
            class="rounded-full"
          >
            <Eye /> {{ t('guestOrders.viewDetails') }}
          </RouterLink>
          <Button v-if="order.status === 'pending_payment'" as-child size="sm" class="rounded-full">
            <RouterLink :to="`/pay?guest=1&order_no=${order.order_no}`"><CreditCard /> {{ t('guestOrders.payNow') }}</RouterLink>
          </Button>
        </div>
      </div>

      <div v-if="pagination.total_page > 1" class="mt-2 flex items-center justify-center gap-3.5">
        <Button type="button" variant="outline" size="icon" class="rounded-full" :disabled="loading || pagination.page <= 1" @click="changePage(pagination.page - 1)"><ChevronLeft /></Button>
        <span class="text-sm font-bold">{{ pagination.page }} / {{ pagination.total_page }}</span>
        <Button type="button" variant="outline" size="icon" class="rounded-full" :disabled="loading || pagination.page >= pagination.total_page" @click="changePage(pagination.page + 1)"><ChevronRight /></Button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ChevronLeft, ChevronRight, ClipboardList, Search, Eye, CreditCard } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button, buttonVariants } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useGuestOrders } from '../../composables/useGuestOrders'

const { t } = useI18n()

const {
  savedAuth, email, orderPassword, orderNo, loading, error, orders, pagination,
  hasSavedAuth, clearSaved, handleSearch, emptyMessage, changePage,
  statusLabel, statusVariant, formatMoney, formatDiscountMoney, hasDiscountAmount, hasDiscount, formatDate,
} = useGuestOrders()
</script>
