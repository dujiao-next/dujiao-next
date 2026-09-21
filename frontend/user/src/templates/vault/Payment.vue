<template>
  <div class="mx-auto w-full max-w-[1180px] px-6 pb-8">
    <div class="my-5 flex items-start justify-between gap-4">
      <div>
        <h1 class="mb-1.5 text-[32px] font-extrabold">{{ t('payment.title') }}</h1>
        <p class="text-muted-foreground">{{ t('payment.subtitle') }}</p>
      </div>
      <RouterLink :to="backLink" class="text-[13px] font-semibold text-muted-foreground transition-colors hover:text-primary">{{ t('payment.backToOrders') }}</RouterLink>
    </div>

    <VaultCheckoutSteps current="payment" />

    <!-- Loading -->
    <div v-if="loading" class="mb-[18px] rounded-xl border bg-card p-[22px]">
      <div class="mb-4 h-5 w-[40%] rounded bg-secondary"></div>
      <div class="h-[180px] rounded-md bg-secondary"></div>
    </div>

    <!-- 游客验证 -->
    <div v-else-if="showGuestAuthForm" class="mb-[18px] rounded-xl border bg-card p-[22px]">
      <h2 class="mb-1.5 text-lg font-bold">{{ t('payment.guestAuthTitle') }}</h2>
      <p class="mb-3.5 text-[13px] text-muted-foreground">{{ t('payment.guestAuthHint') }}</p>
      <div class="grid gap-3 sm:grid-cols-2">
        <Input v-model="guestAuth.email" type="email" class="h-11" :placeholder="t('guestOrders.emailPlaceholder')" />
        <Input v-model="guestAuth.order_password" type="password" class="h-11" :placeholder="t('guestOrders.passwordPlaceholder')" />
      </div>
      <div v-if="guestAuthError" class="mt-3.5 rounded-sm bg-destructive/10 px-3 py-2.5 text-[13px] font-semibold text-destructive">{{ guestAuthError }}</div>
      <Button size="sm" class="mt-3.5 rounded-full" @click="handleGuestAuthSubmit">{{ t('payment.guestAuthSubmit') }}</Button>
    </div>

    <!-- 订单不存在 -->
    <div v-else-if="!order" class="my-8 flex flex-col items-center gap-3 rounded-xl border border-dashed py-16 text-center text-muted-foreground">
      <AlertCircle class="h-10 w-10 opacity-60" />
      <p>{{ t('payment.orderNotFound') }}</p>
      <Button as-child class="mt-2 rounded-full" size="sm"><RouterLink :to="backLink">{{ t('payment.backToOrders') }}</RouterLink></Button>
    </div>

    <!-- 结果视图 -->
    <div v-else-if="showResultView" class="mb-[18px] rounded-xl border bg-card p-[22px]">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h2 class="text-lg font-bold">{{ paymentResultTitle }}</h2>
          <p class="mt-1 text-[13px] text-muted-foreground">{{ paymentGuideTip }}</p>
          <p class="mt-1.5 text-xs text-muted-foreground">{{ t('payment.methodLabel') }}：{{ resultChannelName }}</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <Button variant="outline" size="sm" class="rounded-full" :disabled="loading" @click="handleRefresh">{{ t('payment.refreshStatus') }}</Button>
          <Button variant="outline" size="sm" class="rounded-full" @click="handleChangePaymentMethod">{{ t('payment.changeMethod') }}</Button>
          <Button v-if="canCancelOrder" variant="outline" size="sm" class="rounded-full border-destructive text-destructive hover:bg-destructive/10" @click="cancelOrder">{{ t('orderDetail.cancel') }}</Button>
        </div>
      </div>

      <div class="mt-5 grid gap-6 lg:grid-cols-[1.4fr_1fr]">
        <div>
          <!-- QR -->
          <div v-if="showQRCode" class="flex flex-col items-center rounded-xl border bg-secondary p-[22px] text-center">
            <div class="mb-3 text-[13px] text-muted-foreground">{{ paymentGuideTitle }}</div>
            <div class="aspect-square w-full max-w-[240px] overflow-hidden rounded-md bg-white p-2"><img :src="qrImageUrl" alt="QR Code" class="h-full w-full object-contain" /></div>
            <div v-if="qrUsingPayLinkFallback" class="mt-2.5 text-xs text-muted-foreground">{{ t('payment.qrFallbackHint') }}</div>
            <div v-if="hasCryptoPaymentDetails" class="mt-4 grid w-full gap-2 rounded-md border p-3 text-left">
              <div v-for="item in cryptoPaymentDetails" :key="item.key" class="flex justify-between gap-3 border-b pb-1.5 last:border-b-0 last:pb-0">
                <span class="flex-none text-xs text-muted-foreground">{{ item.label }}</span>
                <span class="break-all text-right text-[13px] font-semibold text-foreground">{{ item.value }}<span v-if="item.detail" class="text-muted-foreground"> ({{ item.detail }})</span></span>
              </div>
              <div v-if="cryptoWalletAddress" class="flex items-center justify-end gap-2 pt-1.5">
                <Button variant="outline" size="sm" class="rounded-full" @click="handleCopyWalletAddress">{{ t('payment.copyWalletAddress') }}</Button>
                <span v-if="walletAddressCopied" class="text-xs text-[color:var(--teal-strong)]">{{ t('payment.copied') }}</span>
              </div>
            </div>
          </div>

          <!-- 跳转链接 -->
          <div v-else class="rounded-xl border bg-secondary p-[22px]">
            <div class="mb-2.5 text-[13px] text-muted-foreground">{{ t('payment.openPayLink') }}</div>
            <Button size="sm" class="rounded-full" @click="handleOpenPayLink">{{ t('payment.openPayLink') }}</Button>
            <div v-if="openedPayWindow" class="mt-2.5 text-xs text-[color:var(--teal-strong)]">{{ payLinkOpenedTip }}</div>
            <div v-if="showTelegramPayHint" class="mt-2.5 text-xs text-muted-foreground">{{ t('payment.telegramExternalHint') }}</div>
            <div class="mt-2.5 flex items-center gap-2">
              <Button variant="outline" size="sm" class="rounded-full" @click="handleCopyPayLink">{{ t('payment.copyPayLink') }}</Button>
              <span v-if="copied" class="text-xs text-[color:var(--teal-strong)]">{{ t('payment.copied') }}</span>
            </div>
          </div>
        </div>

        <div class="grid content-start gap-3.5">
          <div class="rounded-md border bg-secondary p-3.5">
            <div class="text-xs text-muted-foreground">{{ t('payment.orderNo') }}</div>
            <div class="mt-1 font-bold text-foreground">{{ order.order_no }}</div>
            <div class="mt-2.5 text-xs text-muted-foreground">{{ t('payment.orderStatus') }}：{{ statusLabel(order.status) }}</div>
            <div class="mt-1 text-xs text-muted-foreground">{{ t('payment.methodLabel') }}：{{ resultChannelName }}</div>
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
          <div v-if="paymentResult.expires_at" class="rounded-md border bg-secondary p-3.5 text-xs text-muted-foreground">
            {{ t('payment.expiresAt') }}：{{ formatDate(paymentResult.expires_at) }}
          </div>
        </div>
      </div>
    </div>

    <!-- 过期 / 取消 -->
    <div v-else-if="orderExpired || orderCanceled" class="mb-[18px] rounded-xl border bg-card p-[22px]">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h2 class="text-lg font-bold">{{ orderCanceled ? t('payment.orderCanceled') : t('payment.orderExpired') }}</h2>
          <p class="mt-1 text-[13px] text-muted-foreground">{{ order.order_no }}</p>
        </div>
        <Button as-child variant="outline" size="sm" class="rounded-full"><RouterLink :to="backLink">{{ t('payment.backToOrders') }}</RouterLink></Button>
      </div>
      <div class="mt-[18px] grid gap-3 sm:grid-cols-3">
        <div class="rounded-md border bg-secondary p-3.5"><div class="text-xs text-muted-foreground">{{ t('payment.orderNo') }}</div><div class="mt-1 font-semibold text-foreground">{{ order.order_no }}</div></div>
        <div class="rounded-md border bg-secondary p-3.5"><div class="text-xs text-muted-foreground">{{ t('payment.orderStatus') }}</div><div class="mt-1 font-semibold text-foreground">{{ statusLabel(order.status) }}</div></div>
        <div class="rounded-md border bg-secondary p-3.5"><div class="text-xs text-muted-foreground">{{ t('orderDetail.amountTotal') }}</div><div class="mt-1 font-semibold text-foreground tabular-nums">{{ formatMoney(order.total_amount, order.currency) }}</div></div>
      </div>
    </div>

    <!-- 默认：订单 + 渠道 + 操作 -->
    <div v-else class="grid items-start gap-6 lg:grid-cols-[1fr_340px]">
      <div class="grid gap-[18px]">
        <!-- 订单信息 -->
        <section class="rounded-xl border bg-card p-[22px]">
          <h2 class="mb-3.5 text-lg font-bold">{{ t('payment.orderInfo') }}</h2>
          <div class="flex flex-wrap items-start justify-between gap-[18px]">
            <div>
              <div class="text-[11px] uppercase tracking-[0.05em] text-muted-foreground">{{ t('payment.orderNo') }}</div>
              <div class="mt-1 font-bold text-foreground">{{ order.order_no }}</div>
              <div class="mt-2 text-xs text-muted-foreground">{{ t('orderDetail.createdAtLabel') }}：{{ formatDate(order.created_at) }}</div>
            </div>
            <div class="min-w-[280px] flex-1 rounded-md border bg-secondary p-4">
              <div class="text-[11px] uppercase tracking-[0.05em] text-muted-foreground">{{ t('payment.payableAmountLabel') }}</div>
              <div class="my-1 mb-3 text-[26px] font-extrabold text-foreground tabular-nums">{{ payableAmountDisplay }}</div>
              <div class="grid gap-[7px] text-[12.5px]">
                <div class="flex justify-between gap-3"><span class="text-muted-foreground">{{ t('orderDetail.amountTotal') }}</span><span class="font-semibold text-foreground">{{ formatMoney(order.total_amount, order.currency) }}</span></div>
                <div v-if="showBalanceOption && useBalance" class="flex justify-between gap-3"><span class="text-muted-foreground">{{ t('payment.walletDeductLabel') }}</span><span class="font-semibold text-foreground">{{ expectedWalletPaidDisplay }}</span></div>
                <div v-if="showBalanceOption && useBalance" class="flex justify-between gap-3"><span class="text-muted-foreground">{{ t('payment.onlinePayLabel') }}</span><span class="font-semibold text-foreground">{{ expectedOnlinePayDisplay }}</span></div>
                <div class="flex justify-between gap-3 border-t pt-2"><span class="text-muted-foreground">{{ t('payment.orderStatus') }}</span><span class="font-semibold text-foreground">{{ statusLabel(order.status) }}</span></div>
              </div>
            </div>
          </div>
          <div class="mt-4 grid gap-3 sm:grid-cols-3">
            <div class="rounded-md border bg-secondary p-3.5"><div class="text-xs text-muted-foreground">{{ t('orderDetail.amountOriginal') }}</div><div class="mt-1 font-semibold text-foreground tabular-nums">{{ formatMoney(order.original_amount, order.currency) }}</div></div>
            <div class="rounded-md border bg-secondary p-3.5"><div class="text-xs text-muted-foreground">{{ t('orderDetail.amountDiscount') }}</div><div class="mt-1 font-semibold tabular-nums" :class="hasDiscountAmount(order.discount_amount) ? 'text-destructive' : 'text-foreground'">{{ formatDiscountMoney(order.discount_amount, order.currency) }}</div></div>
            <div class="rounded-md border bg-secondary p-3.5"><div class="text-xs text-muted-foreground">{{ t('orderDetail.promotionDiscountLabel') }}</div><div class="mt-1 font-semibold tabular-nums" :class="hasDiscountAmount(order.promotion_discount_amount) ? 'text-destructive' : 'text-foreground'">{{ formatDiscountMoney(order.promotion_discount_amount, order.currency) }}</div></div>
            <div v-if="hasDiscountAmount(order.wholesale_discount_amount)" class="rounded-md border border-[color:var(--teal-strong)] bg-secondary p-3.5 text-[color:var(--teal-strong)]"><div class="text-xs">{{ t('orderDetail.amountWholesaleDiscount') }}</div><div class="mt-1 font-semibold tabular-nums">{{ formatDiscountMoney(order.wholesale_discount_amount, order.currency) }}</div></div>
          </div>
          <div v-if="order.expires_at" class="mt-3 text-[13px] text-muted-foreground">{{ t('payment.expiresAt') }}：{{ formatDate(order.expires_at) }}</div>
          <div v-if="showCountdown" class="mt-3 inline-flex items-center gap-2 rounded-full px-3 py-1.5 text-[13px] font-semibold" :class="countdownExpired ? 'bg-destructive/10 text-destructive' : 'bg-[color:var(--teal-soft)] text-[color:var(--teal-strong)]'">
            <span>{{ t('payment.countdownLabel') }}</span><span class="tabular-nums">{{ countdownText }}</span>
          </div>
          <div v-if="pollingActive" class="mt-2.5 text-xs text-muted-foreground">{{ t('payment.pollingHint') }}</div>
        </section>

        <!-- 商品 -->
        <section v-if="orderItems.length" class="rounded-xl border bg-card p-[22px]">
          <h2 class="mb-3.5 text-lg font-bold">{{ t('payment.itemsTitle') }}</h2>
          <div class="grid gap-3">
            <div v-for="(item, idx) in orderItems" :key="idx" class="flex flex-wrap justify-between gap-3 border-b pb-3 last:border-b-0 last:pb-0">
              <div>
                <div class="font-bold text-foreground">{{ getLocalizedText(item.title) }}</div>
                <div class="mt-0.5 text-xs text-muted-foreground">{{ t('orderDetail.quantityLabel') }}：{{ item.quantity }} · {{ t('orderDetail.itemFulfillmentLabel') }}：{{ fulfillmentTypeLabelText(item.fulfillment_type) }}</div>
                <div v-if="orderItemSkuText(item)" class="mt-0.5 text-xs text-muted-foreground">{{ t('orderDetail.itemSkuLabel') }}：{{ orderItemSkuText(item) }}</div>
              </div>
              <div class="text-xs text-muted-foreground">{{ t('orderDetail.totalPriceLabel') }}：{{ formatMoney(item.total_price, order.currency) }}</div>
            </div>
          </div>
        </section>

        <!-- 渠道选择 -->
        <section class="rounded-xl border bg-card p-[22px]">
          <h2 class="mb-3.5 text-lg font-bold">{{ t('payment.channelTitle') }}</h2>
          <div v-if="!configReady" class="text-[13px] text-muted-foreground">{{ t('common.loading') }}</div>
          <template v-else>
            <div v-if="showBalanceOption" class="mb-3 rounded-sm border bg-secondary p-3.5">
              <div class="flex items-start justify-between gap-2.5">
                <div>
                  <div class="text-xs text-muted-foreground">{{ t('payment.walletBalanceLabel') }}</div>
                  <div class="mt-0.5 font-bold text-foreground">{{ walletLoading ? t('common.loading') : walletBalanceDisplay }}</div>
                </div>
                <label class="inline-flex items-center gap-1.5 text-xs text-muted-foreground"><input v-model="useBalance" type="checkbox" class="h-4 w-4 accent-[var(--ui-accent)]" :disabled="walletOnlyPayment" /><span>{{ t('payment.useBalance') }}</span></label>
              </div>
              <div v-if="walletOnlyPayment" class="mt-2 text-xs text-warning">{{ t('payment.walletOnlyHint') }}</div>
              <div v-if="useBalance" class="mt-2.5 grid gap-0.5 text-xs text-muted-foreground">
                <div>{{ t('payment.walletDeductLabel') }}：{{ expectedWalletPaidDisplay }}</div>
                <div v-if="!walletOnlyPayment">{{ t('payment.onlinePayLabel') }}：{{ expectedOnlinePayDisplay }}</div>
                <div v-if="walletOnlyPayment && expectedOnlinePayCents > 0" class="text-warning">{{ t('payment.walletInsufficientHint') }}</div>
              </div>
            </div>
            <div v-if="cachedPayment" class="mb-3 grid gap-1.5 rounded-sm border border-[color:var(--gold-strong)] bg-[color:var(--gold-soft)] p-3 text-[13px] text-[color:var(--gold-strong)]">
              <div class="font-bold">{{ t('payment.cachedTitle') }}</div>
              <div>{{ t('payment.cachedHint', { channel: cachedChannelName }) }}</div>
              <div class="mt-1.5"><Button variant="outline" size="sm" class="rounded-full" @click="restoreCachedPayment">{{ t('payment.useCached') }}</Button></div>
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
        <section v-if="paymentResult" class="rounded-xl border bg-card p-[22px]">
          <h2 class="mb-3.5 text-lg font-bold">{{ t('payment.infoTitle') }}</h2>
          <div class="grid gap-1.5 text-[13px] text-muted-foreground">
            <div>{{ t('payment.methodLabel') }}：{{ resultChannelName }}</div>
            <div>{{ t('payment.interactionLabel') }}：{{ interactionLabel }}</div>
            <div v-if="paymentResult.expires_at">{{ t('payment.expiresAt') }}：{{ formatDate(paymentResult.expires_at) }}</div>
          </div>
          <div v-if="showPayLink" class="mt-3.5 flex flex-wrap gap-2">
            <Button size="sm" class="rounded-full" @click="handleOpenPayLink">{{ t('payment.openPayLink') }}</Button>
            <Button variant="outline" size="sm" class="rounded-full" @click="handleCopyPayLink">{{ t('payment.copyPayLink') }}</Button>
          </div>
        </section>
      </div>

      <!-- 右栏：操作 -->
      <aside class="sticky top-[90px] rounded-xl border bg-card p-[22px]">
        <h2 class="mb-3.5 text-lg font-bold">{{ t('payment.actionTitle') }}</h2>
        <div v-if="showCountdown" class="mb-3 text-xs text-muted-foreground">{{ t('payment.countdownLabel') }}：<span class="tabular-nums">{{ countdownText }}</span></div>
        <div v-if="paymentAlert" class="mb-3 rounded-sm px-3 py-2.5 text-[13px] font-semibold" :class="paymentAlert.level === 'error' ? 'bg-destructive/10 text-destructive' : (paymentAlert.level === 'success' ? 'bg-[color:var(--teal-soft)] text-[color:var(--teal-strong)]' : 'bg-warning/10 text-warning')">{{ paymentAlert.message }}</div>

        <div v-if="selectedChannel" class="mb-3 rounded-sm bg-[color:var(--teal-soft)] px-3 py-2.5 text-xs font-semibold text-[color:var(--teal-strong)]">{{ t('payment.methodLabel') }}：{{ selectedChannelName }}</div>
        <div v-else-if="!requiresOnlineChannel && !orderExpired && !orderCanceled" class="mb-3 rounded-sm bg-[color:var(--teal-soft)] px-3 py-2.5 text-xs font-semibold text-[color:var(--teal-strong)]">{{ t('payment.walletPayOnly') }}</div>
        <div v-else-if="walletOnlyPayment && expectedOnlinePayCents > 0 && !orderExpired && !orderCanceled" class="mb-3 rounded-sm bg-warning/10 px-3 py-2.5 text-xs font-semibold text-warning">{{ t('payment.walletInsufficientHint') }}</div>
        <div v-else-if="!walletOnlyPayment && requiresOnlineChannel && !orderExpired && !orderCanceled" class="mb-3 rounded-sm bg-warning/10 px-3 py-2.5 text-xs font-semibold text-warning">{{ t('payment.selectChannelError') }}</div>

        <Button class="h-11 w-full rounded-full font-bold" :disabled="!canSubmitPayment" @click="handlePayment">
          {{ submitting ? t('payment.submitting') : t('payment.submitButton') }}
        </Button>
        <Button variant="outline" class="mt-2.5 h-11 w-full rounded-full font-bold" :disabled="loading" @click="handleRefresh">{{ t('payment.refreshStatus') }}</Button>
      </aside>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { AlertCircle } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
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
  handlePayment, handleGuestAuthSubmit, handleRefresh, canCancelOrder, cancelOrder,
} = usePayment()
</script>
