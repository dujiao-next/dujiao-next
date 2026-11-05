<template>
  <RouterLink
    :to="`/products/${product.slug}`"
    class="group flex items-center gap-3 rounded-lg border bg-card p-2.5 transition hover:border-hairline-strong hover:shadow-[var(--shadow-sm)] sm:gap-3.5 sm:p-3"
    :class="{ 'opacity-[0.74]': soldOut }"
  >
    <!-- Thumbnail -->
    <span
      class="relative grid h-14 w-14 flex-none place-items-center overflow-hidden rounded-[11px] sm:h-16 sm:w-16"
      :class="coverClass"
    >
      <img v-if="coverImage" :src="coverImage" :alt="title" loading="lazy" class="absolute inset-0 h-full w-full object-cover" @error="imageErrored = true" />
      <Package v-else class="relative z-[1] h-6 w-6 text-white" />
      <span v-if="soldOut" class="absolute inset-0 grid place-items-center bg-black/55 text-[10px] font-bold text-white">{{ t('products.stockStatus.outOfStock') }}</span>
    </span>

    <!-- Info -->
    <div class="flex min-w-0 flex-1 flex-col gap-1">
      <div class="flex min-w-0 items-center gap-1.5">
        <span v-if="categoryName" class="hidden max-w-[88px] flex-none truncate text-[11px] font-semibold uppercase tracking-wide text-muted-foreground sm:inline">{{ categoryName }}</span>
        <span v-if="categoryName" class="hidden flex-none text-muted-foreground/40 sm:inline">·</span>
        <h3 class="truncate text-sm font-bold text-foreground">{{ title }}</h3>
      </div>
      <div class="flex flex-wrap items-center gap-1.5">
        <span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-semibold" :class="stockPill.tone">
          <component :is="stockPill.icon" class="h-3 w-3" />
          {{ stockPill.label }}
        </span>
        <span class="inline-flex items-center gap-1 rounded-full bg-[color:var(--teal-soft)] px-2 py-0.5 text-[11px] font-semibold text-[color:var(--teal-strong)]">
          <component :is="product.fulfillment_type === 'auto' ? Zap : Pencil" class="h-3 w-3" />
          {{ getFulfillmentTypeLabel(product.fulfillment_type) }}
        </span>
        <span class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-semibold" :class="product.purchase_type === 'guest' ? 'bg-[color:var(--gold-soft)] text-[color:var(--gold-strong)]' : 'bg-[color:var(--teal-soft)] text-[color:var(--teal-strong)]'">
          <component :is="product.purchase_type === 'guest' ? UserPlus : Lock" class="h-3 w-3" />
          {{ getPurchaseTypeLabel(product.purchase_type) }}
        </span>
        <span v-if="priceSignal" class="inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-semibold" :class="priceSignal.tone">{{ priceSignal.label }}</span>
        <template v-if="!soldOut && product.tags && product.tags.length">
          <span v-for="(tag, i) in product.tags.slice(0, 2)" :key="i" class="inline-flex max-w-[120px] items-center truncate rounded-full bg-secondary px-2 py-0.5 text-[11px] font-semibold text-muted-foreground">{{ tag }}</span>
        </template>
      </div>
    </div>

    <!-- Price + actions -->
    <div class="flex flex-none items-center gap-2 sm:gap-3">
      <div class="text-right">
        <template v-if="promo">
          <span class="block text-sm font-extrabold tabular-nums text-primary sm:text-base">{{ formatPrice(getPromotionPriceAmount(product), siteCurrency) }}</span>
          <span class="block text-[11px] font-semibold text-muted-foreground line-through">{{ formatPrice(product.price_amount, siteCurrency) }}</span>
        </template>
        <span v-else class="block text-sm font-extrabold tabular-nums text-foreground sm:text-base">{{ formatPrice(product.price_amount, siteCurrency) }}</span>
      </div>
      <button
        v-if="!soldOut"
        type="button"
        class="grid h-9 w-9 flex-none place-items-center rounded-full bg-primary text-white transition hover:bg-primary/90"
        :aria-label="t('products.quickBuyAria')"
        @click.prevent.stop="$emit('quickBuy', product)"
      >
        <ShoppingCart class="h-4 w-4" />
      </button>
      <ChevronRight class="hidden h-4 w-4 flex-none text-muted-foreground transition group-hover:translate-x-0.5 group-hover:text-foreground sm:block" />
    </div>
  </RouterLink>
</template>

<script setup lang="ts">
import { computed, ref, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import { AlarmClock, ChevronRight, Lock, Package, Pencil, ShoppingCart, UserPlus, XCircle, Zap } from 'lucide-vue-next'
import { getFirstImageUrl, getImageUrl } from '../../../utils/image'
import { useLocalized, useProductLabels } from '../../../composables/useProduct'

const props = withDefaults(defineProps<{ product: any; index?: number }>(), { index: 0 })

defineEmits<{ quickBuy: [product: any] }>()

const { t } = useI18n()
const { getLocalizedText, siteCurrency, formatPrice } = useLocalized()
const {
  getStockStatusLabel, getPurchaseTypeLabel, getFulfillmentTypeLabel,
  isSoldOut, hasPromotionPrice, getPromotionPriceAmount, hasWholesalePrices, hasPromotionRules,
} = useProductLabels()

const covers = [
  'bg-[linear-gradient(135deg,#7b74f2,var(--red))]',
  'bg-[linear-gradient(135deg,#1cc0bf,var(--teal))]',
  'bg-[linear-gradient(135deg,#9b6cf5,var(--plum))]',
  'bg-[linear-gradient(135deg,#f7bd4e,var(--gold))]',
  'bg-[linear-gradient(135deg,#3a3950,var(--ink))]',
]
const coverClass = computed(() => covers[(props.index ?? 0) % covers.length])

const title = computed(() => getLocalizedText(props.product?.title))
const categoryName = computed(() => getLocalizedText(props.product?.category?.name))
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
    return { tone: 'bg-[color:var(--gold-soft)] text-[color:var(--gold-strong)]', icon: AlarmClock, label: getStockStatusLabel(props.product) }
  }
  return { tone: 'bg-[color:var(--teal-soft)] text-[color:var(--teal-strong)]', icon: Zap, label: getStockStatusLabel(props.product) }
})

// 价签徽章：促销 / 批发 / 活动（与卡片模式一致）
const priceSignal = computed<{ tone: string; label: string } | null>(() => {
  if (promo.value) return { tone: 'bg-primary/10 text-primary', label: t('products.promotionTag') }
  if (hasWholesalePrices(props.product)) return { tone: 'bg-[color:var(--teal-soft)] text-[color:var(--teal-strong)]', label: t('products.wholesaleTag') }
  if (hasPromotionRules(props.product)) return { tone: 'bg-[color:var(--gold-soft)] text-[color:var(--gold-strong)]', label: t('products.promotionBadge') }
  return null
})
</script>
