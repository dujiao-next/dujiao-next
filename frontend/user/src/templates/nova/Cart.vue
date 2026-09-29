<template>
  <div class="mx-auto w-full max-w-[1200px] px-4 pb-12 sm:px-6">
    <!-- 面包屑 -->
    <nav class="flex flex-wrap items-center gap-1.5 py-6 text-[13px] text-[color:var(--ink-3)]">
      <RouterLink class="transition-colors hover:text-foreground" to="/">{{ t('nav.home') }}</RouterLink>
      <ChevronRight class="h-3.5 w-3.5 flex-none opacity-50" />
      <span class="text-foreground">{{ t('cart.title') }}</span>
    </nav>

    <header class="mb-7 flex flex-col gap-3">
      <span class="nova-eyebrow">{{ t('nova.steps.cart') }}</span>
      <h1 class="text-[clamp(1.75rem,1.35rem+1.8vw,2.5rem)] font-semibold tracking-[-0.03em]">{{ t('cart.title') }}</h1>
      <p v-if="totalItems > 0" class="text-[15px] text-[color:var(--ink-3)]">{{ t('nova.list.resultCount', { count: totalItems }) }}</p>
    </header>

    <NovaCheckoutSteps v-if="cartItems.length > 0" current="cart" />

    <!-- 空态 -->
    <div v-if="cartItems.length === 0" class="flex flex-col items-center gap-3 rounded-[var(--r-xl)] border border-dashed border-border py-24 text-center">
      <span class="grid h-14 w-14 place-items-center rounded-full bg-[color:var(--surface-2)]">
        <ShoppingCart class="h-6 w-6 text-[color:var(--ink-3)]" />
      </span>
      <p class="text-[15px] font-medium">{{ t('ember.cart.empty') }}</p>
      <p class="max-w-[42ch] text-[13.5px] text-[color:var(--ink-3)]">{{ t('ember.cart.emptyHint') }}</p>
      <RouterLink
        class="mt-2 inline-flex h-10 items-center rounded-full bg-[color:var(--accent)] px-5 text-[13.5px] font-medium text-[color:var(--on-accent)] transition-colors hover:bg-[color:var(--accent-strong)]"
        to="/products"
      >{{ t('ember.cart.browse') }}</RouterLink>
    </div>

    <div v-else class="grid items-start gap-10 lg:grid-cols-[minmax(0,1fr)_356px]">
      <!-- 商品清单：行间 1px 分隔线，整块一张卡 -->
      <div class="nova-card px-5 py-2 sm:px-6">
        <article
          v-for="(item, idx) in cartItems"
          :key="cartItemKey(item)"
          class="grid grid-cols-[72px_minmax(0,1fr)_auto] items-start gap-4 py-5"
          :class="idx > 0 ? 'border-t border-[color:var(--border)]' : ''"
        >
          <RouterLink
            :to="`/products/${item.slug}`"
            class="h-[72px] w-[72px] overflow-hidden rounded-[var(--r-sm)] bg-[color:var(--surface-2)]"
          >
            <img v-if="cartItemImage(item)" :src="cartItemImage(item)" :alt="getLocalizedText(item.title)" loading="lazy" class="h-full w-full object-cover" />
            <span v-else class="grid h-full w-full place-items-center text-[color:var(--ink-4)]"><Package class="h-5 w-5" /></span>
          </RouterLink>

          <div class="flex min-w-0 flex-col gap-1.5">
            <RouterLink class="truncate text-[14.5px] font-medium tracking-[-0.012em] transition-colors hover:text-[color:var(--accent)]" :to="`/products/${item.slug}`">
              {{ getLocalizedText(item.title) }}
            </RouterLink>
            <span v-if="itemSkuDisplay(item)" class="text-[12px] text-[color:var(--ink-3)]">{{ itemSkuDisplay(item) }}</span>
            <span v-if="itemStockHint(item)" class="nova-pill nova-pill--warning w-fit">{{ itemStockHint(item) }}</span>
            <span v-if="quantityWarning(item)" class="nova-pill nova-pill--danger w-fit">{{ quantityWarning(item) }}</span>

            <!-- 数量步进器 -->
            <div class="mt-2.5 inline-flex w-fit items-center overflow-hidden rounded-full border border-border">
              <button
                type="button"
                class="grid h-8 w-8 place-items-center text-[color:var(--ink-3)] transition-colors hover:bg-[color:var(--surface-2)] disabled:opacity-30"
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
                class="grid h-8 w-8 place-items-center text-[color:var(--ink-3)] transition-colors hover:bg-[color:var(--surface-2)] disabled:opacity-30"
                :disabled="item.quantity >= itemMaxQuantity(item)"
                :aria-label="'+'"
                @click="updateQty(item, item.quantity + 1)"
              ><Plus class="h-3.5 w-3.5" /></button>
            </div>
          </div>

          <div class="flex flex-col items-end gap-2.5">
            <span class="text-[15px] font-semibold tabular-nums tracking-[-0.02em]">{{ itemSubtotal(item) }}</span>
            <button
              type="button"
              class="text-[12px] text-[color:var(--ink-3)] transition-colors hover:text-[color:var(--danger)]"
              @click="removeWithUndo(item)"
            >{{ t('cart.remove') }}</button>
          </div>
        </article>
      </div>

      <!-- 汇总 -->
      <aside class="sticky top-[96px] nova-card p-6">
        <h2 class="nova-eyebrow mb-5">{{ t('cart.title') }}</h2>

        <div class="flex items-baseline justify-between text-[13.5px]">
          <span class="text-[color:var(--ink-3)]">{{ t('ember.cart.subtotal') }}</span>
          <span class="tabular-nums text-foreground">{{ formatPrice(totalAmount, totalCurrency) }}</span>
        </div>

        <div class="mt-4 flex items-baseline justify-between border-t border-border pt-4">
          <span class="text-[13.5px] text-[color:var(--ink-3)]">{{ t('checkout.previewTotal') }}</span>
          <span class="text-[24px] font-semibold tabular-nums tracking-[-0.03em] text-foreground">
            {{ formatPrice(totalAmount, totalCurrency) }}
          </span>
        </div>

        <RouterLink
          class="mt-5 flex h-12 w-full items-center justify-center rounded-full bg-[color:var(--accent)] text-[15px] font-medium text-[color:var(--on-accent)] transition-colors duration-200 hover:bg-[color:var(--accent-strong)]"
          to="/checkout"
        >{{ t('ember.cart.checkout') }}</RouterLink>
        <p class="mt-3 text-center text-[11.5px] text-[color:var(--ink-4)]">{{ t('ember.cart.shippingNote') }}</p>

        <RouterLink class="mt-4 block text-center text-[13px] text-[color:var(--ink-3)] transition-colors hover:text-foreground" to="/products">
          {{ t('nav.products') }}
        </RouterLink>
      </aside>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ChevronRight, Minus, Package, Plus, ShoppingCart } from 'lucide-vue-next'
import { useCart } from '../../composables/useCart'
import NovaCheckoutSteps from './components/NovaCheckoutSteps.vue'

const { t } = useI18n()

const {
  getLocalizedText, formatPrice, totalCurrency,
  cartItems, totalItems, totalAmount,
  cartItemKey, cartItemImage, itemSkuDisplay, itemSubtotal, itemStockHint, quantityWarning,
  itemPurchaseMin, itemMaxQuantity,
  removeWithUndo, updateQty, handleQtyChange,
} = useCart()
</script>
