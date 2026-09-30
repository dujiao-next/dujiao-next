import { ref, onMounted, onUnmounted, computed, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useHead } from '@unhead/vue'
import { useAppStore } from '../stores/app'
import { productAPI } from '../api'
import { getImageUrl } from '../utils/image'
import { useCartStore } from '../stores/cart'
import { useBuyNowStore } from '../stores/buyNow'
import { useUserAuthStore } from '../stores/userAuth'
import { useUserProfileStore } from '../stores/userProfile'
import { debounceAsync } from '../utils/debounce'
import { buildSkuDisplayText, normalizeSkuId } from '../utils/sku'
import { resolveSkuAvailableStock, resolveSkuStockDisplay, type PublicStockDisplay } from '../utils/publicStock'
import { useLocalized, useProductLabels } from './useProduct'
import { toast } from './useToast'

/**
 * 商品详情页的全部业务逻辑（数据加载、SKU/数量、促销/会员/批发定价、库存约束、
 * 加购 / 立即购买、SEO/JSON-LD）。classic 与 vault 模板共用此 composable，
 * 仅各自负责 markup，保证功能严格一致。
 *
 * 视图专属的移动端购买条（IntersectionObserver + DOM ref）仍留在各视图中，
 * 通过 options.onLoaded 在商品加载完成后回调以初始化。
 */
export function useProductDetail(options: { onLoaded?: () => void } = {}) {
  const route = useRoute()
  const router = useRouter()
  const { t } = useI18n()
  const appStore = useAppStore()
  const cartStore = useCartStore()
  const buyNowStore = useBuyNowStore()
  const userAuthStore = useUserAuthStore()
  const userProfileStore = useUserProfileStore()

  const { getLocalizedText, siteCurrency, formatPrice } = useLocalized()
  const {
    getPurchaseTypeLabel, getFulfillmentTypeLabel, getStockBadgeVariant, getStockStatusLabel,
    hasPromotionPrice, getPromotionPriceAmount, getPromotionSaveAmount,
    hasSkuPromotionPrice, getSkuPromotionPriceAmount, getSkuPromotionSaveAmount,
    hasPromotionRules, getPromotionRules, hasWholesalePrices, getWholesalePrices,
    resolveWholesalePriceAmount, resolveMemberPriceAmount,
  } = useProductLabels()

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

  const loading = ref(true)
  const product = ref<any>(null)
  const relatedPosts = computed<any[]>(() => product.value?.related_posts || [])
  const formatRelatedPostDate = (dateString: string) => {
    if (!dateString) return ''
    const date = new Date(dateString)
    return date.toLocaleDateString(appStore.locale, { year: 'numeric', month: 'long', day: 'numeric' })
  }
  const currentImage = ref<string>('')
  const selectedSkuId = ref(0)
  const quantity = ref(1)
  const purchaseWarning = ref('')

  const activeSkus = computed(() => {
    const rows = Array.isArray(product.value?.skus) ? product.value.skus : []
    return rows.filter((sku: any) => Boolean(sku?.is_active))
  })

  const selectedSku = computed(() => {
    if (selectedSkuId.value <= 0) return null
    return activeSkus.value.find((sku: any) => normalizeSkuId(sku?.id) === selectedSkuId.value) || null
  })

  // 会员价相关
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
    const price = resolveMemberPriceAmount(product.value, skuId, basePrice, userMemberLevelId.value, currentMemberDiscountRate.value)
    return price === null ? null : Number(price)
  }

  const selectedSkuMemberPrice = computed(() => {
    if (!selectedSku.value) return null
    const skuId = normalizeSkuId(selectedSku.value.id)
    return getMemberPriceForSku(skuId, selectedSku.value.price_amount)
  })

  const hasMemberPrice = computed(() => {
    if (!selectedSkuMemberPrice.value) return false
    const basePrice = Number(selectedSku.value?.price_amount || 0)
    return selectedSkuMemberPrice.value < basePrice
  })

  const selectedSkuWholesaleRules = computed(() => {
    if (!product.value || !selectedSku.value) return []
    return getWholesalePrices(
      product.value,
      normalizeSkuId(selectedSku.value.id),
      selectedSku.value.sku_code,
    )
  })

  const selectedSkuWholesalePrice = computed(() => {
    if (!product.value || !selectedSku.value) return null
    return resolveWholesalePriceAmount(
      product.value,
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
    if (!product.value || !selectedSku.value || !selectedSkuWholesalePrice.value) return null
    const skuId = normalizeSkuId(selectedSku.value.id)
    return getMemberPriceForSku(skuId, selectedSkuWholesalePrice.value)
  })

  const selectedSkuWholesaleFinalIsMember = computed(() => {
    return hasSelectedSkuWholesalePrice.value && selectedSkuWholesaleMemberPrice.value !== null
  })

  const selectedSkuWholesaleFinalPrice = computed(() => {
    if (!hasSelectedSkuWholesalePrice.value || selectedSkuWholesalePrice.value === null) return null
    if (selectedSkuWholesaleMemberPrice.value !== null) return selectedSkuWholesaleMemberPrice.value
    return selectedSkuWholesalePrice.value
  })

  const selectedSkuPromotionPrice = computed(() => {
    if (!selectedSku.value || !hasSkuPromotionPrice(selectedSku.value)) return null
    return getSkuPromotionPriceAmount(selectedSku.value)
  })

  const selectedSkuPromotionMemberPrice = computed(() => {
    if (!product.value || !selectedSku.value || selectedSkuPromotionPrice.value === null) return null
    const skuId = normalizeSkuId(selectedSku.value.id)
    return getMemberPriceForSku(skuId, selectedSkuPromotionPrice.value)
  })

  const selectedSkuPromotionFinalIsMember = computed(() => selectedSkuPromotionMemberPrice.value !== null)

  const selectedSkuPromotionFinalPrice = computed(() => {
    if (selectedSkuPromotionPrice.value === null) return null
    if (selectedSkuPromotionMemberPrice.value !== null) return selectedSkuPromotionMemberPrice.value
    return selectedSkuPromotionPrice.value
  })

  const showSelectedSkuMemberBadge = computed(() => {
    if (!selectedSku.value) return false
    if (hasSelectedSkuWholesalePrice.value) return selectedSkuWholesaleFinalIsMember.value
    if (hasSkuPromotionPrice(selectedSku.value)) return selectedSkuPromotionFinalIsMember.value
    return hasMemberPrice.value
  })

  const formatWholesaleTier = (tier: any) => {
    return t('products.wholesaleTier', {
      count: Number(tier?.min_quantity || 0),
      price: formatPrice(tier?.unit_price, siteCurrency.value),
    })
  }

  const normalizeStockNumber = (value: unknown) => {
    const numberValue = Number(value)
    if (!Number.isFinite(numberValue)) return 0
    return Math.max(Math.floor(numberValue), 0)
  }

  const normalizeManualStockTotal = (value: unknown) => {
    const numberValue = Number(value)
    if (!Number.isFinite(numberValue)) return 0
    const integerValue = Math.floor(numberValue)
    if (integerValue === -1) return -1
    return Math.max(integerValue, 0)
  }

  const normalizeOptionalLimitNumber = (value: unknown) => {
    const numberValue = Number(value)
    if (!Number.isFinite(numberValue)) return null
    const integerValue = Math.floor(numberValue)
    if (integerValue <= 0) return null
    return integerValue
  }

  const shouldEnforceSkuStock = (sku: any) => {
    if (!sku) return false
    if (sku?.stock_quantity_hidden === true || product.value?.stock_quantity_hidden === true) return false
    if (product.value?.fulfillment_type === 'auto') return true
    if (product.value?.fulfillment_type === 'upstream') return true
    if (product.value?.fulfillment_type !== 'manual') return false
    const total = normalizeManualStockTotal(sku?.manual_stock_total)
    if (total === -1) return false
    return true
  }

  const skuAvailableStock = (sku: any) => {
    if (!sku) return 0
    if (!shouldEnforceSkuStock(sku) && !sku?.stock_quantity_hidden) return null
    return resolveSkuAvailableStock(product.value, sku)
  }

  const isSkuPurchasable = (sku: any) => {
    const available = skuAvailableStock(sku)
    if (available === null) return true
    return available > 0
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

  const skuStockText = (sku: any) => {
    const display = resolveSkuStockDisplay(product.value, sku)
    return formatSkuStockDisplay(display)
  }

  const skuStockBadgeClass = (sku: any) => {
    const display = resolveSkuStockDisplay(product.value, sku)
    if (display.kind === 'unlimited' || display.kind === 'hidden') return 'border-slate-200 text-slate-600 dark:border-slate-700 dark:text-slate-300'
    if (display.kind === 'out') return 'border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-700 dark:bg-rose-950/30 dark:text-rose-300'
    if (display.kind === 'low_stock' || (display.kind === 'range' && display.max <= 5)) return 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-700 dark:bg-amber-950/30 dark:text-amber-300'
    return 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-700 dark:bg-emerald-950/30 dark:text-emerald-300'
  }

  const quantityEffectiveLimit = computed(() => {
    const sku = selectedSku.value
    const available = skuAvailableStock(sku)
    const productLimit = normalizeOptionalLimitNumber(product.value?.max_purchase_quantity)
    let limit: number | null = productLimit
    if (available !== null) {
      limit = limit === null ? available : Math.min(limit, available)
    }
    return limit
  })

  const quantityEffectiveMin = computed(() => {
    const productMin = normalizeOptionalLimitNumber(product.value?.min_purchase_quantity)
    return productMin && productMin > 0 ? productMin : 1
  })

  const handleQuantityInput = (event: Event) => {
    const val = parseInt((event.target as HTMLInputElement).value, 10)
    const minimum = quantityEffectiveMin.value
    if (isNaN(val) || val < minimum) {
      quantity.value = minimum
    } else if (quantityEffectiveLimit.value !== null && val > quantityEffectiveLimit.value) {
      quantity.value = quantityEffectiveLimit.value
    } else {
      quantity.value = val
    }
  }

  const purchaseType = computed(() => product.value?.purchase_type || 'member')
  const requiresLogin = computed(() => purchaseType.value === 'member' && !userAuthStore.isAuthenticated)
  const requiresSKUSelection = computed(() => activeSkus.value.length > 1 && !selectedSku.value)
  const stockBelowMinPurchase = computed(() => {
    const limit = quantityEffectiveLimit.value
    if (limit === null) return false
    return limit < quantityEffectiveMin.value
  })
  const canPurchase = computed(() => {
    if (!product.value) return false
    if (activeSkus.value.length === 0) return false
    if (product.value.is_sold_out) return false
    if (requiresSKUSelection.value) return false
    if (product.value.stock_status === 'out_of_stock') return false
    if (selectedSku.value && !isSkuPurchasable(selectedSku.value)) return false
    if (stockBelowMinPurchase.value) return false
    return true
  })
  const cannotPurchaseReason = computed(() => {
    if (!product.value) return ''
    if (requiresLogin.value) return ''
    if (requiresSKUSelection.value) return t('productDetail.skuRequired')
    if (stockBelowMinPurchase.value) return t('productDetail.stockBelowMinPurchase', { count: quantityEffectiveMin.value })
    if (canPurchase.value) return ''
    return t('productDetail.stockUnavailable')
  })
  const categoryName = computed(() => {
    const category = product.value?.category?.name
    return category ? getLocalizedText(category) : ''
  })

  const images = computed(() => {
    if (!product.value?.images) return []
    let imageArray: string[] = []
    if (Array.isArray(product.value.images)) {
      imageArray = product.value.images
    } else if (product.value.images.images && Array.isArray(product.value.images.images)) {
      imageArray = product.value.images.images
    }
    return imageArray.map(img => getImageUrl(img))
  })

  const skuDisplayText = (sku: any) => {
    return buildSkuDisplayText({
      skuCode: sku?.sku_code,
      specValues: sku?.spec_values,
      fallback: t('productDetail.skuFallback'),
      locale: appStore.locale,
    })
  }

  const syncSelectedSku = () => {
    const rows = activeSkus.value
    if (rows.length === 0) {
      selectedSkuId.value = 0
      return
    }
    if (rows.length === 1) {
      selectedSkuId.value = normalizeSkuId(rows[0]?.id)
      return
    }
    if (rows.some((sku: any) => normalizeSkuId(sku?.id) === selectedSkuId.value)) {
      return
    }
    const firstAvailable = rows.find((sku: any) => isSkuPurchasable(sku))
    if (firstAvailable) {
      selectedSkuId.value = normalizeSkuId(firstAvailable?.id)
      return
    }
    selectedSkuId.value = normalizeSkuId(rows[0]?.id)
  }

  const selectedCartQuantity = () => {
    if (!product.value || !selectedSku.value) return 0
    const productId = Number(product.value.id || 0)
    const skuId = normalizeSkuId(selectedSku.value?.id)
    if (productId <= 0 || skuId <= 0) return 0
    const matched = cartStore.items.find((item) => item.productId === productId && normalizeSkuId(item.skuId) === skuId)
    return Number(matched?.quantity || 0)
  }

  const buildItemPayload = (sku: any) => ({
    productId: product.value.id,
    skuId: normalizeSkuId(sku?.id),
    skuCode: String(sku?.sku_code || ''),
    skuSpecValues: (sku?.spec_values && typeof sku.spec_values === 'object') ? sku.spec_values : undefined,
    skuManualStockTotal: normalizeManualStockTotal(sku?.manual_stock_total),
    skuManualStockLocked: normalizeStockNumber(sku?.manual_stock_locked),
    skuManualStockSold: normalizeStockNumber(sku?.manual_stock_sold),
    skuAutoStockAvailable: normalizeManualStockTotal(sku?.auto_stock_available),
    skuUpstreamStock: normalizeManualStockTotal(sku?.upstream_stock),
    skuStockStatus: String(sku?.stock_status || ''),
    skuStockDisplayMode: String(sku?.stock_display_mode || product.value?.stock_display_mode || ''),
    skuStockDisplay: String(sku?.stock_display || ''),
    skuStockRangeMin: normalizeStockNumber(sku?.stock_range_min) || undefined,
    skuStockRangeMax: normalizeStockNumber(sku?.stock_range_max) || undefined,
    skuStockQuantityHidden: Boolean(sku?.stock_quantity_hidden || product.value?.stock_quantity_hidden),
    skuStockEnforced: shouldEnforceSkuStock(sku),
    slug: product.value.slug,
    title: product.value.title,
    priceAmount: String(sku?.price_amount || product.value.price_amount || '0.00'),
    wholesalePrices: Array.isArray(product.value.wholesale_prices) ? product.value.wholesale_prices : undefined,
    image: images.value[0],
    minPurchaseQuantity: normalizeOptionalLimitNumber(product.value.min_purchase_quantity) ?? undefined,
    maxPurchaseQuantity: normalizeOptionalLimitNumber(product.value.max_purchase_quantity) ?? undefined,
    purchaseType: product.value.purchase_type,
    fulfillmentType: product.value.fulfillment_type,
    manualFormSchema: product.value.manual_form_schema || {},
    paymentChannelIds: Array.isArray(product.value.payment_channel_ids) && product.value.payment_channel_ids.length > 0 ? product.value.payment_channel_ids : undefined,
    quantity: quantity.value,
  })

  const addToCart = () => {
    if (!product.value) return
    if (!canPurchase.value) return
    purchaseWarning.value = ''
    if (requiresLogin.value) {
      router.push(`/auth/login?redirect=${encodeURIComponent(route.fullPath)}`)
      return
    }
    const sku = selectedSku.value
    const available = skuAvailableStock(sku)
    const cartQty = selectedCartQuantity()
    const nextQuantity = cartQty + quantity.value
    const productLimit = normalizeOptionalLimitNumber(product.value?.max_purchase_quantity)
    let effectiveLimit: number | null = productLimit
    if (available !== null) {
      effectiveLimit = effectiveLimit === null ? available : Math.min(effectiveLimit, available)
    }
    if (effectiveLimit !== null && nextQuantity > effectiveLimit) {
      if (available !== null && effectiveLimit === available && (productLimit === null || available <= productLimit)) {
        purchaseWarning.value = available > 0
          ? (cartQty > 0
              ? t('productDetail.addCartStockExceededWithCart', { count: available, cartCount: cartQty })
              : t('productDetail.addCartStockExceeded', { count: available }))
          : t('productDetail.stockUnavailable')
        return
      }
      purchaseWarning.value = cartQty > 0
        ? t('productDetail.addCartLimitExceededWithCart', { count: effectiveLimit, cartCount: cartQty })
        : t('productDetail.addCartLimitExceeded', { count: effectiveLimit })
      return
    }
    cartStore.addItem(buildItemPayload(sku), quantity.value)
    toast.success(t('toast.addedToCart'))
  }

  const buyNow = () => {
    purchaseWarning.value = ''
    if (!canPurchase.value) return
    if (!product.value) return
    if (requiresLogin.value) {
      router.push(`/auth/login?redirect=${encodeURIComponent(route.fullPath)}`)
      return
    }

    const sku = selectedSku.value
    const available = skuAvailableStock(sku)
    const productLimit = normalizeOptionalLimitNumber(product.value?.max_purchase_quantity)
    let limit: number | null = productLimit
    if (available !== null) {
      limit = limit === null ? available : Math.min(limit, available)
    }
    if (limit !== null && quantity.value > limit) {
      purchaseWarning.value = available !== null && limit === available
        ? (available > 0 ? t('productDetail.addCartStockExceeded', { count: available }) : t('productDetail.stockUnavailable'))
        : t('productDetail.addCartLimitExceeded', { count: limit })
      return
    }

    buyNowStore.setItem(buildItemPayload(sku))
    router.push('/checkout?mode=buynow')
  }

  // 移动端购买条价格展示
  const mobileBarShowMemberPrice = computed(() => {
    if (!selectedSku.value) return false
    if (hasSelectedSkuWholesalePrice.value) return selectedSkuWholesaleFinalIsMember.value
    if (hasSkuPromotionPrice(selectedSku.value)) return selectedSkuPromotionFinalIsMember.value
    return hasMemberPrice.value
  })
  const mobileBarMemberPriceDisplay = computed(() => {
    if (hasSelectedSkuWholesalePrice.value && selectedSkuWholesaleFinalIsMember.value) {
      return formatPrice(selectedSkuWholesaleFinalPrice.value, siteCurrency.value)
    }
    if (selectedSku.value && hasSkuPromotionPrice(selectedSku.value) && selectedSkuPromotionFinalIsMember.value) {
      return formatPrice(selectedSkuPromotionFinalPrice.value, siteCurrency.value)
    }
    if (!selectedSkuMemberPrice.value) return ''
    return formatPrice(selectedSkuMemberPrice.value, siteCurrency.value)
  })
  const mobileBarShowSkuPromotionPrice = computed(() => {
    if (mobileBarShowMemberPrice.value) return false
    if (hasSelectedSkuWholesalePrice.value) return false
    return !!selectedSku.value && hasSkuPromotionPrice(selectedSku.value)
  })
  const mobileBarSkuPromotionPriceDisplay = computed(() => {
    if (!selectedSku.value) return ''
    return formatPrice(getSkuPromotionPriceAmount(selectedSku.value), siteCurrency.value)
  })
  const mobileBarShowSkuPrice = computed(() => {
    if (mobileBarShowMemberPrice.value || mobileBarShowSkuPromotionPrice.value) return false
    return !!selectedSku.value
  })
  const mobileBarSkuPriceDisplay = computed(() => {
    if (!selectedSku.value) return ''
    return formatPrice(selectedSku.value.price_amount, siteCurrency.value)
  })
  const mobileBarShowProductPromotionPrice = computed(() => {
    if (selectedSku.value) return false
    return product.value ? hasPromotionPrice(product.value) : false
  })
  const mobileBarProductPromotionPriceDisplay = computed(() => {
    if (!product.value) return ''
    return formatPrice(getPromotionPriceAmount(product.value), siteCurrency.value)
  })
  const mobileBarProductPriceDisplay = computed(() => {
    if (!product.value) return ''
    return formatPrice(product.value.price_amount, siteCurrency.value)
  })

  const goLogin = () => {
    router.push(`/auth/login?redirect=${encodeURIComponent(route.fullPath)}`)
  }

  const loadProduct = async () => {
    loading.value = true
    try {
      const slug = route.params.slug as string
      const response = await productAPI.detail(slug)
      product.value = response.data.data || null
      if (images.value.length > 0) {
        currentImage.value = images.value[0] || ''
      }
      syncSelectedSku()
      await nextTick()
      options.onLoaded?.()
    } catch (error) {
      console.error('Failed to load product:', error)
      product.value = null
      selectedSkuId.value = 0
    } finally {
      loading.value = false
    }
  }

  const debouncedLoadProduct = debounceAsync(loadProduct, 300)

  const canonicalUrl = computed(() => {
    if (!product.value?.slug) return ''
    const fromConfig = String(appStore.config?.brand?.site_url || '').trim().replace(/\/+$/, '')
    const base = fromConfig || window.location.origin.replace(/\/+$/, '')
    return `${base}/products/${product.value.slug}`
  })

  useHead({
    title: () => product.value ? getLocalizedText(product.value.title) : '',
    link: () => {
      if (!canonicalUrl.value) return []
      return [{ rel: 'canonical', href: canonicalUrl.value }]
    },
    meta: () => {
      if (!product.value) return []
      const seoMeta = product.value.seo_meta || {}
      const seoKeywords = getLocalizedText(seoMeta.keywords) || (typeof seoMeta.keywords === 'string' ? seoMeta.keywords : '')
      const seoDescription = getLocalizedText(seoMeta.description) || (typeof seoMeta.description === 'string' ? seoMeta.description : '')
      const tags = []

      if (seoKeywords) tags.push({ name: 'keywords', content: seoKeywords })
      if (seoDescription) tags.push({ name: 'description', content: seoDescription })

      tags.push({ property: 'og:type', content: 'product' })
      if (product.value.title) {
        tags.push({ property: 'og:title', content: getLocalizedText(product.value.title) })
      }
      if (seoDescription) {
        tags.push({ property: 'og:description', content: seoDescription })
      }
      if (images.value && images.value.length > 0) {
        tags.push({ property: 'og:image', content: images.value[0] })
      }
      if (canonicalUrl.value) {
        tags.push({ property: 'og:url', content: canonicalUrl.value })
      }

      tags.push({ name: 'twitter:card', content: 'summary_large_image' })
      if (product.value.title) {
        tags.push({ name: 'twitter:title', content: getLocalizedText(product.value.title) })
      }
      if (seoDescription) {
        tags.push({ name: 'twitter:description', content: seoDescription })
      }
      if (images.value && images.value.length > 0) {
        tags.push({ name: 'twitter:image', content: images.value[0] })
      }

      return tags
    },
    script: () => {
      if (!product.value) return []
      const title = getLocalizedText(product.value.title)
      const seoMeta = product.value.seo_meta || {}
      const description = getLocalizedText(seoMeta.description) || (typeof seoMeta.description === 'string' ? seoMeta.description : '')
      const priceAmount = product.value.price_amount || '0'
      const currency = siteCurrency.value || 'CNY'

      const jsonLd: Record<string, any> = {
        '@context': 'https://schema.org',
        '@type': 'Product',
        name: title,
        url: canonicalUrl.value || window.location.href,
        offers: {
          '@type': 'Offer',
          price: priceAmount,
          priceCurrency: currency,
          availability: product.value.stock_status === 'out_of_stock'
            ? 'https://schema.org/OutOfStock'
            : 'https://schema.org/InStock',
        },
      }
      if (description) jsonLd.description = description
      if (images.value.length > 0) jsonLd.image = images.value
      if (product.value.category?.name) {
        jsonLd.category = getLocalizedText(product.value.category.name)
      }

      return [{
        type: 'application/ld+json',
        innerHTML: JSON.stringify(jsonLd),
      }]
    },
  })

  onMounted(() => {
    loadProduct()
    ensureMemberLevels()
  })

  watch(userMemberLevelId, () => {
    ensureMemberLevels()
  })

  watch(
    () => selectedSkuId.value,
    () => {
      purchaseWarning.value = ''
      quantity.value = quantityEffectiveMin.value
    }
  )

  watch(quantityEffectiveMin, (minimum) => {
    if (minimum > quantity.value) {
      quantity.value = minimum
    }
  })

  watch(quantityEffectiveLimit, (limit) => {
    if (limit !== null && quantity.value > limit) {
      quantity.value = Math.max(quantityEffectiveMin.value, limit)
    }
  })

  onUnmounted(() => {
    debouncedLoadProduct.cancel()
  })

  return {
    // 国际化/格式化（模板需要）
    getLocalizedText, siteCurrency, formatPrice,
    getPurchaseTypeLabel, getFulfillmentTypeLabel, getStockBadgeVariant, getStockStatusLabel,
    hasPromotionPrice, getPromotionPriceAmount, getPromotionSaveAmount,
    hasSkuPromotionPrice, getSkuPromotionPriceAmount, getSkuPromotionSaveAmount,
    hasPromotionRules, getPromotionRules, hasWholesalePrices, getWholesalePrices,
    formatPromotionRule, formatWholesaleTier, formatRelatedPostDate,
    normalizeSkuId,
    // 状态
    loading, product, relatedPosts, currentImage, selectedSkuId, quantity, purchaseWarning,
    activeSkus, selectedSku,
    // 价格计算
    selectedSkuMemberPrice, hasMemberPrice,
    hasSelectedSkuWholesalePrice, selectedSkuWholesaleFinalIsMember, selectedSkuWholesaleFinalPrice,
    selectedSkuWholesaleRules,
    selectedSkuPromotionPrice, selectedSkuPromotionFinalIsMember, selectedSkuPromotionFinalPrice,
    showSelectedSkuMemberBadge,
    // SKU / 库存 / 数量
    isSkuPurchasable, skuDisplayText, skuStockText, skuStockBadgeClass,
    quantityEffectiveLimit, quantityEffectiveMin, handleQuantityInput,
    // 购买能力
    purchaseType, requiresLogin, requiresSKUSelection, canPurchase, cannotPurchaseReason,
    categoryName, images,
    // 动作
    addToCart, buyNow, goLogin, loadProduct,
    // 移动端购买条
    mobileBarShowMemberPrice, mobileBarMemberPriceDisplay,
    mobileBarShowSkuPromotionPrice, mobileBarSkuPromotionPriceDisplay,
    mobileBarShowSkuPrice, mobileBarSkuPriceDisplay,
    mobileBarShowProductPromotionPrice, mobileBarProductPromotionPriceDisplay, mobileBarProductPriceDisplay,
  }
}
