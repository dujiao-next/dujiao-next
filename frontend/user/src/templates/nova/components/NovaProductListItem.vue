<template>
  <!--
    商品列表项（列表视图）
    参考站点列表的构成：较大的方形封面 + 主标题行 + 一行灰色元信息 + 右侧价格与行动点，
    整行靠 1px 暖灰边框分隔，不用阴影。
  -->
  <div
    class="group flex items-center gap-4 rounded-[var(--r-lg)] border border-border bg-card px-4 py-4 transition-colors duration-200 hover:border-[color:var(--border-strong)] hover:bg-[color:var(--surface-2)]"
    :class="{ 'opacity-60': soldOut }"
  >
    <!-- 封面 -->
    <RouterLink
      :to="`/products/${product.slug}`"
      class="relative grid h-[68px] w-[68px] flex-none place-items-center overflow-hidden rounded-[var(--r-sm)] bg-[color:var(--surface-2)]"
    >
      <img
        v-if="coverImage"
        :src="coverImage"
        :alt="title"
        loading="lazy"
        class="h-full w-full object-cover transition-transform duration-500 group-hover:scale-[1.05]"
        @error="imageErrored = true"
      />
      <Package v-else class="h-6 w-6 text-muted-foreground" />
    </RouterLink>

    <!-- 主体 -->
    <div class="min-w-0 flex-1">
      <div class="flex flex-wrap items-center gap-2">
        <RouterLink
          class="min-w-0 truncate text-[15px] font-medium tracking-[-0.012em] transition-colors hover:text-[color:var(--accent)]"
          :to="`/products/${product.slug}`"
        >{{ title }}</RouterLink>
        <span v-if="soldOut" class="nova-pill nova-pill--danger">{{ t('products.stockStatus.outOfStock') }}</span>
        <span v-else-if="product.stock_status === 'low_stock'" class="nova-pill nova-pill--warning">
          {{ getStockStatusLabel(product) }}
        </span>
      </div>

      <div class="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-[12px] text-[color:var(--ink-3)]">
        <span v-if="categoryName">{{ categoryName }}</span>
        <span v-if="categoryName && product.fulfillment_type" class="text-[color:var(--ink-4)]">·</span>
        <span v-if="product.fulfillment_type">{{ getFulfillmentTypeLabel(product.fulfillment_type) }}</span>
        <template v-if="visibleTags.length">
          <span class="text-[color:var(--ink-4)]">·</span>
          <span>{{ visibleTags.join(' · ') }}</span>
        </template>
      </div>
    </div>

    <!-- 价格与操作 -->
    <div class="flex flex-none items-center gap-3.5">
      <div class="flex flex-col items-end">
        <template v-if="promo">
          <span class="text-[16px] font-semibold tabular-nums tracking-[-0.022em] text-[color:var(--accent-strong)]">
            {{ formatPrice(getPromotionPriceAmount(product), siteCurrency) }}
          </span>
          <span class="text-[11.5px] text-[color:var(--ink-4)] line-through">
            {{ formatPrice(product.price_amount, siteCurrency) }}
          </span>
        </template>
        <span v-else class="text-[16px] font-semibold tabular-nums tracking-[-0.022em] text-foreground">
          {{ formatPrice(product.price_amount, siteCurrency) }}
        </span>
      </div>

      <button
        v-if="!soldOut"
        type="button"
        class="flex flex-none items-center gap-1.5 rounded-full bg-[color:var(--accent)] px-4 py-2 text-[12.5px] font-medium text-[color:var(--on-accent)] transition-colors duration-200 hover:bg-[color:var(--accent-strong)] max-[560px]:hidden"
        :aria-label="t('products.quickBuyAria')"
        @click="$emit('quickBuy', product)"
      >
        <Zap class="h-3.5 w-3.5" />
        {{ t('products.quickBuy') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Package, Zap } from 'lucide-vue-next'
import { getFirstImageUrl, getImageUrl } from '../../../utils/image'
import { useLocalized, useProductLabels } from '../../../composables/useProduct'

const props = defineProps<{ product: any }>()

defineEmits<{ quickBuy: [product: any] }>()

const { t } = useI18n()
const { getLocalizedText, siteCurrency, formatPrice } = useLocalized()
const {
  isSoldOut, hasPromotionPrice, getPromotionPriceAmount,
  getStockStatusLabel, getFulfillmentTypeLabel,
} = useProductLabels()

const title = computed(() => getLocalizedText(props.product?.title))
const categoryName = computed(() => getLocalizedText(props.product?.category?.name))
const soldOut = computed(() => isSoldOut(props.product))
const promo = computed(() => hasPromotionPrice(props.product))

const visibleTags = computed<string[]>(() => {
  const tags = props.product?.tags
  if (!Array.isArray(tags)) return []
  return tags
    .slice(0, 2)
    .map((tag: any) => (typeof tag === 'string' ? tag : getLocalizedText(tag)))
    .filter(Boolean)
})

const imageErrored = ref(false)
const coverImage = computed(() => {
  if (imageErrored.value) return ''
  const primary = getFirstImageUrl(props.product?.images)
  if (primary) return primary
  const icon = props.product?.category?.icon
  return icon ? getImageUrl(icon) : ''
})
</script>
