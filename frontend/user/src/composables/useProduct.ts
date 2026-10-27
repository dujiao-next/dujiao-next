import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '../stores/app'
import { amountToCents, centsToAmount } from '../utils/money'
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
  if (userAuthStore.isAuthenticated) void userProfileStore.loadMyProductPrices()
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

  // 用户专属一口价：命中后直接作为最终价，不再叠加批发价、活动价与会员等级价（口径与下单一致）。
  // 仅当专属价低于该 SKU 的原价时生效；已存在 SKU 级专属价时不回退到商品级。
  const resolveUserPriceAmount = (product: any, skuId: any, userPrices?: any[]) => {
    if (!Array.isArray(userPrices) || userPrices.length === 0) return null
    const productId = Number(product?.id || 0)
    const normalizedSkuId = Number(skuId || 0)
    if (productId <= 0) return null
    const rows = userPrices.filter((p: any) => Number(p?.product_id || 0) === productId)
    const row = (normalizedSkuId > 0 ? rows.find((p: any) => Number(p?.sku_id || 0) === normalizedSkuId) : undefined)
      ?? rows.find((p: any) => Number(p?.sku_id || 0) === 0)
    const priceCents = parsePriceAmount(row?.price_amount)
    if (priceCents === null || priceCents <= 0) return null
    const skus = Array.isArray(product?.skus) ? product.skus : []
    const sku = skus.find((s: any) => Number(s?.id || 0) === normalizedSkuId)
    const skuBase = parsePriceAmount(sku?.price_amount ?? product?.price_amount)
    if (skuBase === null || priceCents >= skuBase) return null
    return centsToAmount(priceCents)
  }

  const resolveMemberPriceAmount = (product: any, skuId: any, basePrice: any, memberLevelId: any, discountRate?: any, userPrices?: any[]) => {
    const exclusive = resolveUserPriceAmount(product, skuId, userPrices)
    if (exclusive !== null) return exclusive

    const base = parsePriceAmount(basePrice)
    const levelId = Number(memberLevelId || 0)
    if (base === null || base <= 0 || !Number.isFinite(levelId) || levelId <= 0) return null

    const prices = Array.isArray(product?.member_prices) ? product.member_prices : []
    const normalizedSkuId = Number(skuId || 0)
    const skuPrice = prices.find((p: any) => Number(p?.member_level_id || 0) === levelId && Number(p?.sku_id || 0) === normalizedSkuId)
    const productPrice = prices.find((p: any) => Number(p?.member_level_id || 0) === levelId && Number(p?.sku_id || 0) === 0)
    const overrideCents = parsePriceAmount(skuPrice?.price_amount ?? productPrice?.price_amount)
    if (overrideCents !== null && overrideCents > 0) {
      return overrideCents < base ? centsToAmount(overrideCents) : null
    }

    const rate = Number(discountRate || 0)
    if (!Number.isFinite(rate) || rate <= 0 || rate >= 100) return null
    const memberCents = Math.round((base * rate) / 100)
    return memberCents < base ? centsToAmount(memberCents) : null
  }

  // 列表卡片展示价：用户有专属一口价时优先于活动价；取各 SKU 中最低的专属价，且必须低于当前展示价才生效。
  const getCardExclusivePrice = (product: any): string | null => {
    if (!userAuthStore.isAuthenticated) return null
    const prices = userProfileStore.myProductPrices
    if (!Array.isArray(prices) || prices.length === 0) return null
    const skus = (Array.isArray(product?.skus) ? product.skus : []).filter((s: any) => s?.is_active !== false)
    const skuIds: any[] = skus.length > 0 ? skus.map((s: any) => s.id) : [0]
    let best: number | null = null
    for (const id of skuIds) {
      const cents = parsePriceAmount(resolveUserPriceAmount(product, id, prices))
      if (cents !== null && (best === null || cents < best)) best = cents
    }
    if (best === null) return null
    const current = parsePriceAmount(hasPromotionPrice(product) ? getPromotionPriceAmount(product) : product?.price_amount)
    if (current !== null && best >= current) return null
    return centsToAmount(best)
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
