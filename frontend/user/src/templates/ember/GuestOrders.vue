<template>
  <div class="mx-auto w-full max-w-[880px] px-4 pb-10 sm:px-6">
    <header class="my-5 flex flex-col gap-2">
      <h1 class="text-[clamp(1.75rem,1.3rem+1.9vw,2.6rem)] font-semibold tracking-[-0.022em]">{{ t('guestOrders.title') }}</h1>
      <p class="text-[15px] leading-[1.65] text-muted-foreground">{{ t('guestOrders.subtitle') }}</p>
    </header>

    <!-- 查询表单 -->
    <section class="rounded-[var(--r-lg)] border border-border bg-card p-5">
      <h2 class="mb-4 text-[15px] font-semibold tracking-[-0.015em]">{{ t('guestOrders.title') }}</h2>
      <div class="grid gap-3 sm:grid-cols-2">
        <label class="flex flex-col gap-2">
          <span class="text-[12.5px] font-medium">{{ t('ember.guestOrders.emailLabel') }}</span>
          <input
            v-model="email"
            type="email"
            class="h-11 rounded-[var(--r)] border border-border bg-[color:var(--surface-2)] px-4 text-[14px] outline-none transition-colors focus:border-[color:var(--accent)] focus:bg-card focus:ring-[var(--ring)]"
            :placeholder="t('guestOrders.emailPlaceholder')"
            @keydown.enter="handleSearch"
          />
        </label>
        <label class="flex flex-col gap-2">
          <span class="text-[12.5px] font-medium">{{ t('ember.guestOrders.passwordLabel') }}</span>
          <input
            v-model="orderPassword"
            type="password"
            class="h-11 rounded-[var(--r)] border border-border bg-[color:var(--surface-2)] px-4 text-[14px] outline-none transition-colors focus:border-[color:var(--accent)] focus:bg-card focus:ring-[var(--ring)]"
            :placeholder="t('guestOrders.passwordPlaceholder')"
            @keydown.enter="handleSearch"
          />
        </label>
      </div>

      <p v-if="error" class="mt-3 rounded-[var(--r)] bg-[color:var(--danger-soft)] px-3.5 py-2.5 text-[13px] text-[color:var(--danger-strong)]">
        {{ error }}
      </p>

      <div class="mt-4 flex flex-wrap items-center gap-3">
        <Button size="sm" class="rounded-full" :disabled="loading" @click="handleSearch">
          {{ loading ? t('common.loading') : t('guestOrders.search') }}
        </Button>
        <button
          v-if="hasSavedAuth"
          type="button"
          class="text-[13px] text-muted-foreground transition-colors hover:text-foreground"
          @click="clearSaved"
        >{{ t('guestOrders.clearSaved') }}</button>
      </div>
    </section>

    <!-- 结果 -->
    <section v-if="orders.length" class="mt-4 flex flex-col gap-3">
      <div class="flex items-center justify-between text-[13px] text-muted-foreground">
        <span>{{ t('ember.guestOrders.resultCount', { count: pagination?.total ?? orders.length }) }}</span>
      </div>

      <RouterLink
        v-for="order in orders"
        :key="order.order_no"
        class="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-4 rounded-[var(--r-lg)] border border-border bg-card p-4 transition-all hover:-translate-y-0.5 hover:border-[color:var(--border-strong)] hover:shadow-[var(--shadow-sm)]"
        :to="`/guest/orders/${order.order_no}`"
      >
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2.5">
            <span class="ember-pill" :class="pillClass(order.status)">{{ statusLabel(order.status) }}</span>
            <span class="truncate font-mono text-[12px] text-muted-foreground">{{ order.order_no }}</span>
          </div>
          <div class="mt-2 truncate text-[14px] font-medium">
            {{ orderSummary(order) }}
          </div>
          <div class="mt-0.5 text-[12px] text-muted-foreground">{{ formatDate(order.created_at) }}</div>
        </div>
        <div class="flex items-center gap-3">
          <span class="text-[14px] font-semibold tabular-nums">{{ formatMoney(order.total_amount, order.currency) }}</span>
          <ChevronRight class="h-4 w-4 flex-none text-muted-foreground" />
        </div>
      </RouterLink>

      <nav v-if="(pagination?.total_page ?? 1) > 1" class="mt-6 flex items-center justify-center gap-3">
        <button
          class="grid h-10 w-10 place-items-center rounded-full border border-border bg-card text-muted-foreground transition-colors hover:border-[color:var(--border-strong)] disabled:opacity-40"
          :disabled="(pagination?.page ?? 1) <= 1"
          @click="changePage((pagination?.page ?? 1) - 1)"
        ><ChevronLeft class="h-4 w-4" /></button>
        <span class="text-[13px] tabular-nums text-muted-foreground">{{ pagination?.page }} / {{ pagination?.total_page }}</span>
        <button
          class="grid h-10 w-10 place-items-center rounded-full border border-border bg-card text-muted-foreground transition-colors hover:border-[color:var(--border-strong)] disabled:opacity-40"
          :disabled="(pagination?.page ?? 1) >= (pagination?.total_page ?? 1)"
          @click="changePage((pagination?.page ?? 1) + 1)"
        ><ChevronRight class="h-4 w-4" /></button>
      </nav>
    </section>

    <!-- 空态 -->
    <div v-else-if="!loading && emptyMessage" class="mt-4 flex flex-col items-center gap-3 rounded-[var(--r-lg)] border border-dashed border-border py-16 text-center">
      <Receipt class="h-11 w-11 text-muted-foreground" />
      <p class="text-[14px] text-muted-foreground">{{ emptyMessage }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ChevronLeft, ChevronRight, Receipt } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { useGuestOrders } from '../../composables/useGuestOrders'
import { useLocalized } from '../../composables/useProduct'

const { t } = useI18n()
const { getLocalizedText } = useLocalized()

const {
  email, orderPassword, loading, error, orders, pagination,
  hasSavedAuth, clearSaved, handleSearch, emptyMessage, changePage,
  statusLabel, statusVariant, formatMoney, formatDate,
} = useGuestOrders()

/** 订单摘要：取前两件商品标题 */
const orderSummary = (order: any) => {
  const items = Array.isArray(order?.items) ? order.items : []
  const titles = items.map((item: any) => getLocalizedText(item.title)).filter(Boolean).slice(0, 2)
  return titles.length ? titles.join('、') : t('common.noContent')
}

/** 状态徽标配色：Ember 只用中性灰 + 系统色，不引入额外色相 */
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
