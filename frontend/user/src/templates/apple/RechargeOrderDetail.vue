<template>
  <div class="st-root st-page">
    <div class="st-container">
      <div class="rc-shell">
        <header class="st-page-head rc-head">
          <div class="rc-head-copy">
            <h1 class="st-page-title">{{ t('rechargeOrder.title') }}</h1>
            <p class="st-page-lead">{{ t('rechargeOrder.subtitle') }}</p>
          </div>
          <RouterLink class="rc-back" to="/me/orders">{{ t('rechargeOrder.backList') }}</RouterLink>
        </header>

        <!-- Loading -->
        <div v-if="loading" class="rc-skeleton" aria-hidden="true">
          <div class="rc-skel rc-skel--line"></div>
          <div class="rc-skel rc-skel--block"></div>
        </div>

        <!-- 不存在 -->
        <div v-else-if="!recharge" class="rc-empty">
          <AlertCircle class="rc-empty-icon" />
          <p>{{ t('rechargeOrder.notFound') }}</p>
          <button type="button" class="st-btn st-btn--primary" @click="loadDetail()">{{ t('errorBoundary.retry') }}</button>
        </div>

        <template v-else>
          <!-- 顶部摘要 -->
          <section class="rc-summary">
            <div class="rc-summary-top">
              <div class="rc-summary-main">
                <p class="rc-kicker">{{ t('personalCenter.wallet.rechargeNoLabel') }}</p>
                <p class="rc-number">{{ recharge.recharge_no }}</p>
                <p class="rc-date">{{ t('rechargeOrder.createdAtLabel') }}：{{ formatDate(recharge.created_at) }}</p>
              </div>
              <div class="rc-summary-side">
                <span class="rc-status" :class="`rc-status--${rechargeStatusVariant(recharge.status)}`">{{ rechargeStatusText(recharge.status) }}</span>
                <p class="rc-total">{{ formatMoney(recharge.amount, recharge.currency) }}</p>
                <p class="rc-summary-label">{{ t('rechargeOrder.rechargeAmount') }}</p>
              </div>
            </div>
          </section>

          <!-- 金额明细 -->
          <section class="rc-section">
            <h2 class="st-section-title">{{ t('rechargeOrder.amountTitle') }}</h2>
            <div class="rc-rows">
              <div class="rc-row">
                <span>{{ t('rechargeOrder.rechargeAmount') }}</span>
                <span>{{ formatMoney(recharge.amount, recharge.currency) }}</span>
              </div>
              <div v-if="customerFeeApplied" class="rc-row rc-row--fee">
                <span>{{ t('payment.feeAmountLabel') }}</span>
                <span>{{ formatMoney(recharge.fee_amount, recharge.currency) }}</span>
              </div>
              <div class="rc-row rc-row--total">
                <span>{{ t('personalCenter.wallet.payAmountLabel') }}</span>
                <span>{{ formatMoney(recharge.payable_amount, recharge.currency) }}</span>
              </div>
            </div>
          </section>

          <!-- 时间 -->
          <section class="rc-section">
            <h2 class="st-section-title">{{ t('rechargeOrder.timeTitle') }}</h2>
            <div class="rc-rows">
              <div class="rc-row">
                <span>{{ t('rechargeOrder.createdAtLabel') }}</span>
                <span>{{ formatDate(recharge.created_at) }}</span>
              </div>
              <div v-if="recharge.paid_at" class="rc-row">
                <span>{{ t('rechargeOrder.paidAtLabel') }}</span>
                <span>{{ formatDate(recharge.paid_at) }}</span>
              </div>
              <div v-if="payment?.expires_at" class="rc-row">
                <span>{{ t('payment.expiresAt') }}</span>
                <span>{{ formatDate(payment.expires_at) }}</span>
              </div>
            </div>
          </section>

          <!-- 备注 -->
          <section v-if="recharge.remark" class="rc-section">
            <h2 class="st-section-title">{{ t('rechargeOrder.remarkLabel') }}</h2>
            <p class="rc-remark">{{ recharge.remark }}</p>
          </section>

          <!-- 支付区域 -->
          <section v-if="isPending" class="rc-section">
            <h2 class="st-section-title">{{ t('rechargeOrder.paymentTitle') }}</h2>
            <p class="rc-hint">{{ t('personalCenter.wallet.pendingHint') }}</p>

            <div class="st-panel rc-payment-panel">
              <div class="rc-pay-grid">
                <div v-if="showQRCode" class="rc-qr">
                  <p class="rc-hint">{{ t('payment.qrTitle') }}</p>
                  <div class="rc-qr-frame"><img :src="qrImageUrl" alt="Recharge QR" /></div>
                  <p v-if="qrUsingPayLinkFallback" class="rc-hint">{{ t('payment.qrFallbackHint') }}</p>
                </div>

                <div class="rc-pay-panel">
                  <div v-if="hasCryptoPaymentDetails" class="rc-crypto">
                    <div v-for="item in cryptoPaymentDetails" :key="item.key" class="rc-crypto-row">
                      <span class="rc-crypto-label">{{ item.label }}</span>
                      <span class="rc-crypto-value">{{ item.value }}<span v-if="item.detail" class="rc-crypto-detail"> ({{ item.detail }})</span></span>
                    </div>
                    <div v-if="cryptoWalletAddress" class="rc-copy-row">
                      <button type="button" class="st-btn st-btn--ghost st-btn--small" @click="handleCopyWalletAddress">{{ t('payment.copyWalletAddress') }}</button>
                      <span v-if="walletAddressCopied" class="rc-ok">{{ t('payment.copied') }}</span>
                    </div>
                  </div>

                  <div class="rc-pay-actions">
                    <button v-if="payLink" type="button" class="st-btn st-btn--ghost st-btn--small" @click="handleOpenPayLink">{{ t('payment.openPayLink') }}</button>
                    <button type="button" class="st-btn st-btn--primary st-btn--small" :disabled="checkingPayment" @click="checkPayment">
                      {{ checkingPayment ? t('personalCenter.wallet.checkingPayStatus') : t('personalCenter.wallet.checkPayStatus') }}
                    </button>
                  </div>

                  <p v-if="showTelegramPayHint" class="rc-hint">{{ t('payment.telegramExternalHint') }}</p>
                </div>
              </div>
            </div>
          </section>

          <!-- 成功 -->
          <section v-if="recharge.status === 'success'" class="rc-section rc-success">
            <CheckCircle2 class="rc-success-icon" />
            <p>{{ t('personalCenter.wallet.rechargeSuccess') }}</p>
          </section>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { AlertCircle, CheckCircle2 } from 'lucide-vue-next'
import { useRechargeOrderDetail } from '../../composables/useRechargeOrderDetail'

const { t } = useI18n()

const {
  loading, checkingPayment, recharge, payment, walletAddressCopied, qrImageUrl,
  isPending, payLink, showTelegramPayHint, qrUsingPayLinkFallback, showQRCode,
  cryptoWalletAddress, cryptoPaymentDetails, hasCryptoPaymentDetails, customerFeeApplied,
  rechargeStatusText, rechargeStatusVariant, formatMoney, formatDate,
  loadDetail, checkPayment, handleOpenPayLink, handleCopyWalletAddress,
} = useRechargeOrderDetail()
</script>

<style scoped>
/* ---------- 页面基础 ---------- */
/* 标题、页头、按钮、面板等通用件继承 store.css 的 .st-* 样式。 */
.st-page {
  --st-line: color-mix(in srgb, var(--st-text) 14%, transparent);
}
.rc-shell {
  width: 100%;
  max-width: 900px;
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
.rc-head {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
  padding: 38px 0 32px;
}
.rc-head-copy {
  min-width: 0;
}
.rc-back {
  margin-top: 8px;
  font-size: 13px;
  line-height: 1.5;
  color: var(--st-link);
  text-decoration: none;
}
.rc-back:hover {
  text-decoration: underline;
}

/* ---------- Loading / 空态 ---------- */
.rc-skeleton {
  display: grid;
  gap: 14px;
  padding: 8px 0 24px;
}
.rc-skel {
  border-radius: 10px;
  background: var(--st-bg-alt);
  animation: rc-pulse 1.4s ease-in-out infinite;
}
.rc-skel--line {
  width: 32%;
  height: 17px;
}
.rc-skel--block {
  width: 100%;
  height: 220px;
  border-radius: 16px;
}
@keyframes rc-pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.55;
  }
}
.rc-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
  padding: 76px 0 36px;
  text-align: center;
  font-size: 14px;
  color: var(--st-text-2);
}
.rc-empty-icon {
  width: 40px;
  height: 40px;
  opacity: 0.3;
}

/* ---------- 顶部摘要 ---------- */
.rc-summary {
  padding-bottom: 26px;
  border-bottom: 1px solid var(--st-line);
}
.rc-summary-top {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
}
.rc-summary-main {
  min-width: 0;
}
.rc-kicker {
  margin: 0 0 8px;
  font-size: 13px;
  line-height: 1.4;
  color: var(--st-text-2);
}
.rc-number {
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
.rc-date {
  margin: 8px 0 0;
  font-size: 13px;
  line-height: 1.5;
  color: var(--st-text-2);
}
.rc-summary-side {
  flex: none;
  text-align: right;
}
.rc-status {
  display: inline-block;
  margin-bottom: 10px;
  font-size: 14px;
  font-weight: 500;
  line-height: 1.4;
}
.rc-status--success {
  color: var(--st-green);
}
.rc-status--warning,
.rc-status--danger {
  color: var(--st-orange);
}
.dark .rc-status--success {
  color: #30d158;
}
.dark .rc-status--warning,
.dark .rc-status--danger {
  color: #ff9f0a;
}
.rc-total {
  margin: 0;
  font-family: var(--st-font-display);
  font-size: 28px;
  font-weight: 600;
  line-height: 1.1;
  letter-spacing: -0.01em;
  color: var(--st-text);
  font-variant-numeric: tabular-nums;
}
.rc-summary-label {
  margin: 6px 0 0;
  font-size: 13px;
  line-height: 1.4;
  color: var(--st-text-2);
}

/* ---------- 细线分区 ---------- */
.rc-section {
  padding: 30px 0;
  border-top: 1px solid var(--st-line);
}
.rc-rows {
  display: flex;
  flex-direction: column;
  margin-top: 16px;
}
.rc-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 24px;
  padding: 13px 0;
  border-bottom: 1px solid var(--st-line);
  font-size: 14px;
  line-height: 1.5;
}
.rc-row:first-child {
  padding-top: 0;
}
.rc-row:last-child {
  padding-bottom: 0;
  border-bottom: 0;
}
.rc-row > span:first-child {
  font-size: 13px;
  color: var(--st-text-2);
}
.rc-row > span:last-child {
  text-align: right;
  font-weight: 500;
  color: var(--st-text);
  font-variant-numeric: tabular-nums;
  overflow-wrap: anywhere;
}
.rc-row--fee > span:last-child {
  color: var(--st-orange);
}
.rc-row--total {
  padding-top: 18px;
}
.rc-row--total > span {
  font-weight: 600;
  color: var(--st-text);
}
.rc-row--total > span:first-child {
  font-size: 14px;
  color: var(--st-text);
}
.rc-row--total > span:last-child {
  font-family: var(--st-font-display);
  font-size: 24px;
  font-weight: 600;
  line-height: 1.15;
  letter-spacing: -0.01em;
}
.rc-remark {
  margin: 14px 0 0;
  max-width: 680px;
  font-size: 14px;
  line-height: 1.6;
  color: var(--st-text-2);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

/* ---------- 支付 ---------- */
.rc-hint {
  margin: 10px 0 0;
  font-size: 13px;
  line-height: 1.5;
  color: var(--st-text-2);
}
.rc-payment-panel {
  padding: 24px;
  margin-top: 20px;
}
.rc-pay-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 28px;
  align-items: start;
}
.rc-qr {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  text-align: center;
}
.rc-qr .rc-hint {
  margin: 0;
}
.rc-qr-frame {
  width: 100%;
  max-width: 220px;
  aspect-ratio: 1 / 1;
  padding: 10px;
  border-radius: 12px;
  background: #fff;
  overflow: hidden;
}
.rc-qr-frame img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: contain;
}
.rc-pay-panel {
  display: flex;
  flex-direction: column;
  gap: 18px;
  min-width: 0;
}
.rc-pay-panel .rc-hint {
  margin: 0;
}
.rc-crypto {
  display: flex;
  flex-direction: column;
}
.rc-crypto-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 20px;
  padding: 11px 0;
  border-bottom: 1px solid var(--st-line);
  font-size: 13px;
  line-height: 1.5;
}
.rc-crypto-row:first-child {
  padding-top: 0;
}
.rc-crypto-label {
  flex: none;
  color: var(--st-text-2);
}
.rc-crypto-value {
  min-width: 0;
  text-align: right;
  font-weight: 500;
  color: var(--st-text);
  overflow-wrap: anywhere;
}
.rc-crypto-detail {
  font-weight: 400;
  color: var(--st-text-3);
}
.rc-copy-row {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  padding-top: 14px;
}
.rc-pay-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}
.rc-ok {
  font-size: 13px;
  color: var(--st-green);
}
.dark .rc-ok {
  color: #30d158;
}

/* ---------- 成功 ---------- */
.rc-success {
  display: flex;
  align-items: center;
  gap: 12px;
  color: var(--st-green);
}
.rc-success-icon {
  flex: none;
  width: 24px;
  height: 24px;
}
.rc-success p {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  line-height: 1.4;
  color: var(--st-green);
}
.dark .rc-success,
.dark .rc-success p {
  color: #30d158;
}

@media (min-width: 735px) {
  .rc-payment-panel {
    padding: 28px;
  }
  .rc-pay-grid {
    grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
  }
}
@media (max-width: 640px) {
  .rc-head {
    flex-direction: column;
    gap: 14px;
    padding: 28px 0 26px;
  }
  .rc-back {
    margin-top: 0;
  }
  .rc-summary-side {
    text-align: left;
  }
  .rc-total {
    font-size: 24px;
  }
  .rc-payment-panel {
    padding: 20px;
  }
  .rc-section {
    padding: 26px 0;
  }
  .rc-row {
    gap: 16px;
  }
}
@media (prefers-reduced-motion: reduce) {
  .rc-skel {
    animation: none;
  }
  .st-btn {
    transition: none;
  }
}
</style>
