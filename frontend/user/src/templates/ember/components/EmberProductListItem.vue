<template>
  <div
    class="group flex items-center gap-4 rounded-[var(--r-lg)] border border-border bg-card px-4 py-3.5 transition-all hover:border-[color:var(--border-strong)] hover:shadow-[var(--shadow-sm)]"
    :class="{ 'opacity-60': soldOut }"
  >
    <!-- 封面 -->
    <RouterLink
      :to="`/products/${product.slug}`"
      class="relative grid h-[62px] w-[62px] flex-none place-items-center overflow-hidden rounded-[var(--r-sm)] bg-[color:var(--surface-2)]"
    >
      <img
        v-if="coverImage"
        :src="coverImage"
        :alt="title"
        loading="lazy"
        class="h-full w-full object-cover"
        @error="imageErrored = true"
      />
      <Package v-else class="h-6 w-6 text-muted-foreground" />
    </RouterLink>

    <!-- 主体 -->
    <div class="min-w-0 flex-1">
      <div class="flex flex-wrap items-center gap-2">
        <RouterLink
          class="min-w-0 truncate text-[14.5px] font-medium transition-colors hover:text-[color:var(--accent)]"
          :to="`/products/${product.slug}`"
        >{{ title }}</RouterLink>
        <span v-if="soldOut" class="ember-pill ember-pill--danger">{{ t('products.stockStatus.outOfStock') }}</span>
        <span v-else-if="product.stock_status === 'low_stock'" class="ember-pill ember-pill--warning">
          {{ getStockStatusLabel(product) }}
        </span>
      </div>

      <div class="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-[12px] text-muted-foreground">
        <span v-if="categoryName">{{ categoryName }}</span>
        <span v-if="product.fulfillment_type">{{ getFulfillmentTypeLabel(product.fulfillment_type) }}</span>
        <span v-if="visibleTags.length">{{ visibleTags.join(' · ') }}</span>
      </div>
    </div>

    <!-- 价格与操作 -->
    <div class="flex flex-none items-center gap-3">
      <div class="flex flex-col items-end">
        <template v-if="promo">
          <span class="text-[16px] font-semibold tabular-nums tracking-[-0.02em] text-[color:var(--accent-strong)]">
            {{ formatPrice(getPromotionPriceAmount(product), siteCurrency) }}
          </span>
          <span class="text-[11.5px] text-muted-foreground line-through">
            {{ formatPrice(product.price_amount, siteCurrency) }}
          </span>
        </template>
        <span v-else class="text-[16px] font-semibold tabular-nums tracking-[-0.02em] text-foreground">
          {{ formatPrice(product.price_amount, siteCurrency) }}
        </span>
      </div>

      <button
        v-if="!soldOut"
        type="button"
        class="flex flex-none items-center gap-1.5 rounded-full bg-[color:var(--accent)] px-3.5 py-2 text-[12.5px] font-medium text-white transition-colors hover:bg-[color:var(--accent-strong)]"
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
