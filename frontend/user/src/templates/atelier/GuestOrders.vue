<template>
  <div class="min-h-screen pb-16 pt-[88px] text-foreground">
    <div class="mx-auto w-full max-w-[1240px] px-4 sm:px-6">
      <div class="mb-7 border-b pb-5">
        <h1 class="text-[26px] font-bold tracking-[-0.025em] md:text-[30px]">{{ t('guestOrders.title') }}</h1>
        <p class="mt-1 text-sm text-muted-foreground">{{ t('guestOrders.subtitle') }}</p>
      </div>

      <!-- 查询表单卡 -->
      <section class="atelier-card mb-7 rounded-[16px] bg-card p-5 md:p-6">
        <div
          v-if="hasSavedAuth"
          class="mb-4 flex flex-col justify-between gap-2 rounded-[12px] border bg-background px-4 py-2.5 text-xs text-muted-foreground sm:flex-row sm:items-center"
        >
          <span>{{ t('guestOrders.savedHint', { email: savedAuth.email || '-' }) }}</span>
          <Button type="button" variant="link" class="h-auto p-0 text-xs font-medium text-foreground" @click="clearSaved">
            {{ t('guestOrders.clearSaved') }}
          </Button>
        </div>

        <div class="grid grid-cols-1 gap-3 md:grid-cols-4">
          <Input v-model="email" type="email" class="h-11 rounded-[12px]" :placeholder="t('guestOrders.emailPlaceholder')" @keydown.enter="handleSearch" />
          <Input v-model="orderPassword" type="password" class="h-11 rounded-[12px]" :placeholder="t('guestOrders.passwordPlaceholder')" @keydown.enter="handleSearch" />
          <Input v-model="orderNo" type="text" class="h-11 rounded-[12px]" :placeholder="t('guestOrders.orderNoPlaceholder')" @keydown.enter="handleSearch" />
          <Button
            :disabled="loading"
            class="h-11 rounded-[11px] bg-primary px-6 font-semibold text-primary-foreground shadow-xs hover:bg-primary/90"
            @click="handleSearch"
          >
            <Search class="h-4 w-4" />
            {{ loading ? t('guestOrders.searching') : t('guestOrders.search') }}
          </Button>
        </div>

        <p class="mt-3 text-xs text-muted-foreground">{{ t('guestOrders.tip') }}</p>
        <Alert v-if="error" variant="destructive" class="mt-4">
          <AlertDescription>{{ error }}</AlertDescription>
        </Alert>
      </section>

      <EmptyState
        v-if="orders.length === 0 && !loading"
        icon="order"
        :description="emptyMessage"
      />

      <!-- 精密列表卡片 -->
      <div v-else class="atelier-card divide-y overflow-hidden rounded-[16px] bg-card">
        <article
          v-for="order in orders"
          :key="order.order_no"
          class="flex flex-col justify-between gap-4 p-5 transition-colors hover:bg-background/60 md:flex-row md:items-center md:px-6"
        >
          <div>
            <div class="font-mono text-xs text-muted-foreground">
              {{ t('orders.orderNo') }}：<span class="font-semibold text-foreground">{{ order.order_no }}</span>
            </div>
            <div class="mt-1.5 text-xl font-bold leading-none tracking-[-0.02em] tabular-nums text-foreground">
              {{ formatMoney(order.total_amount, order.currency) }}
            </div>
            <div v-if="hasDiscount(order)" class="mt-1.5 flex flex-wrap gap-2 text-xs text-danger">
              <span v-if="hasDiscountAmount(order.discount_amount)">
                {{ t('orderDetail.couponDiscountLabel') }}：{{ formatDiscountMoney(order.discount_amount, order.currency) }}
              </span>
              <span v-if="hasDiscountAmount(order.promotion_discount_amount)">
                {{ t('orderDetail.promotionDiscountLabel') }}：{{ formatDiscountMoney(order.promotion_discount_amount, order.currency) }}
              </span>
            </div>
            <div class="mt-1 font-mono text-xs text-muted-foreground">{{ formatDate(order.created_at) }}</div>
          </div>

          <div class="flex flex-wrap items-center gap-2.5">
            <Badge :variant="statusVariant(order.status)" size="sm">
              {{ statusLabel(order.status) }}
            </Badge>
            <RouterLink
              :to="{ name: 'guest-order-detail', params: { order_no: order.order_no } }"
              class="theme-pill-link"
            >
              <Eye class="mr-1 h-3.5 w-3.5 opacity-70" />
              {{ t('guestOrders.viewDetails') }}
            </RouterLink>
            <Button
              v-if="order.status === 'pending_payment'"
              as-child
              size="sm"
              class="rounded-[10px] bg-primary px-4 font-semibold text-primary-foreground hover:bg-primary/90"
            >
              <RouterLink :to="`/pay?guest=1&order_no=${order.order_no}`">
                <CreditCard class="h-3.5 w-3.5" />
                {{ t('guestOrders.payNow') }}
              </RouterLink>
            </Button>
          </div>
        </article>
      </div>

      <PaginationNav
        :current-page="pagination.page"
        :total-pages="pagination.total_page"
        :loading="loading"
        @change-page="changePage"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { CreditCard, Eye, Search } from 'lucide-vue-next'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import EmptyState from '../../components/EmptyState.vue'
import PaginationNav from '../../components/PaginationNav.vue'
import { useGuestOrders } from '../../composables/useGuestOrders'

const { t } = useI18n()

const {
  savedAuth, email, orderPassword, orderNo, loading, error, orders, pagination,
  hasSavedAuth, clearSaved, handleSearch, emptyMessage, changePage,
  statusLabel, statusVariant, formatMoney, formatDiscountMoney, hasDiscountAmount, hasDiscount, formatDate,
} = useGuestOrders()
</script>
