<template>
  <div class="mx-auto w-full max-w-[1180px] px-6 pb-8">
    <nav class="flex flex-wrap items-center gap-1.5 py-5 pb-2 text-[13.5px] font-semibold text-muted-foreground">
      <RouterLink to="/" class="hover:text-primary">{{ t('nav.home') }}</RouterLink>
      <ChevronRight class="h-4 w-4 flex-none" />
      <RouterLink v-if="!isBuyNowMode" to="/cart" class="hover:text-primary">{{ t('cart.title') }}</RouterLink>
      <ChevronRight v-if="!isBuyNowMode" class="h-4 w-4 flex-none" />
      <span class="text-foreground">{{ t('checkout.title') }}</span>
    </nav>

    <header class="mb-5 mt-2">
      <h1 class="mb-1.5 text-[32px] font-extrabold">{{ t('checkout.title') }}</h1>
      <p class="text-muted-foreground">{{ t('checkout.subtitle') }}</p>
    </header>

    <VaultCheckoutSteps current="checkout" :skip-cart="isBuyNowMode" />

    <!-- 空 -->
    <div v-if="cartItems.length === 0" class="my-8 flex flex-col items-center gap-3 rounded-xl border border-dashed py-16 text-center text-muted-foreground">
      <ShoppingCart class="h-10 w-10 opacity-60" />
      <p>{{ t('checkout.empty') }}</p>
      <Button as-child class="mt-2 rounded-full" size="sm"><RouterLink to="/products">{{ t('checkout.emptyAction') }}</RouterLink></Button>
    </div>

    <div v-else class="grid items-start gap-7 lg:grid-cols-[1fr_360px]">
      <div class="grid gap-[18px]">
        <!-- 商品确认 -->
        <section class="rounded-xl border bg-card p-5">
          <h2 class="mb-3.5 text-lg font-bold">{{ t('checkout.itemsTitle') }}</h2>
          <div class="grid gap-3">
            <div
              v-for="item in cartItems"
              :key="cartItemKey(item)"
              class="flex gap-3.5 rounded-md border bg-secondary p-3.5"
              :class="{ 'border-[color:var(--gold-strong)] bg-warning/10': itemStockExceeded(item) }"
            >
              <div class="relative grid h-16 w-16 flex-none place-items-center overflow-hidden rounded-sm bg-secondary">
                <img v-if="checkoutItemImage(item)" :src="checkoutItemImage(item)" :alt="getLocalizedText(item.title)" loading="lazy" class="absolute inset-0 h-full w-full object-cover" />
                <Package v-else class="h-7 w-7 text-muted-foreground" />
              </div>
              <div class="min-w-0 flex-1">
                <RouterLink :to="`/products/${item.slug}`" class="block font-bold hover:text-primary">{{ getLocalizedText(item.title) }}</RouterLink>
                <div class="mt-0.5 text-[12.5px] text-muted-foreground">{{ t('checkout.quantityLabel') }}：{{ item.quantity }}</div>
                <div v-if="itemSkuDisplay(item)" class="mt-0.5 text-[12.5px] text-muted-foreground">{{ t('checkout.skuLabel') }}：{{ itemSkuDisplay(item) }}</div>
                <div v-if="itemStockHint(item)" class="mt-0.5 text-[12.5px]" :class="itemStockExceeded(item) ? 'text-warning' : 'text-muted-foreground'">{{ itemStockHint(item) }}</div>
              </div>
              <div class="flex-none text-right">
                <span class="inline-flex items-baseline font-bold" :class="checkoutItemHasPriceDiscount(item) ? 'text-primary' : 'text-foreground'">
                  <b class="text-[19px]">{{ checkoutItemPriceParts(item).integer }}</b><small class="text-xs font-bold">{{ checkoutItemPriceParts(item).decimal }}</small><em class="ml-0.5 text-[11px] font-bold not-italic">{{ checkoutItemCurrency }}</em>
                </span>
                <span v-if="checkoutItemHasPriceDiscount(item)" class="mt-0.5 block text-[11px] text-muted-foreground line-through">
                  {{ checkoutItemOriginalPriceParts(item).integer }}{{ checkoutItemOriginalPriceParts(item).decimal }} {{ checkoutItemCurrency }}
                </span>
              </div>
            </div>
          </div>
        </section>

        <!-- 自定义表单（复用） -->
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
        <section v-if="!isResellerTenant" class="rounded-xl border bg-card p-5">
          <h2 class="mb-3.5 text-lg font-bold">{{ t('checkout.couponTitle') }}</h2>
          <Input v-model="couponCode" type="text" class="h-11" :placeholder="t('checkout.couponPlaceholder')" />
        </section>

        <!-- 下单方式 -->
        <section v-if="!userAuthStore.isAuthenticated" class="rounded-xl border bg-card p-5">
          <h2 class="mb-3.5 text-lg font-bold">{{ t('checkout.modeTitle') }}</h2>
          <div class="mb-3.5 flex flex-wrap gap-2.5">
            <Button type="button" size="sm" class="rounded-full" :variant="checkoutMode === 'guest' ? 'default' : 'outline'" @click="checkoutMode = 'guest'">{{ t('checkout.guestPurchase') }}</Button>
            <Button as-child variant="outline" size="sm" class="rounded-full"><RouterLink to="/auth/login">{{ t('checkout.memberPurchase') }}</RouterLink></Button>
          </div>

          <template v-if="checkoutMode === 'guest'">
            <div class="grid gap-3 sm:grid-cols-2">
              <Input v-model="guestEmail" type="email" class="h-11" :placeholder="t('checkout.guestEmailPlaceholder')" />
              <Input v-model="guestPassword" type="password" :minlength="MIN_GUEST_ORDER_PASSWORD_LENGTH" autocomplete="new-password" class="h-11" :placeholder="t('checkout.guestPasswordPlaceholder')" />
            </div>

            <div v-if="guestCaptchaEnabled" class="mt-3.5">
              <p class="mb-2 text-[11px] font-bold uppercase tracking-[0.1em] text-muted-foreground">{{ t('auth.common.captchaLabel') }}</p>
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

            <div class="mt-3.5 rounded-sm border border-[color:var(--teal-strong)] bg-[color:var(--teal-soft)] px-3.5 py-3 text-[13px] text-[color:var(--teal-strong)]">
              <p class="font-bold">{{ t('checkout.guestInstructions.title') }}</p>
              <ul class="mt-2 grid list-disc gap-0.5 pl-4.5">
                <li>{{ t('checkout.guestInstructions.email') }}</li>
                <li>{{ t('checkout.guestInstructions.password') }}</li>
              </ul>
            </div>
            <p v-if="guestEmail && !guestEmailValid" class="mt-2 text-[13px] text-destructive">{{ t('error.email_invalid') }}</p>
          </template>
        </section>
      </div>

      <!-- 右栏：汇总 + 支付 -->
      <aside class="sticky top-[90px] rounded-xl border bg-card p-[22px]">
        <h2 class="mb-2.5 text-lg font-bold">{{ t('checkout.submitTitle') }}</h2>
        <p class="mb-4 rounded-sm border bg-secondary px-3 py-2.5 text-xs leading-relaxed text-muted-foreground">{{ t('checkout.submitHint') }}</p>

        <div class="grid gap-2.5">
          <div class="flex items-center justify-between text-[13.5px]"><span class="text-muted-foreground">{{ t('cart.itemsCount') }}</span><span class="font-semibold text-foreground">{{ totalItems }}</span></div>
          <div class="flex items-center justify-between text-[13.5px]"><span class="text-muted-foreground">{{ t('checkout.previewOriginal') }}</span><span class="font-semibold text-foreground">{{ formatPrice(previewOriginal, previewCurrency) }}</span></div>
          <template v-if="!isResellerTenant">
            <div class="flex items-center justify-between text-[13.5px]"><span class="text-muted-foreground">{{ t('checkout.previewCoupon') }}</span><span class="font-semibold" :class="hasPositiveAmount(previewCoupon) ? 'text-primary' : 'text-foreground'">{{ formatDiscountPrice(previewCoupon, previewCurrency) }}</span></div>
            <div class="flex items-center justify-between text-[13.5px]"><span class="text-muted-foreground">{{ t('checkout.previewPromotion') }}</span><span class="font-semibold" :class="hasPositiveAmount(previewPromotion) ? 'text-primary' : 'text-foreground'">{{ formatDiscountPrice(previewPromotion, previewCurrency) }}</span></div>
            <div class="flex items-center justify-between text-[13.5px]"><span class="text-muted-foreground">{{ t('checkout.previewWholesale') }}</span><span class="font-semibold" :class="hasPositiveAmount(previewWholesale) ? 'text-[color:var(--teal-strong)]' : 'text-foreground'">{{ formatDiscountPrice(previewWholesale, previewCurrency) }}</span></div>
          </template>
          <div v-if="Number(previewMemberDiscount) > 0" class="flex items-center justify-between text-[13.5px]"><span class="text-muted-foreground">{{ t('checkout.previewMemberDiscount') }}</span><span class="font-semibold text-[color:var(--gold-strong)]">-{{ formatPrice(previewMemberDiscount, previewCurrency) }}</span></div>
          <div class="mt-1 flex items-center justify-between border-t pt-3 font-bold text-foreground"><span>{{ t('checkout.previewTotal') }}</span><span class="text-[22px] text-primary tabular-nums">{{ formatPrice(previewTotal, previewCurrency) }}</span></div>
        </div>

        <div v-if="previewLoading || couponRefreshing" class="mt-3 text-xs text-muted-foreground">{{ previewStatusText }}</div>
        <div v-if="checkoutAlert" class="mt-3.5 rounded-sm px-3 py-2.5 text-[13px] font-semibold" :class="checkoutAlert.level === 'error' ? 'bg-destructive/10 text-destructive' : 'bg-warning/10 text-warning'">{{ checkoutAlert.message }}</div>

        <!-- 支付方式 -->
        <div class="my-[18px] border-t pt-4">
          <h3 class="mb-3 text-sm font-bold">{{ t('checkout.paymentMethod') }}</h3>

          <div v-if="showBalanceOption" class="mb-3 rounded-sm border bg-secondary p-3.5">
            <div class="flex items-start justify-between gap-2.5">
              <div>
                <div class="text-xs text-muted-foreground">{{ t('payment.walletBalanceLabel') }}</div>
                <div class="mt-0.5 font-bold text-foreground">{{ walletLoading ? t('common.loading') : formatPrice(walletBalance, previewCurrency) }}</div>
              </div>
              <label class="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
                <input v-model="useBalance" type="checkbox" class="h-4 w-4 accent-[var(--ui-accent)]" :disabled="walletOnlyPayment" />
                <span>{{ t('payment.useBalance') }}</span>
              </label>
            </div>
            <div v-if="walletOnlyPayment" class="mt-2 text-xs text-warning">{{ t('payment.walletOnlyHint') }}</div>
            <div v-if="useBalance" class="mt-2.5 grid gap-0.5 text-xs text-muted-foreground">
              <div>{{ t('payment.walletDeductLabel') }}：{{ expectedWalletPaidDisplay }}</div>
              <div v-if="!walletOnlyPayment">{{ t('payment.onlinePayLabel') }}：{{ expectedOnlinePayDisplay }}</div>
              <div v-if="walletOnlyPayment && expectedOnlinePayCents > 0" class="text-warning">{{ t('payment.walletInsufficientHint') }}</div>
            </div>
          </div>

          <template v-if="!walletOnlyPayment">
            <div v-if="requiresOnlineChannel && paymentChannels.length > 0" class="grid gap-2.5 sm:grid-cols-2">
              <button
                v-for="channel in paymentChannels"
                :key="channel.id"
                type="button"
                class="rounded-sm border-2 bg-card p-2.5 text-left"
                :class="[
                  selectedChannelId === channel.id && !isChannelDisabledForAmount(channel) ? 'border-primary bg-primary/10' : 'border-hairline-strong',
                  isChannelDisabledForAmount(channel) ? 'cursor-not-allowed opacity-55' : '',
                ]"
                :disabled="isChannelDisabledForAmount(channel)"
                :title="isChannelDisabledForAmount(channel) ? channelAmountLimitHint(channel) : ''"
                @click="handleSelectChannel(channel)"
              >
                <div class="flex items-center gap-2">
                  <img v-if="channel.icon" :src="getImageUrl(channel.icon)" loading="lazy" class="h-5 w-5 flex-none rounded-[4px] object-contain" />
                  <span class="truncate font-semibold text-foreground">{{ channel.name }}</span>
                </div>
                <div class="mt-1.5 grid gap-0.5 text-[11.5px] text-muted-foreground">
                  <div>{{ t('payment.feeLabel') }}：{{ formatChannelFeeRate(channel) }}</div>
                  <div>{{ t('payment.fixedFeeLabel') }}：{{ formatChannelFixedFee(channel) }}</div>
                </div>
                <div v-if="isChannelDisabledForAmount(channel)" class="mt-1 text-[11px] text-warning">{{ channelAmountLimitHint(channel) }}</div>
              </button>
            </div>
            <div v-else-if="requiresOnlineChannel && paymentChannels.length === 0" class="text-[13px] text-muted-foreground">{{ t('checkout.noPaymentChannels') }}</div>
          </template>
          <div v-if="!requiresOnlineChannel" class="text-[13px] text-[color:var(--teal-strong)]">{{ t('checkout.walletCoversAll') }}</div>
        </div>

        <Button class="h-11 w-full rounded-full font-bold" :disabled="!canSubmit" @click="handleSubmit">
          {{ submitting ? t('checkout.submitting') : t('checkout.submitButton') }}
        </Button>
      </aside>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ChevronRight, Package, ShoppingCart } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import ImageCaptcha from '../../components/captcha/ImageCaptcha.vue'
import TurnstileCaptcha from '../../components/captcha/TurnstileCaptcha.vue'
import CheckoutManualForm from '../../components/checkout/CheckoutManualForm.vue'
import VaultCheckoutSteps from './components/VaultCheckoutSteps.vue'
import { useCheckout } from '../../composables/useCheckout'
import { MIN_GUEST_ORDER_PASSWORD_LENGTH } from '../../utils/guestOrderAuth'

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
