<template>
  <div class="mx-auto w-full max-w-[1180px] px-4 pb-10 sm:px-6">
    <div class="my-5 flex flex-wrap items-start justify-between gap-4">
      <div class="flex flex-col gap-2">
        <h1 class="text-[clamp(1.75rem,1.3rem+1.9vw,2.6rem)] font-semibold tracking-[-0.022em]">{{ t('payment.title') }}</h1>
        <p class="text-[15px] text-muted-foreground">{{ t('payment.subtitle') }}</p>
      </div>
      <RouterLink class="text-[13px] text-muted-foreground transition-colors hover:text-foreground" :to="backLink">
        {{ t('payment.backToOrders') }}
      </RouterLink>
    </div>

    <!-- 加载 -->
    <div v-if="loading" class="flex flex-col gap-4">
      <div class="h-[180px] rounded-[var(--r-lg)] border border-border bg-card"></div>
      <div class="h-[240px] rounded-[var(--r-lg)] border border-border bg-card"></div>
    </div>

    <!-- 游客身份验证 -->
    <section v-else-if="showGuestAuthForm" class="rounded-[var(--r-lg)] border border-border bg-card p-6">
      <h2 class="mb-1.5 text-[15px] font-semibold">{{ t('payment.guestAuthTitle') }}</h2>
      <p class="mb-4 text-[13px] text-muted-foreground">{{ t('payment.guestAuthHint') }}</p>
      <div class="grid gap-3 sm:grid-cols-2">
        <input
          v-model="guestAuth.email"
          type="email"
          class="h-11 rounded-[var(--r)] border border-border bg-[color:var(--surface-2)] px-4 text-[14px] outline-none transition-colors focus:border-[color:var(--accent)] focus:bg-card focus:ring-[var(--ring)]"
          :placeholder="t('guestOrders.emailPlaceholder')"
        />
        <input
          v-model="guestAuth.order_password"
          type="password"
          class="h-11 rounded-[var(--r)] border border-border bg-[color:var(--surface-2)] px-4 text-[14px] outline-none transition-colors focus:border-[color:var(--accent)] focus:bg-card focus:ring-[var(--ring)]"
          :placeholder="t('guestOrders.passwordPlaceholder')"
        />
      </div>
      <p v-if="guestAuthError" class="mt-3.5 rounded-[var(--r)] bg-[color:var(--danger-soft)] px-3.5 py-2.5 text-[13px] text-[color:var(--danger-strong)]">
        {{ guestAuthError }}
      </p>
      <Button size="sm" class="mt-4 rounded-full" @click="handleGuestAuthSubmit">{{ t('payment.guestAuthSubmit') }}</Button>
    </section>

    <!-- 订单不存在 -->
    <div v-else-if="!order" class="flex flex-col items-center gap-3 rounded-[var(--r-lg)] border border-dashed border-border py-20 text-center">
      <AlertCircle class="h-11 w-11 text-muted-foreground" />
      <p class="font-medium">{{ t('payment.orderNotFound') }}</p>
      <Button as-child size="sm" class="mt-2 rounded-full"><RouterLink :to="backLink">{{ t('payment.backToOrders') }}</RouterLink></Button>
    </div>

    <!-- 支付结果视图 -->
    <section v-else-if="showResultView" class="rounded-[var(--r-lg)] border border-border bg-card p-6">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div class="flex flex-col gap-1">
          <h2 class="text-[16px] font-semibold">{{ paymentResultTitle }}</h2>
          <p class="text-[13px] text-muted-foreground">{{ paymentGuideTip }}</p>
          <p class="text-[12px] text-muted-foreground">{{ t('payment.methodLabel') }}：{{ resultChannelName }}</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <Button variant="outline" size="sm" class="rounded-full" :disabled="loading" @click="handleRefresh">{{ t('payment.refreshStatus') }}</Button>
          <Button variant="outline" size="sm" class="rounded-full" @click="handleChangePaymentMethod">{{ t('payment.changeMethod') }}</Button>
        </div>
      </div>

      <div class="mt-6 grid gap-6 lg:grid-cols-[1.4fr_1fr]">
        <div>
          <!-- 二维码 -->
          <div v-if="showQRCode" class="flex flex-col items-center rounded-[var(--r-lg)] border border-border bg-[color:var(--surface-2)] p-6 text-center">
            <div class="mb-3 text-[13px] text-muted-foreground">{{ paymentGuideTitle }}</div>
            <div class="aspect-square w-full max-w-[240px] overflow-hidden rounded-[var(--r)] bg-white p-3">
              <img :src="qrImageUrl" alt="QR Code" class="h-full w-full object-contain" />
            </div>
            <div v-if="qrUsingPayLinkFallback" class="mt-2.5 text-[12px] text-muted-foreground">{{ t('payment.qrFallbackHint') }}</div>

            <div v-if="hasCryptoPaymentDetails" class="mt-4 grid w-full gap-2 rounded-[var(--r)] border border-border bg-card p-3.5 text-left">
              <div v-for="item in cryptoPaymentDetails" :key="item.key" class="flex justify-between gap-3 border-b border-[color:var(--border-soft)] pb-1.5 last:border-b-0 last:pb-0">
                <span class="flex-none text-[12px] text-muted-foreground">{{ item.label }}</span>
                <span class="break-all text-right text-[13px] font-medium">
                  {{ item.value }}<span v-if="item.detail" class="text-muted-foreground"> ({{ item.detail }})</span>
                </span>
              </div>
              <div v-if="cryptoWalletAddress" class="flex items-center justify-end gap-2 pt-1.5">
                <Button variant="outline" size="sm" class="rounded-full" @click="handleCopyWalletAddress">{{ t('payment.copyWalletAddress') }}</Button>
                <span v-if="walletAddressCopied" class="text-[12px] text-[color:var(--success-strong)]">{{ t('payment.copied') }}</span>
              </div>
            </div>
          </div>

          <!-- 跳转支付 -->
          <div v-else class="rounded-[var(--r-lg)] border border-border bg-[color:var(--surface-2)] p-6">
            <div class="mb-3 text-[13px] text-muted-foreground">{{ t('payment.openPayLink') }}</div>
            <div class="flex flex-wrap items-center gap-3">
              <Button size="sm" class="rounded-full" @click="handleOpenPayLink">{{ t('payment.openPayLink') }}</Button>
              <Button variant="outline" size="sm" class="rounded-full" @click="handleCopyPayLink">{{ t('payment.copyPayLink') }}</Button>
              <span v-if="copied" class="text-[12px] text-[color:var(--success-strong)]">{{ t('payment.copied') }}</span>
            </div>
            <div v-if="openedPayWindow" class="mt-2.5 text-[12px] text-[color:var(--success-strong)]">{{ payLinkOpenedTip }}</div>
            <div v-if="showTelegramPayHint" class="mt-2.5 text-[12px] text-muted-foreground">{{ t('payment.telegramExternalHint') }}</div>
          </div>
        </div>

        <div class="flex flex-col gap-3.5">
          <div class="rounded-[var(--r)] border border-border bg-[color:var(--surface-2)] p-3.5">
            <div class="text-[12px] text-muted-foreground">{{ t('payment.orderNo') }}</div>
            <div class="mt-1 font-semibold">{{ order.order_no }}</div>
            <div class="mt-2.5 text-[12px] text-muted-foreground">{{ t('payment.orderStatus') }}：{{ statusLabel(order.status) }}</div>
            <div class="mt-1 text-[12px] text-muted-foreground">{{ t('payment.methodLabel') }}：{{ resultChannelName }}</div>
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

          <div v-if="paymentResult.expires_at" class="rounded-[var(--r)] border border-border bg-[color:var(--surface-2)] p-3.5 text-[12px] text-muted-foreground">
            {{ t('payment.expiresAt') }}：{{ formatDate(paymentResult.expires_at) }}
          </div>
        </div>
      </div>
    </section>

    <!-- 已过期 / 已取消 -->
    <section v-else-if="orderExpired || orderCanceled" class="rounded-[var(--r-lg)] border border-border bg-card p-6">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div class="flex flex-col gap-1">
          <h2 class="text-[16px] font-semibold">{{ orderCanceled ? t('payment.orderCanceled') : t('payment.orderExpired') }}</h2>
          <p class="text-[13px] text-muted-foreground">{{ order.order_no }}</p>
        </div>
        <Button as-child variant="outline" size="sm" class="rounded-full"><RouterLink :to="backLink">{{ t('payment.backToOrders') }}</RouterLink></Button>
      </div>
      <div class="mt-5 grid gap-3 sm:grid-cols-3">
        <div class="rounded-[var(--r)] border border-border bg-[color:var(--surface-2)] p-3.5">
          <div class="text-[12px] text-muted-foreground">{{ t('payment.orderNo') }}</div>
          <div class="mt-1 font-medium">{{ order.order_no }}</div>
        </div>
        <div class="rounded-[var(--r)] border border-border bg-[color:var(--surface-2)] p-3.5">
          <div class="text-[12px] text-muted-foreground">{{ t('payment.orderStatus') }}</div>
          <div class="mt-1 font-medium">{{ statusLabel(order.status) }}</div>
        </div>
        <div class="rounded-[var(--r)] border border-border bg-[color:var(--surface-2)] p-3.5">
          <div class="text-[12px] text-muted-foreground">{{ t('orderDetail.amountTotal') }}</div>
          <div class="mt-1 font-medium tabular-nums">{{ formatMoney(order.total_amount, order.currency) }}</div>
        </div>
      </div>
    </section>

    <!-- 默认：选择渠道并支付 -->
    <div v-else class="grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_360px]">
      <div class="flex flex-col gap-4">
        <EmberCheckoutSteps current="payment" />
        <!-- 订单信息 -->
        <section class="rounded-[var(--r-lg)] border border-border bg-card p-5">
          <h2 class="mb-4 text-[15px] font-semibold tracking-[-0.015em]">{{ t('payment.orderInfo') }}</h2>

          <div class="flex flex-wrap items-start justify-between gap-5">
            <div>
              <div class="text-[12px] text-muted-foreground">{{ t('payment.orderNo') }}</div>
              <div class="mt-1 font-semibold">{{ order.order_no }}</div>
              <div class="mt-2 text-[12px] text-muted-foreground">{{ t('orderDetail.createdAtLabel') }}：{{ formatDate(order.created_at) }}</div>
            </div>

            <div class="min-w-[260px] flex-1 rounded-[var(--r)] bg-[color:var(--surface-2)] p-4">
              <div class="text-[12px] text-muted-foreground">{{ t('payment.payableAmountLabel') }}</div>
              <div class="mb-3 mt-1 text-[26px] font-semibold tabular-nums tracking-[-0.04em]">{{ payableAmountDisplay }}</div>
              <div class="flex flex-col gap-1.5 text-[12.5px]">
                <div class="flex justify-between gap-3">
                  <span class="text-muted-foreground">{{ t('orderDetail.amountTotal') }}</span>
                  <span class="font-medium tabular-nums">{{ formatMoney(order.total_amount, order.currency) }}</span>
                </div>
                <template v-if="showBalanceOption && useBalance">
                  <div class="flex justify-between gap-3">
                    <span class="text-muted-foreground">{{ t('payment.walletDeductLabel') }}</span>
                    <span class="font-medium tabular-nums">{{ expectedWalletPaidDisplay }}</span>
                  </div>
                  <div class="flex justify-between gap-3">
                    <span class="text-muted-foreground">{{ t('payment.onlinePayLabel') }}</span>
                    <span class="font-medium tabular-nums">{{ expectedOnlinePayDisplay }}</span>
                  </div>
                </template>
                <div class="flex justify-between gap-3 border-t border-border pt-2">
                  <span class="text-muted-foreground">{{ t('payment.orderStatus') }}</span>
                  <span class="font-medium">{{ statusLabel(order.status) }}</span>
                </div>
              </div>
            </div>
          </div>

          <div class="mt-4 grid gap-3 sm:grid-cols-3">
            <div class="rounded-[var(--r)] border border-[color:var(--border-soft)] bg-[color:var(--surface-2)] p-3.5">
              <div class="text-[12px] text-muted-foreground">{{ t('orderDetail.amountOriginal') }}</div>
              <div class="mt-1 font-medium tabular-nums">{{ formatMoney(order.original_amount, order.currency) }}</div>
            </div>
            <div class="rounded-[var(--r)] border border-[color:var(--border-soft)] bg-[color:var(--surface-2)] p-3.5">
              <div class="text-[12px] text-muted-foreground">{{ t('orderDetail.amountDiscount') }}</div>
              <div class="mt-1 font-medium tabular-nums" :class="hasDiscountAmount(order.discount_amount) ? 'text-[color:var(--accent-strong)]' : ''">
                {{ formatDiscountMoney(order.discount_amount, order.currency) }}
              </div>
            </div>
            <div class="rounded-[var(--r)] border border-[color:var(--border-soft)] bg-[color:var(--surface-2)] p-3.5">
              <div class="text-[12px] text-muted-foreground">{{ t('orderDetail.promotionDiscountLabel') }}</div>
              <div class="mt-1 font-medium tabular-nums" :class="hasDiscountAmount(order.promotion_discount_amount) ? 'text-[color:var(--accent-strong)]' : ''">
                {{ formatDiscountMoney(order.promotion_discount_amount, order.currency) }}
              </div>
            </div>
            <div
              v-if="hasDiscountAmount(order.wholesale_discount_amount)"
              class="rounded-[var(--r)] border border-[color:var(--success)] bg-[color:var(--success-soft)] p-3.5"
            >
              <div class="text-[12px] text-[color:var(--success-strong)]">{{ t('orderDetail.amountWholesaleDiscount') }}</div>
              <div class="mt-1 font-medium tabular-nums text-[color:var(--success-strong)]">{{ formatDiscountMoney(order.wholesale_discount_amount, order.currency) }}</div>
            </div>
          </div>

          <div v-if="order.expires_at" class="mt-3 text-[13px] text-muted-foreground">{{ t('payment.expiresAt') }}：{{ formatDate(order.expires_at) }}</div>

          <div
            v-if="showCountdown"
            class="mt-3 inline-flex items-center gap-2 rounded-full px-3.5 py-1.5 text-[13px] tabular-nums"
            :class="countdownExpired ? 'bg-[color:var(--danger-soft)] text-[color:var(--danger-strong)]' : 'bg-[color:var(--surface-2)] text-muted-foreground'"
          >
            <span>{{ t('payment.countdownLabel') }}</span>
            <span>{{ countdownText }}</span>
          </div>
          <div v-if="pollingActive" class="mt-2.5 text-[12px] text-muted-foreground">{{ t('payment.pollingHint') }}</div>
        </section>

        <!-- 商品 -->
        <section v-if="orderItems.length" class="rounded-[var(--r-lg)] border border-border bg-card p-5">
          <h2 class="mb-4 text-[15px] font-semibold tracking-[-0.015em]">{{ t('payment.itemsTitle') }}</h2>
          <div class="flex flex-col gap-3">
            <div v-for="(item, idx) in orderItems" :key="idx" class="flex flex-wrap justify-between gap-3 border-b border-[color:var(--border-soft)] pb-3 last:border-b-0 last:pb-0">
              <div>
                <div class="text-[14px] font-medium">{{ getLocalizedText(item.title) }}</div>
                <div class="mt-0.5 text-[12px] text-muted-foreground">
                  {{ t('orderDetail.quantityLabel') }}：{{ item.quantity }} · {{ t('orderDetail.itemFulfillmentLabel') }}：{{ fulfillmentTypeLabelText(item.fulfillment_type) }}
                </div>
                <div v-if="orderItemSkuText(item)" class="mt-0.5 text-[12px] text-muted-foreground">{{ t('orderDetail.itemSkuLabel') }}：{{ orderItemSkuText(item) }}</div>
              </div>
              <div class="text-[13px] tabular-nums text-muted-foreground">{{ formatMoney(item.total_price, order.currency) }}</div>
            </div>
          </div>
        </section>

        <!-- 支付渠道 -->
        <section class="rounded-[var(--r-lg)] border border-border bg-card p-5">
          <h2 class="mb-4 text-[15px] font-semibold tracking-[-0.015em]">{{ t('payment.channelTitle') }}</h2>

          <div v-if="!configReady" class="text-[13px] text-muted-foreground">{{ t('common.loading') }}</div>
          <template v-else>
            <div v-if="showBalanceOption" class="mb-3 rounded-[var(--r)] border border-[color:var(--border-soft)] bg-[color:var(--surface-2)] p-3.5">
              <div class="flex items-start justify-between gap-2.5">
                <div>
                  <div class="text-[12px] text-muted-foreground">{{ t('payment.walletBalanceLabel') }}</div>
                  <div class="mt-0.5 font-semibold">{{ walletLoading ? t('common.loading') : walletBalanceDisplay }}</div>
                </div>
                <label class="inline-flex items-center gap-1.5 text-[12px] text-muted-foreground">
                  <input v-model="useBalance" type="checkbox" class="h-4 w-4 accent-[var(--ui-accent)]" :disabled="walletOnlyPayment" />
                  <span>{{ t('payment.useBalance') }}</span>
                </label>
              </div>
              <div v-if="walletOnlyPayment" class="mt-2 text-[12px] text-[color:var(--warning-strong)]">{{ t('payment.walletOnlyHint') }}</div>
              <div v-if="useBalance" class="mt-2 flex flex-col gap-0.5 text-[12px] text-muted-foreground">
                <div>{{ t('payment.walletDeductLabel') }}：{{ expectedWalletPaidDisplay }}</div>
                <div v-if="!walletOnlyPayment">{{ t('payment.onlinePayLabel') }}：{{ expectedOnlinePayDisplay }}</div>
                <div v-if="walletOnlyPayment && expectedOnlinePayCents > 0" class="text-[color:var(--warning-strong)]">{{ t('payment.walletInsufficientHint') }}</div>
              </div>
            </div>

            <div v-if="cachedPayment" class="mb-3 flex flex-col gap-1.5 rounded-[var(--r)] border border-[color:var(--warning)] bg-[color:var(--warning-soft)] p-3.5 text-[13px] text-[color:var(--warning-strong)]">
              <div class="font-semibold">{{ t('payment.cachedTitle') }}</div>
              <div>{{ t('payment.cachedHint', { channel: cachedChannelName }) }}</div>
              <div class="mt-1.5">
                <Button variant="outline" size="sm" class="rounded-full" @click="restoreCachedPayment">{{ t('payment.useCached') }}</Button>
              </div>
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
        <section v-if="paymentResult" class="rounded-[var(--r-lg)] border border-border bg-card p-5">
          <h2 class="mb-4 text-[15px] font-semibold tracking-[-0.015em]">{{ t('payment.infoTitle') }}</h2>
          <div class="flex flex-col gap-1.5 text-[13px] text-muted-foreground">
            <div>{{ t('payment.methodLabel') }}：{{ resultChannelName }}</div>
            <div>{{ t('payment.interactionLabel') }}：{{ interactionLabel }}</div>
            <div v-if="paymentResult.expires_at">{{ t('payment.expiresAt') }}：{{ formatDate(paymentResult.expires_at) }}</div>
          </div>
          <div v-if="showPayLink" class="mt-4 flex flex-wrap gap-2">
            <Button size="sm" class="rounded-full" @click="handleOpenPayLink">{{ t('payment.openPayLink') }}</Button>
            <Button variant="outline" size="sm" class="rounded-full" @click="handleCopyPayLink">{{ t('payment.copyPayLink') }}</Button>
          </div>
        </section>
      </div>

      <!-- 右栏 -->
      <aside class="sticky top-[92px] rounded-[var(--r-lg)] border border-border bg-card p-5">
        <h2 class="mb-3.5 text-[15px] font-semibold tracking-[-0.015em]">{{ t('payment.actionTitle') }}</h2>

        <div v-if="showCountdown" class="mb-3 text-[12px] text-muted-foreground">
          {{ t('payment.countdownLabel') }}：<span class="tabular-nums">{{ countdownText }}</span>
        </div>

        <div
          v-if="paymentAlert"
          class="mb-3 rounded-[var(--r)] px-3.5 py-2.5 text-[13px] leading-[1.6]"
          :class="paymentAlert.level === 'error'
            ? 'bg-[color:var(--danger-soft)] text-[color:var(--danger-strong)]'
            : paymentAlert.level === 'success'
              ? 'bg-[color:var(--success-soft)] text-[color:var(--success-strong)]'
              : 'bg-[color:var(--warning-soft)] text-[color:var(--warning-strong)]'"
        >{{ paymentAlert.message }}</div>

        <div v-if="selectedChannel" class="mb-3 rounded-[var(--r)] bg-[color:var(--accent-soft)] px-3.5 py-2.5 text-[12.5px] font-medium text-[color:var(--accent-strong)]">
          {{ t('payment.methodLabel') }}：{{ selectedChannelName }}
        </div>
        <div v-else-if="!requiresOnlineChannel && !orderExpired && !orderCanceled" class="mb-3 rounded-[var(--r)] bg-[color:var(--success-soft)] px-3.5 py-2.5 text-[12.5px] font-medium text-[color:var(--success-strong)]">
          {{ t('payment.walletPayOnly') }}
        </div>
        <div v-else-if="walletOnlyPayment && expectedOnlinePayCents > 0 && !orderExpired && !orderCanceled" class="mb-3 rounded-[var(--r)] bg-[color:var(--warning-soft)] px-3.5 py-2.5 text-[12.5px] font-medium text-[color:var(--warning-strong)]">
          {{ t('payment.walletInsufficientHint') }}
        </div>
        <div v-else-if="!walletOnlyPayment && requiresOnlineChannel && !orderExpired && !orderCanceled" class="mb-3 rounded-[var(--r)] bg-[color:var(--warning-soft)] px-3.5 py-2.5 text-[12.5px] font-medium text-[color:var(--warning-strong)]">
          {{ t('payment.selectChannelError') }}
        </div>

        <Button class="h-12 w-full rounded-full text-[15px] font-medium" :disabled="!canSubmitPayment" @click="handlePayment">
          {{ submitting ? t('payment.submitting') : t('payment.submitButton') }}
        </Button>
        <Button variant="outline" class="mt-2.5 h-12 w-full rounded-full border-[color:var(--border-strong)] text-[15px] font-medium" :disabled="loading" @click="handleRefresh">
          {{ t('payment.refreshStatus') }}
        </Button>
      </aside>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { AlertCircle } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import PaymentAmountBreakdown from '../../components/payment/PaymentAmountBreakdown.vue'
import PaymentChannelSelector from '../../components/payment/PaymentChannelSelector.vue'
import EmberCheckoutSteps from './components/EmberCheckoutSteps.vue'
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
