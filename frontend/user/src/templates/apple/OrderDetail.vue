<template>
  <div class="st-root st-page">
    <div class="st-container">
      <div class="od-shell">
        <nav class="od-crumbs">
          <RouterLink to="/">{{ t('nav.home') }}</RouterLink>
          <span class="od-crumb-sep">/</span>
          <RouterLink to="/me/orders">{{ t('orders.title') }}</RouterLink>
          <span class="od-crumb-sep">/</span>
          <span class="od-crumb-current">{{ t('orderDetail.title') }}</span>
        </nav>

        <header class="st-page-head od-head">
          <h1 class="st-page-title">{{ t('orderDetail.title') }}</h1>
          <p class="st-page-lead">{{ t('orderDetail.subtitle') }}</p>
        </header>

        <!-- Loading -->
        <div v-if="loading" class="od-skeleton" aria-hidden="true">
          <div class="od-skel od-skel--line"></div>
          <div class="od-skel od-skel--block"></div>
        </div>

        <!-- 不存在 -->
        <div v-else-if="!order" class="od-empty">
          <AlertCircle class="od-empty-icon" />
          <p>{{ t('orderDetail.notFound') }}</p>
          <button type="button" class="st-btn st-btn--primary" @click="debouncedLoadOrder()">{{ t('errorBoundary.retry') }}</button>
        </div>

        <template v-else>
          <section class="od-summary">
            <div class="od-summary-top">
              <div class="od-summary-main">
                <p class="od-kicker">{{ t('orders.orderNo') }}</p>
                <p class="od-number">{{ order.order_no }}</p>
                <p class="od-date">{{ t('orderDetail.createdAtLabel') }}：{{ formatDate(order.created_at) }}</p>
              </div>
              <div class="od-summary-side">
                <p class="od-kicker">{{ t('orderDetail.amountTotal') }}</p>
                <p class="od-total">{{ formatMoney(order.total_amount, order.currency) }}</p>
              </div>
            </div>

            <div class="od-summary-foot">
              <span class="od-status" :class="`od-status--${statusVariant(order.status)}`">{{ statusLabel(order.status) }}</span>
              <div class="od-actions">
                <RouterLink v-if="order.status === 'pending_payment'" class="st-btn st-btn--primary" :to="`/pay?order_no=${order.order_no}`">{{ t('orderDetail.payNow') }}</RouterLink>
                <button v-if="order.status === 'pending_payment'" type="button" class="st-btn st-btn--ghost od-cancel" @click="cancelOrder">{{ t('orderDetail.cancel') }}</button>
              </div>
            </div>
          </section>

          <VaultOrderBody
            class="od-body"
            :order="order"
            variant="user"
            :fulfillment-downloading="fulfillmentDownloading"
            @download="handleDownloadFulfillment"
          />
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { AlertCircle } from 'lucide-vue-next'
import VaultOrderBody from './components/VaultOrderBody.vue'
import { useOrderDetail } from '../../composables/useOrderDetail'

const { t } = useI18n()

const {
  loading, order, debouncedLoadOrder, cancelOrder, fulfillmentDownloading, handleDownloadFulfillment,
  statusLabel, statusVariant, formatDate, formatMoney,
} = useOrderDetail()
</script>

<style scoped>
/* ---------- 页面基础 ---------- */
/* 标题、页头、按钮、面板等通用件继承 store.css 的 .st-* 样式。 */
.st-page {
  --st-line: color-mix(in srgb, var(--st-text) 14%, transparent);
}
.od-shell {
  width: 100%;
  max-width: 960px;
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

/* ---------- 面包屑 / 页头 ---------- */
.od-crumbs {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  padding-top: 20px;
  font-size: 12px;
  line-height: 1.4;
  color: var(--st-text-3);
}
.od-crumbs a {
  color: var(--st-text-2);
  text-decoration: none;
}
.od-crumbs a:hover {
  color: var(--st-text);
}
.od-crumb-sep {
  color: var(--st-text-3);
}
.od-crumb-current {
  color: var(--st-text);
}
.od-head {
  padding: 30px 0 34px;
}

/* ---------- 订单摘要 ---------- */
.od-summary {
  padding-bottom: 26px;
  border-bottom: 1px solid var(--st-line);
}
.od-summary-top {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
}
.od-summary-main {
  min-width: 0;
}
.od-kicker {
  margin: 0 0 8px;
  font-size: 13px;
  line-height: 1.4;
  color: var(--st-text-2);
}
.od-number {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  line-height: 1.4;
  letter-spacing: 0.01em;
  color: var(--st-text);
  font-variant-numeric: tabular-nums;
  font-feature-settings: "tnum" 1;
  overflow-wrap: anywhere;
}
.od-date {
  margin: 8px 0 0;
  font-size: 13px;
  line-height: 1.5;
  color: var(--st-text-2);
}
.od-summary-side {
  flex: none;
  text-align: right;
}
.od-total {
  margin: 0;
  font-family: var(--st-font-display);
  font-size: 28px;
  font-weight: 600;
  line-height: 1.1;
  letter-spacing: -0.01em;
  font-variant-numeric: tabular-nums;
}
.od-summary-foot {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-top: 26px;
  padding-top: 18px;
  border-top: 1px solid var(--st-line);
}
.od-status {
  font-size: 14px;
  font-weight: 500;
  line-height: 1.4;
}
.od-status--success,
.od-status--info {
  color: var(--st-green);
}
.od-status--warning,
.od-status--accent,
.od-status--danger {
  color: var(--st-orange);
}
.od-status--neutral {
  color: var(--st-text-2);
}
.dark .od-status--success,
.dark .od-status--info {
  color: #30d158;
}
.dark .od-status--warning,
.dark .od-status--accent,
.dark .od-status--danger {
  color: #ff9f0a;
}
.od-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}
.od-cancel {
  color: #c9342f;
  border-color: color-mix(in srgb, #c9342f 32%, transparent);
}
.od-cancel:hover:not(:disabled) {
  color: #c9342f;
  background: color-mix(in srgb, #c9342f 6%, transparent);
}

/* ---------- Loading / 空态 ---------- */
.od-skeleton {
  display: grid;
  gap: 14px;
  padding: 8px 0 24px;
}
.od-skel {
  border-radius: 10px;
  background: var(--st-bg-alt);
  animation: od-pulse 1.4s ease-in-out infinite;
}
.od-skel--line {
  width: 32%;
  height: 17px;
}
.od-skel--block {
  width: 100%;
  height: 240px;
  border-radius: 16px;
}
@keyframes od-pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.55;
  }
}
.od-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
  padding: 76px 0 36px;
  text-align: center;
  font-size: 14px;
  color: var(--st-text-2);
}
.od-empty-icon {
  width: 40px;
  height: 40px;
  opacity: 0.3;
}

/* ---------- 订单正文：去掉子组件卡片，改为细线分区 ---------- */
.od-body {
  display: block;
  margin-top: 4px;
}
.od-body :deep(section) {
  margin: 0;
  padding: 30px 0;
  border: 0;
  border-top: 1px solid var(--st-line);
  border-radius: 0;
  background: transparent;
  box-shadow: none;
}
.od-body :deep(section:first-child) {
  padding-top: 22px;
  border-top: 0;
}
.od-body :deep(section > h2) {
  margin: 0 0 18px;
  font-family: var(--st-font-display);
  font-size: 24px;
  line-height: 1.2;
  font-weight: 600;
  letter-spacing: -0.01em;
  color: var(--st-text);
}

/* 子订单：用浅灰分组，不用描边卡片 */
.od-body :deep(section > div.grid.gap-4 > div) {
  padding: 20px;
  border: 0;
  border-radius: 16px;
  background: var(--st-bg-alt);
}
.od-body :deep(section > div.grid.gap-4 > div > div.flex > div:last-child) {
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
  font-size: 13px;
  font-weight: 500;
}

/* 商品行：72px 缩略图 + 16px 名称 + 13px 灰色规格/数量 + 右侧金额 */
.od-body :deep(.grid > .flex.flex-wrap.border-b) {
  align-items: flex-start;
  gap: 24px;
  padding: 22px 0;
  border-bottom: 1px solid var(--st-line);
}
.od-body :deep(.grid > .flex.flex-wrap.border-b:first-child) {
  padding-top: 0;
}
.od-body :deep(.grid > .flex.flex-wrap.border-b:last-child) {
  padding-bottom: 0;
  border-bottom: 0;
}
.od-body :deep(.grid > .flex.flex-wrap.border-b > div:first-child) {
  gap: 16px;
}
.od-body :deep(.grid > .flex.flex-wrap.border-b > div:first-child > div:first-child) {
  width: 72px;
  height: 72px;
  border: 0;
  border-radius: 12px;
  background: var(--st-bg-alt);
}
.od-body :deep(.grid > .flex.flex-wrap.border-b > div:first-child > div:first-child img) {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.od-body :deep(.grid > .flex.flex-wrap.border-b > div:first-child > div:last-child > div:first-child) {
  font-size: 16px;
  font-weight: 600;
  line-height: 1.35;
  color: var(--st-text);
  overflow-wrap: anywhere;
}
.od-body :deep(.grid > .flex.flex-wrap.border-b > div:first-child > div:last-child > div:not(:first-child)) {
  margin-top: 4px;
  font-size: 13px;
  line-height: 1.5;
  color: var(--st-text-2);
}
.od-body :deep(.grid > .flex.flex-wrap.border-b > div:last-child) {
  min-width: 168px;
  gap: 2px;
  font-size: 13px;
  line-height: 1.5;
  color: var(--st-text-2);
}
.od-body :deep(.grid > .flex.flex-wrap.border-b > div:last-child > div) {
  font-size: 13px;
  line-height: 1.5;
}
.od-body :deep(.grid > .flex.flex-wrap.border-b > div:last-child > div:last-child) {
  margin-top: 4px;
  font-size: 16px;
  font-weight: 600;
  color: var(--st-text);
}
.od-body :deep(.rounded-sm.border.bg-secondary) {
  border: 0;
  border-radius: 12px;
  background: var(--st-bg-alt);
}
@media (max-width: 640px) {
  .od-body :deep(.grid > .flex.flex-wrap.border-b) {
    flex-direction: column;
    gap: 16px;
  }
  .od-body :deep(.grid > .flex.flex-wrap.border-b > div:last-child) {
    width: 100%;
    min-width: 0;
    padding-left: 88px;
    text-align: left;
  }
}

/* 金额 / 时间明细：只留文字与列间距，不画卡片 */
.od-body :deep(section > .grid.gap-3 > .rounded-md) {
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
}
.od-body :deep(section > .grid.gap-3 > .rounded-md > div:first-child) {
  font-size: 13px;
  line-height: 1.4;
  color: var(--st-text-2);
}
.od-body :deep(section > .grid.gap-3 > .rounded-md > div:last-child) {
  margin-top: 6px;
  font-size: 16px;
  font-weight: 600;
  line-height: 1.4;
  color: inherit;
  font-variant-numeric: tabular-nums;
}

/* 退款记录 */
.od-body :deep(section > .overflow-hidden.rounded-md.border) {
  border: 0;
  border-radius: 0;
  overflow: visible;
}
.od-body :deep(section > .overflow-hidden.rounded-md.border > .grid) {
  padding: 0;
  background: transparent;
}
.od-body :deep(section > .overflow-hidden.rounded-md.border > .grid:first-child) {
  padding-bottom: 10px;
  border-bottom: 1px solid var(--st-line);
}
.od-body :deep(section > .overflow-hidden.rounded-md.border > .border-t) {
  padding: 12px 0;
  border-top: 0;
  border-bottom: 1px solid var(--st-line);
}
.od-body :deep(pre) {
  margin: 12px 0 0;
  padding: 16px;
  border: 0;
  border-radius: 12px;
  background: var(--st-bg-alt);
  color: var(--st-text-2);
  font-size: 12px;
  line-height: 1.6;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
}

@media (min-width: 735px) {
  .od-body :deep(section > h2) {
    font-size: 32px;
  }
}
@media (max-width: 640px) {
  .od-head {
    padding: 24px 0 28px;
  }
  .od-summary-side {
    text-align: left;
  }
  .od-total {
    font-size: 24px;
  }
}
@media (prefers-reduced-motion: reduce) {
  .od-skel {
    animation: none;
  }
  .st-btn {
    transition: none;
  }
}
</style>
