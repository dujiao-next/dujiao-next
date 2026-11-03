<template>
  <div class="st-root st-page">
    <div class="st-container">
      <nav class="co-crumbs">
        <RouterLink to="/">{{ t('nav.home') }}</RouterLink>
        <ChevronRight class="co-crumb-icon" />
        <template v-if="!isBuyNowMode">
          <RouterLink to="/cart">{{ t('cart.title') }}</RouterLink>
          <ChevronRight class="co-crumb-icon" />
        </template>
        <span>{{ t('checkout.title') }}</span>
      </nav>

      <header class="st-page-head">
        <h1 class="st-page-title">{{ t('checkout.title') }}</h1>
        <p class="st-page-lead">{{ t('checkout.subtitle') }}</p>
      </header>

      <div class="co-steps">
        <VaultCheckoutSteps current="checkout" :skip-cart="isBuyNowMode" />
      </div>
    </div>

    <!-- 空 -->
    <div v-if="cartItems.length === 0" class="st-container">
      <div class="co-empty">
        <ShoppingCart class="co-empty-icon" />
        <p>{{ t('checkout.empty') }}</p>
        <RouterLink class="st-btn st-btn--primary" to="/products">{{ t('checkout.emptyAction') }}</RouterLink>
      </div>
    </div>

    <div v-else class="st-container co-grid">
      <div class="co-main">
        <!-- 商品确认 -->
        <section class="co-block">
          <h2 class="st-section-title">{{ t('checkout.itemsTitle') }}</h2>
          <div class="co-items">
            <div v-for="item in cartItems" :key="cartItemKey(item)" class="co-item" :class="{ 'is-warn': itemStockExceeded(item) }">
              <div class="co-thumb">
                <img v-if="checkoutItemImage(item)" :src="checkoutItemImage(item)" :alt="getLocalizedText(item.title)" loading="lazy" />
                <Package v-else class="co-thumb-empty" />
              </div>
              <div class="co-item-info">
                <RouterLink :to="`/products/${item.slug}`" class="co-item-name">{{ getLocalizedText(item.title) }}</RouterLink>
                <p class="co-item-meta">{{ t('checkout.quantityLabel') }}：{{ item.quantity }}</p>
                <p v-if="itemSkuDisplay(item)" class="co-item-meta">{{ t('checkout.skuLabel') }}：{{ itemSkuDisplay(item) }}</p>
                <p v-if="itemStockHint(item)" class="co-item-meta" :class="{ 'is-warn': itemStockExceeded(item) }">{{ itemStockHint(item) }}</p>
              </div>
              <div class="co-item-price">
                <span :class="{ 'is-orange': checkoutItemHasPriceDiscount(item) }">
                  {{ checkoutItemPriceParts(item).integer }}<small>{{ checkoutItemPriceParts(item).decimal }}</small>
                  <em>{{ checkoutItemCurrency }}</em>
                </span>
                <span v-if="checkoutItemHasPriceDiscount(item)" class="co-item-was">
                  {{ checkoutItemOriginalPriceParts(item).integer }}{{ checkoutItemOriginalPriceParts(item).decimal }} {{ checkoutItemCurrency }}
                </span>
              </div>
            </div>
          </div>
        </section>

        <!-- 自定义表单 -->
        <CheckoutManualForm
          v-if="manualFormProducts.length"
          :manual-form-products="manualFormProducts"
          v-model="manualFormData"
          :submit-attempted="submitAttempted"
          :get-manual-field-label="getManualFieldLabel"
          :get-manual-field-placeholder="getManualFieldPlaceholder"
          :manual-field-error="manualFieldError"
        />

        <!-- 优惠码 -->
        <section v-if="!isResellerTenant" class="co-block">
          <h2 class="st-section-title">{{ t('checkout.couponTitle') }}</h2>
          <input v-model="couponCode" type="text" class="st-input" :placeholder="t('checkout.couponPlaceholder')" />
        </section>

        <!-- 下单方式 -->
        <section v-if="!userAuthStore.isAuthenticated" class="co-block">
          <h2 class="st-section-title">{{ t('checkout.modeTitle') }}</h2>
          <div class="co-modes">
            <button type="button" class="co-mode" :class="{ 'is-active': checkoutMode === 'guest' }" @click="checkoutMode = 'guest'">{{ t('checkout.guestPurchase') }}</button>
            <RouterLink class="co-mode" to="/auth/login">{{ t('checkout.memberPurchase') }}</RouterLink>
          </div>

          <template v-if="checkoutMode === 'guest'">
            <div class="co-fields">
              <label class="co-field">
                <span class="st-label">{{ t('checkout.guestEmailPlaceholder') }}</span>
                <input v-model="guestEmail" type="email" class="st-input" :placeholder="t('checkout.guestEmailPlaceholder')" />
                <span v-if="guestEmail && !guestEmailValid" class="co-error">{{ t('error.email_invalid') }}</span>
              </label>
              <label class="co-field">
                <span class="st-label">{{ t('checkout.guestPasswordPlaceholder') }}</span>
                <input v-model="guestPassword" type="password" class="st-input" :placeholder="t('checkout.guestPasswordPlaceholder')" />
              </label>
            </div>

            <div v-if="guestCaptchaEnabled" class="co-captcha">
              <p class="st-label">{{ t('auth.common.captchaLabel') }}</p>
              <ImageCaptcha
                v-if="captchaProvider === 'image'"
                ref="guestImageCaptchaRef"
                v-model="guestCaptchaPayload"
                :disabled="submitting"
                @config-stale="handleGuestCaptchaConfigStale"
              />
              <TurnstileCaptcha
                v-else-if="captchaProvider === 'turnstile'"
                ref="guestTurnstileRef"
                v-model="guestTurnstileToken"
                :site-key="guestTurnstileSiteKey"
              />
            </div>

            <div class="co-note">
              <p class="co-note-title">{{ t('checkout.guestInstructions.title') }}</p>
              <ul>
                <li>{{ t('checkout.guestInstructions.email') }}</li>
                <li>{{ t('checkout.guestInstructions.password') }}</li>
              </ul>
            </div>
          </template>
        </section>
      </div>

      <!-- 右栏 -->
      <aside class="co-summary">
        <h2 class="co-summary-title">{{ t('checkout.submitTitle') }}</h2>
        <p class="co-summary-hint">{{ t('checkout.submitHint') }}</p>

        <div class="co-summary-rows">
          <div class="co-row"><span>{{ t('cart.itemsCount') }}</span><span>{{ totalItems }}</span></div>
          <div class="co-row"><span>{{ t('checkout.previewOriginal') }}</span><span>{{ formatPrice(previewOriginal, previewCurrency) }}</span></div>
          <template v-if="!isResellerTenant">
            <div class="co-row"><span>{{ t('checkout.previewCoupon') }}</span><span :class="{ 'is-blue': hasPositiveAmount(previewCoupon) }">{{ formatDiscountPrice(previewCoupon, previewCurrency) }}</span></div>
            <div class="co-row"><span>{{ t('checkout.previewPromotion') }}</span><span :class="{ 'is-blue': hasPositiveAmount(previewPromotion) }">{{ formatDiscountPrice(previewPromotion, previewCurrency) }}</span></div>
            <div class="co-row"><span>{{ t('checkout.previewWholesale') }}</span><span :class="{ 'is-blue': hasPositiveAmount(previewWholesale) }">{{ formatDiscountPrice(previewWholesale, previewCurrency) }}</span></div>
          </template>
          <div v-if="Number(previewMemberDiscount) > 0" class="co-row"><span>{{ t('checkout.previewMemberDiscount') }}</span><span class="is-orange">-{{ formatPrice(previewMemberDiscount, previewCurrency) }}</span></div>
          <div class="co-row co-row--total"><span>{{ t('checkout.previewTotal') }}</span><span>{{ formatPrice(previewTotal, previewCurrency) }}</span></div>
        </div>

        <div v-if="previewLoading || couponRefreshing" class="co-status">{{ previewStatusText }}</div>
        <div v-if="checkoutAlert" class="co-alert" :class="checkoutAlert.level === 'error' ? 'is-error' : 'is-warn'">{{ checkoutAlert.message }}</div>

        <!-- 支付方式 -->
        <div class="co-pay">
          <h3 class="co-pay-title">{{ t('checkout.paymentMethod') }}</h3>

          <div v-if="showBalanceOption" class="co-wallet">
            <div class="co-wallet-row">
              <div>
                <div class="co-wallet-label">{{ t('payment.walletBalanceLabel') }}</div>
                <div class="co-wallet-value">{{ walletLoading ? t('common.loading') : formatPrice(walletBalance, previewCurrency) }}</div>
              </div>
              <label class="co-wallet-toggle">
                <input v-model="useBalance" type="checkbox" :disabled="walletOnlyPayment" />
                <span>{{ t('payment.useBalance') }}</span>
              </label>
            </div>
            <div v-if="walletOnlyPayment" class="co-wallet-note">{{ t('payment.walletOnlyHint') }}</div>
            <div v-if="useBalance" class="co-wallet-detail">
              <div>{{ t('payment.walletDeductLabel') }}：{{ expectedWalletPaidDisplay }}</div>
              <div v-if="!walletOnlyPayment">{{ t('payment.onlinePayLabel') }}：{{ expectedOnlinePayDisplay }}</div>
              <div v-if="walletOnlyPayment && expectedOnlinePayCents > 0" class="co-wallet-note">{{ t('payment.walletInsufficientHint') }}</div>
            </div>
          </div>

          <template v-if="!walletOnlyPayment">
            <div v-if="requiresOnlineChannel && paymentChannels.length > 0" class="co-channels">
              <button
                v-for="channel in paymentChannels"
                :key="channel.id"
                type="button"
                class="co-channel"
                :class="{ 'is-active': selectedChannelId === channel.id && !isChannelDisabledForAmount(channel), 'is-disabled': isChannelDisabledForAmount(channel) }"
                :disabled="isChannelDisabledForAmount(channel)"
                :title="isChannelDisabledForAmount(channel) ? channelAmountLimitHint(channel) : ''"
                @click="handleSelectChannel(channel)"
              >
                <span class="co-channel-name">
                  <img v-if="channel.icon" :src="getImageUrl(channel.icon)" loading="lazy" />
                  {{ channel.name }}
                </span>
                <span v-if="channel.fee_policy === 'customer_surcharge'" class="co-channel-fee">
                  <span>{{ t('payment.feeLabel') }}：{{ formatChannelFeeRate(channel) }}</span>
                  <span>{{ t('payment.fixedFeeLabel') }}：{{ formatChannelFixedFee(channel) }}</span>
                </span>
                <span v-if="isChannelDisabledForAmount(channel)" class="co-channel-fee">{{ channelAmountLimitHint(channel) }}</span>
              </button>
            </div>
            <div v-else-if="requiresOnlineChannel && paymentChannels.length === 0" class="co-status">{{ t('checkout.noPaymentChannels') }}</div>
          </template>
          <div v-if="!requiresOnlineChannel" class="co-status is-ok">{{ t('checkout.walletCoversAll') }}</div>
        </div>

        <button type="button" class="st-btn st-btn--primary st-btn--block" :disabled="!canSubmit" @click="handleSubmit">
          {{ submitting ? t('checkout.submitting') : t('checkout.submitButton') }}
        </button>
      </aside>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ChevronRight, Package, ShoppingCart } from 'lucide-vue-next'
import ImageCaptcha from '../../components/captcha/ImageCaptcha.vue'
import TurnstileCaptcha from '../../components/captcha/TurnstileCaptcha.vue'
import CheckoutManualForm from '../../components/checkout/CheckoutManualForm.vue'
import VaultCheckoutSteps from './components/VaultCheckoutSteps.vue'
import { useCheckout } from '../../composables/useCheckout'

const { t } = useI18n()

const {
  userAuthStore, getLocalizedText, formatPrice, getImageUrl,
  isBuyNowMode, cartItems, totalItems, cartItemKey, checkoutItemImage, itemSkuDisplay,
  itemStockExceeded, itemStockHint,
  checkoutItemCurrency, checkoutItemPriceParts, checkoutItemOriginalPriceParts, checkoutItemHasPriceDiscount,
  manualFormProducts, manualFormData, submitAttempted, getManualFieldLabel, getManualFieldPlaceholder, manualFieldError,
  couponCode, isResellerTenant,
  checkoutMode, guestEmail, guestPassword, guestEmailValid,
  guestCaptchaEnabled, captchaProvider, guestCaptchaPayload, guestTurnstileToken, guestTurnstileSiteKey,
  guestImageCaptchaRef, guestTurnstileRef, handleGuestCaptchaConfigStale,
  previewCurrency, previewOriginal, previewCoupon, previewPromotion, previewWholesale, previewMemberDiscount, previewTotal,
  previewLoading, couponRefreshing, previewStatusText, hasPositiveAmount, formatDiscountPrice, checkoutAlert,
  showBalanceOption, walletLoading, walletBalance, useBalance, walletOnlyPayment,
  expectedWalletPaidDisplay, expectedOnlinePayDisplay, expectedOnlinePayCents,
  requiresOnlineChannel, paymentChannels, selectedChannelId, isChannelDisabledForAmount, channelAmountLimitHint,
  handleSelectChannel, formatChannelFeeRate, formatChannelFixedFee,
  submitting, canSubmit, handleSubmit,
} = useCheckout()

// 字符串模板 ref，逻辑在 composable 内，显式标记避免 noUnusedLocals 误报。
void guestImageCaptchaRef
void guestTurnstileRef
</script>

<style scoped>
.co-crumbs {
  display: flex;
  align-items: center;
  gap: 6px;
  padding-top: 20px;
  font-size: 12px;
  color: var(--st-text-2);
}
.co-crumbs a {
  color: var(--st-text-2);
  text-decoration: none;
}
.co-crumb-icon {
  width: 12px;
  height: 12px;
  opacity: 0.5;
}
.co-steps {
  padding-top: 20px;
  padding-bottom: 28px;
}

.co-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
  padding: 90px 0;
  text-align: center;
  color: var(--st-text-2);
}
.co-empty-icon {
  width: 44px;
  height: 44px;
  opacity: 0.35;
}

.co-grid {
  display: grid;
  gap: 32px;
}
@media (min-width: 900px) {
  .co-grid {
    grid-template-columns: minmax(0, 1fr) 360px;
    gap: 56px;
    align-items: start;
  }
}
.co-main {
  min-width: 0;
}
.co-block {
  padding-block: 28px;
  border-top: 1px solid color-mix(in srgb, var(--st-text) 10%, transparent);
}
.co-block:first-child {
  padding-top: 8px;
  border-top: 0;
}

/* 商品行 */
.co-items {
  display: flex;
  flex-direction: column;
  gap: 18px;
}
.co-item {
  display: flex;
  align-items: center;
  gap: 16px;
}
.co-thumb {
  display: grid;
  place-items: center;
  width: 72px;
  height: 72px;
  flex: none;
  border-radius: 12px;
  overflow: hidden;
  background: var(--st-bg-alt);
}
.co-thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.co-thumb-empty {
  width: 26px;
  height: 26px;
  opacity: 0.3;
}
.co-item-info {
  flex: 1;
  min-width: 0;
}
.co-item-name {
  font-size: 16px;
  font-weight: 600;
  color: var(--st-text);
  text-decoration: none;
}
.co-item-meta {
  margin: 3px 0 0;
  font-size: 13px;
  color: var(--st-text-2);
}
.co-item-meta.is-warn {
  color: var(--st-orange);
}
.co-item-price {
  flex: none;
  text-align: right;
  font-size: 17px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}
.co-item-price small {
  font-size: 12px;
}
.co-item-price em {
  margin-left: 3px;
  font-size: 11px;
  font-style: normal;
  color: var(--st-text-2);
}
.co-item-price .is-orange {
  color: var(--st-orange);
}
.co-item-was {
  display: block;
  margin-top: 2px;
  font-size: 12px;
  font-weight: 400;
  color: var(--st-text-3);
  text-decoration: line-through;
}

/* 下单方式 */
.co-modes {
  display: flex;
  gap: 8px;
  margin-bottom: 18px;
}
.co-mode {
  padding: 8px 16px;
  border: 1px solid color-mix(in srgb, var(--st-text) 14%, transparent);
  border-radius: 980px;
  background: none;
  color: var(--st-text);
  font-size: 14px;
  text-decoration: none;
  cursor: pointer;
}
.co-mode.is-active {
  background: var(--st-text);
  border-color: var(--st-text);
  color: var(--st-bg);
}
.co-fields {
  display: grid;
  gap: 16px;
}
@media (min-width: 735px) {
  .co-fields {
    grid-template-columns: 1fr 1fr;
  }
}
.co-field {
  display: block;
}
.co-error {
  display: block;
  margin-top: 6px;
  font-size: 13px;
  color: #c9342f;
}
.co-captcha {
  margin-top: 18px;
}
.co-note {
  margin-top: 18px;
  padding: 14px 16px;
  border-radius: 12px;
  background: var(--st-bg-alt);
  font-size: 13px;
  color: var(--st-text-2);
}
.co-note-title {
  margin: 0 0 6px;
  font-weight: 600;
  color: var(--st-text);
}
.co-note ul {
  margin: 0;
  padding-left: 18px;
}

/* 汇总 */
.co-summary {
  position: sticky;
  top: 96px;
  padding: 24px;
  border-radius: 18px;
  background: var(--st-bg-alt);
}
.co-summary-title {
  margin: 0 0 6px;
  font-family: var(--st-font-display);
  font-size: 21px;
  font-weight: 600;
}
.co-summary-hint {
  margin: 0 0 16px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--st-text-3);
}
.co-summary-rows {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.co-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  font-size: 14px;
  color: var(--st-text-2);
}
.co-row--total {
  margin-top: 6px;
  padding-top: 14px;
  border-top: 1px solid color-mix(in srgb, var(--st-text) 12%, transparent);
  font-size: 16px;
  font-weight: 600;
  color: var(--st-text);
}
.co-row--total span:last-child {
  font-size: 23px;
  font-variant-numeric: tabular-nums;
}
.co-row .is-blue {
  color: var(--st-link);
}
.co-row .is-orange {
  color: var(--st-orange);
}
.co-status {
  margin-top: 10px;
  font-size: 12px;
  color: var(--st-text-3);
}
.co-status.is-ok {
  color: var(--st-green);
}
.co-alert {
  margin-top: 12px;
  padding: 10px 14px;
  border-radius: 10px;
  font-size: 13px;
}
.co-alert.is-error {
  background: color-mix(in srgb, #d70015 12%, transparent);
  color: #c9342f;
}
.co-alert.is-warn {
  background: color-mix(in srgb, #b25000 14%, transparent);
  color: #a35b00;
}

/* 支付方式 */
.co-pay {
  margin-top: 18px;
  padding-top: 18px;
  border-top: 1px solid color-mix(in srgb, var(--st-text) 12%, transparent);
}
.co-pay-title {
  margin: 0 0 12px;
  font-size: 14px;
  font-weight: 600;
}
.co-wallet {
  margin-bottom: 14px;
  padding: 14px;
  border-radius: 12px;
  background: var(--st-bg);
}
.co-wallet-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}
.co-wallet-label {
  font-size: 12px;
  color: var(--st-text-3);
}
.co-wallet-value {
  font-size: 16px;
  font-weight: 600;
}
.co-wallet-toggle {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--st-text-2);
  cursor: pointer;
}
.co-wallet-toggle input {
  accent-color: var(--st-blue);
}
.co-wallet-note,
.co-wallet-detail {
  margin-top: 8px;
  font-size: 12px;
  color: var(--st-text-3);
}
.co-channels {
  display: grid;
  gap: 8px;
}
@media (min-width: 735px) {
  .co-channels {
    grid-template-columns: 1fr 1fr;
  }
}
.co-channel {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
  padding: 12px 14px;
  border: 1px solid color-mix(in srgb, var(--st-text) 14%, transparent);
  border-radius: 12px;
  background: var(--st-bg);
  cursor: pointer;
  text-align: left;
  font-size: 14px;
  color: var(--st-text);
}
.co-channel.is-active {
  border-color: var(--st-text);
  box-shadow: inset 0 0 0 1px var(--st-text);
}
.co-channel.is-disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.co-channel-name {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
}
.co-channel-name img {
  width: 18px;
  height: 18px;
  object-fit: contain;
  border-radius: 4px;
}
.co-channel-fee {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 11px;
  color: var(--st-orange);
}
.co-summary .st-btn {
  margin-top: 18px;
}
</style>
