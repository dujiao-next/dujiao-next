<template>
  <div class="mx-auto w-full max-w-[1180px] px-6 pb-8">
    <nav class="flex flex-wrap items-center gap-1.5 py-5 pb-2 text-[13.5px] font-semibold text-muted-foreground">
      <RouterLink to="/" class="hover:text-primary">{{ t('nav.home') }}</RouterLink>
      <ChevronRight class="h-4 w-4 flex-none" />
      <span class="text-foreground">{{ t('cart.title') }}</span>
    </nav>

    <header class="mb-5 mt-2">
      <h1 class="mb-1.5 text-[32px] font-extrabold">{{ t('cart.title') }}</h1>
      <p class="text-muted-foreground">{{ t('cart.subtitle') }}</p>
    </header>

    <VaultCheckoutSteps current="cart" />

    <!-- 空购物车 -->
    <div v-if="cartItems.length === 0" class="my-8 flex flex-col items-center gap-3 rounded-xl border border-dashed py-16 text-center text-muted-foreground">
      <ShoppingCart class="h-10 w-10 opacity-60" />
      <p>{{ t('cart.empty') }}</p>
      <Button as-child class="mt-2 rounded-full" size="sm"><RouterLink to="/products">{{ t('cart.emptyAction') }}</RouterLink></Button>
    </div>

    <div v-else class="grid items-start gap-7 lg:grid-cols-[minmax(0,1fr)_340px]">
      <!-- 商品列表 -->
      <div class="grid min-w-0 gap-4">
        <article v-for="item in cartItems" :key="cartItemKey(item)" class="flex min-w-0 gap-4 rounded-xl border bg-card p-4">
          <RouterLink :to="`/products/${item.slug}`" class="relative grid h-[88px] w-[88px] flex-none place-items-center overflow-hidden rounded-md bg-secondary">
            <img v-if="cartItemImage(item)" :src="cartItemImage(item)" :alt="getLocalizedText(item.title)" loading="lazy" class="absolute inset-0 h-full w-full object-cover" />
            <Package v-else class="h-10 w-10 text-muted-foreground" />
          </RouterLink>

          <div class="min-w-0 flex-1">
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <RouterLink :to="`/products/${item.slug}`" class="block truncate font-bold hover:text-primary">{{ getLocalizedText(item.title) }}</RouterLink>
                <p class="mt-1 text-[13px] text-muted-foreground">{{ t('cart.priceLabel') }}：{{ formatPrice(item.priceAmount, totalCurrency) }}</p>
                <p v-if="itemSkuDisplay(item)" class="mt-1 text-[13px] text-muted-foreground">{{ t('cart.skuLabel') }}：{{ itemSkuDisplay(item) }}</p>
                <p v-if="itemStockHint(item)" class="mt-1 text-[13px] text-muted-foreground">{{ itemStockHint(item) }}</p>
                <div class="mt-2 flex flex-wrap gap-1.5">
                  <Badge :variant="item.purchaseType === 'guest' ? 'warning' : 'info'" size="sm" class="rounded-full">{{ item.purchaseType === 'guest' ? t('productPurchase.guest') : t('productPurchase.member') }}</Badge>
                  <Badge variant="success" size="sm" class="rounded-full">{{ item.fulfillmentType === 'auto' ? t('products.fulfillmentType.auto') : t('products.fulfillmentType.manual') }}</Badge>
                </div>
              </div>
              <button type="button" class="inline-flex h-10 flex-none items-center gap-1.5 rounded-full border-2 border-hairline-strong px-3 text-muted-foreground transition-colors hover:border-destructive hover:bg-destructive/10 hover:text-destructive" @click="removeWithUndo(item)">
                <Trash2 class="h-4 w-4" />
                <span class="text-[13px] font-semibold">{{ t('cart.remove') }}</span>
              </button>
            </div>

            <div class="mt-3.5 flex flex-wrap items-center justify-between gap-3 border-t pt-3.5">
              <div class="inline-flex items-center overflow-hidden rounded-full border-2 border-hairline-strong">
                <button type="button" class="grid h-10 w-[38px] place-items-center bg-card text-foreground disabled:opacity-35" :aria-label="t('cart.remove')" :disabled="item.quantity <= itemPurchaseMin(item)" @click="updateQty(item, item.quantity - 1)"><Minus class="h-4 w-4" /></button>
                <input
                  type="number"
                  class="h-10 w-[46px] border-x bg-card text-center font-bold tabular-nums outline-none [appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
                  :value="item.quantity"
                  :min="itemPurchaseMin(item)"
                  :max="itemMaxQuantity(item)"
                  @change="handleQtyChange(item, $event)"
                />
                <button type="button" class="grid h-10 w-[38px] place-items-center bg-card text-foreground disabled:opacity-35" :aria-label="t('cart.remove')" :disabled="item.quantity >= itemMaxQuantity(item)" @click="updateQty(item, item.quantity + 1)"><Plus class="h-4 w-4" /></button>
              </div>
              <div class="text-right">
                <span class="block text-[11px] uppercase tracking-[0.04em] text-muted-foreground">{{ t('checkout.totalPriceLabel') }}</span>
                <strong class="text-base">{{ itemSubtotal(item) }}</strong>
              </div>
            </div>

            <div v-if="quantityWarning(item)" class="mt-3 rounded-sm bg-warning/10 px-3 py-2 text-[13px] font-semibold text-warning">{{ quantityWarning(item) }}</div>
          </div>
        </article>
      </div>

      <!-- 汇总 -->
      <aside class="sticky top-[90px] rounded-xl border bg-card p-[22px]">
        <h2 class="mb-4 text-lg font-bold">{{ t('cart.summaryTitle') }}</h2>
        <div class="grid gap-3">
          <div class="flex items-center justify-between text-sm text-muted-foreground">
            <span>{{ t('cart.itemsCount') }}</span>
            <span class="font-bold text-foreground">{{ totalItems }}</span>
          </div>
          <div class="flex items-center justify-between border-t pt-3 font-bold text-foreground">
            <span>{{ t('cart.totalLabel') }}</span>
            <span class="text-[22px] text-primary tabular-nums">{{ formatPrice(totalAmount, totalCurrency) }}</span>
          </div>
        </div>
        <p class="my-4 rounded-sm border bg-secondary px-3 py-2.5 text-xs leading-relaxed text-muted-foreground">{{ t('cart.disclaimer') }}</p>
        <Button as-child class="h-11 w-full rounded-full font-bold"><RouterLink to="/checkout">{{ t('cart.checkout') }} <ArrowRight /></RouterLink></Button>
        <Button as-child variant="outline" class="mt-2.5 h-11 w-full rounded-full font-bold"><RouterLink to="/products">{{ t('cart.emptyAction') }}</RouterLink></Button>
      </aside>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ArrowRight, ChevronRight, Minus, Package, Plus, ShoppingCart, Trash2 } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import VaultCheckoutSteps from './components/VaultCheckoutSteps.vue'
import { useCart } from '../../composables/useCart'

const { t } = useI18n()

const {
  getLocalizedText, formatPrice, totalCurrency,
  cartItems, totalItems, totalAmount,
  cartItemKey, cartItemImage, itemSkuDisplay, itemSubtotal, itemStockHint, quantityWarning,
  itemPurchaseMin, itemMaxQuantity,
  removeWithUndo, updateQty, handleQtyChange,
} = useCart()
</script>
