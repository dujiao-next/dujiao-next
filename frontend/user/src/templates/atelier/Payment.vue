<template>
  <div class="min-h-screen pb-16 pt-[88px] text-foreground">
    <div class="mx-auto w-full max-w-[1240px] px-4 sm:px-6">
      <div class="mb-7 flex flex-wrap items-end justify-between gap-x-8 gap-y-4 border-b pb-5">
        <div>
          <h1 class="flex flex-wrap items-center gap-3 text-[26px] font-bold tracking-[-0.025em] md:text-[30px]">
            <span>{{ t('payment.title') }}</span>
            <RouterLink :to="backLink" class="theme-pill-link tracking-normal">
              {{ t('payment.backToOrders') }}<ChevronRight class="h-3.5 w-3.5" aria-hidden="true" />
            </RouterLink>
          </h1>
          <p class="mt-1 text-sm text-muted-foreground">{{ t('payment.subtitle') }}</p>
        </div>
        <AtelierCheckoutSteps class="w-full sm:w-[22rem]" current-step="payment" />
      </div>

      <!-- 骨架屏 -->
      <div v-if="loading" class="atelier-card space-y-5 rounded-[16px] bg-card p-6">
        <div class="flex items-center justify-between gap-4">
          <div class="space-y-2">
            <div class="h-5 w-44 rounded theme-skeleton"></div>
            <div class="h-3 w-64 rounded theme-skeleton"></div>
          </div>
          <div class="h-7 w-24 rounded-full theme-skeleton"></div>
        </div>
        <div class="grid grid-cols-1 gap-6 border-t pt-5 lg:grid-cols-3">
          <div class="space-y-3 lg:col-span-2">
            <div class="mx-auto aspect-square w-full max-w-[240px] rounded-[16px] theme-skeleton lg:mx-0"></div>
          </div>
          <div class="space-y-3">
            <div class="h-4 w-24 rounded theme-skeleton"></div>
            <div class="h-4 w-36 rounded theme-skeleton"></div>
            <div class="h-4 w-28 rounded theme-skeleton"></div>
          </div>
        </div>
      </div>

      <!-- 游客订单密码验证 -->
      <div v-else-if="showGuestAuthForm" class="atelier-card mx-auto max-w-xl rounded-[16px] bg-card p-6">
        <h2 class="text-lg font-bold">{{ t('payment.guestAuthTitle') }}</h2>
        <p class="mt-1 text-xs text-muted-foreground">{{ t('payment.guestAuthHint') }}</p>
        <div class="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
          <Input v-model="guestAuth.email" type="email" class="h-11 rounded-[12px]" :placeholder="t('guestOrders.emailPlaceholder')" @keydown.enter="handleGuestAuthSubmit" />
          <Input v-model="guestAuth.order_password" type="password" class="h-11 rounded-[12px]" :placeholder="t('guestOrders.passwordPlaceholder')" @keydown.enter="handleGuestAuthSubmit" />
        </div>
        <Alert v-if="guestAuthError" variant="destructive" class="mt-4">
          <AlertDescription>{{ guestAuthError }}</AlertDescription>
        </Alert>
        <Button class="mt-4 h-11 rounded-[11px] bg-primary px-6 font-semibold text-primary-foreground shadow-xs hover:bg-primary/90" @click="handleGuestAuthSubmit">
          {{ t('payment.guestAuthSubmit') }}
        </Button>
      </div>

      <!-- 未找到订单 -->
      <EmptyState
        v-else-if="!order"
        icon="alert"
        :title="t('payment.orderNotFound')"
        :action-label="t('payment.backToOrders')"
        :action-to="backLink"
      />

      <!-- 支付凭证结果视图（扫码 / 加密地址 / 外部支付链接） -->
      <div v-else-if="showResultView" class="atelier-card rounded-[16px] bg-card p-5 md:p-7">
        <div class="flex flex-col justify-between gap-4 border-b pb-5 md:flex-row md:items-center">
          <div>
            <h2 class="text-xl font-bold tracking-[-0.02em] text-foreground">{{ paymentResultTitle }}</h2>
            <p class="mt-1 text-sm text-muted-foreground">{{ paymentGuideTip }}</p>
            <div class="mt-2 text-xs text-muted-foreground">
              {{ t('payment.methodLabel') }}：<span class="font-medium text-foreground">{{ resultChannelName }}</span>
            </div>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <Button variant="secondary" class="rounded-[10px] font-semibold" :disabled="loading" @click="handleRefresh">
              {{ t('payment.refreshStatus') }}
            </Button>
            <Button variant="secondary" class="rounded-[10px] font-semibold" @click="handleChangePaymentMethod">
              {{ t('payment.changeMethod') }}
            </Button>
          </div>
        </div>

        <div class="mt-6 grid grid-cols-1 gap-6 lg:grid-cols-3">
          <div class="space-y-4 lg:col-span-2">
            <div
              v-if="showQRCode"
              class="flex flex-col items-center justify-center rounded-[18px] border bg-background p-6 text-center"
            >
              <div class="mb-4 text-sm font-medium text-muted-foreground">{{ paymentGuideTitle }}</div>
              <div class="aspect-square w-full max-w-[240px] overflow-hidden rounded-[16px] border bg-white p-2.5 shadow-xs">
                <img :src="qrImageUrl" alt="QR Code" class="h-full w-full object-contain" />
              </div>
              <div v-if="qrUsingPayLinkFallback" class="mt-3 text-xs text-muted-foreground">
                {{ t('payment.qrFallbackHint') }}
              </div>
              <div v-if="hasCryptoPaymentDetails" class="mt-5 w-full max-w-xl space-y-2 rounded-[14px] border bg-card p-4 text-left">
                <div
                  v-for="item in cryptoPaymentDetails"
                  :key="item.key"
                  class="flex flex-col gap-1 border-b pb-2 last:border-b-0 last:pb-0 sm:flex-row sm:items-start sm:justify-between sm:gap-4"
                >
                  <span class="shrink-0 text-xs text-muted-foreground">{{ item.label }}</span>
                  <span class="min-w-0 break-all font-mono text-xs font-semibold text-foreground sm:text-right">
                    {{ item.value }}
                    <span v-if="item.detail" class="ml-1 font-sans font-normal text-muted-foreground">({{ item.detail }})</span>
                  </span>
                </div>
                <div v-if="cryptoWalletAddress" class="flex flex-wrap items-center justify-end gap-2 pt-1.5">
                  <Button variant="secondary" size="sm" class="rounded-[9px] font-semibold" @click="handleCopyWalletAddress">
                    {{ t('payment.copyWalletAddress') }}
                  </Button>
                  <span v-if="walletAddressCopied" class="text-xs font-medium text-success">{{ t('payment.copied') }}</span>
                </div>
              </div>
            </div>

            <div v-else class="rounded-[18px] border bg-background p-6">
              <div class="mb-3 text-sm text-muted-foreground">{{ t('payment.openPayLink') }}</div>
              <Button
                type="button"
                class="h-11 rounded-[11px] bg-primary px-5 font-semibold text-primary-foreground shadow-xs hover:bg-primary/90"
                @click="handleOpenPayLink"
              >
                <ExternalLink class="h-4 w-4" aria-hidden="true" />
                {{ t('payment.openPayLink') }}
              </Button>
              <div v-if="openedPayWindow" class="mt-3 text-xs text-success">{{ payLinkOpenedTip }}</div>
              <div v-if="showTelegramPayHint" class="mt-3 text-xs text-muted-foreground">{{ t('payment.telegramExternalHint') }}</div>
              <div class="mt-3 flex flex-wrap items-center gap-2">
                <Button type="button" variant="outline" size="sm" class="rounded-[9px] font-semibold" @click="handleCopyPayLink">
                  <Copy class="h-4 w-4" aria-hidden="true" />
                  {{ t('payment.copyPayLink') }}
                </Button>
                <span v-if="copied" class="text-xs font-medium text-success">{{ t('payment.copied') }}</span>
              </div>
            </div>
          </div>

          <div class="space-y-4">
            <div class="rounded-[16px] border bg-background p-4">
              <div class="text-xs text-muted-foreground">{{ t('payment.orderNo') }}</div>
              <div class="mt-1 font-mono text-sm font-semibold text-foreground">{{ order.order_no }}</div>
              <div class="mt-3 text-xs text-muted-foreground">{{ t('payment.orderStatus') }}：{{ statusLabel(order.status) }}</div>
              <div class="mt-1.5 text-xs text-muted-foreground">{{ t('payment.methodLabel') }}：{{ resultChannelName }}</div>
            </div>

            <PaymentAmountBreakdown
              class="rounded-[16px]! border bg-background!"
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

            <div v-if="paymentResult.expires_at" class="rounded-[14px] border bg-background px-4 py-3 text-xs text-muted-foreground">
              {{ t('payment.expiresAt') }}：<span class="font-mono text-foreground">{{ formatDate(paymentResult.expires_at) }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- 订单已取消 / 已过期 -->
      <div v-else-if="orderExpired || orderCanceled" class="atelier-card rounded-[16px] bg-card p-6">
        <div class="flex flex-col justify-between gap-4 border-b pb-5 md:flex-row md:items-center">
          <div>
            <h2 class="text-xl font-bold text-foreground">
              {{ orderCanceled ? t('payment.orderCanceled') : t('payment.orderExpired') }}
            </h2>
            <p class="mt-1 font-mono text-sm text-muted-foreground">{{ order.order_no }}</p>
          </div>
          <Button as-child variant="secondary" class="rounded-[10px] font-semibold">
            <RouterLink :to="backLink">{{ t('payment.backToOrders') }}</RouterLink>
          </Button>
        </div>
        <div class="mt-5 grid grid-cols-1 gap-3 text-sm sm:grid-cols-3">
          <div class="rounded-[14px] border bg-background p-3.5">
            <div class="text-xs text-muted-foreground">{{ t('payment.orderNo') }}</div>
            <div class="mt-1 font-mono text-foreground">{{ order.order_no }}</div>
          </div>
          <div class="rounded-[14px] border bg-background p-3.5">
            <div class="text-xs text-muted-foreground">{{ t('payment.orderStatus') }}</div>
            <div class="mt-1 font-medium text-foreground">{{ statusLabel(order.status) }}</div>
          </div>
          <div class="rounded-[14px] border bg-background p-3.5">
            <div class="text-xs text-muted-foreground">{{ t('orderDetail.amountTotal') }}</div>
            <div class="mt-1 font-semibold tabular-nums text-foreground">{{ formatMoney(order.total_amount, order.currency) }}</div>
          </div>
        </div>
      </div>

      <!-- 主支付视图：左侧订单信息与渠道选择 + 右侧吸顶支付动作 -->
      <div v-else class="grid grid-cols-1 items-start gap-6 lg:grid-cols-12">
        <div class="space-y-5 lg:col-span-8">
          <section class="atelier-card rounded-[16px] bg-card p-5 md:p-6">
            <div class="flex flex-col justify-between gap-4 md:flex-row md:items-start">
              <div>
                <h2 class="text-base font-bold tracking-[-0.015em]">{{ t('payment.orderInfo') }}</h2>
                <div class="mt-3 text-xs text-muted-foreground">{{ t('payment.orderNo') }}</div>
                <div class="mt-0.5 font-mono text-sm font-semibold text-foreground">{{ order.order_no }}</div>
                <div class="mt-2 text-xs text-muted-foreground">
                  {{ t('orderDetail.createdAtLabel') }}：<span class="font-mono">{{ formatDate(order.created_at) }}</span>
                </div>
                <div v-if="order.expires_at" class="mt-1 text-xs text-muted-foreground">
                  {{ t('payment.expiresAt') }}：<span class="font-mono">{{ formatDate(order.expires_at) }}</span>
                </div>
                <Badge v-if="showCountdown" :variant="countdownExpired ? 'danger' : 'success'" class="mt-3 gap-2">
                  <span>{{ t('payment.countdownLabel') }}</span>
                  <span class="font-mono tabular-nums">{{ countdownText }}</span>
                </Badge>
                <div v-if="pollingActive" class="mt-2.5 text-xs text-muted-foreground">{{ t('payment.pollingHint') }}</div>
              </div>

              <div class="w-full rounded-[16px] border bg-background p-4 md:w-auto md:min-w-[270px]">
                <div class="text-xs text-muted-foreground md:text-right">{{ t('payment.payableAmountLabel') }}</div>
                <div class="mt-1 text-[28px] font-bold leading-none tracking-[-0.03em] tabular-nums md:text-right">
                  {{ payableAmountDisplay }}
                </div>
                <div class="mt-4 space-y-2 border-t pt-3 text-xs">
                  <div class="flex items-center justify-between gap-4">
                    <span class="text-muted-foreground">{{ t('orderDetail.amountTotal') }}</span>
                    <span class="font-semibold tabular-nums text-foreground">{{ formatMoney(order.total_amount, order.currency) }}</span>
                  </div>
                  <div v-if="showBalanceOption && useBalance" class="flex items-center justify-between gap-4">
                    <span class="text-muted-foreground">{{ t('payment.walletDeductLabel') }}</span>
                    <span class="font-medium tabular-nums text-foreground">{{ expectedWalletPaidDisplay }}</span>
                  </div>
                  <div v-if="showBalanceOption && useBalance" class="flex items-center justify-between gap-4">
                    <span class="text-muted-foreground">{{ t('payment.onlinePayLabel') }}</span>
                    <span class="font-medium tabular-nums text-foreground">{{ expectedOnlinePayDisplay }}</span>
                  </div>
                  <div class="flex items-center justify-between gap-4 border-t pt-2">
                    <span class="text-muted-foreground">{{ t('payment.orderStatus') }}</span>
                    <span class="font-semibold text-foreground">{{ statusLabel(order.status) }}</span>
                  </div>
                </div>
              </div>
            </div>

            <div class="mt-5 grid grid-cols-1 gap-2.5 text-sm sm:grid-cols-3">
              <div class="rounded-[12px] border bg-background p-3">
                <div class="text-xs text-muted-foreground">{{ t('orderDetail.amountOriginal') }}</div>
                <div class="mt-1 font-semibold tabular-nums text-foreground">{{ formatMoney(order.original_amount, order.currency) }}</div>
              </div>
              <div class="rounded-[12px] border bg-background p-3">
                <div class="text-xs text-muted-foreground">{{ t('orderDetail.amountDiscount') }}</div>
                <div class="mt-1 font-semibold tabular-nums" :class="hasDiscountAmount(order.discount_amount) ? 'text-danger' : 'text-foreground'">
                  {{ formatDiscountMoney(order.discount_amount, order.currency) }}
                </div>
              </div>
              <div class="rounded-[12px] border bg-background p-3">
                <div class="text-xs text-muted-foreground">{{ t('orderDetail.promotionDiscountLabel') }}</div>
                <div class="mt-1 font-semibold tabular-nums" :class="hasDiscountAmount(order.promotion_discount_amount) ? 'text-danger' : 'text-foreground'">
                  {{ formatDiscountMoney(order.promotion_discount_amount, order.currency) }}
                </div>
              </div>
              <div v-if="hasDiscountAmount(order.wholesale_discount_amount)" class="rounded-[12px] bg-success-soft p-3 text-success">
                <div class="text-xs">{{ t('orderDetail.amountWholesaleDiscount') }}</div>
                <div class="mt-1 font-semibold tabular-nums">
                  {{ formatDiscountMoney(order.wholesale_discount_amount, order.currency) }}
                </div>
              </div>
            </div>
          </section>

          <section v-if="orderItems.length" class="atelier-card rounded-[16px] bg-card p-5 md:p-6">
            <h2 class="border-b pb-3 text-base font-bold tracking-[-0.015em]">{{ t('payment.itemsTitle') }}</h2>
            <div class="mt-3 divide-y text-sm">
              <div
                v-for="(item, idx) in orderItems"
                :key="idx"
                class="flex flex-col justify-between gap-2 py-3 first:pt-1 last:pb-0 md:flex-row md:items-center"
              >
                <div>
                  <div class="font-semibold text-foreground">{{ getLocalizedText(item.title) }}</div>
                  <div class="mt-1 text-xs text-muted-foreground">
                    {{ t('orderDetail.quantityLabel') }}：{{ item.quantity }} · {{ t('orderDetail.itemFulfillmentLabel') }}：{{ fulfillmentTypeLabelText(item.fulfillment_type) }}
                  </div>
                  <div v-if="orderItemSkuText(item)" class="mt-0.5 text-xs text-muted-foreground">
                    {{ t('orderDetail.itemSkuLabel') }}：{{ orderItemSkuText(item) }}
                  </div>
                </div>
                <div class="text-[15px] font-bold tabular-nums text-foreground">
                  {{ formatMoney(item.total_price, order.currency) }}
                </div>
              </div>
            </div>
          </section>

          <section class="atelier-card rounded-[16px] bg-card p-5 md:p-6">
            <h2 class="mb-4 border-b pb-3 text-base font-bold tracking-[-0.015em]">{{ t('payment.channelTitle') }}</h2>
            <div v-if="!configReady" class="text-sm text-muted-foreground">{{ t('common.loading') }}</div>
            <template v-else>
              <div v-if="showBalanceOption" class="mb-4 rounded-[14px] border bg-background p-4">
                <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <div>
                    <div class="text-xs text-muted-foreground">{{ t('payment.walletBalanceLabel') }}</div>
                    <div class="mt-1 text-sm font-semibold tabular-nums text-foreground">
                      {{ walletLoading ? t('common.loading') : walletBalanceDisplay }}
                    </div>
                  </div>
                  <label class="inline-flex cursor-pointer items-center gap-2 text-xs font-medium text-foreground">
                    <Checkbox v-model="useBalance" class="bg-card" :disabled="walletOnlyPayment" />
                    <span>{{ t('payment.useBalance') }}</span>
                  </label>
                </div>
                <div v-if="walletOnlyPayment" class="mt-2.5 text-xs text-warning">{{ t('payment.walletOnlyHint') }}</div>
                <div v-if="useBalance" class="mt-2.5 space-y-1 text-xs text-muted-foreground">
                  <div>{{ t('payment.walletDeductLabel') }}：{{ expectedWalletPaidDisplay }}</div>
                  <div v-if="!walletOnlyPayment">{{ t('payment.onlinePayLabel') }}：{{ expectedOnlinePayDisplay }}</div>
                  <div v-if="walletOnlyPayment && expectedOnlinePayCents > 0" class="text-warning">
                    {{ t('payment.walletInsufficientHint') }}
                  </div>
                </div>
              </div>

              <div v-if="cachedPayment" class="mb-4 space-y-2 rounded-[14px] border border-warning/35 bg-warning-soft p-4 text-sm text-warning">
                <div class="font-semibold">{{ t('payment.cachedTitle') }}</div>
                <div>{{ t('payment.cachedHint', { channel: cachedChannelName }) }}</div>
                <div class="flex flex-wrap items-center gap-2">
                  <Button variant="secondary" size="sm" class="rounded-[9px] font-semibold" @click="restoreCachedPayment">
                    {{ t('payment.useCached') }}
                  </Button>
                  <span class="text-xs opacity-80">{{ t('payment.cachedCreateHint') }}</span>
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

          <section v-if="paymentResult" class="atelier-card rounded-[16px] bg-card p-5 md:p-6">
            <h2 class="mb-4 border-b pb-3 text-base font-bold tracking-[-0.015em]">{{ t('payment.infoTitle') }}</h2>
            <div class="space-y-1.5 text-sm text-muted-foreground">
              <div>{{ t('payment.methodLabel') }}：{{ resultChannelName }}</div>
              <div>{{ t('payment.interactionLabel') }}：{{ interactionLabel }}</div>
              <div v-if="paymentResult.expires_at">{{ t('payment.expiresAt') }}：{{ formatDate(paymentResult.expires_at) }}</div>
            </div>

            <div v-if="showQRCode" class="mt-5 grid grid-cols-1 items-center gap-6 md:grid-cols-2">
              <div class="flex items-center justify-center rounded-[16px] border bg-background p-5">
                <img :src="qrImageUrl" alt="QR Code" class="h-48 w-48 rounded-[10px] bg-white p-2 object-contain" />
              </div>
              <div class="space-y-3 text-sm text-muted-foreground">
                <div class="font-semibold text-foreground">{{ paymentGuideTitle }}</div>
                <div>{{ paymentGuideTip }}</div>
                <div v-if="qrUsingPayLinkFallback" class="text-xs text-muted-foreground">{{ t('payment.qrFallbackHint') }}</div>
                <div v-if="hasCryptoPaymentDetails" class="space-y-2 rounded-[14px] border bg-background p-3">
                  <div v-for="item in cryptoPaymentDetails" :key="item.key" class="flex flex-col gap-1 border-b pb-2 last:border-b-0 last:pb-0">
                    <span class="text-xs text-muted-foreground">{{ item.label }}</span>
                    <span class="min-w-0 break-all font-mono text-xs font-semibold text-foreground">
                      {{ item.value }}
                      <span v-if="item.detail" class="ml-1 font-sans font-normal text-muted-foreground">({{ item.detail }})</span>
                    </span>
                  </div>
                  <div v-if="cryptoWalletAddress" class="flex flex-wrap items-center gap-2 pt-1">
                    <Button variant="secondary" size="sm" class="rounded-[9px] font-semibold" @click="handleCopyWalletAddress">
                      {{ t('payment.copyWalletAddress') }}
                    </Button>
                    <span v-if="walletAddressCopied" class="text-xs text-success">{{ t('payment.copied') }}</span>
                  </div>
                </div>
                <div v-if="paymentResult.pay_url" class="flex flex-wrap items-center gap-2 pt-2">
                  <Button type="button" variant="outline" size="sm" class="rounded-[9px] font-semibold" @click="handleCopyPayLink">
                    <Copy class="h-4 w-4" aria-hidden="true" />
                    {{ t('payment.copyPayLink') }}
                  </Button>
                  <span v-if="copied" class="text-xs text-success">{{ t('payment.copied') }}</span>
                </div>
              </div>
            </div>

            <div v-if="showPayLink" class="mt-5 flex flex-col gap-3 md:flex-row md:items-center">
              <Button type="button" class="h-11 rounded-[11px] bg-primary px-5 font-semibold text-primary-foreground shadow-xs hover:bg-primary/90" @click="handleOpenPayLink">
                <ExternalLink class="h-4 w-4" aria-hidden="true" />
                {{ t('payment.openPayLink') }}
              </Button>
              <Button type="button" variant="outline" class="rounded-[10px] font-semibold" @click="handleCopyPayLink">
                <Copy class="h-4 w-4" aria-hidden="true" />
                {{ t('payment.copyPayLink') }}
              </Button>
              <div v-if="showTelegramPayHint" class="text-xs text-muted-foreground">{{ t('payment.telegramExternalHint') }}</div>
            </div>
          </section>
        </div>

        <!-- 右侧吸顶支付按钮面板 -->
        <aside class="atelier-card h-fit rounded-[16px] bg-card p-6 lg:sticky lg:top-[84px] lg:col-span-4">
          <h2 class="border-b pb-3.5 text-base font-bold tracking-[-0.015em]">{{ t('payment.actionTitle') }}</h2>

          <div v-if="showCountdown" class="mt-4 flex items-center justify-between text-xs text-muted-foreground">
            <span>{{ t('payment.countdownLabel') }}</span>
            <span class="font-mono font-semibold tabular-nums text-foreground">{{ countdownText }}</span>
          </div>

          <Alert
            v-if="paymentAlert"
            :variant="pageAlertVariant(paymentAlert.level)"
            :class="['mt-4', pageAlertToneClass(paymentAlert.level)]"
          >
            <AlertDescription>{{ paymentAlert.message }}</AlertDescription>
          </Alert>

          <div v-if="selectedChannel" class="mt-4 rounded-[12px] bg-success-soft p-3 text-xs font-semibold text-success">
            {{ t('payment.methodLabel') }}：{{ selectedChannelName }}
          </div>
          <div v-else-if="!requiresOnlineChannel && !orderExpired && !orderCanceled" class="mt-4 rounded-[12px] bg-success-soft p-3 text-xs font-medium text-success">
            {{ t('payment.walletPayOnly') }}
          </div>
          <div v-else-if="walletOnlyPayment && expectedOnlinePayCents > 0 && !orderExpired && !orderCanceled" class="mt-4 rounded-[12px] bg-warning-soft p-3 text-xs font-medium text-warning">
            {{ t('payment.walletInsufficientHint') }}
          </div>
          <div v-else-if="!walletOnlyPayment && requiresOnlineChannel && !orderExpired && !orderCanceled" class="mt-4 rounded-[12px] bg-warning-soft p-3 text-xs font-medium text-warning">
            {{ t('payment.selectChannelError') }}
          </div>

          <Button
            class="mt-5 h-11.5 w-full rounded-[11px] bg-primary font-semibold text-primary-foreground shadow-xs hover:bg-primary/90"
            :disabled="!canSubmitPayment"
            @click="handlePayment"
          >
            {{ submitting ? t('payment.submitting') : t('payment.submitButton') }}
          </Button>
          <Button variant="secondary" class="mt-2.5 h-11 w-full rounded-[11px] font-semibold" :disabled="loading" @click="handleRefresh">
            {{ t('payment.refreshStatus') }}
          </Button>
        </aside>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ChevronRight, Copy, ExternalLink } from 'lucide-vue-next'
import { pageAlertVariant, pageAlertToneClass } from '../../utils/alerts'
import PaymentAmountBreakdown from '../../components/payment/PaymentAmountBreakdown.vue'
import PaymentChannelSelector from '../../components/payment/PaymentChannelSelector.vue'
import EmptyState from '../../components/EmptyState.vue'
import AtelierCheckoutSteps from './components/AtelierCheckoutSteps.vue'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Badge } from '@/components/ui/badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
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
