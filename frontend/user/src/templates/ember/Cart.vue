<template>
  <div class="mx-auto w-full max-w-[1180px] px-4 pb-10 sm:px-6">
    <nav class="flex flex-wrap items-center gap-1.5 py-5 text-[13px] text-muted-foreground">
      <RouterLink class="transition-colors hover:text-foreground" to="/">{{ t('nav.home') }}</RouterLink>
      <ChevronRight class="h-3.5 w-3.5 flex-none opacity-60" />
      <span class="text-foreground">{{ t('cart.title') }}</span>
    </nav>

    <header class="mb-8 flex flex-col gap-2">
      <h1 class="text-[clamp(1.75rem,1.3rem+1.9vw,2.6rem)] font-semibold tracking-[-0.022em]">{{ t('cart.title') }}</h1>
      <p v-if="totalItems > 0" class="text-[15px] text-muted-foreground">{{ t('ember.cart.count', { count: totalItems }) }}</p>
    </header>

    <EmberCheckoutSteps v-if="cartItems.length > 0" current="cart" />

    <!-- 空 -->
    <div v-if="cartItems.length === 0" class="flex flex-col items-center gap-3 rounded-[var(--r-lg)] border border-dashed border-border py-20 text-center">
      <ShoppingCart class="h-11 w-11 text-muted-foreground" />
      <p class="font-medium">{{ t('ember.cart.empty') }}</p>
      <p class="max-w-[42ch] text-[13.5px] text-muted-foreground">{{ t('ember.cart.emptyHint') }}</p>
      <Button as-child size="sm" class="mt-2 rounded-full"><RouterLink to="/products">{{ t('ember.cart.browse') }}</RouterLink></Button>
    </div>

    <!-- 列表 -->
    <div v-else class="grid items-start gap-8 lg:grid-cols-[minmax(0,1fr)_360px]">
      <div class="flex flex-col gap-3">
        <article
          v-for="item in cartItems"
          :key="cartItemKey(item)"
          class="grid grid-cols-[64px_minmax(0,1fr)_auto] items-start gap-4 rounded-[var(--r-lg)] border bg-card p-4"
          :class="itemStockHint(item) ? 'border-[color:var(--warning)]' : 'border-border'"
        >
          <RouterLink :to="`/products/${item.slug}`" class="h-16 w-16 overflow-hidden rounded-[var(--r-sm)] bg-[color:var(--surface-2)]">
            <img v-if="cartItemImage(item)" :src="cartItemImage(item)" :alt="getLocalizedText(item.title)" loading="lazy" class="h-full w-full object-cover" />
            <span v-else class="grid h-full w-full place-items-center text-muted-foreground"><Package class="h-5 w-5" /></span>
          </RouterLink>

          <div class="flex min-w-0 flex-col gap-1">
            <RouterLink class="truncate text-[14.5px] font-medium transition-colors hover:text-[color:var(--accent)]" :to="`/products/${item.slug}`">
              {{ getLocalizedText(item.title) }}
            </RouterLink>
            <span v-if="itemSkuDisplay(item)" class="text-[12px] text-muted-foreground">{{ itemSkuDisplay(item) }}</span>
            <span v-if="itemStockHint(item)" class="text-[12px] text-[color:var(--warning-strong)]">{{ itemStockHint(item) }}</span>
            <span v-if="quantityWarning(item)" class="text-[12px] text-[color:var(--danger-strong)]">{{ quantityWarning(item) }}</span>

            <div class="mt-2 inline-flex w-fit items-center overflow-hidden rounded-[var(--r-sm)] border border-border">
              <button
                type="button"
                class="grid h-8 w-8 place-items-center text-muted-foreground transition-colors hover:bg-[color:var(--surface-2)] disabled:opacity-35"
                :disabled="item.quantity <= itemPurchaseMin(item)"
                :aria-label="'-'"
                @click="updateQty(item, item.quantity - 1)"
              ><Minus class="h-3.5 w-3.5" /></button>
              <input
                :value="item.quantity"
                type="text"
                inputmode="numeric"
                class="h-8 w-12 border-x border-border bg-transparent text-center text-[13px] tabular-nums outline-none"
                :aria-label="t('checkout.quantityLabel')"
                @change="handleQtyChange(item, $event)"
              />
              <button
                type="button"
                class="grid h-8 w-8 place-items-center text-muted-foreground transition-colors hover:bg-[color:var(--surface-2)] disabled:opacity-35"
                :disabled="item.quantity >= itemMaxQuantity(item)"
                :aria-label="'+'"
                @click="updateQty(item, item.quantity + 1)"
              ><Plus class="h-3.5 w-3.5" /></button>
            </div>
          </div>

          <div class="flex flex-col items-end gap-2">
            <span class="text-[14.5px] font-semibold tabular-nums">{{ itemSubtotal(item) }}</span>
            <button
              type="button"
              class="text-[12px] text-muted-foreground transition-colors hover:text-[color:var(--danger)]"
              @click="removeWithUndo(item)"
            >{{ t('cart.remove') }}</button>
          </div>
        </article>
      </div>

      <!-- 汇总 -->
      <aside class="sticky top-[92px] flex flex-col gap-3 rounded-[var(--r-lg)] border border-border bg-card p-5">
        <h2 class="border-b border-[color:var(--border-soft)] pb-3 text-[14px] font-semibold">{{ t('cart.title') }}</h2>

        <div class="flex items-baseline justify-between text-[13.5px] text-muted-foreground">
          <span>{{ t('ember.cart.subtotal') }}</span>
          <span class="tabular-nums text-foreground">{{ formatPrice(totalAmount, totalCurrency) }}</span>
        </div>

        <div class="flex items-baseline justify-between border-t border-border pt-3">
          <span class="text-[13.5px] text-muted-foreground">{{ t('checkout.previewTotal') }}</span>
          <span class="text-[22px] font-semibold tabular-nums tracking-[-0.03em] text-[color:var(--accent-strong)]">
            {{ formatPrice(totalAmount, totalCurrency) }}
          </span>
        </div>

        <Button as-child size="lg" class="mt-1 h-12 w-full rounded-full text-[15px] font-medium">
          <RouterLink to="/checkout">{{ t('ember.cart.checkout') }}</RouterLink>
        </Button>
        <p class="text-center text-[11.5px] text-muted-foreground">{{ t('ember.cart.shippingNote') }}</p>

        <RouterLink class="text-center text-[13px] text-muted-foreground transition-colors hover:text-foreground" to="/products">
          {{ t('nav.products') }}
        </RouterLink>
      </aside>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ChevronRight, Minus, Package, Plus, ShoppingCart } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { useCart } from '../../composables/useCart'
import EmberCheckoutSteps from './components/EmberCheckoutSteps.vue'

const { t } = useI18n()

const {
  getLocalizedText, formatPrice, totalCurrency,
  cartItems, totalItems, totalAmount,
  cartItemKey, cartItemImage, itemSkuDisplay, itemSubtotal, itemStockHint, quantityWarning,
  itemPurchaseMin, itemMaxQuantity,
  removeWithUndo, updateQty, handleQtyChange,
} = useCart()
</script>
