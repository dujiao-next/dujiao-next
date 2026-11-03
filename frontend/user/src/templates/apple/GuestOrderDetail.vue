<template>
  <div class="st-root st-page">
    <div class="st-container">
      <div class="god-shell">
        <header class="st-page-head god-head">
          <div class="god-head-copy">
            <h1 class="st-page-title">{{ t('guestOrderDetail.title') }}</h1>
            <p class="st-page-lead">{{ t('guestOrderDetail.subtitle') }}</p>
          </div>
          <RouterLink class="god-back" to="/guest/orders">{{ t('guestOrderDetail.backSearch') }}</RouterLink>
        </header>

        <!-- 游客验证 -->
        <section v-if="viewState === 'auth'" class="st-panel god-auth">
          <h2 class="st-section-title">{{ t('guestOrderDetail.authTitle') }}</h2>
          <p class="god-hint">{{ t('guestOrderDetail.authHint') }}</p>
          <div class="god-fields">
            <input v-model="auth.email" type="email" class="st-input" :placeholder="t('guestOrders.emailPlaceholder')" />
            <input v-model="auth.order_password" type="password" class="st-input" :placeholder="t('guestOrders.passwordPlaceholder')" />
          </div>
          <p v-if="authError" class="god-error">{{ authError }}</p>
          <div class="god-actions">
            <button type="button" class="st-btn st-btn--primary" @click="handleAuthSubmit">{{ t('guestOrderDetail.authSubmit') }}</button>
            <button type="button" class="st-btn st-btn--ghost" @click="clearAuth">{{ t('guestOrderDetail.authClear') }}</button>
          </div>
        </section>

        <!-- Loading -->
        <div v-if="viewState === 'loading'" class="god-skeleton" aria-hidden="true">
          <div class="god-skel god-skel--line"></div>
          <div class="god-skel god-skel--block"></div>
        </div>

        <!-- 不存在 -->
        <div v-else-if="viewState === 'empty'" class="god-empty">
          <AlertCircle class="god-empty-icon" />
          <p>{{ t('guestOrderDetail.notFound') }}</p>
        </div>

        <template v-else-if="viewState === 'detail' && order">
          <section class="god-summary">
            <div class="god-summary-top">
              <div class="god-summary-main">
                <p class="god-kicker">{{ t('orders.orderNo') }}</p>
                <p class="god-number">{{ order.order_no }}</p>
                <p class="god-date">{{ t('orderDetail.createdAtLabel') }}：{{ formatDate(order.created_at) }}</p>
              </div>
              <div class="god-summary-side">
                <p class="god-kicker">{{ t('orderDetail.amountTotal') }}</p>
                <p class="god-total">{{ formatMoney(order.total_amount, order.currency) }}</p>
              </div>
            </div>

            <div class="god-summary-foot">
              <span class="god-status" :class="`god-status--${statusVariant(order.status)}`">{{ statusLabel(order.status) }}</span>
              <div class="god-actions">
                <RouterLink v-if="order.status === 'pending_payment'" class="st-btn st-btn--primary" :to="`/pay?guest=1&order_no=${order.order_no}`">{{ t('orders.payNow') }}</RouterLink>
              </div>
            </div>
          </section>

          <VaultOrderBody
            class="god-body"
            :order="order"
            variant="guest"
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
import { useGuestOrderDetail } from '../../composables/useGuestOrderDetail'

const { t } = useI18n()

const {
  order, authError, auth, viewState, handleAuthSubmit, clearAuth,
  fulfillmentDownloading, handleDownloadFulfillment,
  statusLabel, statusVariant, formatDate, formatMoney,
} = useGuestOrderDetail()
</script>

<style scoped>
/* ---------- 页面基础 ---------- */
/* 标题、页头、按钮、输入框、面板等通用件继承 store.css 的 .st-* 样式。 */
.st-page {
  --st-line: color-mix(in srgb, var(--st-text) 14%, transparent);
}
.god-shell {
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

/* ---------- 页头 ---------- */
.god-head {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
  padding: 38px 0 32px;
}
.god-head-copy {
  min-width: 0;
}
.god-back {
  margin-top: 8px;
  font-size: 13px;
  line-height: 1.5;
  color: var(--st-link);
  text-decoration: none;
}
.god-back:hover {
  text-decoration: underline;
}

/* ---------- 验证面板 ---------- */
.god-auth {
  padding: 24px;
  margin-bottom: 36px;
}
.god-hint {
  margin: 10px 0 0;
  max-width: 620px;
  font-size: 13px;
  line-height: 1.5;
  color: var(--st-text-2);
}
.god-fields {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 10px;
  margin-top: 20px;
}
.god-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 20px;
}
.god-error {
  margin: 12px 0 0;
  font-size: 13px;
  line-height: 1.5;
  color: #c9342f;
}

/* ---------- Loading / 空态 ---------- */
.god-skeleton {
  display: grid;
  gap: 14px;
  padding: 8px 0 24px;
}
.god-skel {
  border-radius: 10px;
  background: var(--st-bg-alt);
  animation: god-pulse 1.4s ease-in-out infinite;
}
.god-skel--line {
  width: 32%;
  height: 17px;
}
.god-skel--block {
  width: 100%;
  height: 240px;
  border-radius: 16px;
}
@keyframes god-pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.55;
  }
}
.god-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
  padding: 76px 0 36px;
  text-align: center;
  font-size: 14px;
  color: var(--st-text-2);
}
.god-empty-icon {
  width: 40px;
  height: 40px;
  opacity: 0.3;
}

/* ---------- 订单摘要 ---------- */
.god-summary {
  padding-bottom: 26px;
  border-bottom: 1px solid var(--st-line);
}
.god-summary-top {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
}
.god-summary-main {
  min-width: 0;
}
.god-kicker {
  margin: 0 0 8px;
  font-size: 13px;
  line-height: 1.4;
  color: var(--st-text-2);
}
.god-number {
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
.god-date {
  margin: 8px 0 0;
  font-size: 13px;
  line-height: 1.5;
  color: var(--st-text-2);
}
.god-summary-side {
  flex: none;
  text-align: right;
}
.god-total {
  margin: 0;
  font-family: var(--st-font-display);
  font-size: 28px;
  font-weight: 600;
  line-height: 1.1;
  letter-spacing: -0.01em;
  font-variant-numeric: tabular-nums;
}
.god-summary-foot {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-top: 26px;
  padding-top: 18px;
  border-top: 1px solid var(--st-line);
}
.god-status {
  font-size: 14px;
  font-weight: 500;
  line-height: 1.4;
}
.god-status--success,
.god-status--info {
  color: var(--st-green);
}
.god-status--warning,
.god-status--accent,
.god-status--danger {
  color: var(--st-orange);
}
.god-status--neutral {
  color: var(--st-text-2);
}
.dark .god-status--success,
.dark .god-status--info {
  color: #30d158;
}
.dark .god-status--warning,
.dark .god-status--accent,
.dark .god-status--danger {
  color: #ff9f0a;
}

/* ---------- 订单正文：去掉子组件卡片，改为细线分区 ---------- */
.god-body {
  display: block;
  margin-top: 4px;
}
.god-body :deep(section) {
  margin: 0;
  padding: 30px 0;
  border: 0;
  border-top: 1px solid var(--st-line);
  border-radius: 0;
  background: transparent;
  box-shadow: none;
}
.god-body :deep(section:first-child) {
  padding-top: 22px;
  border-top: 0;
}
.god-body :deep(section > h2) {
  margin: 0 0 18px;
  font-family: var(--st-font-display);
  font-size: 24px;
  line-height: 1.2;
  font-weight: 600;
  letter-spacing: -0.01em;
  color: var(--st-text);
}

/* 子订单：用浅灰分组，不用描边卡片 */
.god-body :deep(section > div.grid.gap-4 > div) {
  padding: 20px;
  border: 0;
  border-radius: 16px;
  background: var(--st-bg-alt);
}
.god-body :deep(section > div.grid.gap-4 > div > div.flex > div:last-child) {
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
  font-size: 13px;
  font-weight: 500;
}

/* 商品行：72px 缩略图 + 16px 名称 + 13px 灰色规格/数量 + 右侧金额 */
.god-body :deep(.grid > .flex.flex-wrap.border-b) {
  align-items: flex-start;
  gap: 24px;
  padding: 22px 0;
  border-bottom: 1px solid var(--st-line);
}
.god-body :deep(.grid > .flex.flex-wrap.border-b:first-child) {
  padding-top: 0;
}
.god-body :deep(.grid > .flex.flex-wrap.border-b:last-child) {
  padding-bottom: 0;
  border-bottom: 0;
}
.god-body :deep(.grid > .flex.flex-wrap.border-b > div:first-child) {
  gap: 16px;
}
.god-body :deep(.grid > .flex.flex-wrap.border-b > div:first-child > div:first-child) {
  width: 72px;
  height: 72px;
  border: 0;
  border-radius: 12px;
  background: var(--st-bg-alt);
}
.god-body :deep(.grid > .flex.flex-wrap.border-b > div:first-child > div:first-child img) {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.god-body :deep(.grid > .flex.flex-wrap.border-b > div:first-child > div:last-child > div:first-child) {
  font-size: 16px;
  font-weight: 600;
  line-height: 1.35;
  color: var(--st-text);
  overflow-wrap: anywhere;
}
.god-body :deep(.grid > .flex.flex-wrap.border-b > div:first-child > div:last-child > div:not(:first-child)) {
  margin-top: 4px;
  font-size: 13px;
  line-height: 1.5;
  color: var(--st-text-2);
}
.god-body :deep(.grid > .flex.flex-wrap.border-b > div:last-child) {
  min-width: 168px;
  gap: 2px;
  font-size: 13px;
  line-height: 1.5;
  color: var(--st-text-2);
}
.god-body :deep(.grid > .flex.flex-wrap.border-b > div:last-child > div) {
  font-size: 13px;
  line-height: 1.5;
}
.god-body :deep(.grid > .flex.flex-wrap.border-b > div:last-child > div:last-child) {
  margin-top: 4px;
  font-size: 16px;
  font-weight: 600;
  color: var(--st-text);
}
.god-body :deep(.rounded-sm.border.bg-secondary) {
  border: 0;
  border-radius: 12px;
  background: var(--st-bg-alt);
}
@media (max-width: 640px) {
  .god-body :deep(.grid > .flex.flex-wrap.border-b) {
    flex-direction: column;
    gap: 16px;
  }
  .god-body :deep(.grid > .flex.flex-wrap.border-b > div:last-child) {
    width: 100%;
    min-width: 0;
    padding-left: 88px;
    text-align: left;
  }
}

/* 金额 / 时间明细：只留文字与列间距，不画卡片 */
.god-body :deep(section > .grid.gap-3 > .rounded-md) {
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
}
.god-body :deep(section > .grid.gap-3 > .rounded-md > div:first-child) {
  font-size: 13px;
  line-height: 1.4;
  color: var(--st-text-2);
}
.god-body :deep(section > .grid.gap-3 > .rounded-md > div:last-child) {
  margin-top: 6px;
  font-size: 16px;
  font-weight: 600;
  line-height: 1.4;
  color: inherit;
  font-variant-numeric: tabular-nums;
}

/* 退款记录 */
.god-body :deep(section > .overflow-hidden.rounded-md.border) {
  border: 0;
  border-radius: 0;
  overflow: visible;
}
.god-body :deep(section > .overflow-hidden.rounded-md.border > .grid) {
  padding: 0;
  background: transparent;
}
.god-body :deep(section > .overflow-hidden.rounded-md.border > .grid:first-child) {
  padding-bottom: 10px;
  border-bottom: 1px solid var(--st-line);
}
.god-body :deep(section > .overflow-hidden.rounded-md.border > .border-t) {
  padding: 12px 0;
  border-top: 0;
  border-bottom: 1px solid var(--st-line);
}
.god-body :deep(pre) {
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
  .god-auth {
    padding: 28px;
  }
  .god-fields {
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  }
  .god-body :deep(section > h2) {
    font-size: 32px;
  }
}
@media (max-width: 640px) {
  .god-head {
    flex-direction: column;
    gap: 14px;
    padding: 28px 0 26px;
  }
  .god-back {
    margin-top: 0;
  }
  .god-summary-side {
    text-align: left;
  }
  .god-total {
    font-size: 24px;
  }
  .god-auth {
    padding: 20px;
  }
}
@media (prefers-reduced-motion: reduce) {
  .god-skel {
    animation: none;
  }
  .st-btn,
  .st-input {
    transition: none;
  }
}
</style>
