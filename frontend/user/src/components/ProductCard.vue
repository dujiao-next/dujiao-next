<template>
  <!-- row：列表模式的表格行。列宽与 ProductCatalog 里的表头保持一致（见 listColsClass） -->
  <article
    v-if="row"
    class="prow group relative cursor-pointer items-center gap-x-4 rounded-2xl bg-secondary/60 p-2 md:pr-3 focus-visible:outline-2 focus-visible:outline-primary max-md:flex max-md:gap-3 md:px-3"
    :class="[rowColsClass, soldOut && 'opacity-55']"
    :style="{ '--h': hue }"
    role="link"
    tabindex="0"
    @click="$emit('click', product.slug)"
    @keydown.enter="$emit('click', product.slug)"
  >
    <div class="flex min-w-0 items-center gap-3 max-md:flex-1">
      <div
        class="relative grid h-14 w-14 shrink-0 place-items-center overflow-hidden rounded-xl md:h-16 md:w-16"
        :class="[hasImage ? 'bg-muted' : 'bg-[hsl(var(--h)_62%_94%)] dark:bg-[hsl(var(--h)_32%_17%)]', soldOut && 'grayscale']"
      >
        <template v-if="hasImage">
          <img :src="displayImageSrc" alt="" aria-hidden="true" loading="lazy" decoding="async" class="absolute inset-0 h-full w-full scale-150 object-cover opacity-60 blur-lg saturate-150" />
          <img :src="displayImageSrc" :alt="title" loading="lazy" decoding="async" class="pcard-tile relative h-9 w-9 rounded-lg object-cover md:h-10 md:w-10" @error="handleImageError" />
        </template>
        <span v-else aria-hidden="true" class="text-xl font-extrabold text-[hsl(var(--h)_55%_38%)] dark:text-[hsl(var(--h)_55%_76%)]">{{ monogram }}</span>
      </div>
      <div class="min-w-0">
        <h3 class="flex items-center gap-2 text-sm font-semibold leading-snug md:text-[15px]">
          <span class="line-clamp-2 md:line-clamp-1">{{ title }}</span>
          <span v-if="flag" class="pcard-flag shrink-0 max-md:hidden" :class="flag.class">{{ flag.label }}</span>
        </h3>
        <!-- 小屏没有独立的列，交付方式和库存并到标题下面 -->
        <p v-if="description" class="mt-0.5 truncate text-[13px] text-muted-foreground max-md:hidden">{{ description }}</p>
        <p class="mt-1 flex items-center gap-1.5 truncate text-xs text-muted-foreground empty:hidden">
          <span class="inline-flex min-w-0 items-center gap-1 overflow-hidden whitespace-nowrap md:hidden">
            <Zap v-if="product.fulfillment_type === 'auto'" class="h-3 w-3 shrink-0 fill-current text-warm" aria-hidden="true" />
            {{ getFulfillmentTypeLabel(product.fulfillment_type) }}
          </span>
          <span class="shrink-0 md:hidden" :class="stock.text">{{ getStockStatusLabel(product) }}</span>
          <span v-for="(tag, tagIndex) in (product.tags ?? []).slice(0, maxTags)" :key="tagIndex" class="truncate rounded bg-card px-1.5 leading-5 max-md:hidden">
            {{ tag }}
          </span>
        </p>
      </div>
    </div>

    <span class="max-md:hidden">
      <span
        class="inline-flex h-6 items-center gap-1 rounded-full bg-card px-2.5 text-xs font-medium text-foreground"
      >
        <Zap v-if="product.fulfillment_type === 'auto'" class="h-3 w-3 shrink-0 fill-current text-warm" aria-hidden="true" />
        {{ getFulfillmentTypeLabel(product.fulfillment_type) }}
      </span>
    </span>

    <span class="flex min-w-0 items-center gap-1.5 text-[13px] max-md:hidden" :class="stock.text">
      <span class="h-1.5 w-1.5 shrink-0 rounded-full" :class="stock.dot" aria-hidden="true"></span>
      <span class="truncate">{{ getStockStatusLabel(product) }}</span>
    </span>

    <div class="flex shrink-0 flex-col items-end tabular-nums">
      <span v-if="price.discounted" class="theme-price-original text-[11px] leading-tight"
        :aria-label="t('products.originalPriceAria', { price: formatPrice(product.price_amount, siteCurrency) })">
        {{ formatPrice(product.price_amount, siteCurrency) }}
      </span>
      <span class="flex items-baseline gap-1 whitespace-nowrap" :class="price.class" :aria-label="price.aria">
        <b class="text-[19px] font-bold leading-tight tracking-tight">{{ formatPrice(price.amount, '') }}</b>
        <span v-if="siteCurrency" class="text-[11px] font-semibold text-muted-foreground">{{ siteCurrency }}</span>
      </span>
    </div>

    <button
      type="button"
      class="prow-buy inline-flex h-9 shrink-0 items-center justify-center gap-1.5 rounded-full bg-foreground text-[13px] font-semibold text-background disabled:pointer-events-none disabled:bg-secondary disabled:text-muted-foreground max-md:w-9"
      :aria-label="t('products.quickBuyAria')"
      :disabled="soldOut"
      @click.stop="$emit('quickBuy', product)"
      @keydown.enter.stop
    >
      <ShoppingCart v-if="!soldOut" class="h-4 w-4 md:h-3.5 md:w-3.5" />
      <span class="max-md:hidden">{{ soldOut ? t('quickBuy.soldOut') : t('products.quickBuy') }}</span>
    </button>
  </article>

  <!-- 无边框卡片：浅色托盘上浮着一块封面。lead 为分类首位的大卡 -->
  <article
    v-else
    class="pcard theme-float group relative flex h-full cursor-pointer flex-col rounded-[22px] bg-card p-1.5 text-card-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
    :class="[lead && 'col-span-2 max-md:flex-row md:row-span-2 md:p-2', soldOut && 'opacity-60']"
    :style="{ '--h': hue }"
    role="link"
    tabindex="0"
    @click="$emit('click', product.slug)"
    @keydown.enter="$emit('click', product.slug)"
  >
    <div
      class="relative grid shrink-0 place-items-center overflow-hidden rounded-2xl"
      :class="[
        lead ? 'max-md:aspect-square max-md:w-[42%] md:min-h-32 md:flex-1' : 'aspect-[16/10]',
        hasImage ? 'bg-muted' : 'bg-[hsl(var(--h)_62%_94%)] dark:bg-[hsl(var(--h)_32%_17%)]',
        soldOut && 'grayscale',
      ]"
    >
      <template v-if="hasImage">
        <!-- 同一张图放大模糊，给托盘染上商品自己的色调 -->
        <img :src="displayImageSrc" alt="" aria-hidden="true" loading="lazy" decoding="async"
          class="absolute inset-0 h-full w-full scale-150 object-cover opacity-50 blur-2xl saturate-150" />
        <!-- 绝对定位：不让图片的原始尺寸撑高卡片 -->
        <div class="absolute inset-0 flex items-center justify-center">
          <img :src="displayImageSrc" :alt="title" loading="lazy" decoding="async"
            class="pcard-tile h-[62%] w-auto max-w-[80%] rounded-xl object-cover"
            :class="lead && 'md:h-[56%] md:rounded-2xl'"
            @error="handleImageError" />
        </div>
      </template>
      <span v-else role="img" :aria-label="title"
        class="font-extrabold tracking-tighter text-[hsl(var(--h)_55%_38%)] dark:text-[hsl(var(--h)_55%_76%)]"
        :class="lead ? 'text-6xl md:text-8xl' : 'text-[44px]'">
        {{ monogram }}
      </span>

      <span v-if="flag" class="pcard-flag absolute left-2 top-2" :class="flag.class">{{ flag.label }}</span>
    </div>

    <div class="flex min-w-0 flex-1 flex-col px-2.5 pb-2 pt-3" :class="lead && 'md:flex-none md:px-3.5 md:pb-3 md:pt-4'">
      <h3 class="line-clamp-2 text-sm font-semibold leading-snug" :class="lead && 'max-md:text-base md:line-clamp-1 md:text-xl md:tracking-tight'">
        {{ title }}
      </h3>
      <p v-if="lead && description" class="mt-1 hidden truncate text-[13px] text-muted-foreground md:block">
        {{ description }}
      </p>
      <div class="mt-1 flex flex-wrap items-center justify-between gap-x-2 text-xs text-muted-foreground">
        <span class="inline-flex min-w-0 items-center gap-1">
          <Zap v-if="product.fulfillment_type === 'auto'" class="h-3 w-3 shrink-0 fill-current text-warm" aria-hidden="true" />
          <span class="shrink-0">{{ getFulfillmentTypeLabel(product.fulfillment_type) }}</span>
          <span v-for="(tag, tagIndex) in (product.tags ?? []).slice(0, maxTags)" :key="tagIndex"
            class="min-w-0 truncate rounded bg-secondary px-1.5 leading-5">
            {{ tag }}
          </span>
        </span>
        <span v-if="product.stock_status === 'low_stock'" class="shrink-0 font-medium text-warning">
          {{ getStockStatusLabel(product) }}
        </span>
      </div>

      <div class="mt-auto flex items-end justify-between gap-2 pt-3">
        <div class="flex min-w-0 flex-col tabular-nums">
          <span v-if="price.discounted" class="theme-price-original truncate text-[11px] leading-tight"
            :aria-label="t('products.originalPriceAria', { price: formatPrice(product.price_amount, siteCurrency) })">
            {{ formatPrice(product.price_amount, siteCurrency) }}
          </span>
          <span class="flex items-baseline gap-1 whitespace-nowrap" :class="price.class" :aria-label="price.aria">
            <b class="text-[19px] font-bold leading-tight tracking-tight" :class="lead && 'md:text-[28px]'">{{ formatPrice(price.amount, '') }}</b>
            <span v-if="siteCurrency" class="text-[11px] font-semibold text-muted-foreground" :class="lead && 'md:text-[13px]'">{{ siteCurrency }}</span>
          </span>
        </div>
        <button
          v-if="lead"
          type="button"
          class="h-8 shrink-0 whitespace-nowrap rounded-full bg-foreground px-3 text-[13px] font-semibold text-background transition-opacity hover:opacity-85 disabled:opacity-40 md:h-10 md:px-5 md:text-sm"
          :disabled="soldOut"
          @click.stop="$emit('quickBuy', product)"
          @keydown.enter.stop
        >
          {{ t('quickBuy.buyNow') }}
        </button>
        <button
          v-else
          type="button"
          class="pcard-cart grid h-[34px] w-[34px] shrink-0 place-items-center rounded-full bg-secondary text-foreground transition-colors hover:bg-foreground hover:text-background disabled:pointer-events-none"
          :aria-label="t('products.quickBuyAria')"
          :disabled="soldOut"
          @click.stop="$emit('quickBuy', product)"
          @keydown.enter.stop
        >
          <ShoppingCart class="h-4 w-4" />
        </button>
      </div>
    </div>
  </article>
</template>

<script lang="ts">
/** 列表模式的列：商品 | 交付方式 | 库存 | 价格 | 购买。表头和每一行共用，才能对齐 */
export const rowColsClass = 'md:grid md:grid-cols-[minmax(0,1fr)_6.5rem_8.5rem_8rem_5.25rem]'
</script>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { computed, ref, watch } from 'vue'
import { ShoppingCart, Zap } from 'lucide-vue-next'
import { getFirstImageUrl, getImageUrl } from '../utils/image'
import { useLocalized, useProductLabels } from '../composables/useProduct'

const props = withDefaults(defineProps<{
  product: any
  maxTags?: number
  /** 分类首位的大卡：占 2 列 2 行，带描述和购买按钮 */
  lead?: boolean
  /** 列表模式的表格行，需放在使用 rowColsClass 表头的容器里 */
  row?: boolean
}>(), {
  maxTags: 2,
  lead: false,
  row: false,
})

defineEmits<{
  click: [slug: string]
  quickBuy: [product: any]
}>()

const { t } = useI18n()
const { getLocalizedText, siteCurrency, formatPrice } = useLocalized()
const { getFulfillmentTypeLabel, getStockStatusLabel, isSoldOut, hasPromotionPrice, getPromotionPriceAmount, hasPromotionRules, hasWholesalePrices, getCardExclusivePrice } = useProductLabels()
const exclusivePrice = computed(() => getCardExclusivePrice(props.product))

const title = computed(() => getLocalizedText(props.product.title))
const soldOut = computed(() => isSoldOut(props.product))
const description = computed(() => getLocalizedText(props.product.description))

const price = computed(() => {
  const p = props.product
  if (exclusivePrice.value !== null) {
    return { amount: exclusivePrice.value, discounted: true, class: 'text-amber-600 dark:text-amber-300', aria: undefined }
  }
  if (hasPromotionPrice(p)) {
    const amount = getPromotionPriceAmount(p)
    return { amount, discounted: true, class: 'theme-price-promotion', aria: t('products.promotionPriceAria', { price: formatPrice(amount, siteCurrency.value) }) }
  }
  return { amount: p.price_amount, discounted: false, class: soldOut.value ? 'text-muted-foreground' : '', aria: t('products.priceAria', { price: formatPrice(p.price_amount, siteCurrency.value) }) }
})

// 封面左上角只放一个最重要的优惠标记
const deal = computed(() => {
  const p = props.product
  if (exclusivePrice.value !== null) return { label: t('products.memberPriceTag'), class: 'bg-amber-400 text-black' }
  if (hasPromotionPrice(p)) return { label: t('products.promotionTag'), class: 'bg-danger text-white' }
  if (hasWholesalePrices(p)) return { label: t('products.wholesaleTag'), class: 'bg-success text-white' }
  if (hasPromotionRules(p)) return { label: t('products.promotionBadge'), class: 'bg-warm text-white' }
  return null
})

const flag = computed(() => soldOut.value
  ? { label: t('products.stockStatus.outOfStock'), class: 'bg-foreground text-background' }
  : deal.value)

const stock = computed(() => ({
  low_stock: { text: 'font-medium text-warning', dot: 'bg-warning' },
  out_of_stock: { text: 'text-muted-foreground', dot: 'bg-muted-foreground/50' },
} as Record<string, { text: string; dot: string }>)[props.product?.stock_status] ?? { text: 'text-muted-foreground', dot: 'bg-success' })

// 无图时用标题首字，托盘和字的色相由商品 id 派生
const monogram = computed(() => Array.from(title.value.trim())[0] ?? '')
const hue = computed(() => ((Number(props.product?.id) || 0) * 47) % 360)

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
.pcard {
  transition: transform 250ms var(--ui-ease-out), box-shadow 250ms;
}

.pcard-tile {
  box-shadow: 0 10px 20px -10px rgba(0, 0, 0, 0.4);
  transition: transform 350ms var(--ui-ease-out);
}

@media (hover: hover) {
  .pcard:hover {
    transform: translateY(-3px);
    box-shadow: 0 2px 4px rgba(0, 0, 0, 0.05), 0 24px 44px -18px rgba(0, 0, 0, 0.26);
  }
  .dark .pcard:hover {
    box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.16), 0 24px 44px -18px rgba(0, 0, 0, 0.8);
  }
  .pcard:hover .pcard-tile {
    transform: scale(1.06) rotate(-2deg);
  }
  .pcard:hover .pcard-cart:not(:disabled) {
    background: var(--ui-text-primary);
    color: var(--ui-bg-page);
  }
}

.prow {
  transition: background-color 200ms, box-shadow 200ms, transform 200ms var(--ui-ease-out);
}
@media (hover: hover) {
  .prow:hover {
    background: var(--color-card);
    transform: translateY(-1px);
    box-shadow: 0 1px 2px rgba(0, 0, 0, 0.05), 0 14px 28px -16px rgba(0, 0, 0, 0.22);
  }
  .dark .prow:hover {
    background: var(--color-secondary);
    box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.12);
  }
}
.prow-buy {
  transition: transform 200ms var(--ui-ease-out);
}
@media (hover: hover) {
  .prow:hover .prow-buy:not(:disabled) {
    transform: scale(1.06);
  }
  .prow:hover .pcard-tile {
    transform: scale(1.1) rotate(-3deg);
  }
}

.pcard-flag {
  border-radius: 999px;
  padding: 0 0.5rem;
  font-size: 11px;
  font-weight: 700;
  line-height: 22px;
}

@media (prefers-reduced-motion: reduce) {
  .pcard,
  .pcard-tile {
    transition: none;
  }
}
</style>
