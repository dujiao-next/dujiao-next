export type DetailPriceKind = 'member' | 'wholesale' | 'promotion' | ''

export interface DetailPriceInput {
  /** 选中规格的原价；未选规格时不传 */
  skuPrice?: unknown
  wholesale?: { final: unknown; isMember: boolean }
  /** memberFinal：会员价比促销价更低时的成交价 */
  skuPromotion?: { price: unknown; save: unknown; memberFinal?: unknown }
  memberPrice?: unknown
  productPrice?: unknown
  productPromotion?: { price: unknown; save: unknown }
}

export interface DetailPrice {
  final: unknown
  original: unknown
  kind: DetailPriceKind
  saved: unknown
}

/** 商品详情页展示的价格。优先级：批发 > 促销 > 会员 > 原价 */
export function resolveDetailPrice(input: DetailPriceInput): DetailPrice {
  const { skuPrice, wholesale, skuPromotion, memberPrice, productPrice, productPromotion } = input
  const plain = (final: unknown): DetailPrice => ({ final, original: null, kind: '', saved: null })

  if (skuPrice === undefined || skuPrice === null) {
    return productPromotion
      ? { final: productPromotion.price, original: productPrice, kind: 'promotion', saved: productPromotion.save }
      : plain(productPrice)
  }

  const off = (final: unknown, kind: DetailPriceKind): DetailPrice =>
    ({ final, original: skuPrice, kind, saved: Number(skuPrice) - Number(final) })

  if (wholesale) return off(wholesale.final, wholesale.isMember ? 'member' : 'wholesale')
  if (skuPromotion) {
    return skuPromotion.memberFinal !== undefined && skuPromotion.memberFinal !== null
      ? off(skuPromotion.memberFinal, 'member')
      : { final: skuPromotion.price, original: skuPrice, kind: 'promotion', saved: skuPromotion.save }
  }
  if (memberPrice !== undefined && memberPrice !== null) return off(memberPrice, 'member')
  return plain(skuPrice)
}
