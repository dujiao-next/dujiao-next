<template>
  <div class="st-root st-page">
    <div class="st-container">
      <header class="pay-head">
        <div>
          <h1 class="st-page-title">{{ t('payment.title') }}</h1>
          <p class="st-page-lead">{{ t('payment.subtitle') }}</p>
        </div>
        <RouterLink class="pay-back" :to="backLink">{{ t('payment.backToOrders') }}</RouterLink>
      </header>

      <div class="pay-steps">
        <VaultCheckoutSteps current="payment" />
      </div>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="st-container">
      <div class="pay-skeleton"></div>
    </div>

    <!-- 游客验证 -->
    <div v-else-if="showGuestAuthForm" class="st-container">
      <div class="pay-panel">
        <h2 class="st-section-title">{{ t('payment.guestAuthTitle') }}</h2>
        <p class="pay-hint">{{ t('payment.guestAuthHint') }}</p>
        <div class="pay-fields">
          <input v-model="guestAuth.email" type="email" class="st-input" :placeholder="t('guestOrders.emailPlaceholder')" />
          <input v-model="guestAuth.order_password" type="password" class="st-input" :placeholder="t('guestOrders.passwordPlaceholder')" />
        </div>
        <p v-if="guestAuthError" class="pay-error">{{ guestAuthError }}</p>
        <button type="button" class="st-btn st-btn--primary pay-submit" @click="handleGuestAuthSubmit">{{ t('payment.guestAuthSubmit') }}</button>
      </div>
    </div>

    <!-- 订单不存在 -->
    <div v-else-if="!order" class="st-container">
      <div class="pay-empty">
        <AlertCircle class="pay-empty-icon" />
        <p>{{ t('payment.orderNotFound') }}</p>
        <RouterLink class="st-btn st-btn--primary" :to="backLink">{{ t('payment.backToOrders') }}</RouterLink>
      </div>
    </div>

    <!-- 结果视图 -->
    <div v-else-if="showResultView" class="st-container">
      <div class="pay-result-head">
        <div>
          <h2 class="st-section-title">{{ paymentResultTitle }}</h2>
          <p class="pay-hint">{{ paymentGuideTip }}</p>
          <p class="pay-hint">{{ t('payment.methodLabel') }}：{{ resultChannelName }}</p>
        </div>
        <div class="pay-result-actions">
          <button type="button" class="st-btn st-btn--ghost st-btn--small" :disabled="loading" @click="handleRefresh">{{ t('payment.refreshStatus') }}</button>
          <button type="button" class="st-btn st-btn--ghost st-btn--small" @click="handleChangePaymentMethod">{{ t('payment.changeMethod') }}</button>
        </div>
      </div>

      <div class="pay-result-grid">
        <div>
          <div v-if="showQRCode" class="pay-qr">
            <p class="pay-hint">{{ paymentGuideTitle }}</p>
            <div class="pay-qr-img"><img :src="qrImageUrl" alt="QR Code" /></div>
            <p v-if="qrUsingPayLinkFallback" class="pay-hint">{{ t('payment.qrFallbackHint') }}</p>
            <div v-if="hasCryptoPaymentDetails" class="pay-crypto">
              <div v-for="item in cryptoPaymentDetails" :key="item.key" class="pay-crypto-row">
                <span>{{ item.label }}</span>
                <span class="pay-crypto-value">{{ item.value }}<span v-if="item.detail"> ({{ item.detail }})</span></span>
              </div>
              <div v-if="cryptoWalletAddress" class="pay-crypto-actions">
                <button type="button" class="st-btn st-btn--ghost st-btn--small" @click="handleCopyWalletAddress">{{ t('payment.copyWalletAddress') }}</button>
                <span v-if="walletAddressCopied" class="pay-ok">{{ t('payment.copied') }}</span>
              </div>
            </div>
          </div>

          <div v-else class="pay-panel">
            <p class="pay-hint">{{ t('payment.openPayLink') }}</p>
            <button type="button" class="st-btn st-btn--primary" @click="handleOpenPayLink">{{ t('payment.openPayLink') }}</button>
            <p v-if="openedPayWindow" class="pay-ok">{{ payLinkOpenedTip }}</p>
            <p v-if="showTelegramPayHint" class="pay-hint">{{ t('payment.telegramExternalHint') }}</p>
            <div class="pay-inline">
              <button type="button" class="st-btn st-btn--ghost st-btn--small" @click="handleCopyPayLink">{{ t('payment.copyPayLink') }}</button>
              <span v-if="copied" class="pay-ok">{{ t('payment.copied') }}</span>
            </div>
          </div>
        </div>

        <div class="pay-side">
          <div class="pay-order-card">
            <div class="pay-label">{{ t('payment.orderNo') }}</div>
            <div class="pay-order-no">{{ order.order_no }}</div>
            <div class="pay-meta">{{ t('payment.orderStatus') }}：{{ statusLabel(order.status) }}</div>
            <div class="pay-meta">{{ t('payment.methodLabel') }}：{{ resultChannelName }}</div>
          </div>
          <PaymentAmountBreakdown
            :order="order"
            :payment-result="paymentResult"
            :customer-fee-applied="customerFeeApplied"
            :customer-fee-amount-display="customerFeeAmountDisplay"
            :payable-amount-display="payableAmountDisplay"
            :wallet-paid-display="paymentWalletPaidDisplay"
            :online-pay-display="paymentOnlinePayDisplay"
            :show-countdown="showCountdown"
            :countdown-text="countdownText"
            :polling-active="pollingActive"
            :format-money="formatMoney"
            :format-discount-money="formatDiscountMoney"
            :has-discount-amount="hasDiscountAmount"
          />
          <div v-if="paymentResult.expires_at" class="pay-meta-card">{{ t('payment.expiresAt') }}：{{ formatDate(paymentResult.expires_at) }}</div>
        </div>
      </div>
    </div>

    <!-- 过期 / 取消 -->
    <div v-else-if="orderExpired || orderCanceled" class="st-container">
      <div class="pay-panel">
        <div class="pay-result-head">
          <div>
            <h2 class="st-section-title">{{ orderCanceled ? t('payment.orderCanceled') : t('payment.orderExpired') }}</h2>
            <p class="pay-hint">{{ order.order_no }}</p>
          </div>
          <RouterLink class="st-btn st-btn--ghost st-btn--small" :to="backLink">{{ t('payment.backToOrders') }}</RouterLink>
        </div>
        <div class="pay-summary-3">
          <div><span class="pay-label">{{ t('payment.orderNo') }}</span><span class="pay-value">{{ order.order_no }}</span></div>
          <div><span class="pay-label">{{ t('payment.orderStatus') }}</span><span class="pay-value">{{ statusLabel(order.status) }}</span></div>
          <div><span class="pay-label">{{ t('orderDetail.amountTotal') }}</span><span class="pay-value">{{ formatMoney(order.total_amount, order.currency) }}</span></div>
        </div>
      </div>
    </div>

    <!-- 默认：订单 + 渠道 + 操作 -->
    <div v-else class="st-container pay-grid">
      <div class="pay-main">
        <!-- 订单信息 -->
        <section class="pay-block">
          <h2 class="st-section-title">{{ t('payment.orderInfo') }}</h2>
          <div class="pay-order-top">
            <div>
              <div class="pay-label">{{ t('payment.orderNo') }}</div>
              <div class="pay-order-no">{{ order.order_no }}</div>
              <div class="pay-meta">{{ t('orderDetail.createdAtLabel') }}：{{ formatDate(order.created_at) }}</div>
            </div>
            <div class="pay-amount-card">
              <div class="pay-label">{{ t('payment.payableAmountLabel') }}</div>
              <div class="pay-amount">{{ payableAmountDisplay }}</div>
              <div class="pay-rows">
                <div class="pay-row"><span>{{ t('orderDetail.amountTotal') }}</span><span>{{ formatMoney(order.total_amount, order.currency) }}</span></div>
                <div v-if="showBalanceOption && useBalance" class="pay-row"><span>{{ t('payment.walletDeductLabel') }}</span><span>{{ expectedWalletPaidDisplay }}</span></div>
                <div v-if="showBalanceOption && useBalance" class="pay-row"><span>{{ t('payment.onlinePayLabel') }}</span><span>{{ expectedOnlinePayDisplay }}</span></div>
                <div class="pay-row pay-row--line"><span>{{ t('payment.orderStatus') }}</span><span>{{ statusLabel(order.status) }}</span></div>
              </div>
            </div>
          </div>

          <div class="pay-summary-3">
            <div><span class="pay-label">{{ t('orderDetail.amountOriginal') }}</span><span class="pay-value">{{ formatMoney(order.original_amount, order.currency) }}</span></div>
            <div><span class="pay-label">{{ t('orderDetail.amountDiscount') }}</span><span class="pay-value" :class="{ 'is-orange': hasDiscountAmount(order.discount_amount) }">{{ formatDiscountMoney(order.discount_amount, order.currency) }}</span></div>
            <div><span class="pay-label">{{ t('orderDetail.promotionDiscountLabel') }}</span><span class="pay-value" :class="{ 'is-orange': hasDiscountAmount(order.promotion_discount_amount) }">{{ formatDiscountMoney(order.promotion_discount_amount, order.currency) }}</span></div>
            <div v-if="hasDiscountAmount(order.wholesale_discount_amount)"><span class="pay-label">{{ t('orderDetail.amountWholesaleDiscount') }}</span><span class="pay-value is-green">{{ formatDiscountMoney(order.wholesale_discount_amount, order.currency) }}</span></div>
          </div>

          <p v-if="order.expires_at" class="pay-meta">{{ t('payment.expiresAt') }}：{{ formatDate(order.expires_at) }}</p>
          <p v-if="showCountdown" class="pay-countdown" :class="{ 'is-expired': countdownExpired }">
            {{ t('payment.countdownLabel') }} <span>{{ countdownText }}</span>
          </p>
          <p v-if="pollingActive" class="pay-hint">{{ t('payment.pollingHint') }}</p>
        </section>

        <!-- 商品 -->
        <section v-if="orderItems.length" class="pay-block">
          <h2 class="st-section-title">{{ t('payment.itemsTitle') }}</h2>
          <div class="pay-items">
            <div v-for="(item, idx) in orderItems" :key="idx" class="pay-item">
              <div>
                <div class="pay-item-name">{{ getLocalizedText(item.title) }}</div>
                <div class="pay-meta">{{ t('orderDetail.quantityLabel') }}：{{ item.quantity }} · {{ t('orderDetail.itemFulfillmentLabel') }}：{{ fulfillmentTypeLabelText(item.fulfillment_type) }}</div>
                <div v-if="orderItemSkuText(item)" class="pay-meta">{{ t('orderDetail.itemSkuLabel') }}：{{ orderItemSkuText(item) }}</div>
              </div>
              <div class="pay-item-price">{{ formatMoney(item.total_price, order.currency) }}</div>
            </div>
          </div>
        </section>

        <!-- 渠道选择 -->
        <section class="pay-block">
          <h2 class="st-section-title">{{ t('payment.channelTitle') }}</h2>
          <p v-if="!configReady" class="pay-hint">{{ t('common.loading') }}</p>
          <template v-else>
            <div v-if="showBalanceOption" class="pay-wallet">
              <div class="pay-wallet-row">
                <div>
                  <div class="pay-label">{{ t('payment.walletBalanceLabel') }}</div>
                  <div class="pay-wallet-value">{{ walletLoading ? t('common.loading') : walletBalanceDisplay }}</div>
                </div>
                <label class="pay-wallet-toggle"><input v-model="useBalance" type="checkbox" :disabled="walletOnlyPayment" /><span>{{ t('payment.useBalance') }}</span></label>
              </div>
              <p v-if="walletOnlyPayment" class="pay-hint">{{ t('payment.walletOnlyHint') }}</p>
              <div v-if="useBalance" class="pay-hint">
                <div>{{ t('payment.walletDeductLabel') }}：{{ expectedWalletPaidDisplay }}</div>
                <div v-if="!walletOnlyPayment">{{ t('payment.onlinePayLabel') }}：{{ expectedOnlinePayDisplay }}</div>
                <div v-if="walletOnlyPayment && expectedOnlinePayCents > 0">{{ t('payment.walletInsufficientHint') }}</div>
              </div>
            </div>

            <div v-if="cachedPayment" class="pay-cached">
              <div class="pay-cached-title">{{ t('payment.cachedTitle') }}</div>
              <div>{{ t('payment.cachedHint', { channel: cachedChannelName }) }}</div>
              <button type="button" class="st-btn st-btn--ghost st-btn--small" @click="restoreCachedPayment">{{ t('payment.useCached') }}</button>
            </div>

            <PaymentChannelSelector
              v-if="!walletOnlyPayment"
              :channels="channels"
              :model-value="selectedChannelId"
              :show-balance-option="showBalanceOption"
              :is-channel-disabled-for-amount="isChannelDisabledForAmount"
              :channel-amount-limit-hint="channelAmountLimitHint"
              @update:model-value="selectedChannelId = $event"
            />
          </template>
        </section>

        <!-- 支付信息 -->
        <section v-if="paymentResult" class="pay-block">
          <h2 class="st-section-title">{{ t('payment.infoTitle') }}</h2>
          <div class="pay-hint">
            <div>{{ t('payment.methodLabel') }}：{{ resultChannelName }}</div>
            <div>{{ t('payment.interactionLabel') }}：{{ interactionLabel }}</div>
            <div v-if="paymentResult.expires_at">{{ t('payment.expiresAt') }}：{{ formatDate(paymentResult.expires_at) }}</div>
          </div>
          <div v-if="showPayLink" class="pay-inline">
            <button type="button" class="st-btn st-btn--primary st-btn--small" @click="handleOpenPayLink">{{ t('payment.openPayLink') }}</button>
            <button type="button" class="st-btn st-btn--ghost st-btn--small" @click="handleCopyPayLink">{{ t('payment.copyPayLink') }}</button>
          </div>
        </section>
      </div>

      <!-- 右栏 -->
      <aside class="pay-aside">
        <h2 class="pay-aside-title">{{ t('payment.actionTitle') }}</h2>
        <p v-if="showCountdown" class="pay-hint">{{ t('payment.countdownLabel') }}：<span>{{ countdownText }}</span></p>
        <div v-if="paymentAlert" class="pay-alert" :class="`is-${paymentAlert.level}`">{{ paymentAlert.message }}</div>

        <p v-if="selectedChannel" class="pay-picked">{{ t('payment.methodLabel') }}：{{ selectedChannelName }}</p>
        <p v-else-if="!requiresOnlineChannel && !orderExpired && !orderCanceled" class="pay-picked">{{ t('payment.walletPayOnly') }}</p>
        <p v-else-if="walletOnlyPayment && expectedOnlinePayCents > 0 && !orderExpired && !orderCanceled" class="pay-alert is-warning">{{ t('payment.walletInsufficientHint') }}</p>
        <p v-else-if="!walletOnlyPayment && requiresOnlineChannel && !orderExpired && !orderCanceled" class="pay-alert is-warning">{{ t('payment.selectChannelError') }}</p>

        <button type="button" class="st-btn st-btn--primary st-btn--block" :disabled="!canSubmitPayment" @click="handlePayment">
          {{ submitting ? t('payment.submitting') : t('payment.submitButton') }}
        </button>
        <button type="button" class="st-btn st-btn--ghost st-btn--block pay-refresh" :disabled="loading" @click="handleRefresh">{{ t('payment.refreshStatus') }}</button>
      </aside>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { AlertCircle } from 'lucide-vue-next'
import PaymentAmountBreakdown from '../../components/payment/PaymentAmountBreakdown.vue'
import PaymentChannelSelector from '../../components/payment/PaymentChannelSelector.vue'
import VaultCheckoutSteps from './components/VaultCheckoutSteps.vue'
import { usePayment } from '../../composables/usePayment'

const { t } = useI18n()

const {
  loading, submitting, order, paymentResult, selectedChannelId, copied, walletAddressCopied,
  openedPayWindow, cachedPayment, guestAuth, guestAuthError, walletLoading, useBalance,
  backLink, showGuestAuthForm, walletOnlyPayment, showBalanceOption, configReady, channels,
  selectedChannel, selectedChannelName, cachedChannelName, resultChannelName, interactionLabel,
  paymentResultTitle, paymentGuideTitle, paymentGuideTip, showPayLink, showTelegramPayHint, payLinkOpenedTip,
  cryptoWalletAddress, cryptoPaymentDetails, hasCryptoPaymentDetails, qrUsingPayLinkFallback, showQRCode, qrImageUrl,
  orderExpired, orderCanceled, paymentAlert, countdownExpired, countdownText, showCountdown, showResultView, pollingActive, orderItems,
  customerFeeApplied, customerFeeAmountDisplay, payableAmountDisplay, walletBalanceDisplay,
  expectedWalletPaidDisplay, expectedOnlinePayDisplay, expectedOnlinePayCents, requiresOnlineChannel,
  paymentWalletPaidDisplay, paymentOnlinePayDisplay, isChannelDisabledForAmount, channelAmountLimitHint, canSubmitPayment,
  formatDate, statusLabel, formatMoney, hasDiscountAmount, formatDiscountMoney, getLocalizedText, orderItemSkuText, fulfillmentTypeLabelText,
  handleCopyPayLink, handleCopyWalletAddress, handleOpenPayLink, restoreCachedPayment, handleChangePaymentMethod,
  handlePayment, handleGuestAuthSubmit, handleRefresh,
} = usePayment()
</script>

<style scoped>
.pay-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
  padding-top: 40px;
}
.pay-back {
  flex: none;
  font-size: 13px;
  color: var(--st-link);
  text-decoration: none;
  padding-top: 8px;
}
.pay-steps {
  padding-top: 20px;
  padding-bottom: 28px;
}
.pay-skeleton {
  height: 220px;
  border-radius: 18px;
  background: var(--st-bg-alt);
  animation: pay-pulse 1.4s ease-in-out infinite;
}
@keyframes pay-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.pay-panel {
  padding: 24px;
  border-radius: 18px;
  background: var(--st-bg-alt);
}
.pay-hint {
  margin: 6px 0 0;
  font-size: 13px;
  line-height: 1.5;
  color: var(--st-text-2);
}
.pay-fields {
  display: grid;
  gap: 12px;
  margin-top: 16px;
}
@media (min-width: 735px) {
  .pay-fields {
    grid-template-columns: 1fr 1fr;
  }
}
.pay-error {
  margin: 12px 0 0;
  font-size: 13px;
  color: #c9342f;
}
.pay-submit {
  margin-top: 18px;
}
.pay-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
  padding: 90px 0;
  text-align: center;
  color: var(--st-text-2);
}
.pay-empty-icon {
  width: 40px;
  height: 40px;
  opacity: 0.35;
}

/* 结果视图 */
.pay-result-head {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding-block: 8px 20px;
}
.pay-result-actions {
  display: flex;
  gap: 8px;
}
.pay-result-grid {
  display: grid;
  gap: 24px;
}
@media (min-width: 900px) {
  .pay-result-grid {
    grid-template-columns: 1.3fr 1fr;
  }
}
.pay-qr {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 24px;
  border-radius: 18px;
  background: var(--st-bg-alt);
  text-align: center;
}
.pay-qr-img {
  width: 100%;
  max-width: 240px;
  aspect-ratio: 1;
  padding: 10px;
  border-radius: 14px;
  background: #fff;
}
.pay-qr-img img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}
.pay-crypto {
  width: 100%;
  margin-top: 12px;
  text-align: left;
}
.pay-crypto-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding-block: 8px;
  border-bottom: 1px solid color-mix(in srgb, var(--st-text) 10%, transparent);
  font-size: 12px;
  color: var(--st-text-2);
}
.pay-crypto-value {
  font-weight: 600;
  color: var(--st-text);
  word-break: break-all;
  text-align: right;
}
.pay-crypto-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  padding-top: 10px;
}
.pay-ok {
  margin: 0;
  font-size: 12px;
  color: var(--st-green);
}
.pay-side {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.pay-order-card,
.pay-meta-card {
  padding: 18px;
  border-radius: 14px;
  background: var(--st-bg-alt);
}
.pay-meta-card {
  font-size: 12px;
  color: var(--st-text-2);
}
.pay-label {
  display: block;
  font-size: 11px;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--st-text-3);
}
.pay-order-no {
  margin-top: 4px;
  font-size: 16px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}
.pay-meta {
  margin: 6px 0 0;
  font-size: 12px;
  color: var(--st-text-2);
}

/* 默认视图 */
.pay-grid {
  display: grid;
  gap: 32px;
}
@media (min-width: 900px) {
  .pay-grid {
    grid-template-columns: minmax(0, 1fr) 340px;
    gap: 48px;
    align-items: start;
  }
}
.pay-main {
  min-width: 0;
}
.pay-block {
  padding-block: 26px;
  border-top: 1px solid color-mix(in srgb, var(--st-text) 10%, transparent);
}
.pay-block:first-child {
  padding-top: 0;
  border-top: 0;
}
.pay-order-top {
  display: flex;
  flex-wrap: wrap;
  gap: 20px;
  justify-content: space-between;
}
.pay-amount-card {
  flex: 1;
  min-width: 260px;
  padding: 18px;
  border-radius: 14px;
  background: var(--st-bg-alt);
}
.pay-amount {
  margin: 4px 0 12px;
  font-family: var(--st-font-display);
  font-size: 30px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}
.pay-rows {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.pay-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  font-size: 13px;
  color: var(--st-text-2);
}
.pay-row--line {
  margin-top: 4px;
  padding-top: 10px;
  border-top: 1px solid color-mix(in srgb, var(--st-text) 10%, transparent);
}
.pay-summary-3 {
  display: grid;
  gap: 12px;
  margin-top: 18px;
}
@media (min-width: 735px) {
  .pay-summary-3 {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
.pay-summary-3 > div {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.pay-value {
  font-size: 15px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}
.pay-value.is-orange {
  color: var(--st-orange);
}
.pay-value.is-green {
  color: var(--st-green);
}
.pay-countdown {
  display: inline-block;
  margin: 10px 0 0;
  font-size: 13px;
  color: var(--st-text-2);
  font-variant-numeric: tabular-nums;
}
.pay-countdown.is-expired {
  color: #c9342f;
}
.pay-items {
  display: flex;
  flex-direction: column;
}
.pay-item {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  padding-block: 14px;
  border-top: 1px solid color-mix(in srgb, var(--st-text) 8%, transparent);
}
.pay-item:first-child {
  border-top: 0;
}
.pay-item-name {
  font-size: 15px;
  font-weight: 600;
}
.pay-item-price {
  flex: none;
  font-size: 15px;
  font-variant-numeric: tabular-nums;
}
.pay-wallet {
  margin-bottom: 14px;
  padding: 16px;
  border-radius: 14px;
  background: var(--st-bg-alt);
}
.pay-wallet-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
}
.pay-wallet-value {
  font-size: 16px;
  font-weight: 600;
}
.pay-wallet-toggle {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--st-text-2);
}
.pay-wallet-toggle input {
  accent-color: var(--st-blue);
}
.pay-cached {
  display: grid;
  gap: 6px;
  margin-bottom: 14px;
  padding: 16px;
  border-radius: 14px;
  background: color-mix(in srgb, #b25000 14%, transparent);
  font-size: 13px;
  color: #a35b00;
}
.pay-cached-title {
  font-weight: 600;
}
.pay-cached .st-btn {
  justify-self: start;
  margin-top: 6px;
  border-color: #a35b00;
  color: #a35b00;
}
.pay-inline {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  margin-top: 12px;
}
.pay-aside {
  position: sticky;
  top: 96px;
  padding: 24px;
  border-radius: 18px;
  background: var(--st-bg-alt);
}
.pay-aside-title {
  margin: 0 0 10px;
  font-family: var(--st-font-display);
  font-size: 21px;
  font-weight: 600;
}
.pay-picked {
  margin: 12px 0 0;
  font-size: 13px;
  color: var(--st-green);
}
.pay-alert {
  margin-top: 12px;
  padding: 10px 14px;
  border-radius: 10px;
  font-size: 13px;
}
.pay-alert.is-error {
  background: color-mix(in srgb, #d70015 12%, transparent);
  color: #c9342f;
}
.pay-alert.is-warning {
  background: color-mix(in srgb, #b25000 14%, transparent);
  color: #a35b00;
}
.pay-alert.is-success {
  background: color-mix(in srgb, #008009 14%, transparent);
  color: var(--st-green);
}
.pay-aside .st-btn {
  margin-top: 14px;
}
.pay-refresh {
  margin-top: 10px;
}
</style>
