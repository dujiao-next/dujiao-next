<template>
  <!-- 精密规格横表行模式（与 AtelierCatalog 表头共用 atelierRowColsClass 保证严格对齐） -->
  <article
    v-if="row"
    class="prism-row group relative cursor-pointer items-center gap-x-4 border-b px-4 py-3 transition-colors last:border-b-0 hover:bg-secondary/65 focus-visible:outline-2 focus-visible:outline-primary max-md:flex max-md:gap-3 md:px-5"
    :class="[atelierRowColsClass, soldOut && 'opacity-55']"
    :style="{ '--h': hue }"
    role="link"
    tabindex="0"
    @click="$emit('click', product.slug)"
    @keydown.enter="$emit('click', product.slug)"
  >
    <div class="flex min-w-0 items-center gap-3 max-md:flex-1">
      <div
        class="atelier-stage relative grid h-10 w-10 shrink-0 place-items-center overflow-hidden rounded-[10px]"
        :class="soldOut && 'grayscale'"
      >
        <img
          v-if="hasImage"
          :src="displayImageSrc"
          :alt="title"
          loading="lazy"
          decoding="async"
          class="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105"
          @error="handleImageError"
        />
        <span
          v-else
          aria-hidden="true"
          class="text-sm font-extrabold tracking-tight text-[hsl(var(--h)_48%_var(--at-glyph-l))]"
        >
          {{ monogram }}
        </span>
      </div>

      <div class="min-w-0 flex-1">
        <h3 class="flex items-center gap-2 text-sm font-semibold leading-snug text-foreground">
          <span class="line-clamp-2 md:line-clamp-1">{{ title }}</span>
          <span
            v-if="flag"
            class="inline-flex h-[19px] shrink-0 items-center rounded-[5px] border px-1.5 text-[10.5px] font-semibold max-md:hidden"
            :class="flag.chipClass"
          >
            {{ flag.label }}
          </span>
        </h3>
        <p v-if="description" class="mt-0.5 truncate text-xs text-muted-foreground max-md:hidden">
          {{ description }}
        </p>
        <!-- 小屏将交付方式与库存并入标题下方 -->
        <p class="mt-1 flex items-center gap-2 truncate text-xs text-muted-foreground md:hidden">
          <span class="inline-flex items-center gap-1">
            <span
              class="h-1.5 w-1.5 shrink-0 rounded-full"
              :class="product.fulfillment_type === 'auto' ? 'bg-success' : 'bg-muted-foreground/50'"
              aria-hidden="true"
            ></span>
            {{ getFulfillmentTypeLabel(product.fulfillment_type) }}
          </span>
          <span :class="stock.text">{{ getStockStatusLabel(product) }}</span>
        </p>
      </div>
    </div>

    <!-- 交付方式 -->
    <div class="max-md:hidden">
      <span
        class="inline-flex h-6 items-center gap-1.5 rounded-[6px] border px-2 text-[11.5px] font-semibold"
        :class="product.fulfillment_type === 'auto'
          ? 'border-success/30 bg-success-soft text-success'
          : 'border-border bg-secondary text-muted-foreground'"
      >
        <span
          class="h-1.5 w-1.5 shrink-0 rounded-full"
          :class="product.fulfillment_type === 'auto' ? 'bg-success' : 'bg-muted-foreground/60'"
          aria-hidden="true"
        ></span>
        <span class="truncate">{{ getFulfillmentTypeLabel(product.fulfillment_type) }}</span>
      </span>
    </div>

    <!-- 库存状态 -->
    <span class="flex min-w-0 items-center gap-1.5 font-mono text-xs max-md:hidden" :class="stock.text">
      <span class="h-1.5 w-1.5 shrink-0 rounded-full" :class="stock.dot" aria-hidden="true"></span>
      <span class="truncate">{{ getStockStatusLabel(product) }}</span>
    </span>

    <!-- 价格 -->
    <div class="flex shrink-0 flex-col items-end tabular-nums">
      <span
        v-if="price.discounted"
        class="theme-price-original text-[11px] leading-tight"
        :aria-label="t('products.originalPriceAria', { price: formatPrice(product.price_amount, siteCurrency) })"
      >
        {{ formatPrice(product.price_amount, siteCurrency) }}
      </span>
      <span class="flex items-baseline whitespace-nowrap font-mono" :class="price.class" :aria-label="price.aria">
        <span v-if="siteCurrency" class="mr-1 font-sans text-[11px] font-semibold text-muted-foreground">{{ siteCurrency }}</span>
        <b class="text-[16px] font-bold leading-none tracking-tight">{{ priceParts.int }}</b>
        <span v-if="priceParts.dec" class="text-[12px] font-semibold opacity-70">.{{ priceParts.dec }}</span>
      </span>
    </div>

    <!-- 操作按钮 -->
    <button
      type="button"
      class="inline-flex h-8 shrink-0 items-center justify-center gap-1 justify-self-end rounded-[9px] border bg-secondary px-3 text-xs font-semibold text-foreground transition-colors group-hover:border-primary group-hover:bg-primary group-hover:text-primary-foreground disabled:pointer-events-none disabled:opacity-40 max-md:w-8 max-md:px-0"
      :aria-label="t('products.quickBuyAria')"
      :disabled="soldOut"
      @click.stop="$emit('quickBuy', product)"
      @keydown.enter.stop
    >
      <ShoppingCart v-if="!soldOut" class="h-3.5 w-3.5 md:hidden" />
      <span class="max-md:hidden">{{ soldOut ? t('quickBuy.soldOut') : t('atelier.buyShort') }}</span>
    </button>
  </article>

  <!-- 方案 B 精密科技卡片模式：横向头像头区 + 真实简介 + 技术规格芯片 + 底部虚线价格与配置选购 -->
  <article
    v-else
    class="prism-pcard prism-card group relative flex h-full cursor-pointer flex-col rounded-[16px] bg-card p-4.5 text-card-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
    :class="soldOut && 'opacity-60'"
    :style="{ '--h': hue }"
    role="link"
    tabindex="0"
    @click="$emit('click', product.slug)"
    @keydown.enter="$emit('click', product.slug)"
  >
    <!-- 顶部横向信息头：左侧 48px 精密展位 + 右侧分类/徽标/标题 -->
    <div class="flex items-start gap-3.5">
      <div
        class="atelier-stage relative grid h-12 w-12 shrink-0 place-items-center overflow-hidden rounded-[12px]"
        :class="soldOut && 'grayscale'"
      >
        <img
          v-if="hasImage"
          :src="displayImageSrc"
          :alt="title"
          loading="lazy"
          decoding="async"
          class="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105"
          @error="handleImageError"
        />
        <span
          v-else
          role="img"
          :aria-label="title"
          class="select-none text-lg font-extrabold tracking-tight text-[hsl(var(--h)_48%_var(--at-glyph-l))]"
        >
          {{ monogram }}
        </span>
      </div>

      <div class="min-w-0 flex-1">
        <div class="flex items-center justify-between gap-2">
          <span v-if="categoryLabel" class="truncate text-[11px] font-semibold text-muted-foreground">
            {{ categoryLabel }}
          </span>
          <span v-else class="text-[11px] font-semibold text-muted-foreground">
            {{ product.fulfillment_type === 'auto' ? t('atelier.autoShort') : t('atelier.manualShort') }}
          </span>
          <span
            v-if="flag"
            class="inline-flex h-[20px] shrink-0 items-center rounded-[6px] border px-1.5 text-[10.5px] font-semibold"
            :class="flag.chipClass"
          >
            {{ flag.label }}
          </span>
        </div>

        <h3 class="mt-1 line-clamp-2 text-[15px] font-bold leading-snug tracking-[-0.015em] text-foreground transition-colors group-hover:text-primary">
          {{ title }}
        </h3>
      </div>
    </div>

    <!-- 中段：真实商品简介 -->
    <p v-if="description" class="mt-3 line-clamp-2 text-[12.5px] leading-relaxed text-muted-foreground">
      {{ description }}
    </p>

    <!-- 真实属性芯片：交付方式 + 实时库存状态 + 商品标签 -->
    <div class="mb-4 mt-3 flex flex-wrap items-center gap-1.5">
      <span
        class="inline-flex h-[22px] items-center gap-1.5 rounded-[6px] border px-2 text-[11px] font-semibold"
        :class="product.fulfillment_type === 'auto'
          ? 'border-success/30 bg-success-soft text-success'
          : 'border-border bg-secondary text-muted-foreground'"
      >
        <span
          class="h-1.5 w-1.5 shrink-0 rounded-full"
          :class="product.fulfillment_type === 'auto' ? 'bg-success' : 'bg-muted-foreground/60'"
          aria-hidden="true"
        ></span>
        <span>{{ getFulfillmentTypeLabel(product.fulfillment_type) }}</span>
      </span>

      <span
        class="inline-flex h-[22px] items-center gap-1 rounded-[6px] border bg-secondary/70 px-2 font-mono text-[11px] font-medium"
        :class="stock.text"
      >
        <span class="h-1.5 w-1.5 shrink-0 rounded-full" :class="stock.dot" aria-hidden="true"></span>
        <span>{{ getStockStatusLabel(product) }}</span>
      </span>

      <span
        v-for="(tag, tagIndex) in (product.tags ?? []).slice(0, 2)"
        :key="tagIndex"
        class="inline-flex h-[22px] max-w-[9rem] items-center truncate rounded-[6px] border bg-secondary/50 px-2 text-[11px] font-medium text-muted-foreground"
      >
        {{ tag }}
      </span>
    </div>

    <!-- 底部：精密分隔线 + 等宽分级价格 + 快捷选购按钮 -->
    <div class="mt-auto flex items-end justify-between gap-2 border-t pt-3.5">
      <div class="flex min-w-0 flex-col tabular-nums">
        <span
          v-if="price.discounted"
          class="theme-price-original truncate text-[11px] leading-tight"
          :aria-label="t('products.originalPriceAria', { price: formatPrice(product.price_amount, siteCurrency) })"
        >
          {{ formatPrice(product.price_amount, siteCurrency) }}
        </span>
        <span class="flex items-baseline whitespace-nowrap font-mono" :class="price.class" :aria-label="price.aria">
          <span v-if="siteCurrency" class="mr-1 font-sans text-[11px] font-semibold text-muted-foreground">
            {{ siteCurrency }}
          </span>
          <b class="text-[21px] font-extrabold leading-none tracking-[-0.03em]">{{ priceParts.int }}</b>
          <span v-if="priceParts.dec" class="text-[12.5px] font-semibold opacity-70">.{{ priceParts.dec }}</span>
        </span>
      </div>

      <button
        type="button"
        class="prism-buy-btn inline-flex h-8 shrink-0 items-center gap-1 rounded-[9px] border bg-secondary px-3 text-xs font-semibold text-foreground transition-all disabled:pointer-events-none disabled:opacity-40"
        :aria-label="t('products.quickBuyAria')"
        :disabled="soldOut"
        @click.stop="$emit('quickBuy', product)"
        @keydown.enter.stop
      >
        <span>{{ soldOut ? t('quickBuy.soldOut') : t('productDetail.buyNow') }}</span>
        <ArrowRight v-if="!soldOut" class="h-3 w-3 transition-transform group-hover:translate-x-0.5" aria-hidden="true" />
      </button>
    </div>
  </article>
</template>

<script lang="ts">
/** 规格横表列宽：商品 | 交付方式 | 库存 | 单价 | 操作。表头与行共用保证严格对齐 */
export const atelierRowColsClass = 'md:grid md:grid-cols-[minmax(0,1.9fr)_7.25rem_7.5rem_8.25rem_5.5rem]'
</script>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowRight, ShoppingCart } from 'lucide-vue-next'
import { getFirstImageUrl, getImageUrl, placeholderHue } from '../../../utils/image'
import { useLocalized, useProductLabels } from '../../../composables/useProduct'
import { splitPrice } from '../utils/price'

const props = withDefaults(defineProps<{
  product: any
  /** 规格横表行模式 */
  row?: boolean
  /** 可选分类名（从分组上下文传入，若未传则读取 product.category.name） */
  categoryName?: string
}>(), {
  row: false,
  categoryName: '',
})

defineEmits<{
  click: [slug: string]
  quickBuy: [product: any]
}>()

const { t } = useI18n()
const { getLocalizedText, siteCurrency, formatPrice } = useLocalized()
const {
  getFulfillmentTypeLabel,
  getStockStatusLabel,
  isSoldOut,
  hasPromotionPrice,
  getPromotionPriceAmount,
  hasPromotionRules,
  hasWholesalePrices,
  getCardExclusivePrice,
} = useProductLabels()

const exclusivePrice = computed(() => getCardExclusivePrice(props.product))
const title = computed(() => getLocalizedText(props.product.title))
const soldOut = computed(() => isSoldOut(props.product))
const description = computed(() => getLocalizedText(props.product.description))

const categoryLabel = computed(() => {
  if (props.categoryName) return props.categoryName
  const cat = props.product?.category
  return cat?.name ? getLocalizedText(cat.name) : ''
})

const hasMultipleSkus = computed(() => {
  const skus = Array.isArray(props.product?.skus) ? props.product.skus : []
  return skus.filter((s: any) => s?.is_active !== false).length > 1
})

const price = computed(() => {
  const p = props.product
  if (exclusivePrice.value !== null) {
    return {
      amount: exclusivePrice.value,
      // 多规格商品的会员展示价取各生效 SKU 最低价，与商品主价可能不是同一规格，避免跨规格划线误导
      discounted: !hasMultipleSkus.value,
      class: 'text-warning',
      aria: undefined,
    }
  }
  if (hasPromotionPrice(p)) {
    const amount = getPromotionPriceAmount(p)
    return {
      amount,
      discounted: true,
      class: 'theme-price-promotion',
      aria: t('products.promotionPriceAria', { price: formatPrice(amount, siteCurrency.value) }),
    }
  }
  return {
    amount: p.price_amount,
    discounted: false,
    class: soldOut.value ? 'text-muted-foreground' : 'text-foreground',
    aria: t('products.priceAria', { price: formatPrice(p.price_amount, siteCurrency.value) }),
  }
})

/** 将价格拆分为整数与小数部分，用于等宽大小数排印 */
const priceParts = computed(() => splitPrice(formatPrice(price.value.amount, '')))

const deal = computed(() => {
  const p = props.product
  if (exclusivePrice.value !== null) {
    return {
      label: t('products.memberPriceTag'),
      chipClass: 'border-warning/35 bg-warning-soft text-warning',
    }
  }
  if (hasPromotionPrice(p)) {
    return {
      label: t('products.promotionTag'),
      chipClass: 'border-danger/35 bg-danger-soft text-danger',
    }
  }
  if (hasWholesalePrices(p)) {
    return {
      label: t('products.wholesaleTag'),
      chipClass: 'border-success/35 bg-success-soft text-success',
    }
  }
  if (hasPromotionRules(p)) {
    return {
      label: t('products.promotionBadge'),
      chipClass: 'border-primary/35 bg-primary/10 text-primary',
    }
  }
  return null
})

const flag = computed(() => (soldOut.value
  ? {
      label: t('products.stockStatus.outOfStock'),
      chipClass: 'border-border bg-secondary text-muted-foreground',
    }
  : deal.value))

const stock = computed(() => ({
  low_stock: { text: 'text-warning', dot: 'bg-warning' },
  out_of_stock: { text: 'text-muted-foreground', dot: 'bg-muted-foreground/50' },
} as Record<string, { text: string; dot: string }>)[props.product?.stock_status] ?? {
  text: 'text-muted-foreground',
  dot: 'bg-success',
})

const monogram = computed(() => Array.from(title.value.trim())[0] ?? '')
const hue = computed(() => placeholderHue(props.product?.id))

const imageErrored = ref(false)
const attemptIdx = ref(0)

const imageCandidates = computed<string[]>(() => {
  const arr: string[] = []
  const primary = getFirstImageUrl(props.product?.images)
  if (primary) arr.push(primary)
  const categoryIcon = props.product?.category?.icon
  if (categoryIcon) {
    const resolved = getImageUrl(categoryIcon)
    if (resolved && resolved !== primary) arr.push(resolved)
  }
  return arr
})

const displayImageSrc = computed(() => imageCandidates.value[attemptIdx.value] ?? '')
const hasImage = computed(() => Boolean(displayImageSrc.value) && !imageErrored.value)

watch(imageCandidates, () => {
  attemptIdx.value = 0
  imageErrored.value = false
}, { deep: true })

const handleImageError = () => {
  if (attemptIdx.value < imageCandidates.value.length - 1) {
    attemptIdx.value++
  } else {
    imageErrored.value = true
  }
}
</script>

<style scoped>
.prism-pcard {
  transition: transform 200ms var(--ui-ease-out), border-color 180ms ease, box-shadow 220ms ease;
}

@media (hover: hover) {
  .prism-pcard:hover {
    transform: translateY(-2px);
    border-color: var(--at-line-hi);
    box-shadow: var(--at-shadow-card-hover);
  }
  .prism-pcard:hover .prism-buy-btn:not(:disabled) {
    border-color: var(--at-brand);
    background-color: var(--at-brand);
    color: #ffffff;
  }
}

@media (prefers-reduced-motion: reduce) {
  .prism-pcard {
    transition: none;
  }
}
</style>
