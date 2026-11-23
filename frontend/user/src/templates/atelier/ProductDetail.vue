<template>
  <div class="min-h-screen pb-16 pt-[88px] text-foreground">
    <div class="mx-auto w-full max-w-[1240px] px-4 sm:px-6">
      <!-- 骨架屏 -->
      <div v-if="loading" class="space-y-5" aria-hidden="true">
        <div class="h-5 w-48 rounded theme-skeleton"></div>
        <div class="grid gap-5 lg:grid-cols-12">
          <div class="space-y-4 lg:col-span-7 xl:col-span-8">
            <div class="h-44 rounded-[16px] theme-skeleton"></div>
            <div class="h-40 rounded-[16px] theme-skeleton"></div>
            <div class="h-20 rounded-[16px] theme-skeleton"></div>
          </div>
          <div class="prism-card space-y-4 rounded-[18px] bg-card p-6 lg:col-span-5 xl:col-span-4">
            <div class="h-5 w-32 rounded theme-skeleton"></div>
            <div class="h-24 w-full rounded-[12px] theme-skeleton"></div>
            <div class="h-11 w-full rounded-[11px] theme-skeleton"></div>
          </div>
        </div>
      </div>

      <!-- 商品主体：方案 B 步骤式规格配置器（左） + Stripe 风格实时订单回执（右） -->
      <div v-else-if="product">
        <BreadcrumbNav
          class="mb-5"
          :items="[
            { label: t('nav.home'), to: '/' },
            { label: t('nav.products'), to: '/products' },
            { label: getLocalizedText(product.title) },
          ]"
        />

        <div class="grid items-start gap-5 lg:grid-cols-12">
          <!-- 左侧配置器流（头卡 + 画廊 + 步骤 01 SKU + 步骤 02 数量 + 步骤 03 详情与关联文章） -->
          <div class="min-w-0 space-y-4 lg:col-span-7 xl:col-span-8">
            <!-- 商品概览头卡 -->
            <section class="prism-card rounded-[16px] bg-card p-5 sm:p-6">
              <div class="flex flex-col gap-5 sm:flex-row sm:items-start">
                <!-- 商品缩略/多图展台 -->
                <div class="w-full shrink-0 sm:w-[196px]">
                  <AtelierProductStage
                    :images="images"
                    :current-image="currentImage"
                    :product-title="getLocalizedText(product.title)"
                    :product-id="product.id"
                    @update:current-image="currentImage = $event"
                  />
                </div>

                <!-- 标题、简介与真实属性芯片 -->
                <div class="min-w-0 flex-1">
                  <div class="flex flex-wrap items-center gap-2">
                    <span
                      v-if="categoryName"
                      class="inline-flex max-w-[18rem] items-center rounded-[6px] border bg-secondary px-2 py-0.5 text-[11px] font-semibold text-muted-foreground"
                    >
                      <span class="truncate">{{ categoryName }}</span>
                    </span>
                    <span
                      class="inline-flex h-5.5 items-center gap-1.5 rounded-[6px] border px-2 text-[11px] font-semibold"
                      :class="product.fulfillment_type === 'auto'
                        ? 'border-success/30 bg-success-soft text-success'
                        : 'border-border bg-secondary text-muted-foreground'"
                    >
                      <span
                        class="h-1.5 w-1.5 rounded-full"
                        :class="product.fulfillment_type === 'auto' ? 'bg-success' : 'bg-muted-foreground/60'"
                        aria-hidden="true"
                      ></span>
                      {{ getFulfillmentTypeLabel(product.fulfillment_type) }}
                    </span>
                    <span class="inline-flex h-5.5 items-center gap-1 rounded-[6px] border bg-secondary/70 px-2 text-[11px] font-medium text-muted-foreground">
                      <UserPlus v-if="product.purchase_type === 'guest'" class="h-3 w-3" aria-hidden="true" />
                      <Lock v-else class="h-3 w-3" aria-hidden="true" />
                      {{ getPurchaseTypeLabel(product.purchase_type) }}
                    </span>
                  </div>

                  <h1 class="mt-2.5 text-balance text-xl font-bold leading-snug tracking-[-0.025em] sm:text-2xl">
                    {{ getLocalizedText(product.title) }}
                  </h1>

                  <p v-if="getLocalizedText(product.description)" class="mt-2 text-sm leading-relaxed text-muted-foreground">
                    {{ getLocalizedText(product.description) }}
                  </p>

                  <!-- 真实库存与标签 -->
                  <div class="mt-3.5 flex flex-wrap items-center gap-1.5">
                    <span
                      class="inline-flex h-6 items-center rounded-[6px] border px-2 font-mono text-[11px] font-semibold"
                      :class="product.stock_status === 'low_stock'
                        ? 'border-warning/35 bg-warning-soft text-warning'
                        : (product.stock_status === 'out_of_stock'
                          ? 'border-danger/35 bg-danger-soft text-danger'
                          : 'border-border bg-secondary/60 text-foreground/85')"
                    >
                      {{ getStockStatusLabel(product) }}
                    </span>
                    <span
                      v-for="(tag, index) in product.tags || []"
                      :key="index"
                      class="inline-flex h-6 items-center rounded-[6px] border bg-secondary/50 px-2 text-[11px] font-medium text-muted-foreground"
                    >
                      {{ tag }}
                    </span>
                  </div>
                </div>
              </div>

              <!-- 真实阶梯批发价 -->
              <div v-if="selectedSkuWholesaleRules.length" class="mt-4 rounded-[12px] border border-success/25 bg-success-soft px-3.5 py-2.5">
                <div class="flex flex-wrap items-center justify-between gap-2">
                  <span class="text-xs font-semibold text-success">{{ t('products.wholesaleRulesTitle') }}</span>
                  <div class="flex flex-wrap gap-1.5">
                    <span
                      v-for="tier in selectedSkuWholesaleRules"
                      :key="`${tier.sku_id || tier.sku_code || 'all'}-${tier.min_quantity}`"
                      class="rounded-[6px] border border-success/20 bg-card px-2 py-0.5 font-mono text-xs font-semibold text-success"
                    >
                      {{ formatWholesaleTier(tier) }}
                    </span>
                  </div>
                </div>
              </div>

              <!-- 真实活动规则 -->
              <div v-if="hasPromotionRules(product)" class="mt-3 rounded-[12px] border border-warning/30 bg-warning-soft px-3.5 py-2.5">
                <h2 class="mb-1.5 flex items-center gap-1.5 text-xs font-bold text-warning">
                  <Tag class="h-3.5 w-3.5" />
                  {{ t('products.promotionRulesTitle') }}
                </h2>
                <ul class="space-y-1">
                  <li
                    v-for="rule in getPromotionRules(product)"
                    :key="rule.id"
                    class="flex items-center gap-1.5 text-xs text-warning"
                  >
                    <span class="h-1 w-1 shrink-0 rounded-full bg-warning"></span>
                    <span>{{ formatPromotionRule(rule) }}</span>
                  </li>
                </ul>
              </div>
            </section>

            <!-- 步骤 01：真实 SKU 规格选择 -->
            <section v-if="activeSkus.length" class="prism-card rounded-[16px] bg-card p-5 sm:p-6">
              <h2 class="mb-3.5 flex items-center text-sm font-bold tracking-[-0.015em] text-foreground">
                <span class="prism-step-badge mr-2 font-mono">01</span>
                <span>{{ t('atelier.stepSku') }}</span>
              </h2>
              <div class="grid grid-cols-1 gap-2.5 sm:grid-cols-2">
                <button
                  v-for="sku in activeSkus"
                  :key="sku.id"
                  type="button"
                  class="relative flex min-h-[60px] flex-col items-start rounded-[12px] border-[1.5px] px-3.5 py-3 text-left transition-all focus-visible:outline-2 focus-visible:outline-primary"
                  :class="[
                    normalizeSkuId(sku.id) === selectedSkuId
                      ? 'border-primary bg-primary/10 shadow-xs'
                      : 'border-border bg-secondary/40 hover:border-primary/40 hover:bg-secondary/70',
                    !isSkuPurchasable(sku) && 'cursor-not-allowed opacity-45',
                  ]"
                  :disabled="!isSkuPurchasable(sku)"
                  :aria-pressed="normalizeSkuId(sku.id) === selectedSkuId"
                  @click="selectedSkuId = normalizeSkuId(sku.id)"
                >
                  <div class="flex w-full items-start justify-between gap-2">
                    <span class="text-[13.5px] font-semibold leading-snug text-foreground">{{ skuDisplayText(sku) }}</span>
                    <span
                      v-if="sku.price_amount != null"
                      class="shrink-0 font-mono text-xs font-bold"
                      :class="normalizeSkuId(sku.id) === selectedSkuId ? 'text-primary' : 'text-foreground/80'"
                    >
                      {{ formatPrice(sku.price_amount, siteCurrency) }}
                    </span>
                  </div>
                  <div class="mt-1.5 flex w-full items-center justify-between gap-2">
                    <span class="font-mono text-[11.5px] text-muted-foreground">{{ skuStockText(sku) }}</span>
                    <span
                      v-if="normalizeSkuId(sku.id) === selectedSkuId"
                      class="grid h-4 w-4 place-items-center rounded-full bg-primary text-primary-foreground"
                      aria-hidden="true"
                    >
                      <Check class="h-2.5 w-2.5" :stroke-width="3" />
                    </span>
                  </div>
                </button>
              </div>
              <p v-if="requiresSKUSelection" class="mt-2.5 text-xs font-medium text-warning">{{ t('productDetail.skuRequired') }}</p>
            </section>

            <!-- 步骤 02（无 SKU 时为 01）：数量配置 -->
            <section class="prism-card flex flex-wrap items-center justify-between gap-4 rounded-[16px] bg-card p-5 sm:px-6">
              <h2 class="flex items-center text-sm font-bold tracking-[-0.015em] text-foreground">
                <span class="prism-step-badge mr-2 font-mono">{{ activeSkus.length ? '02' : '01' }}</span>
                <span>{{ t('atelier.stepQty') }}</span>
              </h2>
              <div class="inline-flex items-center gap-1 rounded-[10px] border bg-secondary p-1">
                <button
                  type="button"
                  class="grid h-8 w-8 place-items-center rounded-[7px] bg-card text-foreground shadow-2xs transition-colors hover:bg-background disabled:opacity-35"
                  :disabled="quantity <= quantityEffectiveMin"
                  @click="quantity = Math.max(quantityEffectiveMin, quantity - 1)"
                >
                  <Minus class="h-3.5 w-3.5" :stroke-width="2.5" />
                </button>
                <input
                  type="text"
                  inputmode="numeric"
                  class="h-8 w-11 bg-transparent text-center font-mono text-sm font-bold tabular-nums text-foreground outline-none"
                  :aria-label="t('productDetail.quantity')"
                  :value="quantity"
                  @change="handleQuantityInput($event)"
                  @keydown.enter.prevent="($event.target as HTMLInputElement)?.blur()"
                />
                <button
                  type="button"
                  class="grid h-8 w-8 place-items-center rounded-[7px] bg-card text-foreground shadow-2xs transition-colors hover:bg-background disabled:opacity-35"
                  :disabled="quantityEffectiveLimit !== null && quantity >= quantityEffectiveLimit"
                  @click="quantity = quantity + 1"
                >
                  <Plus class="h-3.5 w-3.5" :stroke-width="2.5" />
                </button>
              </div>
            </section>

            <!-- 步骤 03：真实富文本商品详情 -->
            <section v-if="product.content" class="prism-card rounded-[16px] bg-card p-5 sm:p-7">
              <h2 class="mb-4 flex items-center border-b pb-3.5 text-sm font-bold tracking-[-0.015em] text-foreground">
                <span class="prism-step-badge mr-2 font-mono">{{ activeSkus.length ? '03' : '02' }}</span>
                <span>{{ t('atelier.stepDetail') }}</span>
              </h2>
              <div
                v-html="processHtmlForDisplay(getLocalizedText(product.content))"
                class="prose prose-gray max-w-none theme-prose dark:prose-invert"
              ></div>
            </section>

            <!-- 真实关联文章 -->
            <section v-if="relatedPosts.length">
              <h2 class="mb-3.5 text-base font-bold tracking-[-0.02em]">{{ t('productDetail.relatedPosts') }}</h2>
              <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <RouterLink
                  v-for="rp in relatedPosts"
                  :key="rp.id"
                  :to="`/blog/${rp.slug}`"
                  class="prism-card group flex flex-col overflow-hidden rounded-[16px] bg-card p-2 transition-all hover:-translate-y-0.5 hover:border-primary/40"
                >
                  <div v-if="rp.thumbnail" class="h-32 overflow-hidden rounded-[12px]">
                    <img
                      :src="getImageUrl(rp.thumbnail)"
                      :alt="getLocalizedText(rp.title)"
                      loading="lazy"
                      class="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105"
                    />
                  </div>
                  <div class="flex flex-1 flex-col p-3">
                    <h3 class="mb-1.5 line-clamp-2 text-[14.5px] font-semibold group-hover:text-primary">{{ getLocalizedText(rp.title) }}</h3>
                    <p v-if="rp.summary" class="mb-3 line-clamp-2 text-xs leading-relaxed text-muted-foreground">
                      {{ getLocalizedText(rp.summary) }}
                    </p>
                    <time class="mt-auto font-mono text-[11px] text-muted-foreground">
                      {{ formatRelatedPostDate(rp.published_at) }}
                    </time>
                  </div>
                </RouterLink>
              </div>
            </section>
          </div>

          <!-- 右侧吸顶：Stripe Checkout 风格实时订单回执小票（100% 真实数据计算） -->
          <aside class="prism-card-accent rounded-[18px] bg-card p-5 sm:p-6 lg:sticky lg:top-[84px] lg:col-span-5 xl:col-span-4">
            <div class="flex items-center justify-between gap-2 border-b border-dashed pb-3.5">
              <span class="text-sm font-bold tracking-[-0.015em] text-foreground">{{ t('atelier.receiptTitle') }}</span>
              <span
                class="inline-flex h-5.5 items-center gap-1.5 rounded-[6px] border px-2 font-mono text-[10.5px] font-semibold"
                :class="canPurchase || requiresLogin
                  ? 'border-success/35 bg-success-soft text-success'
                  : 'border-border bg-secondary text-muted-foreground'"
              >
                <span
                  class="h-1.5 w-1.5 rounded-full"
                  :class="canPurchase || requiresLogin ? 'bg-success' : 'bg-muted-foreground/50'"
                  aria-hidden="true"
                ></span>
                {{ canPurchase || requiresLogin ? t('atelier.receiptReady') : t('atelier.receiptUnavailable') }}
              </span>
            </div>

            <!-- 真实明细行 -->
            <dl class="space-y-2.5 border-b border-dashed py-4 text-[13px]">
              <div class="flex items-start justify-between gap-4">
                <dt class="shrink-0 text-muted-foreground">{{ t('atelier.receiptProduct') }}</dt>
                <dd class="line-clamp-2 text-right font-semibold text-foreground">{{ getLocalizedText(product.title) }}</dd>
              </div>

              <div v-if="selectedSku" class="flex items-start justify-between gap-4">
                <dt class="shrink-0 text-muted-foreground">{{ t('atelier.receiptSku') }}</dt>
                <dd class="text-right font-medium text-foreground">{{ skuDisplayText(selectedSku) }}</dd>
              </div>

              <div class="flex items-center justify-between gap-4">
                <dt class="shrink-0 text-muted-foreground">{{ t('atelier.receiptDelivery') }}</dt>
                <dd class="font-medium" :class="product.fulfillment_type === 'auto' ? 'text-success' : 'text-foreground'">
                  {{ getFulfillmentTypeLabel(product.fulfillment_type) }}
                </dd>
              </div>

              <div class="flex items-center justify-between gap-4">
                <dt class="shrink-0 text-muted-foreground">{{ t('atelier.receiptUnitQty') }}</dt>
                <dd class="font-mono font-medium text-foreground">
                  {{ formatPrice(price.final, siteCurrency) }} × {{ quantity }}
                </dd>
              </div>

              <div v-if="price.note" class="flex items-center justify-between gap-4">
                <dt class="shrink-0 text-muted-foreground">{{ price.label }}</dt>
                <dd class="text-right text-xs font-semibold" :class="price.tone">{{ price.note }}</dd>
              </div>
            </dl>

            <!-- 实时预计小计 -->
            <div class="py-4">
              <div class="flex flex-wrap items-baseline justify-between gap-2">
                <span class="text-xs font-medium text-muted-foreground">{{ t('atelier.receiptSubtotal') }}</span>
                <div class="flex flex-wrap items-center gap-1.5">
                  <Badge v-if="(selectedSku && hasSkuPromotionPrice(selectedSku)) || (!selectedSku && hasPromotionPrice(product))" variant="danger" size="sm">
                    {{ t('products.promotionTag') }}
                  </Badge>
                  <Badge v-if="showSelectedSkuMemberBadge" variant="warning" size="sm">{{ t('products.memberPriceTag') }}</Badge>
                  <Badge v-if="hasSelectedSkuWholesalePrice" variant="success" size="sm">{{ t('products.wholesaleTag') }}</Badge>
                </div>
              </div>

              <div class="mt-1.5 flex flex-wrap items-baseline justify-between gap-2 tabular-nums">
                <span class="flex items-baseline font-mono" :class="price.tone || 'text-foreground'">
                  <b class="text-[32px] font-extrabold leading-none tracking-[-0.035em]">{{ subtotalParts.int }}</b>
                  <span v-if="subtotalParts.dec" class="text-[18px] font-semibold opacity-70">.{{ subtotalParts.dec }}</span>
                  <span v-if="siteCurrency" class="ml-1.5 font-sans text-xs font-semibold text-muted-foreground">{{ siteCurrency }}</span>
                </span>
                <span v-if="originalSubtotalDisplay" class="theme-price-original font-mono text-xs">
                  {{ originalSubtotalDisplay }}
                </span>
              </div>
            </div>

            <!-- 购买操作区 -->
            <div ref="purchaseActionsRef" class="space-y-2.5">
              <Alert v-if="cannotPurchaseReason" variant="destructive">
                <AlertDescription class="font-semibold">{{ cannotPurchaseReason }}</AlertDescription>
              </Alert>
              <Alert v-if="purchaseWarning" class="border-warning/40 text-warning">
                <AlertDescription class="font-semibold text-warning">{{ purchaseWarning }}</AlertDescription>
              </Alert>

              <Button
                v-if="requiresLogin"
                class="h-11 w-full rounded-[11px] font-semibold shadow-sm"
                @click="goLogin"
              >
                {{ t('productDetail.loginToBuy') }}
              </Button>
              <template v-else>
                <Button
                  class="h-11 w-full rounded-[11px] font-semibold shadow-sm"
                  :disabled="!canPurchase"
                  @click="buyNow"
                >
                  <span>{{ t('productDetail.buyNow') }}</span>
                  <ArrowRight class="h-4 w-4" />
                </Button>
                <Button
                  variant="secondary"
                  class="h-10.5 w-full rounded-[11px] border font-semibold"
                  :disabled="!canPurchase"
                  @click="addToCart"
                >
                  <ShoppingCart class="h-4 w-4" />
                  <span>{{ t('productDetail.addToCart') }}</span>
                </Button>
              </template>
            </div>

            <!-- 底部交付提示 -->
            <div class="mt-4 flex items-center gap-2 rounded-[10px] border bg-secondary/50 px-3 py-2.5 text-xs text-muted-foreground">
              <ShieldCheck class="h-4 w-4 shrink-0 text-success" aria-hidden="true" />
              <span>
                {{ product.fulfillment_type === 'auto' ? t('atelier.assurance.autoDesc') : t('atelier.assurance.manualDesc') }}
              </span>
            </div>
          </aside>
        </div>

        <div class="mt-8">
          <RouterLink :to="backToListTo" class="theme-pill-link gap-1 pl-2.5 pr-3.5">
            <ArrowLeft class="h-3.5 w-3.5" aria-hidden="true" />
            {{ t('productDetail.backToProducts') }}
          </RouterLink>
        </div>

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

      <!-- 错误空态 -->
      <EmptyState
        v-else
        size="lg"
        icon="alert"
        :title="t('productDetail.notFound')"
      >
        <template #action>
          <Button class="h-10 rounded-[10px]" @click="loadProduct">
            <RotateCw class="h-4 w-4" />
            {{ t('errorBoundary.retry') }}
          </Button>
          <Button variant="secondary" as-child class="h-10 rounded-[10px]">
            <RouterLink :to="backToListTo">{{ t('productDetail.backToProducts') }}</RouterLink>
          </Button>
        </template>
      </EmptyState>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  ArrowLeft,
  ArrowRight,
  Check,
  Lock,
  Minus,
  Plus,
  RotateCw,
  ShieldCheck,
  ShoppingCart,
  Tag,
  UserPlus,
} from 'lucide-vue-next'
import { getImageUrl } from '../../utils/image'
import { processHtmlForDisplay } from '../../utils/content'
import { resolveDetailPrice } from '../../utils/detailPrice'
import { splitPrice } from './utils/price'
import { useProductDetail } from '../../composables/useProductDetail'
import AtelierProductStage from './components/AtelierProductStage.vue'
import ProductMobileBar from '../../components/product/ProductMobileBar.vue'
import BreadcrumbNav from '../../components/BreadcrumbNav.vue'
import EmptyState from '../../components/EmptyState.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

const { t } = useI18n()

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
    { threshold: 0.1 }
  )
  observer.observe(purchaseActionsRef.value)
}

const {
  getLocalizedText, siteCurrency, formatPrice,
  getPurchaseTypeLabel, getFulfillmentTypeLabel, getStockStatusLabel,
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

const TONE = {
  member: 'text-warning',
  wholesale: 'text-success',
  promotion: 'text-danger',
}

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
  const kindLabel = resolved.kind
    ? {
        member: t('products.memberPriceTag'),
        wholesale: t('products.wholesaleTag'),
        promotion: t('products.promotionTag'),
      }[resolved.kind]
    : ''
  return {
    final: resolved.final,
    original: resolved.original,
    tone: resolved.kind ? TONE[resolved.kind] : '',
    label: kindLabel,
    note: resolved.kind ? saved : '',
  }
})

/** 实时预计小计（基于真实单价 × 当前数量，不编造任何额外费用） */
const subtotalAmount = computed(() => {
  const unit = Number(price.value.final)
  const qty = Number(quantity.value) || 1
  if (!Number.isFinite(unit)) return price.value.final
  return (unit * qty).toFixed(2)
})

const subtotalParts = computed(() => splitPrice(formatPrice(subtotalAmount.value, '')))

const originalSubtotalDisplay = computed(() => {
  if (!price.value.original) return ''
  const orig = Number(price.value.original)
  const qty = Number(quantity.value) || 1
  if (!Number.isFinite(orig)) return formatPrice(price.value.original, siteCurrency.value)
  return formatPrice((orig * qty).toFixed(2), siteCurrency.value)
})

onUnmounted(() => {
  if (observer) {
    observer.disconnect()
    observer = null
  }
})
</script>
