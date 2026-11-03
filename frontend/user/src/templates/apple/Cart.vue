<template>
  <div class="st-root st-page">
    <div class="st-container">
      <nav class="cart-crumbs">
        <RouterLink to="/">{{ t('nav.home') }}</RouterLink>
        <ChevronRight class="cart-crumb-icon" />
        <span>{{ t('cart.title') }}</span>
      </nav>

      <header class="st-page-head">
        <h1 class="st-page-title">{{ t('cart.title') }}</h1>
        <p class="st-page-lead">{{ t('cart.subtitle') }}</p>
      </header>
    </div>

    <div class="st-container cart-steps">
      <VaultCheckoutSteps current="cart" />
    </div>

    <!-- 空购物车 -->
    <div v-if="cartItems.length === 0" class="st-container">
      <div class="cart-empty">
        <ShoppingCart class="cart-empty-icon" />
        <p>{{ t('cart.empty') }}</p>
        <RouterLink class="st-btn st-btn--primary" to="/products">{{ t('cart.emptyAction') }}</RouterLink>
      </div>
    </div>

    <div v-else class="st-container cart-grid">
      <!-- 商品 -->
      <div class="cart-list">
        <article v-for="item in cartItems" :key="cartItemKey(item)" class="cart-item">
          <RouterLink :to="`/products/${item.slug}`" class="cart-thumb">
            <img v-if="cartItemImage(item)" :src="cartItemImage(item)" :alt="getLocalizedText(item.title)" loading="lazy" />
            <Package v-else class="cart-thumb-empty" />
          </RouterLink>

          <div class="cart-info">
            <div class="cart-row">
              <div class="cart-main">
                <RouterLink :to="`/products/${item.slug}`" class="cart-name">{{ getLocalizedText(item.title) }}</RouterLink>
                <p class="cart-meta">{{ itemSkuDisplay(item) || t('cart.priceLabel') }}</p>
                <p v-if="itemStockHint(item)" class="cart-meta">{{ itemStockHint(item) }}</p>
                <p class="cart-tags">
                  <span>{{ item.purchaseType === 'guest' ? t('productPurchase.guest') : t('productPurchase.member') }}</span>
                  <span>{{ item.fulfillmentType === 'auto' ? t('products.fulfillmentType.auto') : t('products.fulfillmentType.manual') }}</span>
                </p>
              </div>

              <div class="cart-side">
                <span class="cart-subtotal">{{ itemSubtotal(item) }}</span>
                <button type="button" class="cart-remove" @click="removeWithUndo(item)">{{ t('cart.remove') }}</button>
              </div>
            </div>

            <div class="cart-controls">
              <div class="st-qty">
                <button type="button" :aria-label="t('cart.remove')" :disabled="item.quantity <= itemPurchaseMin(item)" @click="updateQty(item, item.quantity - 1)"><Minus class="h-4 w-4" /></button>
                <input
                  type="number"
                  :value="item.quantity"
                  :min="itemPurchaseMin(item)"
                  :max="itemMaxQuantity(item)"
                  @change="handleQtyChange(item, $event)"
                />
                <button type="button" :aria-label="t('cart.remove')" :disabled="item.quantity >= itemMaxQuantity(item)" @click="updateQty(item, item.quantity + 1)"><Plus class="h-4 w-4" /></button>
              </div>
              <span class="cart-unit">{{ t('cart.priceLabel') }} {{ formatPrice(item.priceAmount, totalCurrency) }}</span>
            </div>

            <div v-if="quantityWarning(item)" class="cart-warn">{{ quantityWarning(item) }}</div>
          </div>
        </article>
      </div>

      <!-- 汇总 -->
      <aside class="cart-summary">
        <h2 class="cart-summary-title">{{ t('cart.summaryTitle') }}</h2>

        <div class="cart-summary-row">
          <span>{{ t('cart.itemsCount') }}</span>
          <span>{{ totalItems }}</span>
        </div>
        <div class="cart-summary-row cart-summary-row--total">
          <span>{{ t('cart.totalLabel') }}</span>
          <span>{{ formatPrice(totalAmount, totalCurrency) }}</span>
        </div>

        <p class="cart-disclaimer">{{ t('cart.disclaimer') }}</p>

        <RouterLink class="st-btn st-btn--primary st-btn--block" to="/checkout">
          {{ t('cart.checkout') }}
        </RouterLink>
        <RouterLink class="st-btn st-btn--ghost st-btn--block cart-continue" to="/products">
          {{ t('cart.emptyAction') }}
        </RouterLink>
      </aside>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ChevronRight, Minus, Package, Plus, ShoppingCart } from 'lucide-vue-next'
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

<style scoped>
.cart-crumbs {
  display: flex;
  align-items: center;
  gap: 6px;
  padding-top: 20px;
  font-size: 12px;
  color: var(--st-text-2);
}
.cart-crumbs a {
  color: var(--st-text-2);
  text-decoration: none;
}
.cart-crumb-icon {
  width: 12px;
  height: 12px;
  opacity: 0.5;
}

.cart-steps {
  padding-top: 20px;
  padding-bottom: 28px;
}

/* ---------- 空态 ---------- */
.cart-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
  padding: 90px 0;
  text-align: center;
  color: var(--st-text-2);
}
.cart-empty-icon {
  width: 44px;
  height: 44px;
  opacity: 0.35;
}

/* ---------- 布局 ---------- */
.cart-grid {
  display: grid;
  gap: 32px;
}
@media (min-width: 900px) {
  .cart-grid {
    grid-template-columns: minmax(0, 1fr) 340px;
    gap: 48px;
    align-items: start;
  }
}
.cart-list {
  display: flex;
  flex-direction: column;
}
.cart-item {
  display: flex;
  gap: 20px;
  padding-block: 24px;
  border-top: 1px solid color-mix(in srgb, var(--st-text) 10%, transparent);
}
.cart-item:last-child {
  border-bottom: 1px solid color-mix(in srgb, var(--st-text) 10%, transparent);
}
.cart-thumb {
  display: grid;
  place-items: center;
  width: 104px;
  height: 104px;
  flex: none;
  border-radius: 14px;
  overflow: hidden;
  background: var(--st-bg-alt);
}
.cart-thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.cart-thumb-empty {
  width: 32px;
  height: 32px;
  opacity: 0.3;
}
.cart-info {
  flex: 1;
  min-width: 0;
}
.cart-row {
  display: flex;
  gap: 16px;
  justify-content: space-between;
}
.cart-main {
  min-width: 0;
}
.cart-name {
  display: block;
  font-family: var(--st-font-display);
  font-size: 19px;
  font-weight: 600;
  line-height: 1.3;
  color: var(--st-text);
  text-decoration: none;
}
.cart-name:hover {
  color: var(--st-link);
}
.cart-meta {
  margin: 6px 0 0;
  font-size: 14px;
  color: var(--st-text-2);
}
.cart-tags {
  display: flex;
  gap: 8px;
  margin: 8px 0 0;
  font-size: 12px;
  color: var(--st-text-3);
}
.cart-tags span + span::before {
  content: '·';
  margin-right: 8px;
}
.cart-side {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 8px;
  flex: none;
}
.cart-subtotal {
  font-size: 17px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}
.cart-remove {
  border: 0;
  background: none;
  padding: 0;
  font-size: 13px;
  color: var(--st-link);
  cursor: pointer;
}
.cart-remove:hover {
  text-decoration: underline;
}
.cart-controls {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-top: 16px;
}
.cart-unit {
  font-size: 13px;
  color: var(--st-text-3);
}
.cart-warn {
  margin-top: 12px;
  padding: 10px 14px;
  border-radius: 10px;
  background: color-mix(in srgb, #b25000 14%, transparent);
  color: #a35b00;
  font-size: 13px;
}

/* ---------- 汇总 ---------- */
.cart-summary {
  position: sticky;
  top: 96px;
  padding: 24px;
  border-radius: 18px;
  background: var(--st-bg-alt);
}
.cart-summary-title {
  margin: 0 0 18px;
  font-family: var(--st-font-display);
  font-size: 21px;
  font-weight: 600;
}
.cart-summary-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding-block: 8px;
  font-size: 15px;
  color: var(--st-text-2);
}
.cart-summary-row--total {
  margin-top: 6px;
  padding-top: 16px;
  border-top: 1px solid color-mix(in srgb, var(--st-text) 12%, transparent);
  font-size: 17px;
  font-weight: 600;
  color: var(--st-text);
}
.cart-summary-row--total span:last-child {
  font-size: 24px;
  font-variant-numeric: tabular-nums;
}
.cart-disclaimer {
  margin: 16px 0 20px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--st-text-3);
}
.cart-continue {
  margin-top: 10px;
}
</style>
