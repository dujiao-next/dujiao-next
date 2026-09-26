<template>
  <div class="mx-auto w-full max-w-[1180px] px-4 pb-10 sm:px-6">
    <nav class="flex flex-wrap items-center gap-1.5 py-5 text-[13px] text-muted-foreground">
      <RouterLink class="transition-colors hover:text-foreground" to="/">{{ t('nav.home') }}</RouterLink>
      <ChevronRight class="h-3.5 w-3.5 flex-none opacity-60" />
      <RouterLink v-if="!isBuyNowMode" class="transition-colors hover:text-foreground" to="/cart">{{ t('cart.title') }}</RouterLink>
      <ChevronRight v-if="!isBuyNowMode" class="h-3.5 w-3.5 flex-none opacity-60" />
      <span class="text-foreground">{{ t('checkout.title') }}</span>
    </nav>

    <header class="mb-8 flex flex-col gap-2">
      <h1 class="text-[clamp(1.75rem,1.3rem+1.9vw,2.6rem)] font-semibold tracking-[-0.022em]">{{ t('checkout.title') }}</h1>
      <p class="max-w-[60ch] text-[15px] leading-[1.65] text-muted-foreground">{{ t('checkout.subtitle') }}</p>
    </header>

    <!-- 空 -->
    <div v-if="cartItems.length === 0" class="flex flex-col items-center gap-3 rounded-[var(--r-lg)] border border-dashed border-border py-20 text-center">
      <ShoppingCart class="h-11 w-11 text-muted-foreground" />
      <p class="font-medium">{{ t('checkout.empty') }}</p>
      <Button as-child size="sm" class="mt-2 rounded-full"><RouterLink to="/products">{{ t('checkout.emptyAction') }}</RouterLink></Button>
    </div>

    <div v-else class="grid items-start gap-7 lg:grid-cols-[minmax(0,1fr)_380px]">
      <!-- ==================== 左栏 ==================== -->
      <div class="flex flex-col gap-4">
        <EmberCheckoutSteps current="checkout" :skip-cart="isBuyNowMode" />
        <!-- 商品确认 -->
        <section class="rounded-[var(--r-lg)] border border-border bg-card p-5">
          <h2 class="mb-4 text-[15px] font-semibold tracking-[-0.015em]">{{ t('checkout.itemsTitle') }}</h2>
          <div class="flex flex-col gap-3">
            <div
              v-for="item in cartItems"
              :key="cartItemKey(item)"
              class="grid grid-cols-[56px_minmax(0,1fr)_auto] items-center gap-3.5 rounded-[var(--r)] border p-3.5"
              :class="itemStockExceeded(item) ? 'border-[color:var(--warning)] bg-[color:var(--warning-soft)]' : 'border-[color:var(--border-soft)] bg-[color:var(--surface-2)]'"
            >
              <div class="grid h-14 w-14 place-items-center overflow-hidden rounded-[var(--r-xs)] bg-card">
                <img v-if="checkoutItemImage(item)" :src="checkoutItemImage(item)" :alt="getLocalizedText(item.title)" loading="lazy" class="h-full w-full object-cover" />
                <Package v-else class="h-5 w-5 text-muted-foreground" />
              </div>
              <div class="min-w-0">
                <RouterLink class="block truncate text-[14px] font-medium transition-colors hover:text-[color:var(--accent)]" :to="`/products/${item.slug}`">
                  {{ getLocalizedText(item.title) }}
                </RouterLink>
                <div class="mt-0.5 text-[12px] text-muted-foreground">{{ t('checkout.quantityLabel') }}：{{ item.quantity }}</div>
                <div v-if="itemSkuDisplay(item)" class="mt-0.5 text-[12px] text-muted-foreground">{{ t('checkout.skuLabel') }}：{{ itemSkuDisplay(item) }}</div>
                <div v-if="itemStockHint(item)" class="mt-0.5 text-[12px] text-[color:var(--warning-strong)]">{{ itemStockHint(item) }}</div>
              </div>
              <div class="flex-none text-right">
                <span class="text-[15px] font-semibold tabular-nums" :class="checkoutItemHasPriceDiscount(item) ? 'text-[color:var(--accent-strong)]' : 'text-foreground'">
                  {{ checkoutItemPriceParts(item).integer }}{{ checkoutItemPriceParts(item).decimal }} {{ checkoutItemCurrency }}
                </span>
                <span v-if="checkoutItemHasPriceDiscount(item)" class="mt-0.5 block text-[11.5px] text-muted-foreground line-through">
                  {{ checkoutItemOriginalPriceParts(item).integer }}{{ checkoutItemOriginalPriceParts(item).decimal }} {{ checkoutItemCurrency }}
                </span>
              </div>
            </div>
          </div>
        </section>

        <!-- 手动发货表单 -->
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
        <section v-if="!isResellerTenant" class="rounded-[var(--r-lg)] border border-border bg-card p-5">
          <h2 class="mb-3 text-[15px] font-semibold tracking-[-0.015em]">{{ t('checkout.couponTitle') }}</h2>
          <input
            v-model="couponCode"
            type="text"
            class="h-11 w-full rounded-[var(--r)] border border-border bg-[color:var(--surface-2)] px-4 text-[14px] outline-none transition-colors focus:border-[color:var(--accent)] focus:bg-card focus:ring-[var(--ring)]"
            :placeholder="t('checkout.couponPlaceholder')"
          />
        </section>

        <!-- 联系方式（游客下单） -->
        <section v-if="!userAuthStore.isAuthenticated" class="rounded-[var(--r-lg)] border border-border bg-card p-5">
          <h2 class="mb-4 flex items-center gap-2 text-[15px] font-semibold tracking-[-0.015em]">
            <span class="grid h-6 w-6 place-items-center rounded-full bg-[color:var(--accent-soft)] text-[11px] font-semibold text-[color:var(--accent-strong)]">1</span>
            {{ t('checkout.modeTitle') }}
          </h2>

          <div class="mb-4 flex flex-wrap gap-2.5">
            <button
              type="button"
              class="rounded-full px-4 py-2 text-[13px] transition-colors"
              :class="checkoutMode === 'guest' ? 'bg-foreground text-background' : 'border border-border text-muted-foreground hover:text-foreground'"
              @click="checkoutMode = 'guest'"
            >{{ t('checkout.guestPurchase') }}</button>
            <RouterLink
              class="rounded-full border border-border px-4 py-2 text-[13px] text-muted-foreground transition-colors hover:text-foreground"
              to="/auth/login"
            >{{ t('checkout.memberPurchase') }}</RouterLink>
          </div>

          <template v-if="checkoutMode === 'guest'">
            <div class="grid gap-3 sm:grid-cols-2">
              <label class="flex flex-col gap-2">
                <span class="text-[12.5px] font-medium text-foreground">{{ t('checkout.guestEmailPlaceholder') }}</span>
                <input
                  v-model="guestEmail"
                  type="email"
                  class="h-11 rounded-[var(--r)] border border-border bg-[color:var(--surface-2)] px-4 text-[14px] outline-none transition-colors focus:border-[color:var(--accent)] focus:bg-card focus:ring-[var(--ring)]"
                  :placeholder="t('checkout.guestEmailPlaceholder')"
                />
              </label>
              <label class="flex flex-col gap-2">
                <span class="text-[12.5px] font-medium text-foreground">{{ t('checkout.guestPasswordPlaceholder') }}</span>
                <input
                  v-model="guestPassword"
                  type="password"
                  class="h-11 rounded-[var(--r)] border border-border bg-[color:var(--surface-2)] px-4 text-[14px] outline-none transition-colors focus:border-[color:var(--accent)] focus:bg-card focus:ring-[var(--ring)]"
                  :placeholder="t('checkout.guestPasswordPlaceholder')"
                />
              </label>
            </div>

            <p v-if="guestEmail && !guestEmailValid" class="mt-2 text-[13px] text-[color:var(--danger-strong)]">{{ t('error.email_invalid') }}</p>

            <div v-if="guestCaptchaEnabled" class="mt-4">
              <p class="mb-2 text-[12.5px] font-medium text-foreground">{{ t('auth.common.captchaLabel') }}</p>
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

            <div class="mt-4 rounded-[var(--r)] bg-[color:var(--accent-soft)] px-4 py-3 text-[13px] leading-[1.6] text-[color:var(--accent-strong)]">
              <p class="font-semibold">{{ t('checkout.guestInstructions.title') }}</p>
              <ul class="mt-1.5 flex flex-col gap-0.5">
                <li>· {{ t('checkout.guestInstructions.email') }}</li>
                <li>· {{ t('checkout.guestInstructions.password') }}</li>
              </ul>
            </div>
          </template>
        </section>
      </div>

      <!-- ==================== 右栏 ==================== -->
      <aside class="sticky top-[92px] flex flex-col gap-4">
        <div class="rounded-[var(--r-lg)] border border-border bg-card p-5">
          <h2 class="mb-3 border-b border-[color:var(--border-soft)] pb-3 text-[15px] font-semibold tracking-[-0.015em]">
            {{ t('checkout.submitTitle') }}
          </h2>

          <div class="flex flex-col gap-2.5">
            <div class="flex items-center justify-between text-[13.5px]">
              <span class="text-muted-foreground">{{ t('cart.itemsCount') }}</span>
              <span class="font-medium tabular-nums">{{ totalItems }}</span>
            </div>
            <div class="flex items-center justify-between text-[13.5px]">
              <span class="text-muted-foreground">{{ t('checkout.previewOriginal') }}</span>
              <span class="font-medium tabular-nums">{{ formatPrice(previewOriginal, previewCurrency) }}</span>
            </div>
            <template v-if="!isResellerTenant">
              <div v-if="hasPositiveAmount(previewCoupon)" class="flex items-center justify-between text-[13.5px]">
                <span class="text-muted-foreground">{{ t('checkout.previewCoupon') }}</span>
                <span class="font-medium tabular-nums text-[color:var(--accent-strong)]">{{ formatDiscountPrice(previewCoupon, previewCurrency) }}</span>
              </div>
              <div v-if="hasPositiveAmount(previewPromotion)" class="flex items-center justify-between text-[13.5px]">
                <span class="text-muted-foreground">{{ t('checkout.previewPromotion') }}</span>
                <span class="font-medium tabular-nums text-[color:var(--accent-strong)]">{{ formatDiscountPrice(previewPromotion, previewCurrency) }}</span>
              </div>
              <div v-if="hasPositiveAmount(previewWholesale)" class="flex items-center justify-between text-[13.5px]">
                <span class="text-muted-foreground">{{ t('checkout.previewWholesale') }}</span>
                <span class="font-medium tabular-nums text-[color:var(--success-strong)]">{{ formatDiscountPrice(previewWholesale, previewCurrency) }}</span>
              </div>
            </template>
            <div v-if="Number(previewMemberDiscount) > 0" class="flex items-center justify-between text-[13.5px]">
              <span class="text-muted-foreground">{{ t('checkout.previewMemberDiscount') }}</span>
              <span class="font-medium tabular-nums text-[color:var(--success-strong)]">-{{ formatPrice(previewMemberDiscount, previewCurrency) }}</span>
            </div>

            <div class="mt-1 flex items-baseline justify-between border-t border-border pt-3">
              <span class="text-[13.5px] text-muted-foreground">{{ t('checkout.previewTotal') }}</span>
              <span class="text-[22px] font-semibold tabular-nums tracking-[-0.03em] text-[color:var(--accent-strong)]">
                {{ formatPrice(previewTotal, previewCurrency) }}
              </span>
            </div>
          </div>

          <p v-if="previewLoading || couponRefreshing" class="mt-3 text-[12px] text-muted-foreground">{{ previewStatusText }}</p>

          <div
            v-if="checkoutAlert"
            class="mt-3 rounded-[var(--r)] px-3.5 py-2.5 text-[13px] leading-[1.6]"
            :class="checkoutAlert.level === 'error' ? 'bg-[color:var(--danger-soft)] text-[color:var(--danger-strong)]' : 'bg-[color:var(--warning-soft)] text-[color:var(--warning-strong)]'"
          >{{ checkoutAlert.message }}</div>

          <!-- 支付方式 -->
          <div class="mt-5 border-t border-border pt-4">
            <h3 class="mb-3 text-[13.5px] font-semibold">{{ t('checkout.paymentMethod') }}</h3>

            <!-- 余额 -->
            <div v-if="showBalanceOption" class="mb-3 rounded-[var(--r)] border border-[color:var(--border-soft)] bg-[color:var(--surface-2)] p-3.5">
              <div class="flex items-start justify-between gap-2.5">
                <div>
                  <div class="text-[12px] text-muted-foreground">{{ t('payment.walletBalanceLabel') }}</div>
                  <div class="mt-0.5 font-semibold">{{ walletLoading ? t('common.loading') : formatPrice(walletBalance, previewCurrency) }}</div>
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

            <!-- 渠道 -->
            <template v-if="!walletOnlyPayment">
              <div v-if="requiresOnlineChannel && paymentChannels.length > 0" class="flex flex-col gap-2.5">
                <button
                  v-for="channel in paymentChannels"
                  :key="channel.id"
                  type="button"
                  class="flex items-center gap-3 rounded-[var(--r)] border px-3.5 py-3 text-left transition-colors"
                  :class="[
                    selectedChannelId === channel.id && !isChannelDisabledForAmount(channel)
                      ? 'border-[color:var(--accent)] bg-[color:var(--accent-soft)]'
                      : 'border-border bg-card hover:border-[color:var(--border-strong)]',
                    isChannelDisabledForAmount(channel) ? 'cursor-not-allowed opacity-50' : '',
                  ]"
                  :disabled="isChannelDisabledForAmount(channel)"
                  :title="isChannelDisabledForAmount(channel) ? channelAmountLimitHint(channel) : ''"
                  @click="handleSelectChannel(channel)"
                >
                  <img v-if="channel.icon" :src="getImageUrl(channel.icon)" loading="lazy" class="h-6 w-6 flex-none rounded object-contain" />
                  <span class="grid h-6 w-6 flex-none place-items-center rounded bg-[color:var(--surface-2)] text-muted-foreground" v-else>
                    <Wallet class="h-3.5 w-3.5" />
                  </span>
                  <span class="min-w-0 flex-1">
                    <span class="block truncate text-[13.5px] font-medium text-foreground">{{ channel.name }}</span>
                    <span v-if="channel.fee_policy === 'customer_surcharge'" class="block text-[11px] text-[color:var(--warning-strong)]">
                      {{ t('payment.feeLabel') }}：{{ formatChannelFeeRate(channel) }} · {{ t('payment.fixedFeeLabel') }}：{{ formatChannelFixedFee(channel) }}
                    </span>
                    <span v-if="isChannelDisabledForAmount(channel)" class="block text-[11px] text-[color:var(--warning-strong)]">
                      {{ channelAmountLimitHint(channel) }}
                    </span>
                  </span>
                  <span
                    class="grid h-5 w-5 flex-none place-items-center rounded-full border"
                    :class="selectedChannelId === channel.id && !isChannelDisabledForAmount(channel)
                      ? 'border-[color:var(--accent)] bg-[color:var(--accent)] text-white'
                      : 'border-[color:var(--border-strong)]'"
                  >
                    <Check v-if="selectedChannelId === channel.id && !isChannelDisabledForAmount(channel)" class="h-3 w-3" />
                  </span>
                </button>
              </div>
              <p v-else-if="requiresOnlineChannel && paymentChannels.length === 0" class="text-[13px] text-muted-foreground">
                {{ t('checkout.noPaymentChannels') }}
              </p>
            </template>

            <p v-else class="text-[13px] text-[color:var(--success-strong)]">{{ t('checkout.walletCoversAll') }}</p>
          </div>

          <Button
            size="lg"
            class="mt-4 h-12 w-full rounded-full text-[15px] font-medium"
            :disabled="!canSubmit"
            @click="handleSubmit"
          >
            {{ submitting ? t('checkout.submitting') : t('checkout.submitButton') }}
          </Button>
        </div>

        <RouterLink class="text-center text-[13px] text-muted-foreground transition-colors hover:text-foreground" to="/products">
          {{ t('nav.products') }}
        </RouterLink>
      </aside>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Check, ChevronRight, Package, ShoppingCart, Wallet } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import ImageCaptcha from '../../components/captcha/ImageCaptcha.vue'
import TurnstileCaptcha from '../../components/captcha/TurnstileCaptcha.vue'
import CheckoutManualForm from '../../components/checkout/CheckoutManualForm.vue'
import EmberCheckoutSteps from './components/EmberCheckoutSteps.vue'
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
