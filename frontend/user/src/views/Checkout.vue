<template>
  <div class="min-h-screen bg-background pb-16 pt-24 text-foreground">
    <div class="container mx-auto px-4">
      <div class="mb-6 flex flex-wrap items-end justify-between gap-x-8 gap-y-4">
        <div>
          <h1 class="text-[28px] font-bold tracking-tight">{{ t('checkout.title') }}</h1>
          <p class="mt-1 text-sm text-muted-foreground">{{ t('checkout.subtitle') }}</p>
        </div>
        <CheckoutSteps
          plain
          class="w-full sm:w-[26rem]"
          current-step="checkout"
          :step-keys="isBuyNowMode ? ['checkout', 'payment'] : ['cart', 'checkout', 'payment']"
        />
      </div>

      <EmptyState
        v-if="cartItems.length === 0"
        icon="cart"
        :title="t('checkout.empty')"
        :action-label="t('checkout.emptyAction')"
        action-to="/products"
      />

      <div v-else class="grid grid-cols-1 gap-5 lg:grid-cols-12 lg:gap-6">
        <div class="flex flex-col gap-5 lg:col-span-7">
          <section :class="panel">
            <h2 :class="panelTitle">{{ t('checkout.itemsTitle') }}</h2>
            <div class="mt-4 flex flex-col gap-2">
              <div
                v-for="item in cartItems"
                :key="cartItemKey(item)"
                class="flex items-center gap-3 rounded-2xl p-2 pr-3 md:pr-4"
                :class="itemStockExceeded(item) ? 'bg-warning/10' : 'bg-secondary/60'"
              >
                <div class="h-14 w-14 shrink-0 overflow-hidden rounded-xl bg-secondary md:h-16 md:w-16">
                  <SmartImage :src="checkoutItemImage(item)" alt="" img-class="h-full w-full object-cover" />
                </div>
                <div class="min-w-0 flex-1">
                  <router-link :to="`/products/${item.slug}`" class="line-clamp-2 text-sm font-semibold leading-snug hover:underline md:text-[15px]">
                    {{ getLocalizedText(item.title) }}
                  </router-link>
                  <p class="mt-0.5 truncate text-xs text-muted-foreground">
                    <template v-if="itemSkuDisplay(item)">{{ itemSkuDisplay(item) }} · </template>× {{ item.quantity }}
                  </p>
                  <p v-if="itemStockHint(item)" class="mt-0.5 text-xs" :class="itemStockExceeded(item) ? 'text-warning' : 'text-muted-foreground'">
                    {{ itemStockHint(item) }}
                  </p>
                </div>
                <div class="shrink-0 text-right">
                  <p
                    class="whitespace-nowrap text-[17px] font-bold leading-tight tracking-tight tabular-nums"
                    :class="checkoutItemHasPriceDiscount(item) && 'text-rose-600 dark:text-rose-300'"
                  >
                    {{ checkoutItemPriceParts(item).integer }}{{ checkoutItemPriceParts(item).decimal }}<span class="ml-1 text-xs font-medium text-muted-foreground">{{ checkoutItemCurrency }}</span>
                  </p>
                  <p v-if="checkoutItemHasPriceDiscount(item)" class="whitespace-nowrap text-xs text-muted-foreground">
                    <span class="line-through">{{ checkoutItemOriginalPriceParts(item).integer }}{{ checkoutItemOriginalPriceParts(item).decimal }}</span>
                    {{ t('checkout.discountedPriceLabel') }}
                  </p>
                </div>
              </div>
            </div>
          </section>

          <CheckoutManualForm
            class="theme-float rounded-[22px]! border-0!"
            :manual-form-products="manualFormProducts"
            v-model="manualFormData"
            :submit-attempted="submitAttempted"
            :get-manual-field-label="getManualFieldLabel"
            :get-manual-field-placeholder="getManualFieldPlaceholder"
            :manual-field-error="manualFieldError"
          />

          <section v-if="!userAuthStore.isAuthenticated" :class="panel">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <h2 :class="panelTitle">{{ t('checkout.modeTitle') }}</h2>
              <div class="flex rounded-full bg-secondary p-1 text-sm font-semibold">
                <button
                  type="button"
                  class="h-9 rounded-full px-4 transition-colors"
                  :class="checkoutMode === 'guest' ? 'bg-foreground text-background' : 'text-muted-foreground hover:text-foreground'"
                  :aria-pressed="checkoutMode === 'guest'"
                  @click="checkoutMode = 'guest'"
                >
                  {{ t('checkout.guestPurchase') }}
                </button>
                <router-link to="/auth/login" class="flex h-9 items-center rounded-full px-4 text-muted-foreground transition-colors hover:text-foreground">
                  {{ t('checkout.memberPurchase') }}
                </router-link>
              </div>
            </div>

            <template v-if="checkoutMode === 'guest'">
              <div class="mt-4 grid grid-cols-1 gap-3 md:grid-cols-2">
                <Input v-model="guestEmail" type="email" class="h-11 rounded-xl" :placeholder="t('checkout.guestEmailPlaceholder')" />
                <Input v-model="guestPassword" type="password" class="h-11 rounded-xl" :placeholder="t('checkout.guestPasswordPlaceholder')" />
              </div>
              <p v-if="guestEmail && !guestEmailValid" class="mt-2 text-xs text-destructive">{{ t('error.email_invalid') }}</p>

              <div v-if="guestCaptchaEnabled" class="mt-4 space-y-2">
                <p class="text-[13px] font-semibold text-muted-foreground">{{ t('auth.common.captchaLabel') }}</p>
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

              <div class="mt-4 rounded-2xl bg-secondary/60 px-4 py-3 text-[13px] text-muted-foreground">
                <p class="font-semibold text-foreground">{{ t('checkout.guestInstructions.title') }}</p>
                <ul class="mt-1.5 list-disc space-y-1 pl-5">
                  <li>{{ t('checkout.guestInstructions.email') }}</li>
                  <li>{{ t('checkout.guestInstructions.password') }}</li>
                </ul>
              </div>
            </template>
          </section>

          <section v-if="!isResellerTenant" :class="panel">
            <h2 :class="panelTitle">{{ t('checkout.couponTitle') }}</h2>
            <Input v-model="couponCode" type="text" class="mt-4 h-11 rounded-xl" :placeholder="t('checkout.couponPlaceholder')" />
          </section>
        </div>

        <aside class="theme-float h-fit rounded-[22px] bg-card p-5 md:rounded-[28px] md:p-6 lg:sticky lg:top-24 lg:col-span-5">
          <h2 :class="panelTitle">{{ t('checkout.submitTitle') }}</h2>

          <!-- 没有金额的优惠行不占位 -->
          <dl class="mt-4 space-y-2.5 text-sm text-muted-foreground">
            <div class="flex items-center justify-between">
              <dt>{{ t('cart.itemsCount') }}</dt>
              <dd class="tabular-nums text-foreground">{{ totalItems }}</dd>
            </div>
            <div class="flex items-center justify-between">
              <dt>{{ t('checkout.previewOriginal') }}</dt>
              <dd class="tabular-nums text-foreground">{{ formatPrice(previewOriginal, previewCurrency) }}</dd>
            </div>
            <template v-if="!isResellerTenant">
              <div v-if="hasPositiveAmount(previewCoupon)" class="flex items-center justify-between">
                <dt>{{ t('checkout.previewCoupon') }}</dt>
                <dd class="tabular-nums text-rose-600 dark:text-rose-300">{{ formatDiscountPrice(previewCoupon, previewCurrency) }}</dd>
              </div>
              <div v-if="hasPositiveAmount(previewPromotion)" class="flex items-center justify-between">
                <dt>{{ t('checkout.previewPromotion') }}</dt>
                <dd class="tabular-nums text-rose-600 dark:text-rose-300">{{ formatDiscountPrice(previewPromotion, previewCurrency) }}</dd>
              </div>
              <div v-if="hasPositiveAmount(previewWholesale)" class="flex items-center justify-between">
                <dt>{{ t('checkout.previewWholesale') }}</dt>
                <dd class="tabular-nums text-emerald-600 dark:text-emerald-300">{{ formatDiscountPrice(previewWholesale, previewCurrency) }}</dd>
              </div>
            </template>
            <div v-if="Number(previewMemberDiscount) > 0" class="flex items-center justify-between">
              <dt>{{ t('checkout.previewMemberDiscount') }}</dt>
              <dd class="tabular-nums text-amber-600 dark:text-amber-300">-{{ formatPrice(previewMemberDiscount, previewCurrency) }}</dd>
            </div>
          </dl>
          <div class="mt-4 flex items-baseline justify-between border-t pt-4">
            <span class="text-sm text-muted-foreground">{{ t('checkout.previewTotal') }}</span>
            <span class="text-[32px] font-bold leading-none tracking-tight tabular-nums" :class="(previewLoading || couponRefreshing) && 'opacity-40'">
              {{ formatPrice(previewTotal, '') }}<span class="ml-1.5 text-sm font-medium text-muted-foreground">{{ previewCurrency }}</span>
            </span>
          </div>
          <p v-if="previewLoading || couponRefreshing" class="mt-2 text-right text-xs text-muted-foreground">{{ previewStatusText }}</p>

          <Alert
            v-if="checkoutAlert"
            :variant="pageAlertVariant(checkoutAlert.level)"
            :class="['mt-4', pageAlertToneClass(checkoutAlert.level)]"
          >
            <AlertDescription>{{ checkoutAlert.message }}</AlertDescription>
          </Alert>

          <!-- Payment Channel Selection -->
          <div class="mt-6">
            <h3 class="mb-2.5 text-[13px] font-semibold text-muted-foreground">{{ t('checkout.paymentMethod') }}</h3>

            <!-- Wallet Balance -->
            <div v-if="showBalanceOption" class="mb-2 rounded-2xl bg-secondary px-4 py-3">
              <label class="flex cursor-pointer items-center justify-between gap-3">
                <span>
                  <span class="block text-xs text-muted-foreground">{{ t('payment.walletBalanceLabel') }}</span>
                  <span class="mt-0.5 block text-sm font-semibold tabular-nums">
                    {{ walletLoading ? t('common.loading') : formatPrice(walletBalance, previewCurrency) }}
                  </span>
                </span>
                <span class="inline-flex items-center gap-2 text-[13px] font-medium">
                  {{ t('payment.useBalance') }}
                  <input v-model="useBalance" type="checkbox" class="h-4 w-4 accent-foreground" :disabled="walletOnlyPayment" />
                </span>
              </label>
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
                  :aria-pressed="selectedChannelId === channel.id"
                  @click="handleSelectChannel(channel)"
                  class="min-h-[48px] rounded-2xl border-2 px-3 py-2 text-left transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:cursor-not-allowed disabled:opacity-50"
                  :class="selectedChannelId === channel.id && !isChannelDisabledForAmount(channel) ? 'border-foreground bg-card' : 'border-transparent bg-secondary hover:border-foreground/40'">
                  <div class="flex items-center gap-2">
                    <img v-if="channel.icon" :src="getImageUrl(channel.icon)" alt="" loading="lazy" class="h-5 w-5 shrink-0 rounded object-contain" />
                    <div class="truncate text-sm font-semibold">{{ channel.name }}</div>
                  </div>
                  <div v-if="channel.fee_policy === 'customer_surcharge'" class="mt-1 space-y-0.5 text-xs text-warning">
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
            class="mt-6 h-12 w-full rounded-full bg-foreground font-semibold text-background hover:bg-foreground/85"
            :disabled="!canSubmit"
            @click="handleSubmit"
          >
            {{ submitting ? t('checkout.submitting') : t('checkout.submitButton') }}
          </Button>
          <p class="mt-3 text-xs leading-relaxed text-muted-foreground">{{ t('checkout.submitHint') }}</p>
        </aside>
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

const { t } = useI18n()

const panel = 'theme-float rounded-[22px] bg-card p-5 md:p-6'
const panelTitle = 'text-[17px] font-bold'

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
