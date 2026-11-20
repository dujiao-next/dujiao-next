<template>
  <div class="product-detail-page min-h-screen bg-background pb-16 pt-24 text-foreground">
    <div class="container mx-auto px-4">
      <!-- Loading Skeleton -->
      <div v-if="loading" class="space-y-6" aria-hidden="true">
        <div class="h-5 w-48 rounded theme-skeleton"></div>
        <div class="grid gap-5 lg:grid-cols-12 lg:gap-6">
          <div class="aspect-[4/3] rounded-[28px] theme-skeleton lg:col-span-7"></div>
          <div class="theme-float space-y-5 rounded-[28px] bg-card p-6 md:p-8 lg:col-span-5">
            <div class="h-3 w-24 rounded theme-skeleton"></div>
            <div class="h-8 w-3/4 rounded theme-skeleton"></div>
            <div class="h-4 w-full rounded theme-skeleton"></div>
            <div class="h-10 w-40 rounded theme-skeleton"></div>
            <div class="h-12 w-full rounded-full theme-skeleton"></div>
          </div>
        </div>
      </div>

      <!-- Product Content -->
      <div v-else-if="product">
        <BreadcrumbNav
          class="mb-5"
          :items="[
            { label: t('nav.home'), to: '/' },
            { label: t('nav.products'), to: '/products' },
            { label: getLocalizedText(product.title) },
          ]"
        />

        <!-- 大屏：左列图片 + 详情，右列购买面板跟随滚动；小屏按 图片 → 购买 → 详情 顺序堆叠 -->
        <div class="grid gap-5 lg:grid-cols-12 lg:gap-6">
          <ProductStage
            class="lg:col-span-7"
            :images="images"
            :current-image="currentImage"
            :product-title="getLocalizedText(product.title)"
            :product-id="product.id"
            @update:current-image="currentImage = $event"
          />

          <div class="buy-panel theme-float rounded-[22px] bg-card p-5 md:rounded-[28px] md:p-8 lg:sticky lg:top-24 lg:col-span-5 lg:row-span-2 lg:self-start lg:overflow-y-auto">
            <div v-if="categoryName" class="mb-2 line-clamp-1 text-[13px] text-muted-foreground">{{ categoryName }}</div>

            <h1 class="text-balance text-2xl font-bold leading-snug tracking-tight md:text-[28px]">
              {{ getLocalizedText(product.title) }}
            </h1>

            <p v-if="getLocalizedText(product.description)" class="mt-3 text-[15px] leading-relaxed text-muted-foreground">
              {{ getLocalizedText(product.description) }}
            </p>

            <div class="mt-4 flex flex-wrap items-center gap-1.5">
              <Badge :variant="product.purchase_type === 'guest' ? 'warning' : 'success'">
                <UserPlus v-if="product.purchase_type === 'guest'" class="h-3 w-3" />
                <Lock v-else class="h-3 w-3" />
                {{ getPurchaseTypeLabel(product.purchase_type) }}
              </Badge>
              <Badge :variant="product.fulfillment_type === 'auto' ? 'info' : 'neutral'">
                <Zap v-if="product.fulfillment_type === 'auto'" class="h-3 w-3" />
                <Pencil v-else class="h-3 w-3" />
                {{ getFulfillmentTypeLabel(product.fulfillment_type) }}
              </Badge>
              <Badge :variant="getStockBadgeVariant(product.stock_status)">
                {{ getStockStatusLabel(product) }}
              </Badge>
              <Badge v-for="(tag, index) in product.tags || []" :key="index" variant="neutral">{{ tag }}</Badge>
            </div>

            <!-- 价格：数字大、币种小；有优惠时原价划线跟在后面，下面一行说明省了多少 -->
            <div class="mt-6" :class="price.tone">
              <div class="flex flex-wrap items-baseline gap-x-3 gap-y-1">
                <span class="text-[40px] font-bold leading-none tracking-tight tabular-nums" :class="!price.tone && 'text-foreground'">
                  {{ formatPrice(price.final, '') }}<span class="ml-1.5 text-sm font-medium text-muted-foreground">{{ siteCurrency }}</span>
                </span>
                <span v-if="price.original" class="theme-price-original">{{ formatPrice(price.original, siteCurrency) }}</span>
                <Badge v-if="(selectedSku && hasSkuPromotionPrice(selectedSku)) || (!selectedSku && hasPromotionPrice(product))" variant="danger">
                  {{ t('products.promotionTag') }}
                </Badge>
                <Badge v-if="showSelectedSkuMemberBadge" variant="warning">{{ t('products.memberPriceTag') }}</Badge>
                <Badge v-if="hasSelectedSkuWholesalePrice" variant="success">{{ t('products.wholesaleTag') }}</Badge>
              </div>
              <p v-if="price.note" class="mt-2 text-sm font-medium">{{ price.note }}</p>
            </div>

            <!-- 批发价规则展示 -->
            <div v-if="selectedSkuWholesaleRules.length" class="mt-5 rounded-2xl border border-emerald-200 bg-emerald-50/50 px-4 py-3 dark:border-emerald-800/50 dark:bg-emerald-950/20">
              <h2 class="mb-2 flex items-center gap-1.5 text-sm font-bold text-emerald-700 dark:text-emerald-300">
                {{ t('products.wholesaleRulesTitle') }}
              </h2>
              <div class="flex flex-wrap gap-2">
                <Badge v-for="tier in selectedSkuWholesaleRules" :key="`${tier.sku_id || tier.sku_code || 'all'}-${tier.min_quantity}`" variant="success" class="rounded-full px-3 py-1 font-medium">
                  {{ formatWholesaleTier(tier) }}
                </Badge>
              </div>
            </div>

            <!-- 活动规则展示 -->
            <div v-if="hasPromotionRules(product)" class="mt-5 rounded-2xl border border-orange-200 dark:border-orange-800/50 bg-orange-50/50 dark:bg-orange-950/20 px-4 py-3">
              <h2 class="mb-2 text-sm font-bold text-orange-700 dark:text-orange-300 flex items-center gap-1.5">
                <Tag class="w-4 h-4" />
                {{ t('products.promotionRulesTitle') }}
              </h2>
              <ul class="space-y-1">
                <li v-for="rule in getPromotionRules(product)" :key="rule.id" class="text-sm text-orange-600 dark:text-orange-300/90 flex items-center gap-1.5">
                  <span class="w-1 h-1 rounded-full bg-orange-400 dark:bg-orange-500 shrink-0"></span>
                  <span>{{ formatPromotionRule(rule) }}</span>
                </li>
              </ul>
            </div>


            <div v-if="activeSkus.length" class="mt-6">
              <h2 class="mb-2.5 text-[13px] font-semibold text-muted-foreground">{{ t('productDetail.skuTitle') }}</h2>
              <div class="grid grid-cols-2 gap-2">
                <button
                  v-for="sku in activeSkus"
                  :key="sku.id"
                  type="button"
                  class="flex min-h-[44px] flex-col items-start rounded-2xl border-2 px-3 py-2 text-left text-sm transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
                  :class="[
                    normalizeSkuId(sku.id) === selectedSkuId ? 'border-foreground bg-card' : 'border-transparent bg-secondary',
                    isSkuPurchasable(sku) ? 'hover:border-foreground/40' : 'cursor-not-allowed opacity-50',
                  ]"
                  :disabled="!isSkuPurchasable(sku)"
                  :aria-pressed="normalizeSkuId(sku.id) === selectedSkuId"
                  @click="selectedSkuId = normalizeSkuId(sku.id)"
                >
                  <span class="font-semibold leading-tight">{{ skuDisplayText(sku) }}</span>
                  <span class="mt-1 rounded-full border px-2 py-0.5 text-[11px]" :class="skuStockBadgeClass(sku)">
                    {{ skuStockText(sku) }}
                  </span>
                </button>
              </div>
              <p v-if="requiresSKUSelection" class="mt-2 text-xs text-amber-500">{{ t('productDetail.skuRequired') }}</p>
            </div>

            <div class="mt-6 flex items-center justify-between gap-4">
              <h2 class="text-[13px] font-semibold text-muted-foreground">{{ t('productDetail.quantity') }}</h2>
              <div class="flex items-center rounded-full bg-secondary p-1">
                <button
                  type="button"
                  class="flex h-9 w-9 items-center justify-center rounded-full text-foreground transition-colors hover:bg-card disabled:opacity-30"
                  :disabled="quantity <= quantityEffectiveMin"
                  @click="quantity = Math.max(quantityEffectiveMin, quantity - 1)"
                >
                  <Minus class="h-4 w-4" :stroke-width="2.5" />
                </button>
                <input
                  type="text"
                  inputmode="numeric"
                  class="h-9 w-12 bg-transparent text-center text-sm font-semibold tabular-nums text-foreground outline-none"
                  :aria-label="t('productDetail.quantity')"
                  :value="quantity"
                  @change="handleQuantityInput($event)"
                  @keydown.enter.prevent="($event.target as HTMLInputElement)?.blur()"
                />
                <button
                  type="button"
                  class="flex h-9 w-9 items-center justify-center rounded-full text-foreground transition-colors hover:bg-card disabled:opacity-30"
                  :disabled="quantityEffectiveLimit !== null && quantity >= quantityEffectiveLimit"
                  @click="quantity = quantity + 1"
                >
                  <Plus class="h-4 w-4" :stroke-width="2.5" />
                </button>
              </div>
            </div>

            <!-- 购买按钮：小屏滚出视野后由底部固定条接管 -->
            <div ref="purchaseActionsRef" class="mt-6 space-y-3">
              <Alert v-if="cannotPurchaseReason" variant="destructive">
                <AlertDescription class="font-semibold">{{ cannotPurchaseReason }}</AlertDescription>
              </Alert>
              <Alert v-if="purchaseWarning" class="border-warning/40 text-warning">
                <AlertDescription class="font-semibold text-warning">{{ purchaseWarning }}</AlertDescription>
              </Alert>

              <Button v-if="requiresLogin" :class="[inkButton, 'w-full']" @click="goLogin">
                {{ t('productDetail.loginToBuy') }}
              </Button>
              <div v-else class="grid grid-cols-2 gap-3">
                <Button variant="secondary" class="h-12 rounded-full font-semibold" :disabled="!canPurchase" @click="addToCart">
                  <ShoppingCart />
                  {{ t('productDetail.addToCart') }}
                </Button>
                <Button :class="inkButton" :disabled="!canPurchase" @click="buyNow">
                  {{ t('productDetail.buyNow') }}
                </Button>
              </div>
            </div>
          </div>

          <div v-if="product.content || relatedPosts.length" class="min-w-0 space-y-5 lg:col-span-7 lg:space-y-6">
            <section v-if="product.content" class="theme-float rounded-[22px] bg-card p-5 md:rounded-[28px] md:p-8">
              <h2 class="mb-5 text-[19px] font-bold tracking-tight md:text-[22px]">{{ t('productDetail.details') }}</h2>
              <div
                v-html="processHtmlForDisplay(getLocalizedText(product.content))"
                class="prose prose-gray max-w-none theme-prose dark:prose-invert"
              ></div>
            </section>

            <section v-if="relatedPosts.length">
              <h2 class="mb-4 text-[19px] font-bold tracking-tight md:text-[22px]">{{ t('productDetail.relatedPosts') }}</h2>
              <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <router-link
                  v-for="rp in relatedPosts"
                  :key="rp.id"
                  :to="`/blog/${rp.slug}`"
                  class="theme-float group flex flex-col overflow-hidden rounded-[22px] bg-card p-1.5"
                >
                  <div v-if="rp.thumbnail" class="h-32 overflow-hidden rounded-2xl">
                    <img :src="getImageUrl(rp.thumbnail)" :alt="getLocalizedText(rp.title)" loading="lazy" class="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105" />
                  </div>
                  <div class="flex flex-1 flex-col p-3.5">
                    <h3 class="mb-2 line-clamp-2 font-semibold">{{ getLocalizedText(rp.title) }}</h3>
                    <p v-if="rp.summary" class="mb-3 line-clamp-2 text-sm leading-relaxed text-muted-foreground">
                      {{ getLocalizedText(rp.summary) }}
                    </p>
                    <time class="mt-auto text-xs text-muted-foreground">{{ formatRelatedPostDate(rp.published_at) }}</time>
                  </div>
                </router-link>
              </div>
            </section>
          </div>
        </div>

        <div class="mt-8">
          <router-link :to="backToListTo" class="theme-pill-link gap-1 pl-2.5 pr-3.5">
            <ArrowLeft class="h-3.5 w-3.5" aria-hidden="true" />
            {{ t('productDetail.backToProducts') }}
          </router-link>
        </div>

        <!-- Mobile Fixed Purchase Bar -->
        <ProductMobileBar
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
      </div>

      <!-- Error State -->
      <EmptyState
        v-else
        size="lg"
        icon="alert"
        :title="t('productDetail.notFound')"
      >
        <template #action>
          <Button class="rounded-full h-10" @click="loadProduct">
            <RotateCw />
            {{ t('errorBoundary.retry') }}
          </Button>
          <Button variant="secondary" as-child class="rounded-full h-10">
            <router-link :to="backToListTo">{{ t('productDetail.backToProducts') }}</router-link>
          </Button>
        </template>
      </EmptyState>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowLeft, Lock, Minus, Pencil, Plus, RotateCw, ShoppingCart, Tag, UserPlus, Zap } from 'lucide-vue-next'
import { getImageUrl } from '../utils/image'
import { processHtmlForDisplay } from '../utils/content'
import { useProductDetail } from '../composables/useProductDetail'
import ProductStage from '../components/product/ProductStage.vue'
import { resolveDetailPrice } from '../utils/detailPrice'
import ProductMobileBar from '../components/product/ProductMobileBar.vue'
import BreadcrumbNav from '../components/BreadcrumbNav.vue'
import EmptyState from '../components/EmptyState.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

const { t } = useI18n()

// 视图专属：移动端固定购买条（IntersectionObserver 监听桌面购买区是否出屏）
const purchaseActionsRef = ref<HTMLElement | null>(null)
const showMobileBar = ref(false)
let observer: IntersectionObserver | null = null

const setupMobileBarObserver = () => {
  if (observer) observer.disconnect()
  if (!purchaseActionsRef.value) return
  observer = new IntersectionObserver(
    (entries) => {
      const entry = entries[0]
      if (entry) {
        showMobileBar.value = !entry.isIntersecting
      }
    },
    { threshold: 0.1 }
  )
  observer.observe(purchaseActionsRef.value)
}

// 全部业务逻辑由 useProductDetail 提供（与 vault 模板共用，保证功能一致）
const {
  getLocalizedText, siteCurrency, formatPrice,
  getPurchaseTypeLabel, getFulfillmentTypeLabel, getStockBadgeVariant, getStockStatusLabel,
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
  isSkuPurchasable, skuDisplayText, skuStockText, skuStockBadgeClass,
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

// 主操作用墨色，与首页一致；蓝色只留给链接
const inkButton = 'h-12 rounded-full bg-foreground font-semibold text-background hover:bg-foreground/85'

const TONE = {
  member: 'text-amber-600 dark:text-amber-300',
  wholesale: 'text-emerald-600 dark:text-emerald-300',
  promotion: 'text-rose-600 dark:text-rose-300',
}

// 当前要展示的价格：成交价、划线原价、优惠说明、强调色。取舍规则见 resolveDetailPrice
const price = computed(() => {
  const sku = selectedSku.value
  const item = product.value
  const resolved = resolveDetailPrice({
    skuPrice: sku?.price_amount,
    wholesale: sku && hasSelectedSkuWholesalePrice.value
      ? { final: selectedSkuWholesaleFinalPrice.value, isMember: selectedSkuWholesaleFinalIsMember.value }
      : undefined,
    skuPromotion: sku && hasSkuPromotionPrice(sku)
      ? {
          price: selectedSkuPromotionPrice.value,
          save: getSkuPromotionSaveAmount(sku),
          memberFinal: selectedSkuPromotionFinalIsMember.value ? selectedSkuPromotionFinalPrice.value : undefined,
        }
      : undefined,
    memberPrice: hasMemberPrice.value ? selectedSkuMemberPrice.value : undefined,
    productPrice: item?.price_amount,
    productPromotion: hasPromotionPrice(item)
      ? { price: getPromotionPriceAmount(item), save: getPromotionSaveAmount(item) }
      : undefined,
  })
  const saved = `${t('products.saveAmount')} ${formatPrice(resolved.saved, siteCurrency.value)}`
  const tag = { member: t('products.memberPriceTag'), wholesale: t('products.wholesaleTag') }[resolved.kind as 'member' | 'wholesale']
  return {
    final: resolved.final,
    original: resolved.original,
    tone: resolved.kind ? TONE[resolved.kind] : '',
    note: resolved.kind ? (tag ? `${tag} · ${saved}` : saved) : '',
  }
})

onUnmounted(() => {
  if (observer) {
    observer.disconnect()
    observer = null
  }
})
</script>

<style scoped>
/* 购买面板跟随滚动时不超过视口，规格很多时在面板内滚动 */
@media (min-width: 1024px) {
  .buy-panel {
    max-height: calc(100vh - 7.5rem);
  }
}
</style>
