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
      <span class="inline-flex w-fit items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-semibold" :class="stockPill.tone">
        <component :is="stockPill.icon" class="h-3 w-3" />
        {{ stockPill.label }}
      </span>
    </div>

    <!-- Price + actions -->
    <div class="flex flex-none items-center gap-2 sm:gap-3">
      <div class="text-right">
        <template v-if="exclusivePrice !== null">
          <span class="block text-sm font-extrabold tabular-nums text-[color:var(--gold-strong)] sm:text-base">{{ formatPrice(exclusivePrice, siteCurrency) }}</span>
          <span class="block text-[11px] font-semibold text-muted-foreground line-through">{{ formatPrice(product.price_amount, siteCurrency) }}</span>
        </template>
        <template v-else-if="promo">
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
import { AlarmClock, ChevronRight, Package, ShoppingCart, XCircle, Zap } from 'lucide-vue-next'
import { getFirstImageUrl, getImageUrl } from '../../../utils/image'
import { useLocalized, useProductLabels } from '../../../composables/useProduct'

const props = withDefaults(defineProps<{ product: any; index?: number }>(), { index: 0 })

defineEmits<{ quickBuy: [product: any] }>()

const { t } = useI18n()
const { getLocalizedText, siteCurrency, formatPrice } = useLocalized()
const { getStockStatusLabel, isSoldOut, hasPromotionPrice, getPromotionPriceAmount, getCardExclusivePrice } = useProductLabels()

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
const exclusivePrice = computed(() => getCardExclusivePrice(props.product))

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
</script>
