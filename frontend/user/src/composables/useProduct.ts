import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '../stores/app'
import { amountToCents, centsToAmount } from '../utils/money'
import { resolveCardDisplayPrice, resolveMemberPriceAmount } from '../utils/productPricing'
import { useUserAuthStore } from '../stores/userAuth'
import { useUserProfileStore } from '../stores/userProfile'

export function useLocalized() {
  const appStore = useAppStore()

  const getLocalizedText = (jsonData: any): string => {
    if (!jsonData) return ''
    const locale = appStore.locale
    return jsonData[locale] || jsonData['zh-CN'] || jsonData['en-US'] || ''
  }

  const siteCurrency = computed(() => {
    const raw = String(appStore.config?.currency || '').trim().toUpperCase()
    return /^[A-Z]{3}$/.test(raw) ? raw : 'CNY'
  })

  const formatPrice = (amount: any, currency?: any): string => {
    const cur = currency ?? siteCurrency.value
    if (amount === null || amount === undefined || amount === '') return '-'
    const cents = amountToCents(amount)
    const displayAmount = cents === null ? String(amount) : centsToAmount(cents)
    if (cur === null || cur === undefined || cur === '') {
      return displayAmount
    }
    return `${displayAmount} ${cur}`
  }

  return { getLocalizedText, siteCurrency, formatPrice }
}

export function useProductLabels() {
  const userAuthStore = useUserAuthStore()
  const userProfileStore = useUserProfileStore()
  if (userAuthStore.isAuthenticated) {
    void userProfileStore.loadMyProductPrices()
    // 卡片展示会员价需要等级折扣率；只进过列表页时不加载会导致显示原价。
    // loadMemberLevels 已做去重，重复调用不会多发请求。
    if (Number(userAuthStore.user?.member_level_id || 0) > 0) {
      void userProfileStore.loadMemberLevels()
    }
  }
  const { t } = useI18n()

  const getPurchaseTypeLabel = (purchaseType: string) => {
    return purchaseType === 'guest' ? t('productPurchase.guest') : t('productPurchase.member')
  }

  const getFulfillmentTypeLabel = (fulfillmentType: string) => {
    return fulfillmentType === 'auto' ? t('products.fulfillmentType.auto') : t('products.fulfillmentType.manual')
  }

  const getStockBadgeVariant = (status: string): 'info' | 'warning' | 'destructive' | 'success' => {
    switch (status) {
      case 'unlimited':
        return 'info'
      case 'low_stock':
        return 'warning'
      case 'out_of_stock':
        return 'destructive'
      default:
        return 'success'
    }
  }

  const getStockStatusLabel = (product: any) => {
    const status = product?.stock_status || ''
    if (status === 'unlimited') return t('products.stockStatus.unlimited')
    if (status === 'out_of_stock') return t('products.stockStatus.outOfStock')
    if (status === 'low_stock') {
      const quantityHidden = product?.stock_quantity_hidden === true || String(product?.stock_display_mode || '').trim() !== 'exact'
      if (quantityHidden) {
        return t('products.stockStatus.lowStock')
      }
      const count = Number(product?.fulfillment_type === 'manual' ? product?.manual_stock_available : product?.auto_stock_available)
      if (Number.isFinite(count) && count > 0) {
        return t('products.stockStatus.lowStockCount', { count })
      }
      return t('products.stockStatus.lowStock')
    }
    return t('products.stockStatus.inStock')
  }

  const isSoldOut = (product: any) => Boolean(product?.is_sold_out || product?.stock_status === 'out_of_stock')

  const parsePriceAmount = (amount: any) => amountToCents(amount)

  const getPromotionPriceAmount = (product: any) => product?.promotion_price_amount

  const hasPromotionPrice = (product: any) => {
    if (!product) return false
    const original = parsePriceAmount(product.price_amount)
    const promotion = parsePriceAmount(product.promotion_price_amount)
    if (original === null || promotion === null) return false
    return promotion >= 0 && promotion < original
  }

  const getPromotionSaveAmount = (product: any) => {
    const original = parsePriceAmount(product?.price_amount)
    const promotion = parsePriceAmount(product?.promotion_price_amount)
    if (original === null || promotion === null || promotion >= original) {
      return '0.00'
    }
    return centsToAmount(original - promotion)
  }

  const hasSkuPromotionPrice = (sku: any) => {
    if (!sku) return false
    const original = parsePriceAmount(sku.price_amount)
    const promotion = parsePriceAmount(sku.promotion_price_amount)
    if (original === null || promotion === null) return false
    return promotion >= 0 && promotion < original
  }

  const getSkuPromotionPriceAmount = (sku: any) => sku?.promotion_price_amount

  const getSkuPromotionSaveAmount = (sku: any) => {
    const original = parsePriceAmount(sku?.price_amount)
    const promotion = parsePriceAmount(sku?.promotion_price_amount)
    if (original === null || promotion === null || promotion >= original) {
      return '0.00'
    }
    return centsToAmount(original - promotion)
  }

  const hasPromotionRules = (product: any) => product?.promotion_rules?.length > 0
  const getPromotionRules = (product: any): any[] => product?.promotion_rules ?? []

  const normalizeText = (value: unknown) => String(value ?? '').trim()
  const normalizeSkuId = (value: unknown) => {
    const id = Math.trunc(Number(value || 0))
    return Number.isFinite(id) && id > 0 ? id : 0
  }
  const normalizeSkuCode = (value: unknown) => normalizeText(value).toLowerCase()
  const allWholesalePrices = (product: any): any[] => {
    if (Array.isArray(product?.wholesale_prices)) return product.wholesale_prices
    if (Array.isArray(product?.wholesalePrices)) return product.wholesalePrices
    return []
  }
  const isUniversalWholesaleTier = (tier: any) => normalizeSkuId(tier?.sku_id) <= 0 && normalizeText(tier?.sku_code) === ''
  const wholesaleTierMatchesSku = (tier: any, skuId?: any, skuCode?: any) => {
    if (isUniversalWholesaleTier(tier)) return true
    const tierSkuId = normalizeSkuId(tier?.sku_id)
    const currentSkuId = normalizeSkuId(skuId)
    if (tierSkuId > 0 && currentSkuId > 0 && tierSkuId === currentSkuId) return true
    const tierSkuCode = normalizeSkuCode(tier?.sku_code)
    return Boolean(tierSkuCode && tierSkuCode === normalizeSkuCode(skuCode))
  }
  const hasSkuSpecificWholesaleTier = (tiers: any[], skuId?: any, skuCode?: any) => {
    return tiers.some((tier) => !isUniversalWholesaleTier(tier) && wholesaleTierMatchesSku(tier, skuId, skuCode))
  }
  const getWholesalePrices = (product: any, skuId?: any, skuCode?: any): any[] => {
    const tiers = allWholesalePrices(product)
    const hasSku = normalizeSkuId(skuId) > 0 || normalizeText(skuCode) !== ''
    if (!hasSku) return tiers

    const hasSpecific = hasSkuSpecificWholesaleTier(tiers, skuId, skuCode)
    return tiers.filter((tier) => {
      if (hasSpecific) return !isUniversalWholesaleTier(tier) && wholesaleTierMatchesSku(tier, skuId, skuCode)
      return isUniversalWholesaleTier(tier)
    })
  }
  const hasWholesalePrices = (product: any) => getWholesalePrices(product).length > 0

  const resolveWholesalePriceAmount = (product: any, basePrice: any, matchQuantity: number, skuId?: any, skuCode?: any, lineQuantity?: number) => {
    const base = parsePriceAmount(basePrice)
    if (base === null || !Number.isFinite(matchQuantity) || matchQuantity <= 0) return null
    const hasSku = normalizeSkuId(skuId) > 0 || normalizeText(skuCode) !== ''
    const tiers = hasSku ? getWholesalePrices(product, skuId, skuCode) : allWholesalePrices(product).filter(isUniversalWholesaleTier)
    let matchedCents: number | null = null
    for (const tier of tiers) {
      const minQuantity = Number(tier?.min_quantity || 0)
      const priceCents = parsePriceAmount(tier?.unit_price)
      if (!Number.isFinite(minQuantity) || minQuantity <= 0 || priceCents === null) continue
      const tierMatchQuantity = isUniversalWholesaleTier(tier) ? matchQuantity : Number(lineQuantity ?? matchQuantity)
      if (!Number.isFinite(tierMatchQuantity) || tierMatchQuantity < minQuantity) continue
      if (matchedCents === null || priceCents < matchedCents) {
        matchedCents = priceCents
      }
    }
    if (matchedCents === null || matchedCents >= base) return null
    return centsToAmount(matchedCents)
  }

  // 列表卡片展示价：取专属价与会员价中更低者，且必须低于活动价/原价才展示。
  // 口径与详情页和后端下单保持一致，具体规则见 resolveCardDisplayPrice。
  const getCardExclusivePrice = (product: any): string | null => {
    if (!userAuthStore.isAuthenticated) return null
    const memberLevelId = Number(userAuthStore.user?.member_level_id || 0)
    const memberLevels: any[] = userProfileStore.memberLevels ?? []
    const level = memberLevelId > 0 ? memberLevels.find((l: any) => Number(l?.id || 0) === memberLevelId) : null
    const discountRate = level ? Number(level.discount_rate || 0) : 0
    const userPrices = Array.isArray(userProfileStore.myProductPrices) ? userProfileStore.myProductPrices : []
    return resolveCardDisplayPrice(product, memberLevelId, discountRate, userPrices)
  }

  return {
    getCardExclusivePrice,
    getPurchaseTypeLabel,
    getFulfillmentTypeLabel,
    getStockBadgeVariant,
    getStockStatusLabel,
    isSoldOut,
    hasPromotionPrice,
    getPromotionPriceAmount,
    getPromotionSaveAmount,
    hasSkuPromotionPrice,
    getSkuPromotionPriceAmount,
    getSkuPromotionSaveAmount,
    hasPromotionRules,
    getPromotionRules,
    hasWholesalePrices,
    getWholesalePrices,
    resolveWholesalePriceAmount,
    resolveMemberPriceAmount,
  }
}
