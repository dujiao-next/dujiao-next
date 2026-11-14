import { amountToCents, centsToAmount } from './money.ts'

const normalizeSkuId = (value: unknown) => {
  const id = Math.trunc(Number(value || 0))
  return Number.isFinite(id) && id > 0 ? id : 0
}

/**
 * 用户专属一口价（SKU 级优先于商品级，且必须低于该 SKU 的基础价）。
 * 返回分为单位的整数；未命中返回 null。
 */
export const resolveUserPriceCents = (
  product: any,
  skuId: any,
  userPrices?: any[],
): number | null => {
  if (!Array.isArray(userPrices) || userPrices.length === 0) return null
  const productId = Number(product?.id || 0)
  if (productId <= 0) return null
  const normalizedSkuId = normalizeSkuId(skuId)
  const rows = userPrices.filter((p: any) => Number(p?.product_id || 0) === productId)
  const row = (normalizedSkuId > 0
    ? rows.find((p: any) => Number(p?.sku_id || 0) === normalizedSkuId)
    : undefined)
    ?? rows.find((p: any) => Number(p?.sku_id || 0) === 0)
  const priceCents = amountToCents(row?.price_amount)
  if (priceCents === null || priceCents <= 0) return null
  const skus = Array.isArray(product?.skus) ? product.skus : []
  const sku = skus.find((s: any) => Number(s?.id || 0) === normalizedSkuId)
  const skuBase = amountToCents(sku?.price_amount ?? product?.price_amount)
  if (skuBase === null || priceCents >= skuBase) return null
  return priceCents
}

/**
 * 会员等级价（SKU 级覆盖价优先于等级折扣率），必须低于 base 才生效。
 * 返回分为单位的整数；未命中返回 null。
 */
export const resolveMemberLevelPriceCents = (
  product: any,
  skuId: any,
  baseCents: number,
  memberLevelId: any,
  discountRate?: any,
): number | null => {
  const levelId = Number(memberLevelId || 0)
  if (!Number.isFinite(levelId) || levelId <= 0) return null
  const prices = Array.isArray(product?.member_prices) ? product.member_prices : []
  const normalizedSkuId = normalizeSkuId(skuId)
  const skuPrice = prices.find((p: any) => Number(p?.member_level_id || 0) === levelId && Number(p?.sku_id || 0) === normalizedSkuId)
  const productPrice = prices.find((p: any) => Number(p?.member_level_id || 0) === levelId && Number(p?.sku_id || 0) === 0)
  const overrideCents = amountToCents(skuPrice?.price_amount ?? productPrice?.price_amount)
  if (overrideCents !== null && overrideCents > 0) {
    return overrideCents < baseCents ? overrideCents : null
  }
  const rate = Number(discountRate || 0)
  if (!Number.isFinite(rate) || rate <= 0 || rate >= 100) return null
  const memberCents = Math.round((baseCents * rate) / 100)
  return memberCents < baseCents ? memberCents : null
}

/**
 * 详情页/快速购买弹窗展示的最终会员价：专属价与会员等级价取更低者，
 * 与后端下单口径一致（专属于必须严格低于 base，会员价同样要低于 base）。
 */
export const resolveMemberPriceAmount = (
  product: any,
  skuId: any,
  basePrice: any,
  memberLevelId: any,
  discountRate?: any,
  userPrices?: any[],
): string | null => {
  const baseCents = amountToCents(basePrice)
  if (baseCents === null || baseCents <= 0) return null

  const memberCents = resolveMemberLevelPriceCents(product, skuId, baseCents, memberLevelId, discountRate)
  const exclusiveCents = resolveUserPriceCents(product, skuId, userPrices)

  // 专属价还要和传进来的 basePrice（详情页可能是活动价或批发价）比：
  // 后端下单先算活动价/批发价/会员价，只有专属价更低时才覆盖，否则收的是那个更低的价格。
  if (exclusiveCents !== null && exclusiveCents < baseCents && (memberCents === null || exclusiveCents < memberCents)) {
    return centsToAmount(exclusiveCents)
  }
  if (memberCents !== null) return centsToAmount(memberCents)
  return null
}

/**
 * 商品列表卡片的展示价：取各生效 SKU 中最低的「专属价 / 会员价」，且必须低于
 * 当前展示价（有活动价时是活动价，否则是原价）才返回；否则返回 null 表示不展示。
 * 口径与详情页、后端下单一致。
 *
 * 注意第三参数传的是「元」：resolveMemberPriceAmount 内部会自己转分。
 * 有活动价的 SKU 以活动价为基数，因为后端的会员折扣是叠加在活动价上的。
 */
export const resolveCardDisplayPrice = (
  product: any,
  memberLevelId: any,
  discountRate: any,
  userPrices?: any[],
): string | null => {
  const skus = (Array.isArray(product?.skus) ? product.skus : []).filter((s: any) => s?.is_active !== false)
  const candidates: Array<{ id: any; amount: any; promo: any }> = skus.length > 0
    ? skus.map((s: any) => ({ id: s.id, amount: s.price_amount, promo: s.promotion_price_amount }))
    : [{ id: 0, amount: product?.price_amount, promo: product?.promotion_price_amount }]

  let best: number | null = null
  for (const sku of candidates) {
    const amount = sku.amount ?? product?.price_amount
    const baseCents = amountToCents(amount)
    if (baseCents === null) continue
    // 后端的会员折扣叠加在活动价上，有活动价时要以活动价为基数，否则卡片会比实收贵。
    const promoCents = amountToCents(sku.promo)
    const effective = promoCents !== null && promoCents > 0 && promoCents < baseCents ? sku.promo : amount
    const resolved = amountToCents(
      resolveMemberPriceAmount(product, sku.id, effective, memberLevelId, discountRate, userPrices),
    )
    if (resolved !== null && resolved < baseCents && (best === null || resolved < best)) best = resolved
  }
  if (best === null) return null

  // 有活动价时门槛是活动价：专属价/会员价没比活动价更低就不展示。
  const originalCents = amountToCents(product?.price_amount)
  const promotionCents = amountToCents(product?.promotion_price_amount)
  const currentCents = promotionCents !== null && originalCents !== null && promotionCents < originalCents
    ? promotionCents
    : originalCents
  if (currentCents !== null && best >= currentCents) return null
  return centsToAmount(best)
}
