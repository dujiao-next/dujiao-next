<template>
  <div class="min-h-screen pb-16 pt-[88px] text-foreground">
    <div class="mx-auto w-full max-w-[1240px] px-4 sm:px-6">
      <div class="mb-7 flex flex-wrap items-end justify-between gap-x-8 gap-y-4 border-b pb-5">
        <div>
          <h1 class="text-[26px] font-bold tracking-[-0.025em] md:text-[30px]">{{ t('checkout.title') }}</h1>
          <p class="mt-1 text-sm text-muted-foreground">{{ t('checkout.subtitle') }}</p>
        </div>
        <AtelierCheckoutSteps
          class="w-full sm:w-[22rem]"
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

      <div v-else class="grid grid-cols-1 items-start gap-6 lg:grid-cols-12">
        <!-- 左侧 7 列：商品清单 / 人工交付表单 / 购买方式 / 优惠码 -->
        <div class="flex flex-col gap-5 lg:col-span-7">
          <section class="atelier-card rounded-[16px] bg-card p-5 md:p-6">
            <h2 class="border-b pb-3 text-base font-bold tracking-[-0.015em]">{{ t('checkout.itemsTitle') }}</h2>
            <div class="mt-3 divide-y">
              <div
                v-for="item in cartItems"
                :key="cartItemKey(item)"
                class="flex items-center gap-3.5 py-3 first:pt-1 last:pb-0"
                :class="itemStockExceeded(item) && 'rounded-[12px] bg-warning-soft px-3'"
              >
                <div class="h-14 w-14 shrink-0 overflow-hidden rounded-[12px] border bg-background">
                  <SmartImage :src="checkoutItemImage(item)" alt="" img-class="h-full w-full object-cover" />
                </div>
                <div class="min-w-0 flex-1">
                  <RouterLink
                    :to="`/products/${item.slug}`"
                    class="line-clamp-2 text-sm font-semibold leading-snug text-foreground hover:underline"
                  >
                    {{ getLocalizedText(item.title) }}
                  </RouterLink>
                  <p class="mt-0.5 truncate text-xs text-muted-foreground">
                    <template v-if="itemSkuDisplay(item)">{{ itemSkuDisplay(item) }} · </template>× {{ item.quantity }}
                  </p>
                  <p
                    v-if="itemStockHint(item)"
                    class="mt-0.5 text-xs"
                    :class="itemStockExceeded(item) ? 'font-medium text-warning' : 'text-muted-foreground'"
                  >
                    {{ itemStockHint(item) }}
                  </p>
                </div>
                <div class="shrink-0 text-right tabular-nums">
                  <p
                    class="whitespace-nowrap text-[16px] font-bold leading-none tracking-[-0.02em]"
                    :class="checkoutItemHasPriceDiscount(item) ? 'text-danger' : 'text-foreground'"
                  >
                    {{ checkoutItemPriceParts(item).integer }}<span class="text-xs opacity-75">{{ checkoutItemPriceParts(item).decimal }}</span>
                    <span class="ml-1 text-[11px] font-semibold text-muted-foreground">{{ checkoutItemCurrency }}</span>
                  </p>
                  <p v-if="checkoutItemHasPriceDiscount(item)" class="mt-1 whitespace-nowrap text-[11px] text-muted-foreground">
                    <span class="line-through">{{ checkoutItemOriginalPriceParts(item).integer }}{{ checkoutItemOriginalPriceParts(item).decimal }}</span>
                    {{ t('checkout.discountedPriceLabel') }}
                  </p>
                </div>
              </div>
            </div>
          </section>

          <CheckoutManualForm
            class="atelier-card rounded-[16px]!"
            :manual-form-products="manualFormProducts"
            v-model="manualFormData"
            :submit-attempted="submitAttempted"
            :get-manual-field-label="getManualFieldLabel"
            :get-manual-field-placeholder="getManualFieldPlaceholder"
            :manual-field-error="manualFieldError"
          />

          <section v-if="!userAuthStore.isAuthenticated" class="atelier-card rounded-[16px] bg-card p-5 md:p-6">
            <div class="flex flex-wrap items-center justify-between gap-3 border-b pb-3.5">
              <h2 class="text-base font-bold tracking-[-0.015em]">{{ t('checkout.modeTitle') }}</h2>
              <div class="inline-flex rounded-full bg-secondary p-1 text-xs font-semibold">
                <button
                  type="button"
                  class="h-8 rounded-full px-3.5 transition-colors"
                  :class="checkoutMode === 'guest' ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground'"
                  :aria-pressed="checkoutMode === 'guest'"
                  @click="checkoutMode = 'guest'"
                >
                  {{ t('checkout.guestPurchase') }}
                </button>
                <RouterLink
                  to="/auth/login"
                  class="flex h-8 items-center rounded-full px-3.5 text-muted-foreground transition-colors hover:text-foreground"
                >
                  {{ t('checkout.memberPurchase') }}
                </RouterLink>
              </div>
            </div>

            <template v-if="checkoutMode === 'guest'">
              <div class="mt-4 grid grid-cols-1 gap-3 md:grid-cols-2">
                <Input v-model="guestEmail" type="email" class="h-11 rounded-[12px]" :placeholder="t('checkout.guestEmailPlaceholder')" />
                <Input v-model="guestPassword" type="password" class="h-11 rounded-[12px]" :placeholder="t('checkout.guestPasswordPlaceholder')" />
              </div>
              <p v-if="guestEmail && !guestEmailValid" class="mt-2 text-xs text-destructive">{{ t('error.email_invalid') }}</p>

              <div v-if="guestCaptchaEnabled" class="mt-4 space-y-2">
                <p class="text-[12.5px] font-semibold text-muted-foreground">{{ t('auth.common.captchaLabel') }}</p>
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

              <div class="mt-4 rounded-[14px] border bg-background px-4 py-3 text-xs text-muted-foreground">
                <p class="font-semibold text-foreground">{{ t('checkout.guestInstructions.title') }}</p>
                <ul class="mt-1.5 list-disc space-y-1 pl-4">
                  <li>{{ t('checkout.guestInstructions.email') }}</li>
                  <li>{{ t('checkout.guestInstructions.password') }}</li>
                </ul>
              </div>
            </template>
          </section>

          <section v-if="!isResellerTenant" class="atelier-card rounded-[16px] bg-card p-5 md:p-6">
            <h2 class="text-base font-bold tracking-[-0.015em]">{{ t('checkout.couponTitle') }}</h2>
            <Input v-model="couponCode" type="text" class="mt-3.5 h-11 rounded-[12px]" :placeholder="t('checkout.couponPlaceholder')" />
          </section>
        </div>

        <!-- 右侧 5 列：吸顶结算账单 + 支付渠道 -->
        <aside class="atelier-card h-fit rounded-[16px] bg-card p-6 lg:sticky lg:top-[84px] lg:col-span-5">
          <h2 class="border-b pb-3.5 text-base font-bold tracking-[-0.015em]">{{ t('checkout.submitTitle') }}</h2>

          <dl class="mt-4 space-y-2.5 text-sm text-muted-foreground">
            <div class="flex items-center justify-between">
              <dt>{{ t('cart.itemsCount') }}</dt>
              <dd class="font-mono font-medium text-foreground">{{ totalItems }}</dd>
            </div>
            <div class="flex items-center justify-between">
              <dt>{{ t('checkout.previewOriginal') }}</dt>
              <dd class="tabular-nums text-foreground">{{ formatPrice(previewOriginal, previewCurrency) }}</dd>
            </div>
            <template v-if="!isResellerTenant">
              <div v-if="hasPositiveAmount(previewCoupon)" class="flex items-center justify-between">
                <dt>{{ t('checkout.previewCoupon') }}</dt>
                <dd class="tabular-nums text-danger">{{ formatDiscountPrice(previewCoupon, previewCurrency) }}</dd>
              </div>
              <div v-if="hasPositiveAmount(previewPromotion)" class="flex items-center justify-between">
                <dt>{{ t('checkout.previewPromotion') }}</dt>
                <dd class="tabular-nums text-danger">{{ formatDiscountPrice(previewPromotion, previewCurrency) }}</dd>
              </div>
              <div v-if="hasPositiveAmount(previewWholesale)" class="flex items-center justify-between">
                <dt>{{ t('checkout.previewWholesale') }}</dt>
                <dd class="tabular-nums text-success">{{ formatDiscountPrice(previewWholesale, previewCurrency) }}</dd>
              </div>
            </template>
            <div v-if="Number(previewMemberDiscount) > 0" class="flex items-center justify-between">
              <dt>{{ t('checkout.previewMemberDiscount') }}</dt>
              <dd class="tabular-nums text-warning">-{{ formatPrice(previewMemberDiscount, previewCurrency) }}</dd>
            </div>
          </dl>

          <div class="mt-4 rounded-[14px] border bg-background px-4 py-3.5">
            <div class="flex items-baseline justify-between gap-2">
              <span class="text-xs text-muted-foreground">{{ t('checkout.previewTotal') }}</span>
              <span
                class="flex items-baseline tabular-nums"
                :class="(previewLoading || couponRefreshing) && 'opacity-40'"
              >
                <b class="text-[30px] font-bold leading-none tracking-[-0.03em] text-foreground">{{ previewTotalParts.int }}</b>
                <span v-if="previewTotalParts.dec" class="text-[17px] font-semibold text-foreground/75">.{{ previewTotalParts.dec }}</span>
                <span v-if="previewCurrency" class="ml-1.5 text-xs font-semibold text-muted-foreground">{{ previewCurrency }}</span>
              </span>
            </div>
            <p v-if="previewLoading || couponRefreshing" class="mt-1.5 text-right text-xs text-muted-foreground">
              {{ previewStatusText }}
            </p>
          </div>

          <Alert
            v-if="checkoutAlert"
            :variant="pageAlertVariant(checkoutAlert.level)"
            :class="['mt-4', pageAlertToneClass(checkoutAlert.level)]"
          >
            <AlertDescription>{{ checkoutAlert.message }}</AlertDescription>
          </Alert>

          <!-- 支付方式 -->
          <div class="mt-5">
            <h3 class="mb-2.5 text-[12.5px] font-semibold text-muted-foreground">{{ t('checkout.paymentMethod') }}</h3>

            <div v-if="showBalanceOption" class="mb-2.5 rounded-[14px] border bg-background px-4 py-3">
              <label class="flex cursor-pointer items-center justify-between gap-3">
                <span>
                  <span class="block text-xs text-muted-foreground">{{ t('payment.walletBalanceLabel') }}</span>
                  <span class="mt-0.5 block text-sm font-semibold tabular-nums">
                    {{ walletLoading ? t('common.loading') : formatPrice(walletBalance, previewCurrency) }}
                  </span>
                </span>
                <span class="inline-flex items-center gap-2 text-[13px] font-medium">
                  {{ t('payment.useBalance') }}
                  <Checkbox v-model="useBalance" class="bg-card" :disabled="walletOnlyPayment" />
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

            <template v-if="!walletOnlyPayment">
              <div v-if="requiresOnlineChannel && paymentChannels.length > 0" class="grid grid-cols-2 gap-2">
                <button
                  v-for="channel in paymentChannels"
                  :key="channel.id"
                  type="button"
                  :disabled="isChannelDisabledForAmount(channel)"
                  :title="isChannelDisabledForAmount(channel) ? channelAmountLimitHint(channel) : ''"
                  :aria-pressed="selectedChannelId === channel.id"
                  class="relative min-h-[50px] rounded-[14px] border-[1.5px] px-3 py-2.5 text-left transition-all focus-visible:outline-2 focus-visible:outline-primary disabled:cursor-not-allowed disabled:opacity-45"
                  :class="selectedChannelId === channel.id && !isChannelDisabledForAmount(channel)
                    ? 'border-primary bg-primary/10 shadow-xs'
                    : 'border-border bg-card hover:border-hairline-strong'"
                  @click="handleSelectChannel(channel)"
                >
                  <span
                    v-if="selectedChannelId === channel.id && !isChannelDisabledForAmount(channel)"
                    class="absolute right-2 top-2 grid h-4 w-4 place-items-center rounded-full bg-primary text-primary-foreground"
                    aria-hidden="true"
                  >
                    <Check class="h-2.5 w-2.5" :stroke-width="3" />
                  </span>
                  <div class="flex items-center gap-2 pr-4">
                    <img v-if="channel.icon" :src="getImageUrl(channel.icon)" alt="" loading="lazy" class="h-5 w-5 shrink-0 rounded object-contain" />
                    <div class="truncate text-[13px] font-semibold">{{ channel.name }}</div>
                  </div>
                  <div v-if="channel.fee_policy === 'customer_surcharge'" class="mt-1 space-y-0.5 text-[11px] text-warning">
                    <div>{{ t('payment.feeLabel') }}：{{ formatChannelFeeRate(channel) }}</div>
                    <div>{{ t('payment.fixedFeeLabel') }}：{{ formatChannelFixedFee(channel) }}</div>
                  </div>
                  <div v-if="isChannelDisabledForAmount(channel)" class="mt-1 text-[11px] text-warning">
                    {{ channelAmountLimitHint(channel) }}
                  </div>
                </button>
              </div>
              <div v-else-if="requiresOnlineChannel && paymentChannels.length === 0" class="text-xs text-muted-foreground">
                {{ t('checkout.noPaymentChannels') }}
              </div>
            </template>

            <div v-if="!requiresOnlineChannel" class="mt-2 text-xs font-medium text-success">
              {{ t('checkout.walletCoversAll') }}
            </div>
          </div>

          <Button
            class="mt-6 h-11.5 w-full rounded-[11px] bg-primary font-semibold text-primary-foreground shadow-xs hover:bg-primary/90"
            :disabled="!canSubmit"
            @click="handleSubmit"
          >
            <span>{{ submitting ? t('checkout.submitting') : t('checkout.submitButton') }}</span>
            <ArrowRight v-if="!submitting" class="h-4 w-4" />
          </Button>
          <p class="mt-3 text-xs leading-relaxed text-muted-foreground">{{ t('checkout.submitHint') }}</p>
        </aside>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowRight, Check } from 'lucide-vue-next'
import { pageAlertVariant, pageAlertToneClass } from '../../utils/alerts'
import ImageCaptcha from '../../components/captcha/ImageCaptcha.vue'
import TurnstileCaptcha from '../../components/captcha/TurnstileCaptcha.vue'
import CheckoutManualForm from '../../components/checkout/CheckoutManualForm.vue'
import EmptyState from '../../components/EmptyState.vue'
import SmartImage from '../../components/SmartImage.vue'
import AtelierCheckoutSteps from './components/AtelierCheckoutSteps.vue'
import { splitPrice } from './utils/price'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Alert, AlertDescription } from '@/components/ui/alert'
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

const previewTotalParts = computed(() => splitPrice(formatPrice(previewTotal.value, '')))

void guestImageCaptchaRef
void guestTurnstileRef
</script>
