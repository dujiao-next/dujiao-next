<template>
  <div class="st-root st-page">
    <div class="st-container">
      <!-- 账户头部：大标题 + 右侧身份信息 -->
      <header class="st-page-head pc-head">
        <div class="pc-head-main">
          <h1 class="st-page-title">{{ t('personalCenter.title') }}</h1>
          <p class="st-page-lead">{{ t('personalCenter.subtitle') }}</p>
        </div>

        <div class="pc-account">
          <div class="pc-avatar" aria-hidden="true">{{ displayInitial }}</div>
          <div class="pc-account-body">
            <p class="pc-account-name">{{ userProfileStore.displayName }}</p>
            <p class="pc-account-email">{{ userProfileStore.profile?.email || t('personalCenter.subtitle') }}</p>
            <p class="pc-account-status">
              <span class="pc-status" :class="emailVerifiedVariant === 'success' ? 'pc-status--success' : 'pc-status--warning'">{{ emailVerifiedLabel }}</span>
              <span v-if="userProfileStore.currentLevel" class="pc-status pc-status--quiet">{{ levelName(userProfileStore.currentLevel) }}</span>
            </p>
          </div>
        </div>
      </header>

      <div class="pc-layout">
        <!-- 侧栏：桌面纵向，移动端顶部横向标签条 -->
        <aside class="pc-aside">
          <nav class="pc-nav" :aria-label="t('personalCenter.title')">
            <button
              v-for="item in visibleSectionItems"
              :key="item.key"
              type="button"
              class="pc-nav-item"
              :class="{ 'is-active': currentSection === item.key }"
              :aria-current="currentSection === item.key ? 'page' : undefined"
              @click="switchSection(item.key)"
            >
              {{ t(item.label) }}
            </button>
          </nav>
        </aside>

        <div class="pc-main">
          <div v-if="globalAlert" class="pc-alert" :class="`pc-alert--${globalAlert.level}`">
            {{ globalAlert.message }}
          </div>

          <!-- 概览 -->
          <template v-if="currentSection === 'overview'">
            <section class="pc-section">
              <h2 class="st-section-title">{{ t('personalCenter.tabs.overview') }}</h2>
              <div class="pc-list">
                <div class="pc-row">
                  <span class="pc-row-title">{{ t('personalCenter.tabs.orders') }}</span>
                  <span class="pc-row-value">{{ userProfileStore.loadingOrders ? '—' : userProfileStore.ordersTotal }}</span>
                </div>
                <div class="pc-row">
                  <span class="pc-row-title">{{ t('personalCenter.memberLevel.currentLevel') }}</span>
                  <span class="pc-row-value">{{ levelName(userProfileStore.currentLevel) }}</span>
                </div>
                <div class="pc-row">
                  <span class="pc-row-title">{{ t('personalCenter.memberLevel.discountRate') }}</span>
                  <span class="pc-row-value">{{ discountText }}</span>
                </div>
                <div class="pc-row">
                  <span class="pc-row-title">{{ t('personalCenter.overview.accountLabel') }}</span>
                  <span class="pc-status" :class="emailVerifiedVariant === 'success' ? 'pc-status--success' : 'pc-status--warning'">{{ emailVerifiedLabel }}</span>
                </div>
              </div>
            </section>

            <!-- 会员等级 -->
            <section v-if="userProfileStore.memberLevels.length > 0" class="pc-section">
              <div class="pc-section-head">
                <h2 class="st-section-title">{{ t('personalCenter.memberLevel.currentLevel') }}</h2>
                <span v-if="!userProfileStore.nextLevel && userProfileStore.currentLevel" class="pc-status pc-status--success">{{ t('personalCenter.memberLevel.highestLevel') }}</span>
              </div>

              <div class="pc-list">
                <div class="pc-row">
                  <div class="pc-level-id">
                    <span class="pc-level-icon" aria-hidden="true">
                      <img v-if="isImagePath(userProfileStore.currentLevel?.icon)" :src="getImageUrl(userProfileStore.currentLevel!.icon)" alt="" />
                      <span v-else>{{ userProfileStore.currentLevel?.icon || '👤' }}</span>
                    </span>
                    <span class="pc-row-title">{{ levelName(userProfileStore.currentLevel) }}</span>
                  </div>
                  <div class="pc-level-value">
                    <span class="pc-row-value">{{ discountText }}</span>
                    <span class="pc-row-sub">{{ t('personalCenter.memberLevel.discountRate') }}</span>
                  </div>
                </div>
              </div>

              <div v-if="userProfileStore.nextLevel" class="pc-next">
                <div class="pc-next-head">
                  <div class="pc-level-id">
                    <span class="pc-level-icon" aria-hidden="true">
                      <img v-if="isImagePath(userProfileStore.nextLevel.icon)" :src="getImageUrl(userProfileStore.nextLevel.icon)" alt="" />
                      <span v-else>{{ userProfileStore.nextLevel.icon || '⭐' }}</span>
                    </span>
                    <div class="pc-level-copy">
                      <p class="pc-row-sub">{{ t('personalCenter.memberLevel.nextLevel') }}</p>
                      <p class="pc-row-title">{{ levelName(userProfileStore.nextLevel) }}</p>
                    </div>
                  </div>
                  <span v-if="userProfileStore.nextLevel.discount_rate < 100" class="pc-status pc-status--quiet">{{ t('personalCenter.memberLevel.discountOff', { n: userProfileStore.nextLevel.discount_rate / 10 }) }}</span>
                </div>

                <div v-if="userProfileStore.upgradeProgress" class="pc-progress-list">
                  <div v-if="userProfileStore.upgradeProgress.rechargePercent !== null" class="pc-progress">
                    <div class="pc-progress-meta">
                      <span>{{ t('personalCenter.memberLevel.rechargeProgress') }}</span>
                      <span class="pc-progress-values">{{ userProfileStore.upgradeProgress.recharged.toFixed(2) }} / {{ userProfileStore.upgradeProgress.rechargeThreshold.toFixed(2) }}</span>
                    </div>
                    <div class="pc-progress-track" role="progressbar" :aria-label="t('personalCenter.memberLevel.rechargeProgress')" aria-valuemin="0" aria-valuemax="100" :aria-valuenow="userProfileStore.upgradeProgress.rechargePercent">
                      <div class="pc-progress-fill" :style="{ width: userProfileStore.upgradeProgress.rechargePercent + '%' }"></div>
                    </div>
                  </div>

                  <div v-if="userProfileStore.upgradeProgress.spendPercent !== null" class="pc-progress">
                    <div class="pc-progress-meta">
                      <span>{{ t('personalCenter.memberLevel.spendProgress') }}</span>
                      <span class="pc-progress-values">{{ userProfileStore.upgradeProgress.spent.toFixed(2) }} / {{ userProfileStore.upgradeProgress.spendThreshold.toFixed(2) }}</span>
                    </div>
                    <div class="pc-progress-track" role="progressbar" :aria-label="t('personalCenter.memberLevel.spendProgress')" aria-valuemin="0" aria-valuemax="100" :aria-valuenow="userProfileStore.upgradeProgress.spendPercent">
                      <div class="pc-progress-fill" :style="{ width: userProfileStore.upgradeProgress.spendPercent + '%' }"></div>
                    </div>
                  </div>
                </div>
              </div>
            </section>

            <!-- 最近订单 -->
            <section class="pc-section">
              <div class="pc-section-head">
                <h2 class="st-section-title">{{ t('personalCenter.overview.recentOrdersTitle') }}</h2>
                <RouterLink class="pc-link" to="/me/orders">{{ t('personalCenter.overview.viewAllOrders') }}</RouterLink>
              </div>

              <div v-if="userProfileStore.loadingOrders" class="pc-skeleton" aria-hidden="true">
                <div v-for="idx in 3" :key="idx" class="pc-skeleton-row">
                  <span class="pc-skeleton-bar"></span>
                </div>
              </div>
              <div v-else-if="userProfileStore.recentOrders.length === 0" class="pc-empty">{{ t('personalCenter.overview.emptyOrders') }}</div>
              <div v-else class="pc-list">
                <article v-for="order in userProfileStore.recentOrders" :key="order.order_no" class="pc-order">
                  <div class="pc-order-main">
                    <p class="pc-order-no">{{ t('orders.orderNo') }}：{{ order.order_no }}</p>
                    <p class="pc-order-amount">{{ formatMoney(order.total_amount, order.currency) }}</p>
                    <p class="pc-order-date">{{ formatDate(order.created_at) }}</p>
                  </div>
                  <div class="pc-order-side">
                    <span class="pc-status" :class="`pc-status--${statusVariant(order.status)}`">{{ statusLabel(order.status) }}</span>
                    <div class="pc-order-actions">
                      <RouterLink class="st-btn st-btn--ghost st-btn--small" :to="`/orders/${order.order_no}`">{{ t('orders.viewDetails') }}</RouterLink>
                      <RouterLink v-if="order.status === 'pending_payment'" class="st-btn st-btn--primary st-btn--small" :to="`/pay?order_no=${order.order_no}`">{{ t('orders.payNow') }}</RouterLink>
                    </div>
                  </div>
                </article>
              </div>
            </section>
          </template>

          <!-- 其余面板：复用现有组件（shadcn 风，嵌于 vault 作用域内） -->
          <div v-else class="pc-panel-slot">
            <ProfilePanel v-if="currentSection === 'profile'" />
            <SecurityPanel v-else-if="currentSection === 'security'" />
            <OrdersPanel v-else-if="currentSection === 'orders'" />
            <WalletPanel v-else-if="currentSection === 'wallet'" />
            <AffiliatePanel v-else-if="currentSection === 'affiliate'" />
            <div v-else-if="currentSection === 'reseller' && canAccessResellerConsole" class="pc-simple">
              <h2 class="st-section-title">{{ t('resellerConsole.title') }}</h2>
              <p class="pc-simple-desc">{{ t('resellerConsole.dashboard.description') }}</p>
              <RouterLink class="st-btn st-btn--primary st-btn--small pc-simple-action" to="/reseller">{{ t('resellerConsole.nav.dashboard') }}</RouterLink>
            </div>
            <GiftCardPanel v-else-if="currentSection === 'giftCard'" />
            <ApiPanel v-else-if="currentSection === 'api'" />
            <OrdersPanel v-else />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { getImageUrl } from '../../utils/image'
import ProfilePanel from '../../views/personal/ProfilePanel.vue'
import SecurityPanel from '../../views/personal/SecurityPanel.vue'
import OrdersPanel from '../../views/personal/OrdersPanel.vue'
import WalletPanel from '../../views/personal/WalletPanel.vue'
import GiftCardPanel from '../../views/personal/GiftCardPanel.vue'
import AffiliatePanel from '../../views/personal/AffiliatePanel.vue'
import ApiPanel from '../../views/personal/ApiPanel.vue'
import { usePersonalCenter, type PersonalSection } from '../../composables/usePersonalCenter'

const { t } = useI18n()

const props = withDefaults(defineProps<{ section?: PersonalSection }>(), {
  section: 'overview',
})

const {
  userProfileStore, canAccessResellerConsole, visibleSectionItems, currentSection, globalAlert,
  displayInitial, switchSection, statusLabel, statusVariant, formatMoney, formatDate,
  emailVerifiedLabel, emailVerifiedVariant, discountText, isImagePath, levelName,
} = usePersonalCenter(() => props.section)
</script>

<style scoped>
.st-page {
  --pc-hairline: color-mix(in srgb, var(--st-text) 10%, transparent);
  --pc-success: var(--st-green);
  --pc-warning: var(--st-orange);
}

:global(.dark) .st-page {
  --pc-hairline: rgba(255, 255, 255, 0.16);
  --pc-success: #30d158;
  --pc-warning: #ff9f0a;
}

/* ---------- 页面头部 ---------- */
.pc-head {
  display: flex;
  flex-direction: column;
  padding-bottom: 48px;
}

.pc-head-main {
  min-width: 0;
}

.pc-account {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-top: 28px;
  padding-top: 24px;
  border-top: 1px solid var(--pc-hairline);
}

.pc-avatar {
  display: grid;
  place-items: center;
  width: 48px;
  height: 48px;
  flex: none;
  border-radius: 50%;
  background: var(--st-bg-alt);
  color: var(--st-text);
  font-size: 19px;
  font-weight: 600;
  line-height: 1;
  text-transform: uppercase;
}

.pc-account-body {
  min-width: 0;
}

.pc-account-name {
  margin: 0;
  font-size: 17px;
  font-weight: 600;
  line-height: 1.3;
  color: var(--st-text);
}

.pc-account-email {
  margin: 3px 0 0;
  font-size: 13px;
  line-height: 1.4;
  color: var(--st-text-2);
  overflow-wrap: anywhere;
}

.pc-account-status {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px 10px;
  margin: 6px 0 0;
}

.pc-account-status .pc-status + .pc-status::before {
  content: '·';
  margin-right: 6px;
  color: var(--st-text-3);
}

/* ---------- 布局 ---------- */
.pc-layout {
  min-width: 0;
}

.pc-aside {
  margin: 0 -22px 32px;
}

.pc-nav {
  display: flex;
  gap: 26px;
  overflow-x: auto;
  padding: 0 22px;
  border-bottom: 1px solid var(--pc-hairline);
  scrollbar-width: none;
  overscroll-behavior-x: contain;
}

.pc-nav::-webkit-scrollbar {
  display: none;
}

.pc-nav-item {
  position: relative;
  flex: none;
  padding: 0 0 14px;
  border: 0;
  background: none;
  font-family: var(--st-font);
  font-size: 15px;
  line-height: 1.4;
  font-weight: 400;
  color: var(--st-text-2);
  white-space: nowrap;
  cursor: pointer;
  transition: color var(--st-dur) var(--st-ease);
}

.pc-nav-item:hover {
  color: var(--st-text);
}

.pc-nav-item.is-active {
  color: var(--st-text);
  font-weight: 600;
}

.pc-nav-item.is-active::after {
  content: '';
  position: absolute;
  right: 0;
  bottom: 0;
  left: 0;
  height: 2px;
  background: var(--st-text);
}

.pc-nav-item:focus-visible {
  outline: 2px solid var(--st-blue);
  outline-offset: 4px;
  border-radius: 4px;
}

.pc-main {
  min-width: 0;
}

@media (min-width: 900px) {
  .pc-head {
    flex-direction: row;
    align-items: flex-end;
    justify-content: space-between;
    gap: 56px;
    padding-bottom: 56px;
  }

  .pc-account {
    flex: none;
    max-width: 360px;
    margin-top: 0;
    padding-top: 0;
    padding-left: 36px;
    border-top: 0;
    border-left: 1px solid var(--pc-hairline);
  }

  .pc-layout {
    display: grid;
    grid-template-columns: 240px minmax(0, 1fr);
    gap: 64px;
    align-items: start;
  }

  .pc-aside {
    position: sticky;
    top: 80px;
    margin: 0;
  }

  .pc-nav {
    flex-direction: column;
    gap: 2px;
    overflow: visible;
    padding: 0;
    border-bottom: 0;
  }

  .pc-nav-item {
    width: 100%;
    padding: 9px 0 9px 18px;
    text-align: left;
  }

  .pc-nav-item.is-active::after {
    display: none;
  }

  .pc-nav-item::before {
    content: '';
    position: absolute;
    top: 50%;
    left: 0;
    width: 2px;
    height: 18px;
    margin-top: -9px;
    border-radius: 1px;
    background: transparent;
    transition: background var(--st-dur) var(--st-ease);
  }

  .pc-nav-item.is-active::before {
    background: var(--st-text);
  }

  .pc-main {
    max-width: 840px;
    padding-top: 2px;
  }
}

/* ---------- 提示 ---------- */
.pc-alert {
  padding: 12px 0 12px 14px;
  border-left: 2px solid currentColor;
  font-size: 14px;
  line-height: 1.5;
}

.pc-alert--error,
.pc-alert--warning {
  color: var(--pc-warning);
}

.pc-alert--success {
  color: var(--pc-success);
}

/* ---------- 内容区块 ---------- */
.pc-section + .pc-section {
  margin-top: 56px;
}

.pc-alert + .pc-section,
.pc-alert + .pc-panel-slot {
  margin-top: 40px;
}

.pc-section-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 18px;
}

.pc-section-head .st-section-title {
  margin: 0;
}

.pc-link {
  flex: none;
  font-size: 14px;
  line-height: 1.4;
  color: var(--st-link);
  text-decoration: none;
  white-space: nowrap;
}

.pc-link:hover {
  text-decoration: underline;
}

/* ---------- 列表 ---------- */
.pc-list {
  border-top: 1px solid var(--pc-hairline);
}

.pc-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px 24px;
  min-height: 56px;
  padding: 15px 0;
  border-bottom: 1px solid var(--pc-hairline);
}

.pc-row-title {
  display: block;
  margin: 0;
  font-size: 16px;
  font-weight: 500;
  line-height: 1.4;
  color: var(--st-text);
}

.pc-row-value {
  font-size: 17px;
  font-weight: 600;
  line-height: 1.3;
  color: var(--st-text);
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.pc-row-sub {
  display: block;
  margin: 3px 0 0;
  font-size: 13px;
  line-height: 1.45;
  color: var(--st-text-2);
}

/* ---------- 状态文字 ---------- */
.pc-status {
  display: inline-block;
  font-size: 13px;
  font-weight: 500;
  line-height: 1.4;
  color: var(--st-text-2);
}

.pc-row > .pc-status {
  text-align: right;
}

.pc-status--success {
  color: var(--pc-success);
}

.pc-status--warning,
.pc-status--danger,
.pc-status--accent {
  color: var(--pc-warning);
}

.pc-status--info,
.pc-status--neutral,
.pc-status--quiet {
  color: var(--st-text-2);
}

/* ---------- 会员等级 ---------- */
.pc-level-id {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.pc-level-icon {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  flex: none;
  font-size: 20px;
  line-height: 1;
}

.pc-level-icon img {
  display: block;
  width: 32px;
  height: 32px;
  object-fit: contain;
}

.pc-level-copy {
  min-width: 0;
}

.pc-level-copy .pc-row-sub {
  margin: 0 0 2px;
}

.pc-level-value {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 2px;
  flex: none;
  text-align: right;
}

.pc-level-value .pc-row-sub {
  margin: 0;
}

.pc-next {
  padding-top: 22px;
}

.pc-next-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 10px 24px;
}

.pc-progress-list {
  display: grid;
  gap: 18px;
  margin-top: 20px;
}

.pc-progress-meta {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  font-size: 13px;
  line-height: 1.4;
  color: var(--st-text-2);
}

.pc-progress-values {
  font-variant-numeric: tabular-nums;
}

.pc-progress-track {
  height: 3px;
  margin-top: 8px;
  overflow: hidden;
  border-radius: 999px;
  background: var(--st-bg-alt);
}

.pc-progress-fill {
  height: 100%;
  border-radius: inherit;
  background: var(--st-text);
  transition: width 700ms var(--st-ease);
}

/* ---------- 最近订单 ---------- */
.pc-order {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
  padding: 20px 0;
  border-bottom: 1px solid var(--pc-hairline);
}

.pc-order-main {
  min-width: 0;
}

.pc-order-no {
  margin: 0;
  font-size: 13px;
  line-height: 1.4;
  color: var(--st-text-2);
  overflow-wrap: anywhere;
}

.pc-order-amount {
  margin: 6px 0 4px;
  font-size: 17px;
  font-weight: 600;
  line-height: 1.3;
  color: var(--st-text);
  font-variant-numeric: tabular-nums;
}

.pc-order-date {
  margin: 0;
  font-size: 13px;
  line-height: 1.4;
  color: var(--st-text-2);
}

.pc-order-side {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 10px;
  flex: none;
}

.pc-order-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
}

/* ---------- 空态 / 骨架 ---------- */
.pc-empty {
  padding: 20px 0;
  border-top: 1px solid var(--pc-hairline);
  border-bottom: 1px solid var(--pc-hairline);
  font-size: 14px;
  line-height: 1.5;
  color: var(--st-text-2);
}

.pc-skeleton {
  border-top: 1px solid var(--pc-hairline);
}

.pc-skeleton-row {
  display: flex;
  align-items: center;
  height: 78px;
  border-bottom: 1px solid var(--pc-hairline);
}

.pc-skeleton-bar {
  display: block;
  width: 42%;
  height: 12px;
  border-radius: 6px;
  background: var(--st-bg-alt);
  animation: pc-pulse 1.4s ease-in-out infinite;
}

.pc-skeleton-row:nth-child(2) .pc-skeleton-bar {
  width: 56%;
}

.pc-skeleton-row:nth-child(3) .pc-skeleton-bar {
  width: 34%;
}

@keyframes pc-pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.55;
  }
}

/* ---------- 其它面板 ---------- */
.pc-simple {
  padding-top: 4px;
}

.pc-simple-desc {
  margin: 0;
  max-width: 640px;
  font-size: 15px;
  line-height: 1.6;
  color: var(--st-text-2);
}

.pc-simple-action {
  margin-top: 24px;
}

@media (max-width: 599px) {
  .pc-order {
    flex-direction: column;
    gap: 14px;
  }

  .pc-order-side {
    width: 100%;
    align-items: flex-start;
  }

  .pc-order-actions {
    justify-content: flex-start;
  }
}

@media (prefers-reduced-motion: reduce) {
  .pc-nav-item,
  .pc-nav-item::before,
  .pc-progress-fill {
    transition: none;
  }

  .pc-skeleton-bar {
    animation: none;
  }
}
</style>
