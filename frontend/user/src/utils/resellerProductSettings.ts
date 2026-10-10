const RESELLER_PRICING_MODE_INHERIT = 'inherit'
const RESELLER_PRICING_MODE_MARKUP_PERCENT = 'markup_percent'
const RESELLER_PRICING_MODE_FIXED_MARKUP = 'fixed_markup'
const RESELLER_PRICING_MODE_FIXED_PRICE = 'fixed_price'

type ResellerProductSettingDataLike = {
  is_listed?: boolean
  effective_price_amount?: string
  rule_source?: string
  pricing_mode?: string
}

type ResellerProductSettingProductLike = {
  id?: number
  price_amount?: string
  agency_price_amount?: string
}

type ResellerProductSettingSKULike = {
  id?: number
  is_active?: boolean
  base_price_amount?: string
  master_price_amount?: string
  effective_price_amount?: string
  setting?: ResellerProductSettingDataLike | null
}

type ResellerProductSettingDetailLike = {
  product?: ResellerProductSettingProductLike | null
  product_setting?: ResellerProductSettingDataLike | null
  skus?: ResellerProductSettingSKULike[]
}

type ResellerProductSettingPayloadItem = {
  sku_id: number
  is_listed: boolean
  pricing_mode: string
  markup_percent: string
  fixed_markup_amount: string
  fixed_price_amount: string
  sort_order: number
}

type ResellerProductSettingUpdatePayload = {
  settings: ResellerProductSettingPayloadItem[]
}

export type ResellerProductSettingsPagination = {
  page: number
  page_size: number
  total: number
  total_page: number
}

export type ResellerProductSettingFormItem = {
  sku_id: number
  is_listed?: boolean
  pricing_mode?: string
  markup_percent?: string
  fixed_markup_amount?: string
  fixed_price_amount?: string
  sort_order?: number
}

const moneyOrZero = (value?: string) => {
  const normalized = String(value || '').trim()
  return normalized || '0.00'
}

const normalizeMoneyText = (value?: string) => String(value || '').trim()

const uniqueMoneyValues = (items: string[]) => {
  const seen = new Set<string>()
  const out: string[] = []
  items.forEach((item) => {
    const value = normalizeMoneyText(item)
    if (!value || seen.has(value)) return
    seen.add(value)
    out.push(value)
  })
  return out
}

const sortMoneyValues = (items: string[]) => {
  return [...items].sort((a, b) => {
    const an = Number(a)
    const bn = Number(b)
    if (Number.isFinite(an) && Number.isFinite(bn)) return an - bn
    return a.localeCompare(b)
  })
}

const isSkuListed = (sku: ResellerProductSettingSKULike) => {
  if (sku.is_active === false) return false
  return sku.setting?.is_listed !== false
}

const isRecord = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value)

const positiveInt = (value: unknown, fallback: number) => {
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed > 0 ? Math.floor(parsed) : fallback
}

export const isResellerProductSettingDetail = (value: unknown): value is ResellerProductSettingDetailLike => {
  if (!isRecord(value)) return false
  if (!isRecord(value.product)) return false
  if (!Number.isFinite(Number(value.product.id))) return false
  return Array.isArray(value.skus)
}

export const normalizeResellerProductSettingsPagination = (
  value: unknown,
  fallback: ResellerProductSettingsPagination = { page: 1, page_size: 20, total: 0, total_page: 1 },
): ResellerProductSettingsPagination => {
  if (!isRecord(value)) return { ...fallback }
  return {
    page: positiveInt(value.page, fallback.page),
    page_size: positiveInt(value.page_size, fallback.page_size),
    total: Math.max(0, positiveInt(value.total, fallback.total || 0)),
    total_page: positiveInt(value.total_page, fallback.total_page || 1),
  }
}

export const getResellerPricingModeLabelKey = (mode?: string) => {
  if (mode === RESELLER_PRICING_MODE_INHERIT) return 'inherit'
  if (mode === RESELLER_PRICING_MODE_MARKUP_PERCENT) return 'markupPercent'
  if (mode === RESELLER_PRICING_MODE_FIXED_MARKUP) return 'fixedMarkup'
  if (mode === RESELLER_PRICING_MODE_FIXED_PRICE) return 'fixedPrice'
  return 'unknown'
}

export const normalizeResellerProductSettingForm = (raw: ResellerProductSettingFormItem): Required<ResellerProductSettingFormItem> => ({
  sku_id: Number(raw.sku_id || 0),
  is_listed: raw.is_listed !== false,
  pricing_mode: raw.pricing_mode || RESELLER_PRICING_MODE_INHERIT,
  markup_percent: moneyOrZero(raw.markup_percent),
  fixed_markup_amount: moneyOrZero(raw.fixed_markup_amount),
  fixed_price_amount: moneyOrZero(raw.fixed_price_amount),
  sort_order: Number(raw.sort_order || 0),
})

export const buildResellerProductSettingPayload = (items: ResellerProductSettingFormItem[]): ResellerProductSettingUpdatePayload => ({
  settings: items.map((item): ResellerProductSettingPayloadItem => {
    const normalized = normalizeResellerProductSettingForm(item)
    const useMarkupPercent = normalized.pricing_mode === RESELLER_PRICING_MODE_MARKUP_PERCENT
    const useFixedMarkup = normalized.pricing_mode === RESELLER_PRICING_MODE_FIXED_MARKUP
    const useFixedPrice = normalized.pricing_mode === RESELLER_PRICING_MODE_FIXED_PRICE
    return {
      sku_id: normalized.sku_id,
      is_listed: normalized.is_listed,
      pricing_mode: normalized.pricing_mode,
      markup_percent: useMarkupPercent ? moneyOrZero(normalized.markup_percent) : '0.00',
      fixed_markup_amount: useFixedMarkup ? moneyOrZero(normalized.fixed_markup_amount) : '0.00',
      fixed_price_amount: useFixedPrice ? moneyOrZero(normalized.fixed_price_amount) : '0.00',
      sort_order: normalized.sort_order,
    }
  }),
})

export const summarizeEffectivePrice = (setting?: ResellerProductSettingDataLike | null) => {
  const value = String(setting?.effective_price_amount || '').trim()
  return value || '-'
}

export const countListedSkus = (detail?: ResellerProductSettingDetailLike | null) => {
  const skus = detail?.skus || []
  if (skus.length === 0) {
    return detail?.product_setting?.is_listed === false ? 0 : 1
  }
  return skus.filter(isSkuListed).length
}

export const countActiveSkus = (detail?: ResellerProductSettingDetailLike | null) => {
  const skus = detail?.skus || []
  if (skus.length === 0) return 1
  return skus.filter((sku) => sku.is_active !== false).length
}

export const summarizeProductEffectivePrice = (
  detail?: ResellerProductSettingDetailLike | null,
  fallback = '-',
) => {
  const skus = detail?.skus || []
  const skuPrices = uniqueMoneyValues(
    skus
      .filter(isSkuListed)
      .map((sku) => sku.effective_price_amount || sku.setting?.effective_price_amount || ''),
  )
  if (skuPrices.length > 0) {
    const sorted = sortMoneyValues(skuPrices)
    return sorted.length === 1 ? sorted[0] : `${sorted[0]} - ${sorted[sorted.length - 1]}`
  }
  const productPrice = normalizeMoneyText(detail?.product_setting?.effective_price_amount)
  return productPrice || fallback
}

/**
 * 汇总商品主站基准价展示文本。
 * 1. 存在 SKU 时，取在售（或活跃）SKU 的基准价区间（如 250.00 - 500.00），单价相同则显示单一价格；
 * 2. 无 SKU 时，取商品本身的代理价或售价；
 * 3. 均无有效价格时回退至 fallback。
 */
export const summarizeProductBasePrice = (
  detail?: ResellerProductSettingDetailLike | null,
  fallback = '-',
) => {
  const skus = detail?.skus || []
  const listedSkus = skus.filter(isSkuListed)
  const targetSkus = listedSkus.length > 0 ? listedSkus : skus.filter((sku) => sku.is_active !== false)
  const skuPrices = uniqueMoneyValues(
    targetSkus.map((sku) => sku.base_price_amount || sku.master_price_amount || ''),
  )
  if (skuPrices.length > 0) {
    const sorted = sortMoneyValues(skuPrices)
    return sorted.length === 1 ? sorted[0] : `${sorted[0]} - ${sorted[sorted.length - 1]}`
  }
  const productBasePrice = normalizeMoneyText(
    detail?.product?.agency_price_amount || detail?.product?.price_amount,
  )
  return productBasePrice || fallback
}

export const getResellerRuleSourceLabelKey = (source?: string) => {
  if (source === 'sku') return 'skuRule'
  if (source === 'product') return 'productRule'
  if (source === 'profile') return 'defaultRule'
  return 'unknownRule'
}
