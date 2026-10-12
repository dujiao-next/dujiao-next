<template>
  <RouterLink
    :to="`/products/${product.slug}`"
    class="group flex min-w-0 items-center gap-3 rounded-[10px] border-2 border-border bg-card p-3 text-left shadow-[var(--luna-shadow-sm)] transition hover:-translate-y-0.5 hover:border-primary/45 sm:p-3.5"
    :class="{ 'opacity-70': soldOut }"
  >
    <!-- 缩略图 -->
    <span class="relative grid h-[72px] w-[72px] flex-none place-items-center overflow-hidden rounded-[10px] bg-primary/10 text-primary">
      <img v-if="coverImage" :src="coverImage" :alt="title" loading="lazy" class="absolute inset-0 h-full w-full object-cover" @error="imageErrored = true" />
      <Package v-else class="h-7 w-7" />
    </span>

    <div class="flex min-w-0 flex-1 flex-col gap-1.5">
      <h3 class="line-clamp-2 break-words text-sm font-medium leading-snug text-foreground">{{ title }}</h3>

      <div class="flex flex-wrap items-baseline gap-x-1.5 gap-y-0.5">
        <template v-if="promo">
          <span class="text-base font-bold tabular-nums text-destructive">{{ formatPrice(getPromotionPriceAmount(product), siteCurrency) }}</span>
          <span class="text-xs tabular-nums text-muted-foreground line-through">{{ formatPrice(product.price_amount, siteCurrency) }}</span>
        </template>
        <span v-else class="text-base font-bold tabular-nums text-primary">{{ formatPrice(product.price_amount, siteCurrency) }}</span>
        <span v-if="priceSignal" class="rounded px-1.5 py-px text-[11px] font-medium" :class="priceSignal.tone">{{ priceSignal.label }}</span>
      </div>

      <div class="flex flex-wrap items-center gap-1.5">
        <span class="inline-flex items-center gap-1 rounded px-1.5 py-px text-[11px] font-medium" :class="stockPill.tone">
          <component :is="stockPill.icon" class="h-3 w-3" />
          {{ stockPill.label }}
        </span>
        <span class="rounded px-1.5 py-px text-[11px] font-medium" :class="product.fulfillment_type === 'auto' ? 'bg-success/15 text-success' : 'bg-warning/15 text-warning'">{{ getFulfillmentTypeLabel(product.fulfillment_type) }}</span>
        <!-- 商家标签 -->
        <template v-if="!soldOut && product.tags && product.tags.length">
          <span v-for="(tag, i) in product.tags.slice(0, 2)" :key="i" class="max-w-full truncate rounded bg-destructive/10 px-1.5 py-px text-[11px] font-medium text-destructive">{{ tag }}</span>
        </template>
      </div>
    </div>

    <button
      v-if="!soldOut"
      type="button"
      class="luna-gradient inline-flex h-8 flex-none items-center rounded-full px-3.5 text-[13px] font-semibold transition hover:brightness-105"
      :aria-label="t('products.quickBuyAria')"
      @click.prevent.stop="$emit('quickBuy', product)"
    >{{ t('products.quickBuy') }}</button>
  </RouterLink>
</template>

<script setup lang="ts">
import { computed, ref, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import { AlarmClock, CircleCheck, Package, XCircle } from 'lucide-vue-next'
import { getFirstImageUrl, getImageUrl } from '../../../utils/image'
import { useLocalized, useProductLabels } from '../../../composables/useProduct'

const props = defineProps<{ product: any }>()

defineEmits<{ quickBuy: [product: any] }>()

const { t } = useI18n()
const { getLocalizedText, siteCurrency, formatPrice } = useLocalized()
const {
  getStockStatusLabel, getFulfillmentTypeLabel,
  isSoldOut, hasPromotionPrice, getPromotionPriceAmount, hasWholesalePrices, hasPromotionRules,
} = useProductLabels()

const title = computed(() => getLocalizedText(props.product?.title))
const soldOut = computed(() => isSoldOut(props.product))
const promo = computed(() => hasPromotionPrice(props.product))

const imageErrored = ref(false)
const coverImage = computed(() => {
  if (imageErrored.value) return ''
  const primary = getFirstImageUrl(props.product?.images)
  if (primary) return primary
  const icon = props.product?.category?.icon
  return icon ? getImageUrl(icon) : ''
})

const stockPill = computed<{ tone: string; icon: Component; label: string }>(() => {
  if (soldOut.value) {
    return { tone: 'bg-secondary text-muted-foreground', icon: XCircle, label: t('products.stockStatus.outOfStock') }
  }
  if (props.product?.stock_status === 'low_stock') {
    return { tone: 'bg-warning/15 text-warning', icon: AlarmClock, label: getStockStatusLabel(props.product) }
  }
  return { tone: 'bg-primary/10 text-primary', icon: CircleCheck, label: getStockStatusLabel(props.product) }
})

// 价签徽章：促销 / 批发 / 活动（择一，对齐 classic 优先级）
const priceSignal = computed<{ tone: string; label: string } | null>(() => {
  if (promo.value) return { tone: 'bg-destructive/10 text-destructive', label: t('products.promotionTag') }
  if (hasWholesalePrices(props.product)) return { tone: 'bg-destructive/10 text-destructive', label: t('products.wholesaleTag') }
  if (hasPromotionRules(props.product)) return { tone: 'bg-destructive/10 text-destructive', label: t('products.promotionBadge') }
  return null
})
</script>
