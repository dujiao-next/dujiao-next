<template>
  <div class="st-root st-page">
    <div class="st-container">
      <div class="go-shell">
        <header class="st-page-head go-head">
          <h1 class="st-page-title">{{ t('guestOrders.title') }}</h1>
          <p class="st-page-lead">{{ t('guestOrders.subtitle') }}</p>
        </header>

        <!-- 查询表单 -->
        <section class="st-panel go-lookup">
          <div v-if="hasSavedAuth" class="go-saved">
            <span>{{ t('guestOrders.savedHint', { email: savedAuth.email || '-' }) }}</span>
            <button type="button" class="go-text-link" @click="clearSaved">{{ t('guestOrders.clearSaved') }}</button>
          </div>

          <div class="go-fields">
            <input v-model="email" type="email" class="st-input" :placeholder="t('guestOrders.emailPlaceholder')" />
            <input v-model="orderPassword" type="password" class="st-input" :placeholder="t('guestOrders.passwordPlaceholder')" />
            <input v-model="orderNo" type="text" class="st-input" :placeholder="t('guestOrders.orderNoPlaceholder')" />
            <button type="button" class="st-btn st-btn--primary go-search" :disabled="loading" @click="handleSearch">
              <Search /> {{ loading ? t('guestOrders.searching') : t('guestOrders.search') }}
            </button>
          </div>

          <p class="go-tip">{{ t('guestOrders.tip') }}</p>
          <p v-if="error" class="go-error">{{ error }}</p>
        </section>

        <!-- 空态 -->
        <div v-if="orders.length === 0 && !loading" class="go-empty">
          <ClipboardList class="go-empty-icon" />
          <p>{{ emptyMessage }}</p>
        </div>

        <!-- 列表 -->
        <div v-else class="go-list">
          <article v-for="order in orders" :key="order.order_no" class="go-order">
            <div class="go-order-main">
              <p class="go-order-no">
                <span class="go-order-no-label">{{ t('orders.orderNo') }}：</span>
                <span class="go-order-no-value">{{ order.order_no }}</span>
              </p>
              <p class="go-order-amount">{{ formatMoney(order.total_amount, order.currency) }}</p>
              <div v-if="hasDiscount(order)" class="go-discounts">
                <span v-if="hasDiscountAmount(order.discount_amount)">{{ t('orderDetail.couponDiscountLabel') }}：{{ formatDiscountMoney(order.discount_amount, order.currency) }}</span>
                <span v-if="hasDiscountAmount(order.promotion_discount_amount)">{{ t('orderDetail.promotionDiscountLabel') }}：{{ formatDiscountMoney(order.promotion_discount_amount, order.currency) }}</span>
              </div>
              <p class="go-order-date">{{ formatDate(order.created_at) }}</p>
            </div>

            <div class="go-order-side">
              <span class="go-status" :class="`go-status--${statusVariant(order.status)}`">{{ statusLabel(order.status) }}</span>
              <div class="go-order-actions">
                <RouterLink
                  :to="{ name: 'guest-order-detail', params: { order_no: order.order_no } }"
                  class="st-btn st-btn--ghost st-btn--small"
                >
                  <Eye /> {{ t('guestOrders.viewDetails') }}
                </RouterLink>
                <RouterLink
                  v-if="order.status === 'pending_payment'"
                  class="st-btn st-btn--primary st-btn--small"
                  :to="`/pay?guest=1&order_no=${order.order_no}`"
                >
                  <CreditCard /> {{ t('guestOrders.payNow') }}
                </RouterLink>
              </div>
            </div>
          </article>
        </div>

        <div v-if="pagination.total_page > 1" class="go-pagination">
          <button type="button" class="st-btn st-btn--ghost go-page-btn" :disabled="loading || pagination.page <= 1" @click="changePage(pagination.page - 1)"><ChevronLeft /></button>
          <span class="go-page-count">{{ pagination.page }} / {{ pagination.total_page }}</span>
          <button type="button" class="st-btn st-btn--ghost go-page-btn" :disabled="loading || pagination.page >= pagination.total_page" @click="changePage(pagination.page + 1)"><ChevronRight /></button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ChevronLeft, ChevronRight, ClipboardList, Search, Eye, CreditCard } from 'lucide-vue-next'
import { useGuestOrders } from '../../composables/useGuestOrders'

const { t } = useI18n()

const {
  savedAuth, email, orderPassword, orderNo, loading, error, orders, pagination,
  hasSavedAuth, clearSaved, handleSearch, emptyMessage, changePage,
  statusLabel, statusVariant, formatMoney, formatDiscountMoney, hasDiscountAmount, hasDiscount, formatDate,
} = useGuestOrders()
</script>

<style scoped>
/* ---------- 页面基础 ---------- */
/* 标题、页头、按钮、输入框、面板等通用件继承 store.css 的 .st-* 样式。 */
.st-page {
  --st-line: color-mix(in srgb, var(--st-text) 14%, transparent);
}
.go-shell {
  width: 100%;
  max-width: 980px;
  margin-inline: auto;
}
.st-btn svg {
  flex: none;
  width: 16px;
  height: 16px;
}
.st-btn:focus-visible {
  outline: 2px solid var(--st-link);
  outline-offset: 2px;
}

/* ---------- 页头 / 查询 ---------- */
.go-head {
  padding: 42px 0 32px;
}
.go-lookup {
  padding: 24px;
  margin-bottom: 36px;
}
.go-saved {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 10px 16px;
  margin-bottom: 18px;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--st-line);
  font-size: 13px;
  line-height: 1.5;
  color: var(--st-text-2);
}
.go-text-link {
  padding: 0;
  border: 0;
  background: none;
  color: var(--st-link);
  font-family: var(--st-font);
  font-size: 13px;
  cursor: pointer;
}
.go-text-link:hover {
  text-decoration: underline;
}
.go-fields {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 10px;
}
.go-search {
  height: 48px;
  white-space: nowrap;
}
.go-tip {
  margin: 14px 0 0;
  font-size: 13px;
  line-height: 1.5;
  color: var(--st-text-2);
}
.go-error {
  margin: 12px 0 0;
  font-size: 13px;
  line-height: 1.5;
  color: #c9342f;
}

/* ---------- 空态 ---------- */
.go-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
  padding: 72px 0 36px;
  text-align: center;
  font-size: 14px;
  color: var(--st-text-2);
}
.go-empty-icon {
  width: 40px;
  height: 40px;
  opacity: 0.3;
}

/* ---------- 订单列表：细线分隔，无卡片 ---------- */
.go-list {
  border-bottom: 1px solid var(--st-line);
}
.go-order {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 32px;
  padding: 26px 0;
  border-top: 1px solid var(--st-line);
}
.go-order-main {
  min-width: 0;
}
.go-order-no {
  margin: 0;
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 2px;
  font-size: 13px;
  line-height: 1.5;
  color: var(--st-text-2);
}
.go-order-no-value {
  font-size: 14px;
  font-weight: 600;
  letter-spacing: 0.01em;
  color: var(--st-text);
  font-variant-numeric: tabular-nums;
  font-feature-settings: "tnum" 1;
  overflow-wrap: anywhere;
}
.go-order-amount {
  margin: 10px 0 0;
  font-family: var(--st-font-display);
  font-size: 26px;
  font-weight: 600;
  line-height: 1.15;
  letter-spacing: -0.01em;
  color: var(--st-text);
  font-variant-numeric: tabular-nums;
}
.go-discounts {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 14px;
  margin-top: 8px;
  font-size: 13px;
  line-height: 1.5;
  color: var(--st-text-2);
}
.go-order-date {
  margin: 10px 0 0;
  font-size: 13px;
  line-height: 1.5;
  color: var(--st-text-2);
}
.go-order-side {
  display: flex;
  flex: none;
  flex-direction: column;
  align-items: flex-end;
  gap: 16px;
  padding-top: 1px;
}
.go-status {
  font-size: 14px;
  font-weight: 500;
  line-height: 1.4;
  text-align: right;
}
.go-status--success,
.go-status--info {
  color: var(--st-green);
}
.go-status--warning,
.go-status--accent,
.go-status--danger {
  color: var(--st-orange);
}
.go-status--neutral {
  color: var(--st-text-2);
}
.dark .go-status--success,
.dark .go-status--info {
  color: #30d158;
}
.dark .go-status--warning,
.dark .go-status--accent,
.dark .go-status--danger {
  color: #ff9f0a;
}
.go-order-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}

/* ---------- 分页 ---------- */
.go-pagination {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 14px;
  margin-top: 32px;
}
.go-page-btn {
  width: 38px;
  height: 38px;
  padding: 0;
  border-radius: 999px;
}
.go-page-count {
  font-size: 13px;
  font-weight: 600;
  line-height: 1;
  color: var(--st-text-2);
  font-variant-numeric: tabular-nums;
}

@media (min-width: 735px) {
  .go-lookup {
    padding: 28px;
  }
  .go-fields {
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) minmax(0, 1fr) auto;
  }
}
@media (max-width: 734px) {
  .go-fields {
    grid-template-columns: minmax(0, 1fr);
  }
  .go-search {
    width: 100%;
  }
}
@media (max-width: 640px) {
  .go-head {
    padding: 28px 0 26px;
  }
  .go-lookup {
    padding: 20px;
  }
  .go-order {
    flex-direction: column;
    gap: 18px;
    padding: 24px 0;
  }
  .go-order-side {
    align-items: flex-start;
    gap: 12px;
    width: 100%;
  }
  .go-status {
    text-align: left;
  }
  .go-order-actions {
    justify-content: flex-start;
  }
  .go-order-amount {
    font-size: 24px;
  }
}
@media (prefers-reduced-motion: reduce) {
  .st-btn,
  .st-input {
    transition: none;
  }
}
</style>
