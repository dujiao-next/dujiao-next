<template>
  <Teleport to="body">
    <!-- Overlay -->
    <Transition
      appear
      enter-active-class="transition duration-300 ease-out"
      enter-from-class="opacity-0"
      enter-to-class="opacity-100"
      leave-active-class="transition duration-200 ease-in"
      leave-from-class="opacity-100"
      leave-to-class="opacity-0"
    >
      <div
        v-if="visible"
        class="fixed inset-0 z-50 bg-black/40 backdrop-blur-[2px]"
        @click="close"
      />
    </Transition>

    <!--
      Responsive panel:
      - Mobile (<md): bottom sheet, slides up
      - Desktop (md+): centered modal, fades + scales in
    -->
    <Transition
      appear
      enter-active-class="quick-buy-enter-active"
      enter-from-class="quick-buy-enter-from"
      enter-to-class="quick-buy-enter-to"
      leave-active-class="quick-buy-leave-active"
      leave-from-class="quick-buy-leave-from"
      leave-to-class="quick-buy-leave-to"
    >
      <div
        v-if="visible"
        class="fixed z-50 inset-x-0 bottom-0 md:inset-0 md:flex md:items-center md:justify-center md:p-6"
        @click.self="close"
      >
        <div
          role="dialog"
          aria-modal="true"
          class="
            w-full max-h-[85vh] flex flex-col
            rounded-t-2xl md:rounded-2xl
            bg-card/95 backdrop-blur-xl text-card-foreground border-t md:border
            shadow-2xl
            md:max-w-md md:max-h-[75vh]
          "
          @click.stop
        >
          <!-- Mobile drag handle -->
          <div class="md:hidden flex justify-center pt-2.5 pb-1 shrink-0">
            <div class="w-8 h-1 rounded-full bg-gray-300 dark:bg-gray-600" />
          </div>

          <!-- Desktop header bar -->
          <div class="hidden md:flex items-center justify-between px-5 pt-4 pb-0 shrink-0">
            <h2 class="text-sm font-semibold text-foreground">{{ t('quickBuy.title') }}</h2>
            <Button type="button" variant="ghost" size="icon" class="-mr-1 text-muted-foreground" @click="close">
              <X />
            </Button>
          </div>

          <!-- Scrollable content area -->
          <div class="overflow-y-auto overscroll-contain flex-1 px-4 md:px-5 md:pt-4">
            <!-- Product header -->
            <div class="flex gap-3.5 mb-4">
              <!-- Image -->
              <div
                class="w-[90px] h-[90px] md:w-[100px] md:h-[100px] rounded-xl md:rounded-2xl overflow-hidden shrink-0 border cursor-pointer"
                @click="goToDetail"
              >
                <img
                  v-if="productImage"
                  :src="productImage"
                  :alt="productTitle"
                  class="w-full h-full object-cover"
                />
                <div v-else class="w-full h-full flex items-center justify-center bg-muted">
                  <ImageIcon class="w-7 h-7 text-muted-foreground" :stroke-width="1.5" />
                </div>
              </div>

              <!-- Info -->
              <div class="flex-1 min-w-0 flex flex-col">
                <h3
                  class="text-sm md:text-[15px] font-semibold text-foreground line-clamp-2 leading-snug cursor-pointer hover:underline decoration-gray-300 dark:decoration-gray-600 underline-offset-2"
                  @click="goToDetail"
                >
                  {{ productTitle }}
                </h3>

                <div class="mt-1.5 flex flex-wrap items-center gap-1">
                  <Badge :variant="product.fulfillment_type === 'auto' ? 'info' : 'neutral'" size="xs">
                    {{ getFulfillmentTypeLabel(product.fulfillment_type) }}
                  </Badge>
                  <Badge :variant="getStockBadgeVariant(product.stock_status)" size="xs">
                    {{ getStockStatusLabel(product) }}
                  </Badge>
                  <Badge v-if="hasSelectedSkuWholesalePrice" variant="success" size="xs">
                    {{ t('products.wholesaleTag') }}
                  </Badge>
                  <Badge v-if="showSelectedSkuMemberBadge" variant="warning" size="xs">
                    {{ t('products.memberPriceTag') }}
                  </Badge>
                </div>

                <!-- Price -->
                <div class="mt-auto pt-1">
                  <template v-if="selectedSku && hasSelectedSkuWholesalePrice">
                    <span
                      class="text-lg md:text-xl font-bold"
                      :class="selectedSkuWholesaleFinalIsMember ? 'text-amber-600 dark:text-amber-300' : 'text-emerald-600 dark:text-emerald-400'"
                    >
                      {{ formatPrice(selectedSkuWholesaleFinalPrice!, siteCurrency) }}
                    </span>
                    <span class="ml-1.5 text-xs text-muted-foreground line-through">
                      {{ formatPrice(selectedSku.price_amount, siteCurrency) }}
                    </span>
                  </template>
                  <template v-else-if="selectedSku && hasSkuPromotionPrice(selectedSku)">
                    <span
                      class="text-lg md:text-xl font-bold"
                      :class="selectedSkuPromotionFinalIsMember ? 'text-amber-600 dark:text-amber-300' : 'text-rose-600 dark:text-rose-400'"
                    >
                      {{ formatPrice(selectedSkuPromotionFinalPrice!, siteCurrency) }}
                    </span>
                    <span class="ml-1.5 text-xs text-muted-foreground line-through">
                      {{ formatPrice(selectedSku.price_amount, siteCurrency) }}
                    </span>
                  </template>
                  <template v-else-if="selectedSku && hasMemberPrice">
                    <span class="text-lg md:text-xl font-bold text-amber-600 dark:text-amber-300">
                      {{ formatPrice(selectedSkuMemberPrice!, siteCurrency) }}
                    </span>
                    <span class="ml-1.5 text-xs text-muted-foreground line-through">
                      {{ formatPrice(selectedSku.price_amount, siteCurrency) }}
                    </span>
                  </template>
                  <template v-else-if="selectedSku">
                    <span class="text-lg md:text-xl font-bold text-primary">
                      {{ formatPrice(selectedSku.price_amount, siteCurrency) }}
                    </span>
                  </template>
                  <template v-else-if="hasPromotionPrice(product)">
                    <span class="text-lg md:text-xl font-bold text-rose-600 dark:text-rose-400">
                      {{ formatPrice(getPromotionPriceAmount(product), siteCurrency) }}
                    </span>
                    <span class="ml-1.5 text-xs text-muted-foreground line-through">
                      {{ formatPrice(product.price_amount, siteCurrency) }}
                    </span>
                  </template>
                  <template v-else>
                    <span class="text-lg md:text-xl font-bold text-primary">
                      {{ formatPrice(product.price_amount, siteCurrency) }}
                    </span>
                  </template>
                </div>
              </div>

              <!-- Mobile close -->
              <Button type="button" variant="ghost" size="icon" class="md:hidden self-start -mr-1 text-muted-foreground" @click="close">
                <X />
              </Button>
            </div>

            <!-- Divider -->
            <div class="h-px bg-border -mx-4 md:-mx-5 mb-4" />

            <!-- Product description -->
            <p v-if="productDescription" class="mb-4 text-xs leading-relaxed text-muted-foreground line-clamp-3">
              {{ productDescription }}
            </p>

            <!-- Promotion rules -->
            <div v-if="selectedSkuWholesaleRules.length" class="mb-4 rounded-lg border border-emerald-200 bg-emerald-50/50 px-3 py-2 dark:border-emerald-800/50 dark:bg-emerald-950/20">
              <div class="mb-1 text-[11px] font-semibold text-emerald-700 dark:text-emerald-300">
                {{ t('products.wholesaleRulesTitle') }}
              </div>
              <div class="flex flex-wrap gap-1">
                <Badge v-for="tier in selectedSkuWholesaleRules" :key="`${tier.sku_id || tier.sku_code || 'all'}-${tier.min_quantity}`" variant="success" size="xs" class="rounded-full">
                  {{ formatWholesaleTier(tier) }}
                </Badge>
              </div>
            </div>

            <div v-if="hasPromotionRules(product)" class="mb-4 rounded-lg border border-orange-200 dark:border-orange-800/50 bg-orange-50/50 dark:bg-orange-950/20 px-3 py-2">
              <div class="flex items-center gap-1 mb-1">
                <Tag class="w-3.5 h-3.5 text-orange-500 dark:text-orange-400 shrink-0" />
                <span class="text-[11px] font-semibold text-orange-700 dark:text-orange-300">
                  {{ t('products.promotionRulesTitle') }}
                </span>
              </div>
              <ul class="space-y-0.5">
                <li v-for="rule in getPromotionRules(product)" :key="rule.id" class="text-[11px] text-orange-600 dark:text-orange-300/90 flex items-center gap-1">
                  <span class="w-1 h-1 rounded-full bg-orange-400 dark:bg-orange-500 shrink-0"></span>
                  <span>{{ formatPromotionRule(rule) }}</span>
                </li>
              </ul>
            </div>

            <!-- SKU Selection -->
            <div v-if="activeSkus.length > 1" class="mb-4">
              <div class="mb-2 text-xs font-medium text-muted-foreground">
                {{ t('quickBuy.selectSku') }}
              </div>
              <div class="flex flex-wrap gap-2">
                <button
                  v-for="sku in activeSkus"
                  :key="sku.id"
                  type="button"
                  class="rounded-lg border px-3 py-1.5 text-[13px] transition-all"
                  :class="[
                    normalizeSkuId(sku.id) === selectedSkuId
                      ? 'border-primary/45 bg-primary/10 ring-1 ring-primary/30 font-semibold'
                      : 'bg-secondary text-secondary-foreground hover:bg-secondary/80 font-medium',
                    isSkuPurchasable(sku) ? 'cursor-pointer' : 'cursor-not-allowed opacity-45 border-dashed',
                  ]"
                  :disabled="!isSkuPurchasable(sku)"
                  @click="selectedSkuId = normalizeSkuId(sku.id)"
                >
                  <span>{{ skuDisplayText(sku) }}</span>
                  <span
                    v-if="!isSkuPurchasable(sku)"
                    class="ml-1 text-[10px] opacity-70"
                  >({{ t('productDetail.skuStockOut') }})</span>
                </button>
              </div>
            </div>

            <!-- Selected SKU stock info -->
            <div v-if="selectedSku" class="mb-4 flex items-center gap-2 text-xs">
              <span class="text-muted-foreground">{{ t('quickBuy.stock') }}:</span>
              <span
                class="inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-[11px] font-medium"
                :class="skuStockBadgeClass(selectedSku)"
              >
                <span
                  class="w-1.5 h-1.5 rounded-full"
                  :class="skuStockDotClass(selectedSku)"
                />
                {{ skuStockText(selectedSku) }}
              </span>
            </div>

            <!-- Quantity -->
            <div class="mb-4 flex items-center gap-3">
              <span class="text-xs font-medium text-muted-foreground">{{ t('quickBuy.quantity') }}</span>
              <div class="flex items-center rounded-lg border overflow-hidden">
                <button
                  type="button"
                  class="w-9 h-9 flex items-center justify-center text-muted-foreground hover:bg-secondary transition-colors cursor-pointer disabled:cursor-not-allowed disabled:opacity-30"
                  :disabled="quantity <= effectiveMin"
                  @click="quantity = Math.max(effectiveMin, quantity - 1)"
                >
                  <Minus class="w-3.5 h-3.5" :stroke-width="2.5" />
                </button>
                <input
                  type="text"
                  inputmode="numeric"
                  class="w-12 h-9 text-center text-sm font-semibold text-foreground border-x bg-transparent outline-none tabular-nums [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
                  :value="quantity"
                  @change="handleQuantityInput($event)"
                  @keydown.enter.prevent="($event.target as HTMLInputElement)?.blur()"
                />
                <button
                  type="button"
                  class="w-9 h-9 flex items-center justify-center text-muted-foreground hover:bg-secondary transition-colors cursor-pointer disabled:cursor-not-allowed disabled:opacity-30"
                  :disabled="effectiveLimit !== null && quantity >= effectiveLimit"
                  @click="quantity = quantity + 1"
                >
                  <Plus class="w-3.5 h-3.5" :stroke-width="2.5" />
                </button>
              </div>
            </div>

            <!-- Warning -->
            <p v-if="stockBelowMinPurchase" class="mb-4 rounded-lg border border-warning/40 bg-warning/10 px-3 py-2 text-xs font-medium text-warning">
              {{ t('productDetail.stockBelowMinPurchase', { count: effectiveMin }) }}
            </p>
            <p v-else-if="purchaseWarning" class="mb-4 rounded-lg border border-warning/40 bg-warning/10 px-3 py-2 text-xs font-medium text-warning">
              {{ purchaseWarning }}
            </p>
          </div>

          <!-- Actions (sticky bottom) -->
          <div class="shrink-0 px-4 md:px-5 pt-3 pb-3 md:pb-5 border-t theme-safe-bottom">
            <Button
              v-if="requiresLogin"
              class="w-full py-3 h-auto min-h-[44px] rounded-xl text-sm font-semibold"
              @click="goLogin"
            >
              {{ t('quickBuy.loginToBuy') }}
            </Button>
            <div v-else-if="isSoldOut(product)" class="flex gap-3">
              <Button
                variant="secondary"
                class="flex-1 py-3 h-auto min-h-[44px] rounded-xl text-sm font-semibold"
                @click="goToDetail"
              >
                {{ t('quickBuy.viewDetail') }}
              </Button>
            </div>
            <div v-else class="flex gap-3">
              <Button
                variant="secondary"
                :disabled="!canPurchase"
                class="flex-1 py-3 h-auto min-h-[44px] rounded-xl text-sm font-semibold gap-1.5"
                @click="handleAddToCart"
              >
                <ShoppingCart class="w-4 h-4" />
                {{ t('quickBuy.addToCart') }}
              </Button>
              <Button
                :disabled="!canPurchase"
                class="flex-1 py-3 h-auto min-h-[44px] rounded-xl text-sm font-semibold"
                @click="handleBuyNow"
              >
                {{ t('quickBuy.buyNow') }}
              </Button>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>


<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '../stores/app'
import { useCartStore } from '../stores/cart'
import { useBuyNowStore } from '../stores/buyNow'
import { useUserAuthStore } from '../stores/userAuth'
import { useUserProfileStore } from '../stores/userProfile'
import { getFirstImageUrl, getImageUrl } from '../utils/image'
import { normalizeSkuId, buildSkuDisplayText } from '../utils/sku'
import { resolveSkuAvailableStock, resolveSkuStockDisplay, type PublicStockDisplay } from '../utils/publicStock'
import { useLocalized, useProductLabels } from '../composables/useProduct'
import { toast } from '../composables/useToast'
import { X, Image as ImageIcon, Tag, Minus, Plus, ShoppingCart } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'

const props = defineProps<{
  product: any
  visible: boolean
}>()

const emit = defineEmits<{
  'update:visible': [value: boolean]
}>()

const router = useRouter()
const route = useRoute()
const { t } = useI18n()
const appStore = useAppStore()
const cartStore = useCartStore()
const buyNowStore = useBuyNowStore()
const userAuthStore = useUserAuthStore()
const userProfileStore = useUserProfileStore()

const { getLocalizedText, siteCurrency, formatPrice } = useLocalized()
const {
  getFulfillmentTypeLabel,
  getStockBadgeVariant,
  getStockStatusLabel,
  isSoldOut,
  hasPromotionPrice,
  getPromotionPriceAmount,
  hasSkuPromotionPrice,
  getSkuPromotionPriceAmount,
  hasPromotionRules,
  getPromotionRules,
  getWholesalePrices,
  resolveWholesalePriceAmount,
  resolveMemberPriceAmount,
} = useProductLabels()

const selectedSkuId = ref(0)
const quantity = ref(1)
const purchaseWarning = ref('')

const productTitle = computed(() => getLocalizedText(props.product?.title))
const productDescription = computed(() => getLocalizedText(props.product?.description))

const formatPromotionRule = (rule: any) => {
  const amount = formatPrice(rule.min_amount, siteCurrency.value)
  const value = rule.type === 'percent' ? String(rule.value) : formatPrice(rule.value, siteCurrency.value)
  const hasMin = Number(rule.min_amount) > 0
  switch (rule.type) {
    case 'percent':
      return hasMin ? t('products.promotionHintPercent', { amount, value }) : t('products.promotionHintPercentNoMin', { value })
    case 'fixed':
      return hasMin ? t('products.promotionHintFixed', { amount, value }) : t('products.promotionHintFixedNoMin', { value })
    case 'special_price':
      return hasMin ? t('products.promotionHintSpecial', { amount, value }) : t('products.promotionHintSpecialNoMin', { value })
    default:
      return rule.name || ''
  }
}

const productImage = computed(() => {
  const p = props.product
  if (!p) return ''
  if (p.images) {
    const url = getFirstImageUrl(p.images)
    if (url) return url
  }
  if (p.category?.icon) return getImageUrl(p.category.icon)
  return ''
})

const activeSkus = computed(() => {
  const rows = Array.isArray(props.product?.skus) ? props.product.skus : []
  return rows.filter((sku: any) => Boolean(sku?.is_active))
})

const selectedSku = computed(() => {
  if (selectedSkuId.value <= 0) return null
  return activeSkus.value.find((sku: any) => normalizeSkuId(sku?.id) === selectedSkuId.value) || null
})

const userMemberLevelId = computed(() => Number(userAuthStore.user?.member_level_id || 0))

const currentMemberDiscountRate = computed(() => {
  if (!userMemberLevelId.value) return 0
  const level = userProfileStore.memberLevels.find((item: any) => Number(item?.id || 0) === userMemberLevelId.value)
  return Number(level?.discount_rate || 0)
})

const ensureMemberLevels = () => {
  if (userMemberLevelId.value > 0 && userProfileStore.memberLevels.length === 0) {
    void userProfileStore.loadMemberLevels()
  }
}

const getMemberPriceForSku = (skuId: number, basePrice: any): number | null => {
  const price = resolveMemberPriceAmount(props.product, skuId, basePrice, userMemberLevelId.value, currentMemberDiscountRate.value)
  return price === null ? null : Number(price)
}

const selectedSkuMemberPrice = computed(() => {
  if (!selectedSku.value) return null
  return getMemberPriceForSku(normalizeSkuId(selectedSku.value.id), selectedSku.value.price_amount)
})

const hasMemberPrice = computed(() => {
  if (!selectedSku.value || selectedSkuMemberPrice.value === null) return false
  return selectedSkuMemberPrice.value < Number(selectedSku.value.price_amount || 0)
})

const selectedSkuWholesaleRules = computed(() => {
  if (!props.product || !selectedSku.value) return []
  return getWholesalePrices(
    props.product,
    normalizeSkuId(selectedSku.value.id),
    selectedSku.value.sku_code,
  )
})

const selectedSkuWholesalePrice = computed(() => {
  if (!props.product || !selectedSku.value) return null
  return resolveWholesalePriceAmount(
    props.product,
    selectedSku.value.price_amount,
    quantity.value,
    normalizeSkuId(selectedSku.value.id),
    selectedSku.value.sku_code,
    quantity.value,
  )
})

const hasSelectedSkuWholesalePrice = computed(() => {
  if (!selectedSku.value || !selectedSkuWholesalePrice.value) return false
  const comparisonPrice = hasSkuPromotionPrice(selectedSku.value)
    ? Number(getSkuPromotionPriceAmount(selectedSku.value))
    : Number(selectedSku.value.price_amount || 0)
  return Number(selectedSkuWholesalePrice.value) < comparisonPrice
})

const selectedSkuWholesaleMemberPrice = computed(() => {
  if (!selectedSku.value || !selectedSkuWholesalePrice.value) return null
  return getMemberPriceForSku(normalizeSkuId(selectedSku.value.id), selectedSkuWholesalePrice.value)
})

const selectedSkuWholesaleFinalIsMember = computed(() => hasSelectedSkuWholesalePrice.value && selectedSkuWholesaleMemberPrice.value !== null)

const selectedSkuWholesaleFinalPrice = computed(() => {
  if (!hasSelectedSkuWholesalePrice.value || selectedSkuWholesalePrice.value === null) return null
  return selectedSkuWholesaleMemberPrice.value !== null ? selectedSkuWholesaleMemberPrice.value : selectedSkuWholesalePrice.value
})

const selectedSkuPromotionPrice = computed(() => {
  if (!selectedSku.value || !hasSkuPromotionPrice(selectedSku.value)) return null
  return getSkuPromotionPriceAmount(selectedSku.value)
})

const selectedSkuPromotionMemberPrice = computed(() => {
  if (!selectedSku.value || selectedSkuPromotionPrice.value === null) return null
  return getMemberPriceForSku(normalizeSkuId(selectedSku.value.id), selectedSkuPromotionPrice.value)
})

const selectedSkuPromotionFinalIsMember = computed(() => selectedSkuPromotionMemberPrice.value !== null)

const selectedSkuPromotionFinalPrice = computed(() => {
  if (selectedSkuPromotionPrice.value === null) return null
  return selectedSkuPromotionMemberPrice.value !== null ? selectedSkuPromotionMemberPrice.value : selectedSkuPromotionPrice.value
})

const showSelectedSkuMemberBadge = computed(() => {
  if (!selectedSku.value) return false
  if (hasSelectedSkuWholesalePrice.value) return selectedSkuWholesaleFinalIsMember.value
  if (hasSkuPromotionPrice(selectedSku.value)) return selectedSkuPromotionFinalIsMember.value
  return hasMemberPrice.value
})

const formatWholesaleTier = (tier: any) => t('products.wholesaleTier', {
  count: Number(tier?.min_quantity || 0),
  price: formatPrice(tier?.unit_price, siteCurrency.value),
})

// Stock helpers (same logic as ProductDetail)
const normalizeStockNumber = (value: unknown) => {
  const n = Number(value)
  if (!Number.isFinite(n)) return 0
  return Math.max(Math.floor(n), 0)
}

const normalizeManualStockTotal = (value: unknown) => {
  const n = Number(value)
  if (!Number.isFinite(n)) return 0
  const i = Math.floor(n)
  if (i === -1) return -1
  return Math.max(i, 0)
}

const normalizeOptionalLimitNumber = (value: unknown) => {
  const n = Number(value)
  if (!Number.isFinite(n)) return null
  const i = Math.floor(n)
  if (i <= 0) return null
  return i
}

const effectiveMin = computed(() => {
  const productMin = normalizeOptionalLimitNumber(props.product?.min_purchase_quantity)
  return productMin && productMin > 0 ? productMin : 1
})

const shouldEnforceSkuStock = (sku: any) => {
  if (!sku) return false
  if (sku?.stock_quantity_hidden === true || props.product?.stock_quantity_hidden === true) return false
  if (props.product?.fulfillment_type === 'auto') return true
  if (props.product?.fulfillment_type === 'upstream') return true
  if (props.product?.fulfillment_type !== 'manual') return false
  const total = normalizeManualStockTotal(sku?.manual_stock_total)
  return total !== -1
}

const skuAvailableStock = (sku: any) => {
  if (!sku) return 0
  if (!shouldEnforceSkuStock(sku) && !sku?.stock_quantity_hidden) return null
  return resolveSkuAvailableStock(props.product, sku)
}

const isSkuPurchasable = (sku: any) => {
  const available = skuAvailableStock(sku)
  if (available === null) return true
  return available > 0
}

const syncSelectedSku = () => {
  const rows = activeSkus.value
  if (rows.length === 0) { selectedSkuId.value = 0; return }
  if (rows.length === 1) { selectedSkuId.value = normalizeSkuId(rows[0]?.id); return }
  const firstAvailable = rows.find((sku: any) => isSkuPurchasable(sku))
  if (firstAvailable) { selectedSkuId.value = normalizeSkuId(firstAvailable?.id); return }
  selectedSkuId.value = normalizeSkuId(rows[0]?.id)
}

// Reset state when product changes or drawer opens
watch(() => [props.product, props.visible], () => {
  if (props.visible && props.product) {
    purchaseWarning.value = ''
    quantity.value = effectiveMin.value
    syncSelectedSku()
    ensureMemberLevels()
  }
}, { immediate: true })

const skuStockText = (sku: any) => {
  const display = resolveSkuStockDisplay(props.product, sku)
  return formatSkuStockDisplay(display)
}

const formatSkuStockDisplay = (display: PublicStockDisplay) => {
  switch (display.kind) {
    case 'unlimited':
      return t('productDetail.skuStockUnlimited')
    case 'out':
      return t('productDetail.skuStockOut')
    case 'remaining':
      return t('productDetail.skuStockRemaining', { count: display.count })
    case 'low_stock':
      return t('productDetail.skuStockLow')
    case 'hidden':
      return t('productDetail.skuStockHidden')
    case 'range':
      return t('productDetail.skuStockRange', { min: display.min, max: display.max })
    case 'range_plus':
      return t('productDetail.skuStockRangePlus', { min: display.min })
    case 'in_stock':
    default:
      return t('productDetail.skuStockInStock')
  }
}

const skuStockBadgeClass = (sku: any) => {
  const display = resolveSkuStockDisplay(props.product, sku)
  if (display.kind === 'unlimited' || display.kind === 'hidden') return 'border-slate-200 text-slate-600 dark:border-slate-700 dark:text-slate-300'
  if (display.kind === 'out') return 'border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-700 dark:bg-rose-950/30 dark:text-rose-300'
  if (display.kind === 'low_stock' || (display.kind === 'range' && display.max <= 5)) return 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-700 dark:bg-amber-950/30 dark:text-amber-300'
  return 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-700 dark:bg-emerald-950/30 dark:text-emerald-300'
}

const skuStockDotClass = (sku: any) => {
  const display = resolveSkuStockDisplay(props.product, sku)
  if (display.kind === 'unlimited' || display.kind === 'hidden') return 'bg-slate-400 dark:bg-slate-500'
  if (display.kind === 'out') return 'bg-rose-500 dark:bg-rose-400'
  if (display.kind === 'low_stock' || (display.kind === 'range' && display.max <= 5)) return 'bg-amber-500 dark:bg-amber-400'
  return 'bg-emerald-500 dark:bg-emerald-400'
}

const skuDisplayText = (sku: any) => buildSkuDisplayText({
  skuCode: sku?.sku_code,
  specValues: sku?.spec_values,
  fallback: t('productDetail.skuFallback'),
  locale: appStore.locale,
})

const effectiveLimit = computed(() => {
  const sku = selectedSku.value
  const available = skuAvailableStock(sku)
  const productLimit = normalizeOptionalLimitNumber(props.product?.max_purchase_quantity)
  let limit: number | null = productLimit
  if (available !== null) {
    limit = limit === null ? available : Math.min(limit, available)
  }
  return limit
})

const purchaseType = computed(() => props.product?.purchase_type || 'member')
const requiresLogin = computed(() => purchaseType.value === 'member' && !userAuthStore.isAuthenticated)
const requiresSKUSelection = computed(() => activeSkus.value.length > 1 && !selectedSku.value)
const stockBelowMinPurchase = computed(() => {
  const limit = effectiveLimit.value
  if (limit === null) return false
  return limit < effectiveMin.value
})
const canPurchase = computed(() => {
  if (!props.product) return false
  if (activeSkus.value.length === 0) return false
  if (props.product.is_sold_out) return false
  if (requiresSKUSelection.value) return false
  if (props.product.stock_status === 'out_of_stock') return false
  if (selectedSku.value && !isSkuPurchasable(selectedSku.value)) return false
  if (stockBelowMinPurchase.value) return false
  return true
})

const selectedCartQuantity = () => {
  if (!props.product || !selectedSku.value) return 0
  const productId = Number(props.product.id || 0)
  const skuId = normalizeSkuId(selectedSku.value?.id)
  if (productId <= 0 || skuId <= 0) return 0
  const matched = cartStore.items.find((item) => item.productId === productId && normalizeSkuId(item.skuId) === skuId)
  return Number(matched?.quantity || 0)
}

const handleQuantityInput = (event: Event) => {
  const input = event.target as HTMLInputElement
  const parsed = Math.floor(Number(input.value))
  const minimum = effectiveMin.value
  if (!Number.isFinite(parsed) || parsed < minimum) {
    quantity.value = minimum
    input.value = String(minimum)
    return
  }
  const limit = effectiveLimit.value
  if (limit !== null && parsed > limit) {
    quantity.value = limit
    input.value = String(limit)
    return
  }
  quantity.value = parsed
}

const close = () => {
  emit('update:visible', false)
}

const handleAddToCart = () => {
  if (!props.product || !canPurchase.value) return
  purchaseWarning.value = ''

  const sku = selectedSku.value
  const available = skuAvailableStock(sku)
  const cartQty = selectedCartQuantity()
  const nextQuantity = cartQty + quantity.value
  const productLimit = normalizeOptionalLimitNumber(props.product?.max_purchase_quantity)
  let limit: number | null = productLimit
  if (available !== null) {
    limit = limit === null ? available : Math.min(limit, available)
  }
  if (limit !== null && nextQuantity > limit) {
    if (available !== null && limit === available && (productLimit === null || available <= productLimit)) {
      purchaseWarning.value = available > 0
        ? (cartQty > 0
            ? t('productDetail.addCartStockExceededWithCart', { count: available, cartCount: cartQty })
            : t('productDetail.addCartStockExceeded', { count: available }))
        : t('productDetail.stockUnavailable')
      return
    }
    purchaseWarning.value = cartQty > 0
      ? t('productDetail.addCartLimitExceededWithCart', { count: limit, cartCount: cartQty })
      : t('productDetail.addCartLimitExceeded', { count: limit })
    return
  }

  const images = getProductImages()
  cartStore.addItem({
    productId: props.product.id,
    skuId: normalizeSkuId(sku?.id),
    skuCode: String(sku?.sku_code || ''),
    skuSpecValues: (sku?.spec_values && typeof sku.spec_values === 'object') ? sku.spec_values : undefined,
    skuManualStockTotal: normalizeManualStockTotal(sku?.manual_stock_total),
    skuManualStockLocked: normalizeStockNumber(sku?.manual_stock_locked),
    skuManualStockSold: normalizeStockNumber(sku?.manual_stock_sold),
    skuAutoStockAvailable: normalizeManualStockTotal(sku?.auto_stock_available),
    skuUpstreamStock: normalizeManualStockTotal(sku?.upstream_stock),
    skuStockStatus: String(sku?.stock_status || ''),
    skuStockDisplayMode: String(sku?.stock_display_mode || props.product?.stock_display_mode || ''),
    skuStockDisplay: String(sku?.stock_display || ''),
    skuStockRangeMin: normalizeStockNumber(sku?.stock_range_min) || undefined,
    skuStockRangeMax: normalizeStockNumber(sku?.stock_range_max) || undefined,
    skuStockQuantityHidden: Boolean(sku?.stock_quantity_hidden || props.product?.stock_quantity_hidden),
    skuStockEnforced: shouldEnforceSkuStock(sku),
    slug: props.product.slug,
    title: props.product.title,
    priceAmount: String(sku?.price_amount || props.product.price_amount || '0.00'),
    wholesalePrices: Array.isArray(props.product.wholesale_prices) ? props.product.wholesale_prices : undefined,
    image: images[0] || '',
    minPurchaseQuantity: normalizeOptionalLimitNumber(props.product.min_purchase_quantity) ?? undefined,
    maxPurchaseQuantity: normalizeOptionalLimitNumber(props.product.max_purchase_quantity) ?? undefined,
    purchaseType: props.product.purchase_type,
    fulfillmentType: props.product.fulfillment_type,
    manualFormSchema: props.product.manual_form_schema || {},
    paymentChannelIds: Array.isArray(props.product.payment_channel_ids) && props.product.payment_channel_ids.length > 0 ? props.product.payment_channel_ids : undefined,
    quantity: 1,
  }, quantity.value)
  toast.success(t('toast.addedToCart'))
  close()
}

const handleBuyNow = () => {
  purchaseWarning.value = ''
  if (!canPurchase.value) return
  if (!props.product) return

  const sku = selectedSku.value
  const available = skuAvailableStock(sku)
  const productLimit = normalizeOptionalLimitNumber(props.product?.max_purchase_quantity)
  let limit: number | null = productLimit
  if (available !== null) {
    limit = limit === null ? available : Math.min(limit, available)
  }
  if (limit !== null && quantity.value > limit) {
    purchaseWarning.value = available !== null && limit === available && (productLimit === null || available <= productLimit)
      ? (available > 0 ? t('productDetail.addCartStockExceeded', { count: available }) : t('productDetail.stockUnavailable'))
      : t('productDetail.addCartLimitExceeded', { count: limit })
    return
  }

  const images = getProductImages()
  buyNowStore.setItem({
    productId: props.product.id,
    skuId: normalizeSkuId(sku?.id),
    skuCode: String(sku?.sku_code || ''),
    skuSpecValues: (sku?.spec_values && typeof sku.spec_values === 'object') ? sku.spec_values : undefined,
    skuManualStockTotal: normalizeManualStockTotal(sku?.manual_stock_total),
    skuManualStockLocked: normalizeStockNumber(sku?.manual_stock_locked),
    skuManualStockSold: normalizeStockNumber(sku?.manual_stock_sold),
    skuAutoStockAvailable: normalizeManualStockTotal(sku?.auto_stock_available),
    skuUpstreamStock: normalizeManualStockTotal(sku?.upstream_stock),
    skuStockStatus: String(sku?.stock_status || ''),
    skuStockDisplayMode: String(sku?.stock_display_mode || props.product?.stock_display_mode || ''),
    skuStockDisplay: String(sku?.stock_display || ''),
    skuStockRangeMin: normalizeStockNumber(sku?.stock_range_min) || undefined,
    skuStockRangeMax: normalizeStockNumber(sku?.stock_range_max) || undefined,
    skuStockQuantityHidden: Boolean(sku?.stock_quantity_hidden || props.product?.stock_quantity_hidden),
    skuStockEnforced: shouldEnforceSkuStock(sku),
    slug: props.product.slug,
    title: props.product.title,
    priceAmount: String(sku?.price_amount || props.product.price_amount || '0.00'),
    wholesalePrices: Array.isArray(props.product.wholesale_prices) ? props.product.wholesale_prices : undefined,
    image: images[0] || '',
    minPurchaseQuantity: normalizeOptionalLimitNumber(props.product.min_purchase_quantity) ?? undefined,
    maxPurchaseQuantity: normalizeOptionalLimitNumber(props.product.max_purchase_quantity) ?? undefined,
    purchaseType: props.product.purchase_type,
    fulfillmentType: props.product.fulfillment_type,
    manualFormSchema: props.product.manual_form_schema || {},
    paymentChannelIds: Array.isArray(props.product.payment_channel_ids) && props.product.payment_channel_ids.length > 0 ? props.product.payment_channel_ids : undefined,
    quantity: quantity.value,
  })
  close()
  router.push('/checkout?mode=buynow')
}

const goLogin = () => {
  close()
  router.push(`/auth/login?redirect=${encodeURIComponent(route.fullPath)}`)
}

const goToDetail = () => {
  close()
  router.push(`/products/${props.product?.slug}`)
}

const getProductImages = () => {
  if (!props.product?.images) return []
  let imageArray: string[] = []
  if (Array.isArray(props.product.images)) {
    imageArray = props.product.images
  } else if (props.product.images.images && Array.isArray(props.product.images.images)) {
    imageArray = props.product.images.images
  }
  return imageArray.map((img: string) => getImageUrl(img))
}
</script>

<style scoped>
/* Mobile: slide up from bottom */
.quick-buy-enter-active {
  transition: transform 280ms cubic-bezier(0.32, 0.72, 0, 1), opacity 280ms ease-out;
}
.quick-buy-leave-active {
  transition: transform 200ms ease-in, opacity 200ms ease-in;
}
.quick-buy-enter-from {
  transform: translateY(100%);
  opacity: 1;
}
.quick-buy-enter-to {
  transform: translateY(0);
  opacity: 1;
}
.quick-buy-leave-from {
  transform: translateY(0);
  opacity: 1;
}
.quick-buy-leave-to {
  transform: translateY(100%);
  opacity: 1;
}

/* Desktop: centered scale + fade */
@media (min-width: 768px) {
  .quick-buy-enter-active {
    transition: transform 200ms ease-out, opacity 200ms ease-out;
  }
  .quick-buy-leave-active {
    transition: transform 150ms ease-in, opacity 150ms ease-in;
  }
  .quick-buy-enter-from {
    transform: scale(0.96);
    opacity: 0;
  }
  .quick-buy-enter-to {
    transform: scale(1);
    opacity: 1;
  }
  .quick-buy-leave-from {
    transform: scale(1);
    opacity: 1;
  }
  .quick-buy-leave-to {
    transform: scale(0.96);
    opacity: 0;
  }
}
</style>
