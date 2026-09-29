<template>
  <!--
    商品卡片（卡片视图）
    参考站点的卡片：4:3 封面、极细边框、圆角 22px、悬浮轻微上移 + 封面缓速放大。
  -->
  <article class="nova-card nova-card--hover h-full">
    <RouterLink :to="`/products/${product.slug}`" class="nova-card__media block">
      <img
        v-if="coverImage"
        :src="coverImage"
        :alt="title"
        loading="lazy"
        @error="imageErrored = true"
      />
      <span v-else class="grid h-full w-full place-items-center text-[color:var(--ink-4)]">
        <Package class="h-8 w-8" />
      </span>

      <span
        v-if="soldOut"
        class="absolute left-3 top-3 nova-pill nova-pill--solid"
      >{{ t('products.stockStatus.outOfStock') }}</span>
      <span
        v-else-if="promo"
        class="absolute left-3 top-3 nova-pill nova-pill--accent"
      >{{ t('products.promotion') }}</span>
    </RouterLink>

    <div class="flex flex-1 flex-col gap-3 p-5">
      <div class="flex min-w-0 flex-col gap-1.5">
        <RouterLink
          class="line-clamp-2 text-[15px] font-medium leading-snug tracking-[-0.012em] transition-colors hover:text-[color:var(--accent)]"
          :to="`/products/${product.slug}`"
        >{{ title }}</RouterLink>

        <div v-if="categoryName || product.fulfillment_type" class="flex flex-wrap items-center gap-x-2.5 gap-y-1 text-[12px] text-[color:var(--ink-3)]">
          <span v-if="categoryName">{{ categoryName }}</span>
          <span v-if="categoryName && product.fulfillment_type" class="text-[color:var(--ink-4)]">·</span>
          <span v-if="product.fulfillment_type">{{ getFulfillmentTypeLabel(product.fulfillment_type) }}</span>
        </div>
      </div>

      <div class="mt-auto flex items-end justify-between gap-3 pt-1">
        <div class="flex flex-col">
          <template v-if="promo">
            <span class="text-[17px] font-semibold tabular-nums tracking-[-0.022em] text-[color:var(--accent-strong)]">
              {{ formatPrice(getPromotionPriceAmount(product), siteCurrency) }}
            </span>
            <span class="text-[11.5px] text-[color:var(--ink-4)] line-through">
              {{ formatPrice(product.price_amount, siteCurrency) }}
            </span>
          </template>
          <span v-else class="text-[17px] font-semibold tabular-nums tracking-[-0.022em] text-foreground">
            {{ formatPrice(product.price_amount, siteCurrency) }}
          </span>
        </div>

        <button
          v-if="!soldOut"
          type="button"
          class="grid h-9 w-9 flex-none place-items-center rounded-full border border-border text-[color:var(--ink-2)] transition-colors duration-200 hover:border-[color:var(--accent)] hover:bg-[color:var(--accent-soft)] hover:text-[color:var(--accent-strong)]"
          :aria-label="t('products.quickBuyAria')"
          @click.prevent="$emit('quickBuy', product)"
        >
          <Zap class="h-4 w-4" />
        </button>
      </div>
    </div>
  </article>
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
  isSoldOut, hasPromotionPrice, getPromotionPriceAmount, getFulfillmentTypeLabel,
} = useProductLabels()

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
</script>
