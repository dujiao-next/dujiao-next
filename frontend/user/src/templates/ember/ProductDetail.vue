<template>
  <div class="mx-auto w-full max-w-[1180px] px-4 pb-10 sm:px-6">
    <!-- 面包屑 -->
    <nav class="flex flex-wrap items-center gap-1.5 py-5 text-[13px] text-muted-foreground">
      <RouterLink class="transition-colors hover:text-foreground" to="/">{{ t('nav.home') }}</RouterLink>
      <ChevronRight class="h-3.5 w-3.5 flex-none opacity-60" />
      <RouterLink class="transition-colors hover:text-foreground" to="/products">{{ t('nav.products') }}</RouterLink>
      <ChevronRight class="h-3.5 w-3.5 flex-none opacity-60" />
      <span class="truncate text-foreground">{{ getLocalizedText(product?.title) }}</span>
    </nav>

    <!-- 骨架 -->
    <div v-if="loading" class="grid gap-10 lg:grid-cols-2">
      <div class="aspect-[4/3] rounded-[var(--r-lg)] border border-border bg-card"></div>
      <div class="flex flex-col gap-4">
        <div class="h-8 w-3/4 rounded-[var(--r-xs)] bg-[color:var(--surface-2)]"></div>
        <div class="h-16 rounded-[var(--r)] bg-[color:var(--surface-2)]"></div>
        <div class="h-12 rounded-[var(--r)] bg-[color:var(--surface-2)]"></div>
        <div class="h-12 rounded-[var(--r)] bg-[color:var(--surface-2)]"></div>
      </div>
    </div>

    <!-- 未找到 -->
    <div v-else-if="!product" class="flex flex-col items-center gap-3 py-20 text-center">
      <PackageOpen class="h-11 w-11 text-muted-foreground" />
      <p class="font-medium">{{ t('productDetail.notFound') }}</p>
      <Button as-child variant="outline" size="sm" class="mt-2 rounded-full">
        <RouterLink to="/products">{{ t('productDetail.backToProducts') }}</RouterLink>
      </Button>
    </div>

    <!-- 正文 -->
    <div v-else class="grid items-start gap-8 lg:grid-cols-2 lg:gap-14">
      <!-- 图集 -->
      <div class="flex flex-col gap-3 lg:sticky lg:top-[92px]">
        <div class="aspect-[4/3] overflow-hidden rounded-[var(--r-lg)] border border-border bg-[color:var(--surface-2)]">
          <img
            v-if="currentImage"
            :src="currentImage"
            :alt="getLocalizedText(product.title)"
            class="h-full w-full object-cover"
          />
          <span v-else class="grid h-full w-full place-items-center text-muted-foreground">
            <Package class="h-10 w-10" />
          </span>
        </div>

        <div v-if="images.length > 1" class="flex gap-2 overflow-x-auto pb-1">
          <button
            v-for="(src, i) in images"
            :key="i"
            type="button"
            class="h-16 w-16 flex-none overflow-hidden rounded-[var(--r-sm)] border transition-colors"
            :class="src === currentImage ? 'border-[color:var(--accent)]' : 'border-border hover:border-[color:var(--border-strong)]'"
            :aria-label="`${i + 1}`"
            @click="currentImage = src"
          >
            <img :src="src" alt="" loading="lazy" class="h-full w-full object-cover" />
          </button>
        </div>
      </div>

      <!-- 信息 -->
      <div class="flex flex-col gap-6">
        <div class="flex flex-col gap-3">
          <span v-if="categoryName" class="ember-pill w-fit">{{ categoryName }}</span>
          <h1 class="text-[clamp(1.5rem,1.15rem+1.5vw,2.25rem)] font-semibold tracking-[-0.025em]">
            {{ getLocalizedText(product.title) }}
          </h1>
          <p v-if="getLocalizedText(product.description)" class="text-[15px] leading-[1.65] text-muted-foreground">
            {{ getLocalizedText(product.description) }}
          </p>
        </div>

        <!-- 价格 -->
        <div class="flex flex-wrap items-baseline gap-x-3 gap-y-1 rounded-[var(--r)] bg-[color:var(--surface-2)] px-5 py-4">
          <span class="text-[28px] font-semibold tabular-nums tracking-[-0.04em]" :class="discount ? 'text-[color:var(--accent-strong)]' : 'text-foreground'">
            {{ formatPrice(displayPrice, siteCurrency) }}
          </span>
          <span v-if="discount" class="text-[14px] text-muted-foreground line-through">
            {{ formatPrice(originalPrice, siteCurrency) }}
          </span>
          <span v-if="memberPrice" class="ember-pill ember-pill--accent">{{ t('products.memberPriceTag') }}</span>
          <span v-if="product.promotion_name" class="ember-pill ember-pill--accent">{{ product.promotion_name }}</span>
        </div>

        <!-- 批发阶梯价（当前 SKU） -->
        <div v-if="selectedSkuWholesaleRules.length" class="flex flex-col gap-2.5">
          <span class="text-[13px] font-medium">{{ t('products.wholesaleRulesTitle') }}</span>
          <ul class="flex flex-col gap-1.5">
            <li
              v-for="(tier, index) in selectedSkuWholesaleRules"
              :key="`tier-${index}`"
              class="flex items-start gap-2.5 text-[13px] leading-[1.6] text-muted-foreground"
            >
              <span class="mt-[7px] h-1 w-1 flex-none rounded-full bg-[color:var(--accent)]"></span>
              <span>{{ formatWholesaleTier(tier) }}</span>
            </li>
          </ul>
        </div>

        <!-- 活动规则 -->
        <div v-if="hasPromotionRules(product)" class="flex flex-col gap-2.5">
          <span class="text-[13px] font-medium">{{ t('products.promotionRulesTitle') }}</span>
          <ul class="flex flex-col gap-1.5">
            <li
              v-for="(rule, index) in getPromotionRules(product)"
              :key="`rule-${index}`"
              class="flex items-start gap-2.5 text-[13px] leading-[1.6] text-muted-foreground"
            >
              <span class="mt-[7px] h-1 w-1 flex-none rounded-full bg-[color:var(--accent)]"></span>
              <span>{{ formatPromotionRule(rule) }}</span>
            </li>
          </ul>
        </div>

        <!-- SKU -->
        <div v-if="activeSkus.length" class="flex flex-col gap-3">
          <span class="text-[13px] font-medium text-foreground">
            {{ t('productDetail.skuTitle') }}
            <em v-if="requiresSKUSelection" class="ml-1 not-italic text-[color:var(--danger)]">{{ t('productDetail.skuRequired') }}</em>
          </span>
          <div class="flex flex-wrap gap-2">
            <button
              v-for="sku in activeSkus"
              :key="sku.id"
              type="button"
              class="rounded-[var(--r)] border px-4 py-3 text-left transition-colors"
              :class="[
                normalizeSkuId(selectedSkuId) === normalizeSkuId(sku.id)
                  ? 'border-[color:var(--accent)] bg-[color:var(--accent-soft)] text-[color:var(--accent-strong)]'
                  : 'border-border bg-card text-muted-foreground hover:border-[color:var(--border-strong)] hover:text-foreground',
                sku.is_sold_out ? 'cursor-not-allowed opacity-45 line-through' : '',
              ]"
              :disabled="sku.is_sold_out"
              @click="selectedSkuId = sku.id"
            >
              <span class="block text-[14px] font-medium">{{ skuDisplayText(sku) }}</span>
              <span class="mt-0.5 block text-[12px] tabular-nums opacity-80">{{ formatPrice(skuDisplayPrice(sku), siteCurrency) }}</span>
            </button>
          </div>
        </div>

        <!-- 数量 -->
        <div class="flex flex-col gap-3">
          <span class="text-[13px] font-medium">{{ t('productDetail.quantity') }}</span>
          <div class="flex flex-wrap items-center gap-4">
            <div class="inline-flex items-center overflow-hidden rounded-[var(--r)] border border-border bg-card">
              <button
                type="button"
                class="grid h-11 w-11 place-items-center text-muted-foreground transition-colors hover:bg-[color:var(--surface-2)] disabled:opacity-35"
                :disabled="quantity <= quantityEffectiveMin"
                :aria-label="'-'"
                @click="quantity = Math.max(quantityEffectiveMin, quantity - 1)"
              ><Minus class="h-4 w-4" /></button>
              <input
                :value="quantity"
                type="text"
                inputmode="numeric"
                class="h-11 w-[68px] border-x border-border bg-transparent text-center text-[14px] tabular-nums outline-none"
                :aria-label="t('productDetail.quantity')"
                @input="handleQuantityInput"
              />
              <button
                type="button"
                class="grid h-11 w-11 place-items-center text-muted-foreground transition-colors hover:bg-[color:var(--surface-2)] disabled:opacity-35"
                :disabled="quantityEffectiveLimit !== null && quantity >= quantityEffectiveLimit"
                :aria-label="'+'"
                @click="quantity = quantityEffectiveLimit === null ? quantity + 1 : Math.min(quantityEffectiveLimit, quantity + 1)"
              ><Plus class="h-4 w-4" /></button>
            </div>
            <span class="text-[12.5px] text-muted-foreground">
              {{ quantityEffectiveLimit !== null ? t('productDetail.skuStockRemaining', { count: quantityEffectiveLimit }) : '' }}
            </span>
          </div>
        </div>

        <!-- 阻断提示 -->
        <div
          v-if="!canPurchase && cannotPurchaseReason"
          class="flex items-start gap-3 rounded-[var(--r)] bg-[color:var(--warning-soft)] px-4 py-3 text-[13.5px] leading-[1.6] text-[color:var(--warning-strong)]"
        >
          <AlertCircle class="mt-0.5 h-4 w-4 flex-none" />
          <span>{{ cannotPurchaseReason }}</span>
        </div>

        <!-- 操作 -->
        <div class="flex flex-wrap gap-3 max-[640px]:flex-col">
          <Button
            v-if="requiresLogin"
            size="lg"
            class="h-12 rounded-full px-6 text-[15px] font-medium max-[640px]:w-full"
            @click="goLogin"
          >
            <Lock class="mr-1.5 h-4 w-4" />{{ t('productDetail.loginToBuy') }}
          </Button>
          <template v-else>
            <Button
              size="lg"
              class="h-12 flex-1 rounded-full px-6 text-[15px] font-medium max-[640px]:w-full"
              :disabled="!canPurchase"
              @click="buyNow"
            >
              <Zap class="mr-1.5 h-4 w-4" />{{ t('productDetail.buyNow') }}
            </Button>
            <Button
              variant="outline"
              size="lg"
              class="h-12 flex-1 rounded-full border-[color:var(--border-strong)] px-6 text-[15px] font-medium max-[640px]:w-full"
              :disabled="!canPurchase"
              @click="addToCart"
            >
              <ShoppingCart class="mr-1.5 h-4 w-4" />{{ t('productDetail.addToCart') }}
            </Button>
          </template>
        </div>

        <!-- 元信息 -->
        <dl class="flex flex-col gap-2.5 border-t border-[color:var(--border-soft)] pt-5 text-[13.5px]">
          <div v-if="product.fulfillment_type" class="flex gap-3">
            <dt class="min-w-[76px] text-muted-foreground">{{ t('productDetail.fulfillmentLabel') }}</dt>
            <dd class="text-foreground">{{ getFulfillmentTypeLabel(product.fulfillment_type) }}</dd>
          </div>
          <div v-if="categoryName" class="flex gap-3">
            <dt class="min-w-[76px] text-muted-foreground">{{ t('productDetail.categoryLabel') }}</dt>
            <dd class="text-foreground">{{ categoryName }}</dd>
          </div>
          <div class="flex gap-3">
            <dt class="min-w-[76px] text-muted-foreground">{{ t('productDetail.deliveryReassurance') }}</dt>
            <dd class="text-foreground">{{ getStockStatusLabel(product) }}</dd>
          </div>
        </dl>
      </div>
    </div>

    <!-- 商品说明 -->
    <section v-if="!loading && product && getLocalizedText(product.content)" class="mt-14 border-t border-border pt-10">
      <h2 class="mb-4 text-[20px] font-semibold tracking-[-0.02em]">{{ t('productDetail.details') }}</h2>
      <div
        class="ember-prose max-w-[72ch] text-[15px] leading-[1.75] text-[color:var(--ink-2)]"
        v-html="processHtmlForDisplay(getLocalizedText(product.content))"
      ></div>
    </section>

    <!-- 相关文章 -->
    <section v-if="!loading && relatedPosts.length" class="mt-14 border-t border-border pt-10">
      <h2 class="mb-5 text-[20px] font-semibold tracking-[-0.02em]">{{ t('productDetail.relatedPosts') }}</h2>
      <div class="grid gap-4 grid-cols-[repeat(auto-fill,minmax(240px,1fr))]">
        <RouterLink
          v-for="post in relatedPosts"
          :key="post.id"
          class="rounded-[var(--r-lg)] border border-border bg-card p-5 transition-transform duration-300 hover:-translate-y-1"
          :to="`/blog/${post.slug}`"
        >
          <h3 class="line-clamp-2 text-[15px] font-semibold">{{ getLocalizedText(post.title) }}</h3>
          <p class="mt-2 line-clamp-2 text-[13px] text-muted-foreground">{{ getLocalizedText(post.summary) }}</p>
        </RouterLink>
      </div>
    </section>

    <!-- 移动端底部操作条 -->
    <EmberProductMobileBar
      :visible="!loading && !!product"
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
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  AlertCircle, ChevronRight, Lock, Minus, Package, PackageOpen, Plus, ShoppingCart, Zap,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { processHtmlForDisplay } from '../../utils/content'
import { useProductDetail } from '../../composables/useProductDetail'
import EmberProductMobileBar from './components/EmberProductMobileBar.vue'

const { t } = useI18n()

const {
  getLocalizedText, siteCurrency, formatPrice,
  getFulfillmentTypeLabel, getStockStatusLabel,
  hasPromotionPrice, getPromotionPriceAmount,
  hasSkuPromotionPrice, getSkuPromotionPriceAmount,
  hasPromotionRules, getPromotionRules, formatPromotionRule,
  formatWholesaleTier,
  loading, product, relatedPosts, currentImage, images, selectedSkuId, quantity,
  activeSkus, selectedSku, selectedSkuMemberPrice, selectedSkuWholesaleRules,
  normalizeSkuId, skuDisplayText, handleQuantityInput,
  quantityEffectiveLimit, quantityEffectiveMin,
  canPurchase, cannotPurchaseReason, requiresLogin, requiresSKUSelection,
  categoryName,
  addToCart, buyNow, goLogin,
  mobileBarShowMemberPrice, mobileBarMemberPriceDisplay,
  mobileBarShowSkuPromotionPrice, mobileBarSkuPromotionPriceDisplay,
  mobileBarShowSkuPrice, mobileBarSkuPriceDisplay,
  mobileBarShowProductPromotionPrice, mobileBarProductPromotionPriceDisplay,
  mobileBarProductPriceDisplay,
} = useProductDetail()

/** 会员价仅在存在且低于原价时展示 */
const memberPrice = computed(() => {
  const value = selectedSkuMemberPrice.value
  return value && Number(value) > 0 ? value : null
})

/** 当前生效单价：SKU 促销 → 商品促销 → 会员价 → SKU 原价 → 商品原价 */
const displayPrice = computed(() => {
  const sku = selectedSku.value
  if (sku && hasSkuPromotionPrice(sku)) return getSkuPromotionPriceAmount(sku)
  if (!sku && hasPromotionPrice(product.value)) return getPromotionPriceAmount(product.value)
  if (memberPrice.value) return memberPrice.value
  return sku?.price_amount ?? product.value?.price_amount ?? '0.00'
})

/** 划线原价：仅在确实有优惠时才有意义 */
const originalPrice = computed(() => {
  const sku = selectedSku.value
  return sku?.price_amount ?? product.value?.price_amount ?? '0.00'
})

const discount = computed(() => Number(displayPrice.value) < Number(originalPrice.value))

const skuDisplayPrice = (sku: any) =>
  hasSkuPromotionPrice(sku) ? getSkuPromotionPriceAmount(sku) : sku?.price_amount
</script>

<style scoped>
/* 商品说明的富文本：只在 ember 作用域内生效，避免影响其它模板 */
.ember-prose :deep(img) {
  border-radius: var(--r);
  margin-block: 16px;
}
.ember-prose :deep(a) {
  color: var(--accent);
}
.ember-prose :deep(h2),
.ember-prose :deep(h3) {
  margin-block: 20px 8px;
  font-weight: 600;
}
.ember-prose :deep(p) {
  margin-block: 8px;
}
.ember-prose :deep(ul),
.ember-prose :deep(ol) {
  margin-block: 8px;
  padding-left: 22px;
  list-style: revert;
}
.ember-prose :deep(blockquote) {
  margin-block: 12px;
  padding-left: 16px;
  border-left: 3px solid var(--border-strong);
  color: var(--ink-3);
}
</style>
