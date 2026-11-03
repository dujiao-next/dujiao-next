<template>
  <Transition
    enter-active-class="transition duration-300 ease-out"
    enter-from-class="translate-y-full opacity-0"
    enter-to-class="translate-y-0 opacity-100"
    leave-active-class="transition duration-200 ease-in"
    leave-from-class="translate-y-0 opacity-100"
    leave-to-class="translate-y-full opacity-0"
  >
    <div v-if="visible" class="bar">
      <div class="bar-inner">
        <div class="bar-price">
          <span v-if="showMemberPrice" class="is-orange">{{ memberPriceDisplay }}</span>
          <span v-else-if="showSkuPromotionPrice">{{ skuPromotionPriceDisplay }}</span>
          <span v-else-if="showSkuPrice">{{ skuPriceDisplay }}</span>
          <span v-else-if="showProductPromotionPrice">{{ productPromotionPriceDisplay }}</span>
          <span v-else>{{ productPriceDisplay }}</span>
        </div>

        <button v-if="requiresLogin" type="button" class="st-btn st-btn--primary bar-btn" @click="$emit('goLogin')">{{ t('productDetail.loginToBuy') }}</button>
        <template v-else>
          <button type="button" class="st-btn st-btn--ghost bar-btn bar-btn--ghost" :disabled="!canPurchase" @click="$emit('addToCart')">
            <ShoppingCart class="h-4 w-4" /> {{ t('productDetail.addToCart') }}
          </button>
          <button type="button" class="st-btn st-btn--primary bar-btn" :disabled="!canPurchase" @click="$emit('buyNow')">
            <Zap class="h-4 w-4" /> {{ t('productDetail.buyNow') }}
          </button>
        </template>
      </div>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ShoppingCart, Zap } from 'lucide-vue-next'

const { t } = useI18n()

defineProps<{
  visible: boolean
  requiresLogin: boolean
  canPurchase: boolean
  showMemberPrice: boolean
  memberPriceDisplay: string
  showSkuPromotionPrice: boolean
  skuPromotionPriceDisplay: string
  showSkuPrice: boolean
  skuPriceDisplay: string
  showProductPromotionPrice: boolean
  productPromotionPriceDisplay: string
  productPriceDisplay: string
}>()

defineEmits<{
  addToCart: []
  buyNow: []
  goLogin: []
}>()
</script>

<style scoped>
.bar {
  position: fixed;
  inset-inline: 0;
  bottom: 0;
  z-index: 40;
  padding-bottom: env(safe-area-inset-bottom);
  background: color-mix(in srgb, var(--st-bg) 88%, transparent);
  backdrop-filter: saturate(180%) blur(20px);
  -webkit-backdrop-filter: saturate(180%) blur(20px);
  border-top: 1px solid color-mix(in srgb, var(--st-text) 10%, transparent);
  font-family: var(--st-font);
}
@media (min-width: 1024px) {
  .bar {
    display: none;
  }
}
.bar-inner {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 16px;
}
.bar-price {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 19px;
  font-weight: 600;
  color: var(--st-text);
  font-variant-numeric: tabular-nums;
}
.bar-price .is-orange {
  color: var(--st-orange);
}
.bar-btn {
  flex: none;
  height: 40px;
  padding: 0 16px;
  font-size: 14px;
}
.bar-btn--ghost {
  display: none;
}
@media (min-width: 390px) {
  .bar-btn--ghost {
    display: inline-flex;
  }
}
</style>
