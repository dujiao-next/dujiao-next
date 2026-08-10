<template>
  <div class="min-h-screen bg-background text-foreground pt-24 pb-16">
    <div class="container mx-auto px-4">
      <div class="mb-8">
        <h1 class="mb-2 text-3xl font-black text-foreground">{{ t('checkout.title') }}</h1>
        <p class="text-sm text-muted-foreground">{{ t('checkout.subtitle') }}</p>
      </div>

      <CheckoutSteps
        class="mb-8"
        current-step="checkout"
        :step-keys="isBuyNowMode ? ['checkout', 'payment'] : ['cart', 'checkout', 'payment']"
      />

      <EmptyState
        v-if="cartItems.length === 0"
        icon="cart"
        :title="t('checkout.empty')"
        :action-label="t('checkout.emptyAction')"
        action-to="/products"
      />

      <div v-else class="grid grid-cols-1 gap-8 lg:grid-cols-3">
        <div class="space-y-6 lg:col-span-2">
          <div class="rounded-2xl border bg-card text-card-foreground p-6">
            <h2 class="mb-4 text-lg font-bold text-foreground">{{ t('checkout.itemsTitle') }}</h2>
            <div class="space-y-4">
              <div
                v-for="item in cartItems"
                :key="cartItemKey(item)"
                class="rounded-xl border p-4"
                :class="itemStockExceeded(item)
                  ? 'border-warning/40 bg-warning/10'
                  : 'bg-secondary'"
              >
                <div class="flex min-w-0 items-start gap-3">
                  <div class="h-16 w-16 shrink-0 overflow-hidden rounded-xl border bg-muted transition-all duration-200 hover:-translate-y-0.5 hover:shadow-sm sm:h-20 sm:w-20">
                    <SmartImage
                      :src="checkoutItemImage(item)"
                      :alt="getLocalizedText(item.title)"
                      img-class="h-full w-full object-cover"
                    />
                  </div>
                  <div class="min-w-0">
                    <router-link
                      :to="`/products/${item.slug}`"
                      class="line-clamp-2 font-semibold text-primary hover:underline"
                    >
                      {{ getLocalizedText(item.title) }}
                    </router-link>
                    <div class="mt-1 text-xs text-muted-foreground">{{ t('checkout.quantityLabel') }}：{{ item.quantity }}</div>
                    <div v-if="itemSkuDisplay(item)" class="mt-1 text-xs text-muted-foreground">{{ t('checkout.skuLabel') }}：{{ itemSkuDisplay(item) }}</div>
                    <div
                      v-if="itemStockHint(item)"
                      class="mt-1 text-xs"
                      :class="itemStockExceeded(item)
                        ? 'text-warning'
                        : 'text-muted-foreground'"
                    >
                      {{ itemStockHint(item) }}
                    </div>
                    <div class="mt-2 flex flex-wrap items-baseline gap-3">
                      <span
                        class="inline-flex items-baseline whitespace-nowrap"
                        :class="checkoutItemHasPriceDiscount(item) ? 'text-rose-600 dark:text-rose-300' : 'text-foreground'"
                      >
                        <span class="text-xl font-black leading-none">{{ checkoutItemPriceParts(item).integer }}</span>
                        <span class="text-xs font-semibold">{{ checkoutItemPriceParts(item).decimal }}</span>
                        <span class="ml-1 text-xs font-semibold">{{ checkoutItemCurrency }}</span>
                        <span v-if="checkoutItemHasPriceDiscount(item)" class="ml-1.5 text-xs font-normal">
                          {{ t('checkout.discountedPriceLabel') }}
                        </span>
                      </span>
                      <span
                        v-if="checkoutItemHasPriceDiscount(item)"
                        class="inline-flex items-baseline whitespace-nowrap text-xs text-muted-foreground line-through"
                      >
                        <span>{{ checkoutItemOriginalPriceParts(item).integer }}</span>
                        <span>{{ checkoutItemOriginalPriceParts(item).decimal }}</span>
                        <span class="ml-1">{{ checkoutItemCurrency }}</span>
                      </span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <CheckoutManualForm
            :manual-form-products="manualFormProducts"
            v-model="manualFormData"
            :submit-attempted="submitAttempted"
            :get-manual-field-label="getManualFieldLabel"
            :get-manual-field-placeholder="getManualFieldPlaceholder"
            :manual-field-error="manualFieldError"
          />

          <div v-if="!isResellerTenant" class="rounded-2xl border bg-card text-card-foreground p-6">
            <h2 class="mb-4 text-lg font-bold text-foreground">{{ t('checkout.couponTitle') }}</h2>
            <Input
              v-model="couponCode"
              type="text"
              class="h-11"
              :placeholder="t('checkout.couponPlaceholder')"
            />
          </div>

          <div
            v-if="!userAuthStore.isAuthenticated"
            class="space-y-4 rounded-2xl border bg-card text-card-foreground p-6"
          >
            <h2 class="text-lg font-bold text-foreground">{{ t('checkout.modeTitle') }}</h2>
            <div class="flex flex-wrap gap-3">
              <Button
                :variant="checkoutMode === 'guest' ? 'default' : 'secondary'"
                @click="checkoutMode = 'guest'"
              >
                {{ t('checkout.guestPurchase') }}
              </Button>
              <Button as-child variant="secondary">
                <router-link to="/auth/login">
                  {{ t('checkout.memberPurchase') }}
                </router-link>
              </Button>
            </div>

            <div v-if="checkoutMode === 'guest'" class="grid grid-cols-1 gap-4 md:grid-cols-2">
              <Input
                v-model="guestEmail"
                type="email"
                class="h-11"
                :placeholder="t('checkout.guestEmailPlaceholder')"
              />
              <Input
                v-model="guestPassword"
                type="password"
                :minlength="MIN_GUEST_ORDER_PASSWORD_LENGTH"
                autocomplete="new-password"
                class="h-11"
                :placeholder="t('checkout.guestPasswordPlaceholder')"
              />
            </div>

            <div v-if="checkoutMode === 'guest' && guestCaptchaEnabled" class="space-y-2">
              <p class="text-xs font-semibold uppercase tracking-[0.14em] text-muted-foreground">{{ t('auth.common.captchaLabel') }}</p>
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

            <div v-if="checkoutMode === 'guest'" class="mb-3 rounded-xl border border-success/40 bg-success/10 p-3 text-sm text-success">
              <p class="font-semibold">{{ t('checkout.guestInstructions.title') }}</p>
              <ul class="mt-2 space-y-1 list-disc pl-5">
                <li>{{ t('checkout.guestInstructions.email') }}</li>
                <li>{{ t('checkout.guestInstructions.password') }}</li>
              </ul>
            </div>
            <p v-if="checkoutMode === 'guest' && guestEmail && !guestEmailValid" class="text-xs text-destructive">
              {{ t('error.email_invalid') }}
            </p>
          </div>
        </div>

        <div class="h-fit rounded-2xl border bg-card text-card-foreground p-6 lg:sticky lg:top-24">
          <h2 class="mb-4 text-lg font-bold text-foreground">{{ t('checkout.submitTitle') }}</h2>
          <div class="mb-4 rounded-lg border bg-secondary p-3 text-xs text-muted-foreground">
            {{ t('checkout.submitHint') }}
          </div>

          <div class="mb-4 space-y-3 text-sm text-muted-foreground">
            <div class="flex items-center justify-between">
              <span>{{ t('cart.itemsCount') }}</span>
              <span class="font-mono text-foreground">{{ totalItems }}</span>
            </div>
            <div class="flex items-center justify-between">
              <span>{{ t('checkout.previewOriginal') }}</span>
              <span class="font-mono text-foreground">{{ formatPrice(previewOriginal, previewCurrency) }}</span>
            </div>
            <template v-if="!isResellerTenant">
              <div class="flex items-center justify-between">
                <span>{{ t('checkout.previewCoupon') }}</span>
                <span
                  class="font-mono"
                  :class="hasPositiveAmount(previewCoupon) ? 'text-rose-600 dark:text-rose-300' : 'text-foreground'"
                >
                  {{ formatDiscountPrice(previewCoupon, previewCurrency) }}
                </span>
              </div>
              <div class="flex items-center justify-between">
                <span>{{ t('checkout.previewPromotion') }}</span>
                <span
                  class="font-mono"
                  :class="hasPositiveAmount(previewPromotion) ? 'text-rose-600 dark:text-rose-300' : 'text-foreground'"
                >
                  {{ formatDiscountPrice(previewPromotion, previewCurrency) }}
                </span>
              </div>
              <div class="flex items-center justify-between">
                <span>{{ t('checkout.previewWholesale') }}</span>
                <span
                  class="font-mono"
                  :class="hasPositiveAmount(previewWholesale) ? 'text-emerald-600 dark:text-emerald-300' : 'text-foreground'"
                >
                  {{ formatDiscountPrice(previewWholesale, previewCurrency) }}
                </span>
              </div>
            </template>
            <div v-if="Number(previewMemberDiscount) > 0" class="flex items-center justify-between">
              <span>{{ t('checkout.previewMemberDiscount') }}</span>
              <span class="font-mono text-amber-600 dark:text-amber-300">-{{ formatPrice(previewMemberDiscount, previewCurrency) }}</span>
            </div>
            <div class="flex items-center justify-between border-t pt-3 text-foreground">
              <span class="font-semibold">{{ t('checkout.previewTotal') }}</span>
              <span class="font-mono text-lg font-bold">{{ formatPrice(previewTotal, previewCurrency) }}</span>
            </div>
          </div>

          <div v-if="previewLoading || couponRefreshing" class="mb-3 text-xs text-muted-foreground">
            {{ previewStatusText }}
          </div>
          <Alert
            v-if="checkoutAlert"
            :variant="pageAlertVariant(checkoutAlert.level)"
            :class="['mb-4', pageAlertToneClass(checkoutAlert.level)]"
          >
            <AlertDescription>{{ checkoutAlert.message }}</AlertDescription>
          </Alert>

          <!-- Payment Channel Selection -->
          <div class="mb-4 border-t pt-4">
            <h3 class="mb-3 text-sm font-bold text-foreground">{{ t('checkout.paymentMethod') }}</h3>

            <!-- Wallet Balance -->
            <div v-if="showBalanceOption" class="mb-3 rounded-lg border bg-secondary p-3">
              <div class="flex items-center justify-between">
                <div>
                  <div class="text-xs text-muted-foreground">{{ t('payment.walletBalanceLabel') }}</div>
                  <div class="mt-0.5 text-sm font-semibold text-foreground">
                    {{ walletLoading ? t('common.loading') : formatPrice(walletBalance, previewCurrency) }}
                  </div>
                </div>
                <label class="inline-flex items-center gap-2 text-xs text-muted-foreground">
                  <input v-model="useBalance" type="checkbox" class="h-4 w-4 accent-primary" :disabled="walletOnlyPayment" />
                  <span>{{ t('payment.useBalance') }}</span>
                </label>
              </div>
              <div v-if="walletOnlyPayment" class="mt-2 text-xs text-warning">
                {{ t('payment.walletOnlyHint') }}
              </div>
              <div v-if="useBalance" class="mt-2 space-y-1 text-xs text-muted-foreground">
                <div>{{ t('payment.walletDeductLabel') }}：{{ expectedWalletPaidDisplay }}</div>
                <div v-if="!walletOnlyPayment">{{ t('payment.onlinePayLabel') }}：{{ expectedOnlinePayDisplay }}</div>
                <div v-if="walletOnlyPayment && expectedOnlinePayCents > 0" class="text-warning">
                  {{ t('payment.walletInsufficientHint') }}
                </div>
              </div>
            </div>

            <!-- Channel Grid (hidden in wallet-only mode) -->
            <template v-if="!walletOnlyPayment">
              <div v-if="requiresOnlineChannel && paymentChannels.length > 0" class="grid grid-cols-2 gap-2">
                <button v-for="channel in paymentChannels" :key="channel.id"
                  type="button"
                  :disabled="isChannelDisabledForAmount(channel)"
                  :title="isChannelDisabledForAmount(channel) ? channelAmountLimitHint(channel) : ''"
                  @click="handleSelectChannel(channel)"
                  class="text-left border rounded-lg p-2.5 transition-colors disabled:cursor-not-allowed disabled:opacity-60"
                  :class="selectedChannelId === channel.id && !isChannelDisabledForAmount(channel) ? 'border-primary/45 bg-primary/10' : 'bg-card hover:border-foreground/25'">
                  <div class="flex items-center gap-2">
                    <img v-if="channel.icon" :src="getImageUrl(channel.icon)" loading="lazy" class="h-5 w-5 rounded object-contain shrink-0" />
                    <div class="text-sm text-foreground font-medium truncate">{{ channel.name }}</div>
                  </div>
                  <div class="mt-1 space-y-0.5 text-xs text-muted-foreground">
                    <div>{{ t('payment.feeLabel') }}：{{ formatChannelFeeRate(channel) }}</div>
                    <div>{{ t('payment.fixedFeeLabel') }}：{{ formatChannelFixedFee(channel) }}</div>
                  </div>
                  <div v-if="isChannelDisabledForAmount(channel)" class="mt-1 text-xs text-warning">
                    {{ channelAmountLimitHint(channel) }}
                  </div>
                </button>
              </div>
              <div v-else-if="requiresOnlineChannel && paymentChannels.length === 0" class="text-xs text-muted-foreground">
                {{ t('checkout.noPaymentChannels') }}
              </div>
            </template>
            <div v-if="!requiresOnlineChannel" class="text-xs text-success">
              {{ t('checkout.walletCoversAll') }}
            </div>
          </div>

          <Button
            size="lg"
            class="w-full font-semibold"
            :disabled="!canSubmit"
            @click="handleSubmit"
          >
            {{ submitting ? t('checkout.submitting') : t('checkout.submitButton') }}
          </Button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { pageAlertVariant, pageAlertToneClass } from '../utils/alerts'
import ImageCaptcha from '../components/captcha/ImageCaptcha.vue'
import TurnstileCaptcha from '../components/captcha/TurnstileCaptcha.vue'
import CheckoutManualForm from '../components/checkout/CheckoutManualForm.vue'
import EmptyState from '../components/EmptyState.vue'
import SmartImage from '../components/SmartImage.vue'
import CheckoutSteps from '../components/checkout/CheckoutSteps.vue'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { useCheckout } from '../composables/useCheckout'
import { MIN_GUEST_ORDER_PASSWORD_LENGTH } from '../utils/guestOrderAuth'

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

// 这两个引用仅通过模板字符串 ref 绑定（刷新/重置逻辑在 composable 内），
// vue-tsc 不将字符串模板 ref 计为使用，这里显式标记避免 noUnusedLocals 误报。
void guestImageCaptchaRef
void guestTurnstileRef
</script>
