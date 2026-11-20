<template>
  <div class="min-h-screen bg-background text-foreground pt-24 pb-16">
    <div class="container mx-auto px-4">
      <div class="mb-6">
        <h1 class="mb-1 text-[28px] font-bold tracking-tight flex items-center gap-3">
          <ClipboardList class="w-8 h-8 opacity-70" />
          {{ t('guestOrders.title') }}
        </h1>
        <p class="text-muted-foreground text-sm">{{ t('guestOrders.subtitle') }}</p>
      </div>

      <div class="theme-float rounded-[22px] bg-card p-5 md:p-6 mb-8">
        <div v-if="hasSavedAuth"
          class="mb-4 flex flex-col md:flex-row md:items-center md:justify-between gap-3 text-xs text-muted-foreground rounded-2xl bg-secondary/60 px-4 py-3">
          <span>{{ t('guestOrders.savedHint', { email: savedAuth.email || '-' }) }}</span>
          <Button type="button" variant="link" class="h-auto p-0 text-xs font-normal text-muted-foreground hover:text-foreground" @click="clearSaved">
            {{ t('guestOrders.clearSaved') }}
          </Button>
        </div>
        <div class="grid grid-cols-1 md:grid-cols-4 gap-4">
          <Input v-model="email" type="email" class="h-11" :placeholder="t('guestOrders.emailPlaceholder')" />
          <Input v-model="orderPassword" type="password" class="h-11" :placeholder="t('guestOrders.passwordPlaceholder')" />
          <Input v-model="orderNo" type="text" class="h-11" :placeholder="t('guestOrders.orderNoPlaceholder')" />
          <Button @click="handleSearch" :disabled="loading" class="h-11 px-6 rounded-full bg-foreground font-semibold text-background hover:bg-foreground/85">
            <Search class="h-4 w-4" />
            {{ loading ? t('guestOrders.searching') : t('guestOrders.search') }}
          </Button>
        </div>
        <p class="text-xs text-muted-foreground mt-3">{{ t('guestOrders.tip') }}</p>
        <Alert v-if="error" variant="destructive" class="mt-4">
          <AlertDescription>{{ error }}</AlertDescription>
        </Alert>
      </div>

      <EmptyState
        v-if="orders.length === 0 && !loading"
        icon="order"
        :description="emptyMessage"
      />

      <div v-else class="space-y-4">
        <div v-for="order in orders" :key="order.order_no"
          class="theme-float rounded-[22px] bg-card p-5 md:p-6 flex flex-col md:flex-row md:items-center md:justify-between gap-4">
          <div>
            <div class="text-xs text-muted-foreground">{{ t('orders.orderNo') }}：{{ order.order_no }}</div>
            <div class="text-lg font-bold text-foreground mt-1">{{ formatMoney(order.total_amount,
              order.currency) }}</div>
            <div v-if="hasDiscount(order)" class="text-xs text-muted-foreground mt-1">
              <span v-if="hasDiscountAmount(order.discount_amount)" class="text-rose-600 dark:text-rose-300">
                {{ t('orderDetail.couponDiscountLabel') }}：{{ formatDiscountMoney(order.discount_amount, order.currency) }}
              </span>
              <span v-if="hasDiscountAmount(order.promotion_discount_amount)" class="ml-2 text-rose-600 dark:text-rose-300">
                {{ t('orderDetail.promotionDiscountLabel') }}：{{ formatDiscountMoney(order.promotion_discount_amount,
                  order.currency) }}
              </span>
            </div>
            <div class="text-xs text-muted-foreground mt-1">{{ formatDate(order.created_at) }}</div>
          </div>
          <div class="flex items-center gap-3">
            <Badge :variant="statusVariant(order.status)" size="sm">
              {{ statusLabel(order.status) }}
            </Badge>
            <RouterLink
              :to="{ name: 'guest-order-detail', params: { order_no: order.order_no } }"
              :class="buttonVariants({ variant: 'outline', size: 'sm' })"
            >
              <Eye class="h-4 w-4 opacity-60" />
              {{ t('guestOrders.viewDetails') }}
            </RouterLink>
            <Button v-if="order.status === 'pending_payment'" as-child size="sm" class="rounded-full bg-foreground font-semibold text-background hover:bg-foreground/85">
              <router-link :to="`/pay?guest=1&order_no=${order.order_no}`">
                <CreditCard class="h-4 w-4" />
                {{ t('guestOrders.payNow') }}
              </router-link>
            </Button>
          </div>
        </div>
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
import { ClipboardList, Search, Eye, CreditCard } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button, buttonVariants } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import EmptyState from '../components/EmptyState.vue'
import PaginationNav from '../components/PaginationNav.vue'
import { useGuestOrders } from '../composables/useGuestOrders'

const { t } = useI18n()

const {
  savedAuth, email, orderPassword, orderNo, loading, error, orders, pagination,
  hasSavedAuth, clearSaved, handleSearch, emptyMessage, changePage,
  statusLabel, statusVariant, formatMoney, formatDiscountMoney, hasDiscountAmount, hasDiscount, formatDate,
} = useGuestOrders()
</script>
