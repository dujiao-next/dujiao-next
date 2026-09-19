<template>
  <Card
    class="group relative overflow-hidden flex flex-col h-full rounded-2xl transition-all theme-slide-up"
    :class="isSoldOut(product)
      ? 'cursor-default opacity-85 grayscale-[0.25] saturate-50 border-destructive/30'
      : 'cursor-pointer hover:-translate-y-1 hover:border-primary/30 hover:shadow-lg'"
    :style="{ animationDelay: `${index * animationStep}ms` }"
    @click="$emit('click', product.slug)">
    <!-- Image Area -->
    <div class="aspect-[4/3] overflow-hidden bg-muted relative shrink-0">
      <div
        class="absolute inset-0 z-10 transition-colors duration-300"
        :class="isSoldOut(product) ? 'bg-black/15' : 'bg-black/15 group-hover:bg-black/5'"
      ></div>
      <img v-if="displayImageSrc && !imageErrored" :src="displayImageSrc"
        :alt="getLocalizedText(product.title)" loading="lazy" decoding="async"
        class="w-full h-full object-cover transform transition-transform duration-700 ease-out"
        :class="[
          isSoldOut(product) ? 'grayscale brightness-75' : 'group-hover:scale-105',
        ]"
        @error="handleImageError" />
      <div v-else class="w-full h-full flex items-center justify-center text-muted-foreground" role="img"
        :aria-label="getLocalizedText(product.title)">
        <ImageIcon class="w-8 h-8 md:w-12 md:h-12" :stroke-width="1.5" aria-hidden="true" />
      </div>

      <div v-if="isSoldOut(product)" class="absolute inset-0 z-20 bg-black/45"></div>
      <Badge
        v-if="isSoldOut(product)"
        variant="destructive"
        size="xs"
        class="absolute left-2 top-2 md:left-4 md:top-4 z-30 tracking-wider shadow-sm"
      >
        {{ t('products.stockStatus.outOfStock') }}
      </Badge>

      <!-- Tags -->
      <div v-if="!isSoldOut(product) && product.tags && product.tags.length > 0"
        class="absolute top-2 right-2 md:top-4 md:right-4 z-20 flex flex-wrap gap-1 md:gap-2 justify-end">
        <span v-for="(tag, tagIndex) in product.tags.slice(0, maxTags)" :key="tagIndex"
          class="inline-flex items-center rounded-md border border-white/25 bg-black/55 px-2 md:px-3 py-0.5 md:py-1 text-xs font-medium text-white backdrop-blur-sm">
          {{ tag }}
        </span>
      </div>
    </div>

    <!-- Content Area -->
    <div class="p-3 md:p-4 relative z-20 flex flex-col flex-1">
      <div v-if="product.category?.name" class="text-xs text-muted-foreground uppercase tracking-wider mb-1 md:mb-2 truncate">
        {{ t('products.categoryLabel') }} · {{ getLocalizedText(product.category.name) }}
      </div>
      <h3 class="text-sm md:text-lg font-bold text-foreground mb-1 md:mb-2 transition-colors line-clamp-1">
        {{ getLocalizedText(product.title) }}
      </h3>

      <!-- Badges -->
      <div class="mb-2 md:mb-3 flex flex-wrap items-center gap-1 md:gap-2">
        <!-- Mobile: show only fulfillment type badge -->
        <Badge
          class="md:hidden"
          size="xs"
          :variant="product.fulfillment_type === 'auto' ? 'info' : 'neutral'"
        >
          {{ getFulfillmentTypeLabel(product.fulfillment_type) }}
        </Badge>

        <!-- Desktop: show all badges -->
        <Badge
          class="hidden md:inline-flex"
          size="xs"
          :variant="product.purchase_type === 'guest' ? 'warning' : 'success'"
        >
          <UserPlus v-if="product.purchase_type === 'guest'" class="h-3 w-3" />
          <Lock v-else class="h-3 w-3" />
          {{ getPurchaseTypeLabel(product.purchase_type) }}
        </Badge>

        <Badge
          class="hidden md:inline-flex"
          size="xs"
          :variant="product.fulfillment_type === 'auto' ? 'info' : 'neutral'"
        >
          <Zap v-if="product.fulfillment_type === 'auto'" class="h-3 w-3" />
          <Pencil v-else class="h-3 w-3" />
          {{ getFulfillmentTypeLabel(product.fulfillment_type) }}
        </Badge>

        <Badge class="hidden md:inline-flex" size="xs" :variant="getStockBadgeVariant(product.stock_status)">
          {{ getStockStatusLabel(product) }}
        </Badge>
      </div>

      <p class="hidden md:block text-muted-foreground text-sm mb-6 line-clamp-2">
        {{ getLocalizedText(product.description) }}
      </p>

      <div class="flex items-center justify-between border-t pt-2 md:pt-4 mt-auto">
        <div class="flex flex-col">
          <span class="hidden md:block text-xs text-muted-foreground uppercase tracking-wider">{{ t('products.price') }}</span>
          <span
            v-if="hasPromotionPrice(product)"
            class="theme-price-sm theme-price-promotion"
            :aria-label="t('products.promotionPriceAria', { price: formatPrice(getPromotionPriceAmount(product), siteCurrency) })"
          >
            {{ formatPrice(getPromotionPriceAmount(product), siteCurrency) }}
          </span>
          <span
            v-else
            class="theme-price-sm"
            :aria-label="t('products.priceAria', { price: formatPrice(product.price_amount, siteCurrency) })"
          >
            {{ formatPrice(product.price_amount, siteCurrency) }}
          </span>
          <div v-if="hasPromotionPrice(product)" class="mt-0.5 flex flex-wrap items-center gap-1.5">
            <span
              class="hidden md:inline text-xs text-muted-foreground opacity-80 line-through"
              :aria-label="t('products.originalPriceAria', { price: formatPrice(product.price_amount, siteCurrency) })"
            >{{ formatPrice(product.price_amount, siteCurrency) }}</span>
            <Badge variant="danger" size="xs">
              {{ t('products.promotionTag') }}
            </Badge>
          </div>
          <div v-else-if="hasWholesalePrices(product)" class="mt-0.5 flex flex-wrap items-center gap-1.5">
            <Badge variant="success" size="xs">
              {{ t('products.wholesaleTag') }}
            </Badge>
          </div>
          <div v-else-if="hasPromotionRules(product)" class="mt-0.5 flex flex-wrap items-center gap-1.5">
            <Badge variant="warning" size="xs">
              {{ t('products.promotionBadge') }}
            </Badge>
          </div>
        </div>

        <div class="flex items-center gap-2">
          <!-- Desktop: view details -->
          <span
            class="hidden md:flex text-xs uppercase font-bold transition-colors items-center gap-1"
            :class="isSoldOut(product)
              ? 'text-destructive/90'
              : 'text-muted-foreground group-hover:text-foreground'">
            <ArrowRight class="w-4 h-4 transition-transform" :class="isSoldOut(product) ? '' : 'group-hover:translate-x-1'" />
          </span>
          <!-- Mobile: arrow only -->
          <ChevronRight class="md:hidden w-4 h-4 text-muted-foreground" />
        </div>
      </div>
    </div>
  </Card>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { computed, ref, watch } from 'vue'
import { ArrowRight, ChevronRight, Image as ImageIcon, Lock, Pencil, UserPlus, Zap } from 'lucide-vue-next'
import { getFirstImageUrl, getImageUrl } from '../utils/image'
import { useLocalized, useProductLabels } from '../composables/useProduct'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'

const props = withDefaults(defineProps<{
  product: any
  index?: number
  maxTags?: number
  animationStep?: number
}>(), {
  index: 0,
  maxTags: 2,
  animationStep: 50,
})

defineEmits<{
  click: [slug: string]
}>()

const { t } = useI18n()
const { getLocalizedText, siteCurrency, formatPrice } = useLocalized()
const { getPurchaseTypeLabel, getFulfillmentTypeLabel, getStockBadgeVariant, getStockStatusLabel, isSoldOut, hasPromotionPrice, getPromotionPriceAmount, hasPromotionRules, hasWholesalePrices } = useProductLabels()

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
