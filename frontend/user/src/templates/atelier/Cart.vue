<template>
  <div class="min-h-screen pb-16 pt-[88px] text-foreground">
    <div class="mx-auto w-full max-w-[1240px] px-4 sm:px-6">
      <!-- 页面头部与精密步骤条 -->
      <div class="mb-7 flex flex-wrap items-end justify-between gap-x-8 gap-y-4 border-b pb-5">
        <div>
          <h1 class="flex items-center gap-3 text-[26px] font-bold tracking-[-0.025em] md:text-[30px]">
            <span>{{ t('cart.title') }}</span>
            <span
              v-if="totalItems"
              class="inline-flex h-6 items-center rounded-[7px] border bg-card px-2 font-mono text-xs font-semibold tabular-nums text-muted-foreground"
            >
              {{ totalItems }}
            </span>
          </h1>
          <p class="mt-1 text-sm text-muted-foreground">{{ t('cart.subtitle') }}</p>
        </div>
        <AtelierCheckoutSteps current-step="cart" class="w-full sm:w-[22rem]" />
      </div>

      <!-- 空购物车 -->
      <EmptyState
        v-if="cartItems.length === 0"
        icon="cart"
        :title="t('cart.empty')"
        :action-label="t('cart.emptyAction')"
        action-to="/products"
      />

      <!-- 购物清单 + 吸顶账单卡 -->
      <div v-else class="grid grid-cols-1 items-start gap-6 lg:grid-cols-12">
        <div class="atelier-card overflow-hidden rounded-[16px] bg-card lg:col-span-8">
          <div class="divide-y">
            <article
              v-for="item in cartItems"
              :key="cartItemKey(item)"
              class="flex gap-3.5 p-4 transition-colors hover:bg-background/60 sm:gap-5 sm:p-5"
            >
              <RouterLink
                :to="`/products/${item.slug}`"
                class="h-20 w-20 shrink-0 overflow-hidden rounded-[14px] border bg-background sm:h-24 sm:w-24"
                tabindex="-1"
                aria-hidden="true"
              >
                <SmartImage :src="cartItemImage(item)" alt="" img-class="h-full w-full object-cover" />
              </RouterLink>

              <div class="flex min-w-0 flex-1 flex-col">
                <div class="flex items-start justify-between gap-3">
                  <div class="min-w-0 flex-1">
                    <RouterLink
                      :to="`/products/${item.slug}`"
                      class="line-clamp-2 text-[15px] font-semibold leading-snug text-foreground transition-opacity hover:opacity-75 md:text-base"
                    >
                      {{ getLocalizedText(item.title) }}
                    </RouterLink>
                    <p v-if="itemSkuDisplay(item)" class="mt-1 truncate text-xs font-medium text-muted-foreground">
                      {{ itemSkuDisplay(item) }}
                    </p>
                    <div class="mt-2 flex flex-wrap items-center gap-1.5 text-[11.5px]">
                      <span
                        class="inline-flex h-5 items-center gap-1 rounded-[5px] px-1.5 font-semibold"
                        :class="item.fulfillmentType === 'auto' ? 'bg-success-soft text-success' : 'bg-secondary text-foreground/75'"
                      >
                        <Zap v-if="item.fulfillmentType === 'auto'" class="h-3 w-3 fill-current" aria-hidden="true" />
                        {{ item.fulfillmentType === 'auto' ? t('products.fulfillmentType.auto') : t('products.fulfillmentType.manual') }}
                      </span>
                      <span class="inline-flex h-5 items-center rounded-[5px] bg-secondary px-1.5 font-medium text-muted-foreground">
                        {{ item.purchaseType === 'guest' ? t('productPurchase.guest') : t('productPurchase.member') }}
                      </span>
                      <span v-if="itemStockHint(item)" class="text-muted-foreground">{{ itemStockHint(item) }}</span>
                    </div>
                  </div>

                  <button
                    type="button"
                    class="-mr-1 -mt-1 grid h-9 w-9 shrink-0 place-items-center rounded-full text-muted-foreground transition-colors hover:bg-destructive/10 hover:text-destructive"
                    :aria-label="t('cart.remove')"
                    :title="t('cart.remove')"
                    @click="removeWithUndo(item)"
                  >
                    <Trash2 class="h-4 w-4" />
                  </button>
                </div>

                <div class="mt-auto flex flex-wrap items-end justify-between gap-3 pt-4">
                  <div class="inline-flex items-center gap-1 rounded-[10px] border bg-secondary p-1">
                    <button
                      type="button"
                      class="grid h-7.5 w-7.5 place-items-center rounded-[7px] bg-card text-foreground shadow-2xs transition-colors hover:bg-background disabled:opacity-30"
                      :disabled="item.quantity <= itemPurchaseMin(item)"
                      @click="updateQty(item, item.quantity - 1)"
                    >
                      <Minus class="h-3.5 w-3.5" :stroke-width="2.5" />
                    </button>
                    <input
                      type="number"
                      :id="`at-cart-qty-${item.productId}`"
                      :name="`at-cart-qty-${item.productId}`"
                      :aria-label="t('checkout.quantityLabel')"
                      :value="item.quantity"
                      :min="itemPurchaseMin(item)"
                      :max="itemMaxQuantity(item)"
                      class="cart-qty-input h-7.5 w-10 border-none bg-transparent p-0 text-center font-mono text-[13.5px] font-semibold tabular-nums text-foreground outline-none"
                      @change="handleQtyChange(item, $event)"
                    />
                    <button
                      type="button"
                      class="grid h-7.5 w-7.5 place-items-center rounded-[7px] bg-card text-foreground shadow-2xs transition-colors hover:bg-background disabled:opacity-30"
                      :disabled="item.quantity >= itemMaxQuantity(item)"
                      @click="updateQty(item, item.quantity + 1)"
                    >
                      <Plus class="h-3.5 w-3.5" :stroke-width="2.5" />
                    </button>
                  </div>

                  <div class="text-right tabular-nums">
                    <p class="text-[11.5px] text-muted-foreground">
                      {{ formatPrice(item.priceAmount, totalCurrency) }} × {{ item.quantity }}
                    </p>
                    <p class="mt-0.5 text-[18px] font-bold leading-none tracking-[-0.02em] text-foreground">
                      {{ itemSubtotal(item) }}
                    </p>
                  </div>
                </div>

                <p
                  v-if="quantityWarning(item)"
                  class="mt-3 rounded-[10px] bg-warning-soft px-3 py-2 text-xs font-medium text-warning"
                >
                  {{ quantityWarning(item) }}
                </p>
              </div>
            </article>
          </div>
        </div>

        <!-- 右侧吸顶订单摘要 -->
        <aside class="atelier-card h-fit rounded-[16px] bg-card p-6 lg:sticky lg:top-[84px] lg:col-span-4">
          <h2 class="border-b pb-3.5 text-base font-bold tracking-[-0.015em]">{{ t('cart.summaryTitle') }}</h2>

          <div class="mt-4 flex items-center justify-between text-sm text-muted-foreground">
            <span>{{ t('cart.itemsCount') }}</span>
            <span class="font-mono font-semibold text-foreground">{{ totalItems }}</span>
          </div>

          <div class="mt-4 rounded-[14px] border bg-background px-4 py-3.5">
            <div class="text-xs text-muted-foreground">{{ t('cart.totalLabel') }}</div>
            <div class="mt-1 flex items-baseline tabular-nums">
              <b class="text-[30px] font-bold leading-none tracking-[-0.03em] text-foreground">{{ totalParts.int }}</b>
              <span v-if="totalParts.dec" class="text-[17px] font-semibold text-foreground/75">.{{ totalParts.dec }}</span>
              <span v-if="totalCurrency" class="ml-1.5 text-xs font-semibold text-muted-foreground">{{ totalCurrency }}</span>
            </div>
          </div>

          <div class="mt-5 space-y-2.5">
            <Button as-child class="h-11.5 w-full rounded-[11px] bg-primary font-semibold text-primary-foreground shadow-xs hover:bg-primary/90">
              <RouterLink to="/checkout">
                <span>{{ t('cart.checkout') }}</span>
                <ArrowRight class="h-4 w-4" />
              </RouterLink>
            </Button>
            <Button as-child variant="secondary" class="h-11 w-full rounded-[11px] font-semibold">
              <RouterLink to="/products">{{ t('cart.emptyAction') }}</RouterLink>
            </Button>
          </div>

          <p class="mt-4 text-xs leading-relaxed text-muted-foreground">{{ t('cart.disclaimer') }}</p>
        </aside>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowRight, Minus, Plus, Trash2, Zap } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import EmptyState from '../../components/EmptyState.vue'
import SmartImage from '../../components/SmartImage.vue'
import AtelierCheckoutSteps from './components/AtelierCheckoutSteps.vue'
import { splitPrice } from './utils/price'
import { useCart } from '../../composables/useCart'

const { t } = useI18n()

const {
  getLocalizedText, formatPrice, totalCurrency,
  cartItems, totalItems, totalAmount,
  cartItemKey, cartItemImage, itemSkuDisplay, itemSubtotal, itemStockHint, quantityWarning,
  itemPurchaseMin, itemMaxQuantity,
  removeWithUndo, updateQty, handleQtyChange,
} = useCart()

const totalParts = computed(() => splitPrice(formatPrice(totalAmount.value, '')))
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
