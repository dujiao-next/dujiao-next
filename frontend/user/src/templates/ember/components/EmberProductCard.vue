<template>
  <RouterLink
    :to="`/products/${product.slug}`"
    class="ember-card group h-full"
    :class="{ 'opacity-60': soldOut }"
  >
    <!-- 封面 -->
    <div class="ember-card__media">
      <img
        v-if="coverImage"
        :src="coverImage"
        :alt="title"
        loading="lazy"
        @error="imageErrored = true"
      />
      <span v-else class="grid h-full w-full place-items-center text-muted-foreground">
        <Package class="h-9 w-9" />
      </span>

      <!-- 标签浮层 -->
      <div v-if="!soldOut && visibleTags.length" class="absolute left-3 top-3 flex max-w-[calc(100%-24px)] flex-wrap gap-1.5">
        <span
          v-for="(tag, i) in visibleTags"
          :key="i"
          class="max-w-full truncate rounded-full bg-black/55 px-2.5 py-[3px] text-[11px] font-medium text-white backdrop-blur-sm"
        >{{ tag }}</span>
      </div>

      <span
        v-if="soldOut"
        class="absolute right-3 top-3 rounded-full bg-black/55 px-2.5 py-[3px] text-[11px] font-medium text-white backdrop-blur-sm"
      >{{ t('products.stockStatus.outOfStock') }}</span>
    </div>

    <!-- 正文 -->
    <div class="flex min-w-0 flex-1 flex-col gap-1.5 px-4 pt-4">
      <span v-if="categoryName" class="truncate text-[12px] text-muted-foreground">{{ categoryName }}</span>
      <h3 class="line-clamp-2 text-[15px] font-semibold leading-[1.4] tracking-[-0.015em]">{{ title }}</h3>
      <p v-if="summary" class="line-clamp-2 text-[13px] leading-[1.55] text-muted-foreground">{{ summary }}</p>
    </div>

    <!-- 价格 + 操作 -->
    <div class="mt-auto flex items-end justify-between gap-3 px-4 pb-4 pt-4">
      <div class="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-0.5">
        <template v-if="promo">
          <span class="text-[17px] font-semibold tabular-nums tracking-[-0.03em] text-[color:var(--accent-strong)]">
            {{ formatPrice(getPromotionPriceAmount(product), siteCurrency) }}
          </span>
          <span class="text-[12.5px] text-muted-foreground line-through">
            {{ formatPrice(product.price_amount, siteCurrency) }}
          </span>
        </template>
        <span v-else class="text-[17px] font-semibold tabular-nums tracking-[-0.03em] text-foreground">
          {{ formatPrice(product.price_amount, siteCurrency) }}
        </span>
      </div>

      <span
        v-if="soldOut"
        class="flex-none rounded-full border border-[color:var(--border-strong)] px-3 py-1.5 text-[12px] text-muted-foreground"
      >{{ t('products.stockStatus.outOfStock') }}</span>

      <button
        v-else-if="quickBuyEnabled"
        type="button"
        class="flex flex-none items-center gap-1.5 rounded-full bg-[color:var(--accent)] px-3.5 py-1.5 text-[12.5px] font-medium text-white transition-colors hover:bg-[color:var(--accent-strong)]"
        :aria-label="t('products.quickBuyAria')"
        @click.prevent.stop="$emit('quickBuy', product)"
      >
        <Zap class="h-3.5 w-3.5" />
        {{ t('products.quickBuy') }}
      </button>

      <ChevronRight v-else class="h-4 w-4 flex-none text-muted-foreground transition-transform duration-200 group-hover:translate-x-0.5" />
    </div>
  </RouterLink>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ChevronRight, Package, Zap } from 'lucide-vue-next'
import { getFirstImageUrl, getImageUrl } from '../../../utils/image'
import { useLocalized, useProductLabels } from '../../../composables/useProduct'

const props = withDefaults(
  defineProps<{ product: any; quickBuyEnabled?: boolean }>(),
  { quickBuyEnabled: false },
)

defineEmits<{ quickBuy: [product: any] }>()

const { t } = useI18n()
const { getLocalizedText, siteCurrency, formatPrice } = useLocalized()
const { isSoldOut, hasPromotionPrice, getPromotionPriceAmount } = useProductLabels()

const title = computed(() => getLocalizedText(props.product?.title))
const summary = computed(() => getLocalizedText(props.product?.description))
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
