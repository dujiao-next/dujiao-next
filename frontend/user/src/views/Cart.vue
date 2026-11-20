<template>
  <div class="min-h-screen bg-background pb-16 pt-24 text-foreground">
    <div class="container mx-auto px-4">
      <div class="mb-6 flex flex-wrap items-end justify-between gap-x-8 gap-y-4">
        <div>
          <h1 class="flex items-center gap-3 text-[28px] font-bold tracking-tight">
            {{ t('cart.title') }}
            <span v-if="totalItems" class="rounded-full bg-secondary px-2.5 py-0.5 text-sm font-semibold tabular-nums text-muted-foreground">{{ totalItems }}</span>
          </h1>
          <p class="mt-1 text-sm text-muted-foreground">{{ t('cart.subtitle') }}</p>
        </div>
        <CheckoutSteps plain current-step="cart" class="w-full sm:w-[26rem]" />
      </div>

      <!-- Empty State -->
      <EmptyState
        v-if="cartItems.length === 0"
        icon="cart"
        :title="t('cart.empty')"
        :action-label="t('cart.emptyAction')"
        action-to="/products"
      />

      <div v-else class="grid grid-cols-1 gap-5 lg:grid-cols-12 lg:gap-6">
        <div class="flex flex-col gap-3 lg:col-span-8">
          <article
            v-for="item in cartItems"
            :key="cartItemKey(item)"
            class="theme-float flex gap-3 rounded-[22px] bg-card p-3 md:gap-5 md:p-4"
          >
            <router-link :to="`/products/${item.slug}`" class="h-20 w-20 shrink-0 overflow-hidden rounded-2xl bg-secondary md:h-24 md:w-24" tabindex="-1" aria-hidden="true">
              <SmartImage :src="cartItemImage(item)" alt="" img-class="h-full w-full object-cover" />
            </router-link>

            <div class="flex min-w-0 flex-1 flex-col">
              <div class="flex items-start gap-2">
                <div class="min-w-0 flex-1">
                  <router-link :to="`/products/${item.slug}`" class="line-clamp-2 text-[15px] font-semibold leading-snug hover:underline md:text-base">
                    {{ getLocalizedText(item.title) }}
                  </router-link>
                  <p v-if="itemSkuDisplay(item)" class="mt-1 truncate text-[13px] text-muted-foreground">{{ itemSkuDisplay(item) }}</p>
                  <div class="mt-2 flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
                    <span class="inline-flex items-center gap-1 rounded-full bg-secondary px-2 py-0.5 font-medium text-foreground">
                      <Zap v-if="item.fulfillmentType === 'auto'" class="h-3 w-3 text-warm" aria-hidden="true" />
                      {{ item.fulfillmentType === 'auto' ? t('products.fulfillmentType.auto') : t('products.fulfillmentType.manual') }}
                    </span>
                    <span class="rounded-full bg-secondary px-2 py-0.5 font-medium text-foreground">
                      {{ item.purchaseType === 'guest' ? t('productPurchase.guest') : t('productPurchase.member') }}
                    </span>
                    <span v-if="itemStockHint(item)">{{ itemStockHint(item) }}</span>
                  </div>
                </div>
                <button
                  type="button"
                  class="-mr-1 -mt-1 flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-destructive/10 hover:text-destructive focus-visible:outline-2 focus-visible:outline-primary"
                  :aria-label="t('cart.remove')"
                  :title="t('cart.remove')"
                  @click="removeWithUndo(item)"
                >
                  <Trash2 class="h-4 w-4" />
                </button>
              </div>

              <div class="mt-auto flex flex-wrap items-end justify-between gap-3 pt-3">
                <div class="flex items-center rounded-full bg-secondary p-1">
                  <button
                    type="button"
                    class="flex h-8 w-8 items-center justify-center rounded-full transition-colors hover:bg-card disabled:opacity-30"
                    :disabled="item.quantity <= itemPurchaseMin(item)"
                    @click="updateQty(item, item.quantity - 1)"
                  >
                    <Minus class="h-4 w-4" :stroke-width="2.5" />
                  </button>
                  <input
                    type="number"
                    :id="`cart-qty-${item.productId}`"
                    :name="`cart-qty-${item.productId}`"
                    :aria-label="t('checkout.quantityLabel')"
                    :value="item.quantity"
                    @change="handleQtyChange(item, $event)"
                    :min="itemPurchaseMin(item)"
                    :max="itemMaxQuantity(item)"
                    class="cart-qty-input h-8 w-11 border-none bg-transparent p-0 text-center text-sm font-semibold tabular-nums text-foreground focus:outline-none focus:ring-0"
                  />
                  <button
                    type="button"
                    class="flex h-8 w-8 items-center justify-center rounded-full transition-colors hover:bg-card disabled:opacity-30"
                    :disabled="item.quantity >= itemMaxQuantity(item)"
                    @click="updateQty(item, item.quantity + 1)"
                  >
                    <Plus class="h-4 w-4" :stroke-width="2.5" />
                  </button>
                </div>

                <div class="text-right">
                  <p class="text-xs tabular-nums text-muted-foreground">{{ formatPrice(item.priceAmount, totalCurrency) }} × {{ item.quantity }}</p>
                  <p class="text-[19px] font-bold leading-tight tracking-tight tabular-nums">{{ itemSubtotal(item) }}</p>
                </div>
              </div>
              <p v-if="quantityWarning(item)" class="mt-3 rounded-xl bg-warning/10 px-3 py-2 text-xs font-medium text-warning">
                {{ quantityWarning(item) }}
              </p>
            </div>
          </article>
        </div>

        <aside class="theme-float h-fit rounded-[22px] bg-card p-6 md:rounded-[28px] lg:sticky lg:top-24 lg:col-span-4">
          <h2 class="text-[17px] font-bold">{{ t('cart.summaryTitle') }}</h2>
          <div class="mt-4 flex items-center justify-between text-sm text-muted-foreground">
            <span>{{ t('cart.itemsCount') }}</span>
            <span class="font-semibold tabular-nums text-foreground">{{ totalItems }}</span>
          </div>
          <div class="mt-4 flex items-baseline justify-between border-t pt-4">
            <span class="text-sm text-muted-foreground">{{ t('cart.totalLabel') }}</span>
            <span class="text-[32px] font-bold leading-none tracking-tight tabular-nums">
              {{ formatPrice(totalAmount, '') }}<span class="ml-1.5 text-sm font-medium text-muted-foreground">{{ totalCurrency }}</span>
            </span>
          </div>

          <div class="mt-6 space-y-2">
            <Button as-child class="h-12 w-full rounded-full bg-foreground font-semibold text-background hover:bg-foreground/85">
              <router-link to="/checkout">{{ t('cart.checkout') }}</router-link>
            </Button>
            <Button as-child variant="secondary" class="h-12 w-full rounded-full font-semibold">
              <router-link to="/products">{{ t('cart.emptyAction') }}</router-link>
            </Button>
          </div>
          <p class="mt-4 text-xs leading-relaxed text-muted-foreground">{{ t('cart.disclaimer') }}</p>
        </aside>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Minus, Plus, Trash2, Zap } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import EmptyState from '../components/EmptyState.vue'
import SmartImage from '../components/SmartImage.vue'
import CheckoutSteps from '../components/checkout/CheckoutSteps.vue'
import { useCart } from '../composables/useCart'

const { t } = useI18n()

const {
  getLocalizedText, formatPrice, totalCurrency,
  cartItems, totalItems, totalAmount,
  cartItemKey, cartItemImage, itemSkuDisplay, itemSubtotal, itemStockHint, quantityWarning,
  itemPurchaseMin, itemMaxQuantity,
  removeWithUndo, updateQty, handleQtyChange,
} = useCart()
</script>
<style scoped>
.cart-qty-input {
  appearance: textfield;
}
.cart-qty-input::-webkit-outer-spin-button,
.cart-qty-input::-webkit-inner-spin-button {
  appearance: none;
  margin: 0;
}
</style>
