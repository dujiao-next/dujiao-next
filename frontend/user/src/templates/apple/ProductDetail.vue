<template>
  <div class="st-root pd">
    <!-- 吸顶二级导航（Apple localnav：产品名 + 价格 + 购买） -->
    <div v-if="product" class="pd-localnav">
      <div class="st-container pd-localnav-inner">
        <span class="pd-localnav-title">{{ getLocalizedText(product.title) }}</span>
        <span class="pd-localnav-price">{{ formatPrice(product.price_amount, siteCurrency) }}</span>
        <button type="button" class="pd-localnav-cta" @click="scrollToBuy">
          {{ t('productDetail.buyNow') }}
        </button>
      </div>
    </div>

    <!-- 面包屑 -->
    <nav class="st-container pd-crumbs">
      <RouterLink to="/">{{ t('nav.home') }}</RouterLink>
      <ChevronRight class="pd-crumb-icon" />
      <RouterLink to="/products">{{ t('nav.products') }}</RouterLink>
      <ChevronRight class="pd-crumb-icon" />
      <span class="pd-crumb-current">{{ product ? getLocalizedText(product.title) : '' }}</span>
    </nav>

    <!-- Loading -->
    <div v-if="loading" class="st-container pd-grid">
      <div class="pd-stage pd-skel"></div>
      <div class="pd-buy">
        <div class="pd-skel pd-skel--line" style="width: 45%"></div>
        <div class="pd-skel pd-skel--line" style="width: 70%; height: 44px"></div>
        <div class="pd-skel pd-skel--line" style="height: 140px"></div>
      </div>
    </div>

    <template v-else-if="product">
      <div class="st-container pd-grid">
        <!-- 图区 -->
        <div class="pd-gallery">
          <div class="pd-stage">
            <img v-if="currentImage" :src="currentImage" :alt="getLocalizedText(product.title)" />
            <Package v-else class="pd-stage-empty" />
          </div>
          <div v-if="images.length > 1" class="pd-thumbs">
            <button
              v-for="(img, idx) in images"
              :key="idx"
              type="button"
              class="pd-thumb"
              :class="{ 'is-active': img === currentImage }"
              @click="currentImage = img"
            >
              <img :src="img" :alt="`${idx + 1}`" loading="lazy" />
            </button>
          </div>
        </div>

        <!-- 购买区 -->
        <div ref="buyPanel" class="pd-buy">
          <p v-if="categoryName" class="pd-eyebrow">{{ categoryName }}</p>
          <h1 class="pd-title">{{ getLocalizedText(product.title) }}</h1>

          <p class="pd-status">
            <span>{{ getStockStatusLabel(product) }}</span>
            <span class="pd-sep">·</span>
            <span>{{ getFulfillmentTypeLabel(product.fulfillment_type) }}</span>
            <span class="pd-sep">·</span>
            <span>{{ getPurchaseTypeLabel(product.purchase_type) }}</span>
          </p>

          <div v-if="product.tags && product.tags.length" class="pd-tags">
            <span v-for="(tag, i) in product.tags" :key="i">{{ tag }}</span>
          </div>

          <!-- 价格 -->
          <div class="pd-price">
            <div class="pd-price-tags">
              <span v-if="(selectedSku && hasSkuPromotionPrice(selectedSku)) || (!selectedSku && hasPromotionPrice(product))" class="pd-tag pd-tag--blue">{{ t('products.promotionTag') }}</span>
              <span v-if="showSelectedSkuMemberBadge" class="pd-tag pd-tag--orange">{{ t('products.memberPriceTag') }}</span>
              <span v-if="hasSelectedSkuWholesalePrice" class="pd-tag">{{ t('products.wholesaleTag') }}</span>
            </div>

            <template v-if="selectedSku && hasSelectedSkuWholesalePrice">
              <div class="pd-amount-row">
                <span class="pd-amount">{{ formatPrice(selectedSkuWholesaleFinalPrice!, siteCurrency) }}</span>
                <span class="pd-was">{{ formatPrice(selectedSku.price_amount, siteCurrency) }}</span>
              </div>
              <p class="pd-note">{{ selectedSkuWholesaleFinalIsMember ? t('products.memberPriceTag') : t('products.wholesaleTag') }} · {{ t('products.saveAmount') }} {{ formatPrice(Number(selectedSku.price_amount) - Number(selectedSkuWholesaleFinalPrice), siteCurrency) }}</p>
            </template>
            <template v-else-if="selectedSku && hasSkuPromotionPrice(selectedSku)">
              <div class="pd-amount-row">
                <span class="pd-amount" :class="{ 'pd-amount--orange': selectedSkuPromotionFinalIsMember }">{{ formatPrice(selectedSkuPromotionFinalIsMember ? selectedSkuPromotionFinalPrice! : selectedSkuPromotionPrice!, siteCurrency) }}</span>
                <span class="pd-was">{{ formatPrice(selectedSku.price_amount, siteCurrency) }}</span>
              </div>
              <p class="pd-note">{{ selectedSkuPromotionFinalIsMember ? t('products.memberPriceTag') : t('products.saveAmount') }} · {{ formatPrice(selectedSkuPromotionFinalIsMember ? Number(selectedSku.price_amount) - Number(selectedSkuPromotionFinalPrice) : getSkuPromotionSaveAmount(selectedSku), siteCurrency) }}</p>
            </template>
            <template v-else-if="selectedSku && hasMemberPrice">
              <div class="pd-amount-row">
                <span class="pd-amount pd-amount--orange">{{ formatPrice(selectedSkuMemberPrice!, siteCurrency) }}</span>
                <span class="pd-was">{{ formatPrice(selectedSku.price_amount, siteCurrency) }}</span>
              </div>
              <p class="pd-note">{{ t('products.memberPriceTag') }} · {{ t('products.saveAmount') }} {{ formatPrice(Number(selectedSku.price_amount) - selectedSkuMemberPrice!, siteCurrency) }}</p>
            </template>
            <div v-else-if="selectedSku" class="pd-amount-row">
              <span class="pd-amount">{{ formatPrice(selectedSku.price_amount, siteCurrency) }}</span>
            </div>
            <template v-else-if="hasPromotionPrice(product)">
              <div class="pd-amount-row">
                <span class="pd-amount">{{ formatPrice(getPromotionPriceAmount(product), siteCurrency) }}</span>
                <span class="pd-was">{{ formatPrice(product.price_amount, siteCurrency) }}</span>
              </div>
              <p class="pd-note">{{ t('products.saveAmount') }} {{ formatPrice(getPromotionSaveAmount(product), siteCurrency) }}</p>
            </template>
            <div v-else class="pd-amount-row">
              <span class="pd-amount">{{ formatPrice(product.price_amount, siteCurrency) }}</span>
            </div>
          </div>

          <!-- 批发规则 -->
          <div v-if="selectedSkuWholesaleRules.length" class="pd-block">
            <h4 class="pd-block-title">{{ t('products.wholesaleRulesTitle') }}</h4>
            <div class="pd-block-list">
              <span v-for="tier in selectedSkuWholesaleRules" :key="`${tier.sku_id || tier.sku_code || 'all'}-${tier.min_quantity}`">{{ formatWholesaleTier(tier) }}</span>
            </div>
          </div>

          <!-- 活动规则 -->
          <div v-if="hasPromotionRules(product)" class="pd-block">
            <h4 class="pd-block-title"><Tag class="h-4 w-4" /> {{ t('products.promotionRulesTitle') }}</h4>
            <ul class="pd-block-rules">
              <li v-for="rule in getPromotionRules(product)" :key="rule.id">{{ formatPromotionRule(rule) }}</li>
            </ul>
          </div>

          <!-- 规格 -->
          <div v-if="activeSkus.length" class="pd-field">
            <div class="pd-label">{{ t('productDetail.skuTitle') }}</div>
            <div class="pd-skus">
              <button
                v-for="sku in activeSkus"
                :key="sku.id"
                type="button"
                class="pd-sku"
                :class="{ 'is-active': normalizeSkuId(sku.id) === selectedSkuId, 'is-disabled': !isSkuPurchasable(sku) }"
                :disabled="!isSkuPurchasable(sku)"
                @click="selectedSkuId = normalizeSkuId(sku.id)"
              >
                <span class="pd-sku-name">{{ skuDisplayText(sku) }}</span>
                <span class="pd-sku-stock">{{ skuStockText(sku) }}</span>
              </button>
            </div>
            <p v-if="requiresSKUSelection" class="pd-hint">{{ t('productDetail.skuRequired') }}</p>
          </div>

          <!-- 数量 -->
          <div class="pd-field">
            <div class="pd-label">{{ t('productDetail.quantity') }}</div>
            <div class="pd-qty">
              <button type="button" :disabled="quantity <= quantityEffectiveMin" :aria-label="t('productDetail.quantity')" @click="quantity = Math.max(quantityEffectiveMin, quantity - 1)"><Minus class="h-4 w-4" /></button>
              <input inputmode="numeric" :value="quantity" :aria-label="t('productDetail.quantity')" @change="handleQuantityInput($event)" @keydown.enter.prevent="($event.target as HTMLInputElement)?.blur()" />
              <button type="button" :disabled="quantityEffectiveLimit !== null && quantity >= quantityEffectiveLimit" :aria-label="t('productDetail.quantity')" @click="quantity = quantity + 1"><Plus class="h-4 w-4" /></button>
            </div>
          </div>

          <div v-if="cannotPurchaseReason" class="pd-alert">{{ cannotPurchaseReason }}</div>
          <div v-if="purchaseWarning" class="pd-alert pd-alert--warn">{{ purchaseWarning }}</div>

          <!-- 操作 -->
          <div ref="purchaseActionsRef" class="pd-actions">
            <button v-if="requiresLogin" type="button" class="pd-btn pd-btn--primary pd-btn--full" @click="goLogin">{{ t('productDetail.loginToBuy') }}</button>
            <template v-else>
              <button type="button" class="pd-btn pd-btn--primary" :disabled="!canPurchase" @click="buyNow"><Zap class="h-4 w-4" /> {{ t('productDetail.buyNow') }}</button>
              <button type="button" class="pd-btn pd-btn--ghost" :disabled="!canPurchase" @click="addToCart"><ShoppingCart class="h-4 w-4" /> {{ t('productDetail.addToCart') }}</button>
            </template>
          </div>

          <div class="pd-reassure">
            <component :is="product.fulfillment_type === 'manual' ? Pencil : TicketCheck" class="h-5 w-5" />
            <span>{{ product.fulfillment_type === 'manual' ? t('productDetail.deliveryReassuranceManual') : t('productDetail.deliveryReassurance') }}</span>
          </div>
        </div>
      </div>

      <!-- 商品详情 -->
      <section v-if="getLocalizedText(product.description) || product.content" class="st-container pd-section">
        <h2 class="pd-section-title">{{ t('productDetail.details') }}</h2>
        <div class="prose pd-prose">
          <p v-if="getLocalizedText(product.description)">{{ getLocalizedText(product.description) }}</p>
          <div v-if="product.content" v-html="processHtmlForDisplay(getLocalizedText(product.content))"></div>
        </div>
      </section>

      <!-- 相关文章 -->
      <section v-if="relatedPosts.length" class="st-container pd-section">
        <h2 class="pd-section-title">{{ t('productDetail.relatedPosts') }}</h2>
        <div class="pd-posts">
          <RouterLink v-for="rp in relatedPosts" :key="rp.id" class="pd-post" :to="`/blog/${rp.slug}`">
            <span class="pd-post-date">{{ formatRelatedPostDate(rp.published_at) }}</span>
            <span class="pd-post-title">{{ getLocalizedText(rp.title) }}</span>
            <span v-if="rp.summary" class="pd-post-summary">{{ getLocalizedText(rp.summary) }}</span>
            <span class="pd-post-more">{{ t('blog.readMore') }} <ChevronRight class="h-4 w-4" /></span>
          </RouterLink>
        </div>
      </section>

      <div class="st-container pd-back">
        <RouterLink :to="backToListTo"><ArrowLeft class="h-4 w-4" /> {{ t('productDetail.backToProducts') }}</RouterLink>
      </div>

      <!-- 移动端固定购买条 -->
      <VaultProductMobileBar
        :visible="showMobileBar && !!product && !loading"
        :requires-login="requiresLogin"
        :can-purchase="canPurchase"
        :show-member-price="mobileBarShowMemberPrice"
        :member-price-display="mobileBarMemberPriceDisplay"
        :show-sku-promotion-price="mobileBarShowSkuPromotionPrice"
        :sku-promotion-price-display="mobileBarSkuPromotionPriceDisplay"
        :show-sku-price="mobileBarShowSkuPrice"
        :sku-price-display="mobileBarSkuPriceDisplay"
        :show-product-promotion-price="mobileBarShowProductPromotionPrice"
        :product-promotion-price-display="mobileBarProductPromotionPriceDisplay"
        :product-price-display="mobileBarProductPriceDisplay"
        @add-to-cart="addToCart"
        @buy-now="buyNow"
        @go-login="goLogin"
      />
    </template>

    <!-- 错误 -->
    <div v-else class="st-container pd-error">
      <AlertCircle class="h-10 w-10" />
      <p>{{ t('productDetail.notFound') }}</p>
      <div class="pd-error-actions">
        <button type="button" class="pd-btn pd-btn--primary" @click="loadProduct">{{ t('errorBoundary.retry') }}</button>
        <RouterLink class="pd-btn pd-btn--ghost" :to="backToListTo">{{ t('productDetail.backToProducts') }}</RouterLink>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  AlertCircle, ArrowLeft, ChevronRight, Minus, Package, Pencil, Plus,
  ShoppingCart, Tag, TicketCheck, Zap,
} from 'lucide-vue-next'
import { processHtmlForDisplay } from '../../utils/content'
import { useProductDetail } from '../../composables/useProductDetail'
import VaultProductMobileBar from './components/VaultProductMobileBar.vue'

const { t } = useI18n()

// 移动端固定购买条：IntersectionObserver 监听桌面购买区是否出屏
const purchaseActionsRef = ref<HTMLElement | null>(null)
const showMobileBar = ref(false)
let observer: IntersectionObserver | null = null

const setupMobileBarObserver = () => {
  if (observer) observer.disconnect()
  if (!purchaseActionsRef.value) return
  observer = new IntersectionObserver(
    (entries) => {
      const entry = entries[0]
      if (entry) showMobileBar.value = !entry.isIntersecting
    },
    { threshold: 0.1 },
  )
  observer.observe(purchaseActionsRef.value)
}

const buyPanel = ref<HTMLElement | null>(null)
const scrollToBuy = () => {
  buyPanel.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

const {
  getLocalizedText, siteCurrency, formatPrice,
  getFulfillmentTypeLabel, getPurchaseTypeLabel, getStockStatusLabel,
  hasPromotionPrice, getPromotionPriceAmount, getPromotionSaveAmount,
  hasSkuPromotionPrice, getSkuPromotionSaveAmount,
  hasPromotionRules, getPromotionRules,
  formatPromotionRule, formatWholesaleTier, formatRelatedPostDate, normalizeSkuId,
  loading, product, relatedPosts, currentImage, selectedSkuId, quantity, purchaseWarning,
  activeSkus, selectedSku,
  selectedSkuMemberPrice, hasMemberPrice,
  hasSelectedSkuWholesalePrice, selectedSkuWholesaleFinalIsMember, selectedSkuWholesaleFinalPrice,
  selectedSkuWholesaleRules,
  selectedSkuPromotionPrice, selectedSkuPromotionFinalIsMember, selectedSkuPromotionFinalPrice,
  showSelectedSkuMemberBadge,
  isSkuPurchasable, skuDisplayText, skuStockText,
  quantityEffectiveLimit, quantityEffectiveMin, handleQuantityInput,
  requiresLogin, requiresSKUSelection, canPurchase, cannotPurchaseReason,
  categoryName, images,
  backToListTo,
  addToCart, buyNow, goLogin, loadProduct,
  mobileBarShowMemberPrice, mobileBarMemberPriceDisplay,
  mobileBarShowSkuPromotionPrice, mobileBarSkuPromotionPriceDisplay,
  mobileBarShowSkuPrice, mobileBarSkuPriceDisplay,
  mobileBarShowProductPromotionPrice, mobileBarProductPromotionPriceDisplay, mobileBarProductPriceDisplay,
} = useProductDetail({ onLoaded: () => setupMobileBarObserver() })

onUnmounted(() => {
  if (observer) {
    observer.disconnect()
    observer = null
  }
})
</script>

<style scoped>
.pd-localnav {
  position: sticky;
  top: 48px;
  z-index: 40;
  height: 52px;
  background: var(--st-material);
  backdrop-filter: saturate(180%) blur(20px);
  -webkit-backdrop-filter: saturate(180%) blur(20px);
  border-bottom: 1px solid color-mix(in srgb, var(--st-text) 10%, transparent);
}
@media (min-width: 1024px) {
  .pd-localnav {
    top: 44px;
  }
}
.pd-localnav-inner {
  display: flex;
  align-items: center;
  gap: 16px;
  height: 52px;
}
.pd-localnav-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: var(--st-font-display);
  font-size: 19px;
  font-weight: 600;
  color: var(--st-text);
}
.pd-localnav-price {
  flex: none;
  font-size: 14px;
  color: var(--st-text-2);
  font-variant-numeric: tabular-nums;
}
.pd-localnav-cta {
  flex: none;
  height: 30px;
  padding: 0 14px;
  border: 0;
  border-radius: 980px;
  background: var(--st-blue);
  color: #fff;
  font-size: 13px;
  cursor: pointer;
  transition: background 320ms var(--st-ease);
}
.pd-localnav-cta:hover {
  background: var(--st-blue-hover);
}
@media (prefers-reduced-motion: reduce) {
  .pd-localnav-cta {
    transition: none;
  }
}

.pd {
  background: var(--st-bg);
  color: var(--st-text);
  padding-bottom: 72px;
}

/* ---------- 面包屑 ---------- */
.pd-crumbs {
  display: flex;
  align-items: center;
  gap: 6px;
  padding-top: 20px;
  padding-bottom: 20px;
  font-size: 12px;
  color: var(--st-text-2);
}
.pd-crumbs a {
  color: var(--st-text-2);
  text-decoration: none;
}
.pd-crumbs a:hover {
  color: var(--st-text);
}
.pd-crumb-icon {
  width: 12px;
  height: 12px;
  opacity: 0.5;
}
.pd-crumb-current {
  color: var(--st-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* ---------- 布局 ---------- */
.pd-grid {
  display: grid;
  gap: 32px;
}
@media (min-width: 900px) {
  .pd-grid {
    grid-template-columns: minmax(0, 1.05fr) minmax(0, 0.95fr);
    gap: 56px;
    align-items: start;
  }
}

/* ---------- 图 ---------- */
.pd-stage {
  display: grid;
  place-items: center;
  aspect-ratio: 1 / 1;
  border-radius: 22px;
  overflow: hidden;
  background: var(--st-bg-alt);
}
.pd-stage img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.pd-stage-empty {
  width: 64px;
  height: 64px;
  opacity: 0.25;
}
.pd-thumbs {
  display: flex;
  gap: 10px;
  margin-top: 12px;
  overflow-x: auto;
  scrollbar-width: none;
}
.pd-thumbs::-webkit-scrollbar {
  display: none;
}
.pd-thumb {
  flex: none;
  width: 64px;
  height: 64px;
  padding: 0;
  border: 1px solid transparent;
  border-radius: 12px;
  overflow: hidden;
  background: var(--st-bg-alt);
  cursor: pointer;
}
.pd-thumb.is-active {
  border-color: var(--st-text);
}
.pd-thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

/* ---------- 购买面板 ---------- */
.pd-buy {
  padding-top: 4px;
}
.pd-eyebrow {
  margin: 0 0 6px;
  font-size: 14px;
  font-weight: 600;
  color: var(--st-text-2);
}
.pd-title {
  margin: 0;
  font-family: var(--st-font-display);
  font-size: 28px;
  line-height: 1.1;
  font-weight: 600;
  letter-spacing: -0.01em;
}
@media (min-width: 735px) {
  .pd-title {
    font-size: 40px;
  }
}
.pd-status {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  margin: 14px 0 0;
  font-size: 14px;
  color: var(--st-text-2);
}
.pd-sep {
  opacity: 0.5;
}
.pd-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 10px;
}
.pd-tags span {
  font-size: 12px;
  color: var(--st-text-2);
}
.pd-tags span + span::before {
  content: ' · ';
}

/* ---------- 价格 ---------- */
.pd-price {
  margin-top: 26px;
  padding-top: 24px;
  border-top: 1px solid color-mix(in srgb, var(--st-text) 10%, transparent);
}
.pd-price-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 8px;
}
.pd-tag {
  font-size: 12px;
  font-weight: 600;
  color: var(--st-green);
}
.pd-tag--orange {
  color: var(--st-orange);
}
.pd-tag--blue {
  color: var(--st-link);
}
.pd-amount-row {
  display: flex;
  align-items: baseline;
  gap: 12px;
  flex-wrap: wrap;
}
.pd-amount {
  font-family: var(--st-font-display);
  font-size: 28px;
  font-weight: 600;
  letter-spacing: -0.01em;
}
@media (min-width: 735px) {
  .pd-amount {
    font-size: 40px;
  }
}
.pd-amount--orange {
  color: var(--st-orange);
}
.pd-was {
  font-size: 17px;
  color: var(--st-text-3);
  text-decoration: line-through;
}
.pd-note {
  margin: 8px 0 0;
  font-size: 14px;
  color: var(--st-text-2);
}

/* ---------- 规则块 ---------- */
.pd-block {
  margin-top: 18px;
  padding: 16px 18px;
  border-radius: 14px;
  background: var(--st-bg-alt);
}
.pd-block-title {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 0 0 8px;
  font-size: 13px;
  font-weight: 600;
  color: var(--st-text);
}
.pd-block-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.pd-block-list span,
.pd-block-rules li {
  font-size: 13px;
  color: var(--st-text-2);
}
.pd-block-rules {
  margin: 0;
  padding-left: 18px;
  display: grid;
  gap: 4px;
}

/* ---------- 规格 / 数量 ---------- */
.pd-field {
  margin-top: 24px;
}
.pd-label {
  margin-bottom: 10px;
  font-size: 13px;
  font-weight: 600;
  color: var(--st-text-2);
}
.pd-skus {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.pd-sku {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  min-width: 92px;
  padding: 10px 14px;
  border: 1px solid color-mix(in srgb, var(--st-text) 14%, transparent);
  border-radius: 12px;
  background: none;
  cursor: pointer;
  font-size: 14px;
  color: var(--st-text);
  transition: border-color 320ms var(--st-ease), background 320ms var(--st-ease);
}
.pd-sku:hover {
  border-color: var(--st-text);
}
.pd-sku.is-active {
  border-color: var(--st-text);
  background: var(--st-bg-alt);
}
.pd-sku.is-disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
.pd-sku-stock {
  font-size: 12px;
  color: var(--st-text-3);
}
.pd-hint {
  margin: 8px 0 0;
  font-size: 13px;
  color: var(--st-orange);
}

.pd-qty {
  display: inline-flex;
  align-items: center;
  border: 1px solid color-mix(in srgb, var(--st-text) 14%, transparent);
  border-radius: 12px;
  overflow: hidden;
}
.pd-qty button {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border: 0;
  background: none;
  color: var(--st-text);
  cursor: pointer;
}
.pd-qty button:disabled {
  opacity: 0.3;
  cursor: default;
}
.pd-qty input {
  width: 52px;
  height: 44px;
  border: 0;
  border-inline: 1px solid color-mix(in srgb, var(--st-text) 10%, transparent);
  text-align: center;
  font-size: 15px;
  font-weight: 600;
  color: var(--st-text);
  outline: none;
  background: none;
  font-variant-numeric: tabular-nums;
}

/* ---------- 提示 / 按钮 ---------- */
.pd-alert {
  margin-top: 16px;
  padding: 12px 16px;
  border-radius: 12px;
  background: color-mix(in srgb, #d70015 12%, transparent);
  color: #c9342f;
  font-size: 14px;
}
.pd-alert--warn {
  background: color-mix(in srgb, #b25000 14%, transparent);
  color: #a35b00;
}
.pd-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 26px;
}
.pd-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  height: 46px;
  padding: 0 24px;
  border: 0;
  border-radius: 980px;
  font-size: 15px;
  font-weight: 400;
  text-decoration: none;
  cursor: pointer;
  transition: background 320ms var(--st-ease), color 320ms var(--st-ease);
}
.pd-btn--primary {
  flex: 1;
  background: var(--st-blue);
  color: #fff;
}
.pd-btn--primary:hover:not(:disabled) {
  background: var(--st-blue-hover);
}
.pd-btn--ghost {
  flex: 1;
  background: none;
  color: var(--st-link);
  border: 1px solid var(--st-link);
}
.pd-btn--ghost:hover {
  background: var(--st-link);
  color: #fff;
}
.pd-btn--full {
  flex: 1 1 100%;
}
.pd-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
.pd-reassure {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 18px;
  padding: 14px 16px;
  border-radius: 14px;
  background: var(--st-bg-alt);
  font-size: 14px;
  color: var(--st-text-2);
}
.pd-reassure svg {
  flex: none;
  color: var(--st-green);
}

/* ---------- 详情 / 相关文章 ---------- */
.pd-section {
  margin-top: 72px;
  padding-top: 48px;
  border-top: 1px solid color-mix(in srgb, var(--st-text) 8%, transparent);
}
.pd-section-title {
  margin: 0 0 20px;
  font-family: var(--st-font-display);
  font-size: 28px;
  font-weight: 600;
  letter-spacing: -0.01em;
}
@media (min-width: 735px) {
  .pd-section-title {
    font-size: 40px;
  }
}
.pd-prose {
  max-width: 720px;
  font-size: 17px;
  line-height: 1.47;
  color: var(--st-text-2);
}
.pd-prose :deep(img) {
  border-radius: 16px;
  margin-block: 16px;
}

.pd-posts {
  display: grid;
  gap: 12px;
  grid-template-columns: 1fr;
}
@media (min-width: 735px) {
  .pd-posts {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
.pd-post {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 22px;
  border-radius: 18px;
  background: var(--st-bg-alt);
  text-decoration: none;
  color: var(--st-text);
}
.pd-post-date {
  font-size: 12px;
  color: var(--st-text-3);
}
.pd-post-title {
  font-family: var(--st-font-display);
  font-size: 19px;
  font-weight: 600;
  line-height: 1.3;
}
.pd-post-summary {
  font-size: 14px;
  line-height: 1.5;
  color: var(--st-text-2);
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.pd-post-more {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  margin-top: auto;
  padding-top: 8px;
  font-size: 14px;
  color: var(--st-link);
}

.pd-back {
  display: flex;
  justify-content: center;
  padding-block: 40px;
}
.pd-back a {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  color: var(--st-link);
  text-decoration: none;
}

/* ---------- 错误 / 骨架 ---------- */
.pd-error {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 96px 0;
  color: var(--st-text-2);
}
.pd-error-actions {
  display: flex;
  gap: 10px;
  margin-top: 8px;
}
.pd-skel {
  border-radius: 22px;
  background: var(--st-bg-alt);
  animation: pd-pulse 1.4s ease-in-out infinite;
}
.pd-skel--line {
  height: 20px;
  border-radius: 8px;
  margin-bottom: 12px;
}
@keyframes pd-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.55; }
}
</style>
