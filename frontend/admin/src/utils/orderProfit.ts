import type { AdminOrder, AdminOrderItem } from '@/api/types'

// 订单详情的利润口径。退款只记录在订单层，这里把退款分摊到订单项，
// 并按「退款冲回成本」开关决定是否同比例冲回成本，口径与后台仪表盘一致
// （internal/modules/dashboard/infrastructure/gormstore/profit.go）。

// toNumber 把金额字段（字符串或数字）安全转成数值，空值与非法值算 0。
// 订单详情弹窗也用它解析金额，避免两处各写一份。
export const toNumber = (value?: string | number | null) => {
  if (value === null || value === undefined || value === '') return 0
  const parsed = Number(value)
  return Number.isNaN(parsed) ? 0 : parsed
}

const toCents = (amount: number) => Math.round(amount * 100)
const fromCents = (cents: number) => Number((cents / 100).toFixed(2))

export const itemRevenueAmount = (item: AdminOrderItem) =>
  toNumber(item.total_price) - toNumber(item.coupon_discount_amount)

export const itemCostAmount = (item: AdminOrderItem) =>
  toNumber(item.cost_price) * (item.quantity || 1)

// 按权重把 totalCents 分给各项，末项吃掉尾差；权重全为 0 时平均分。
const distributeCents = (totalCents: number, weights: number[]) => {
  const shares = weights.map(() => 0)
  if (totalCents <= 0 || weights.length === 0) return shares
  const totalWeight = weights.reduce((sum, value) => sum + value, 0)
  if (totalWeight <= 0) {
    const base = Math.floor(totalCents / weights.length)
    let remainder = totalCents - base * weights.length
    return shares.map(() => {
      const extra = remainder > 0 ? 1 : 0
      if (remainder > 0) remainder -= 1
      return base + extra
    })
  }
  let assigned = 0
  return shares.map((_, index) => {
    if (index === weights.length - 1) return Math.max(totalCents - assigned, 0)
    const share = Math.floor((totalCents * (weights[index] ?? 0)) / totalWeight)
    assigned += share
    return share
  })
}

const weightsOf = (items: AdminOrderItem[]) => items.map((item) => Math.max(itemRevenueAmount(item), 0))

/**
 * 把订单已退款金额分摊到订单项，返回 item.id -> 分摊金额。
 *
 * 父订单退款只记在父订单上，子订单的 refunded_amount 不会跟着变；
 * 直接退子订单则两边都会累加。所以先把子订单自己的退款精确分给其订单项，
 * 剩余的（父订单层）退款再按实付金额权重分给全部订单项。
 */
export const buildItemRefundAllocation = (order: AdminOrder | null | undefined) => {
  const allocation = new Map<number, number>()
  if (!order) return allocation

  const children = (order.children || []).filter((child) => child.items && child.items.length > 0)
  const groups: { items: AdminOrderItem[]; ownRefundCents: number }[] =
    children.length > 0
      ? children.map((child) => ({ items: child.items || [], ownRefundCents: toCents(Math.max(toNumber(child.refunded_amount), 0)) }))
      : [{ items: order.items || [], ownRefundCents: 0 }]
  const allItems = groups.flatMap((group) => group.items)
  if (allItems.length === 0) return allocation

  const refundedCents = toCents(Math.max(toNumber(order.refunded_amount), 0))
  if (refundedCents <= 0) return allocation

  const cents = new Map<number, number>()
  const add = (item: AdminOrderItem, value: number) => cents.set(item.id, (cents.get(item.id) || 0) + value)

  let ownTotal = 0
  if (children.length > 0) {
    groups.forEach((group) => {
      ownTotal += group.ownRefundCents
      distributeCents(group.ownRefundCents, weightsOf(group.items)).forEach((value, index) => add(group.items[index]!, value))
    })
  }
  distributeCents(Math.max(refundedCents - ownTotal, 0), weightsOf(allItems)).forEach((value, index) => add(allItems[index]!, value))

  cents.forEach((value, id) => allocation.set(id, fromCents(value)))
  return allocation
}

/**
 * 订单项利润 = 实付 - 成本 - 退款分摊 + 冲回成本。
 * 冲回成本仅在 reversesCost 开启时生效，比例为该项被退款的占比（全额退款即冲回全部成本）。
 */
export const calcItemProfit = (
  order: AdminOrder | null | undefined,
  item: AdminOrderItem,
  allocation: Map<number, number>,
  reversesCost: boolean,
) => {
  const revenue = itemRevenueAmount(item)
  const cost = itemCostAmount(item)
  const refund = allocation.get(item.id) || 0
  let reversed = 0
  // 实付为 0 的订单项（例如被定向优惠券抵成 0）分不到退款，
  // 但它整单退款时同样要冲回成本，否则利润永远比仪表盘少一块。
  if (reversesCost && cost !== 0 && (refund > 0 || revenue <= 0)) {
    let ratio: number
    if (revenue > 0) {
      ratio = refund / revenue
    } else {
      // 实付为 0 的订单项没有可比金额，按订单整体退款比例冲回，与后端一致。
      const total = toNumber(order?.total_amount)
      ratio = total > 0 ? toNumber(order?.refunded_amount) / total : 0
    }
    reversed = cost * Math.min(Math.max(ratio, 0), 1)
  }
  return Number((revenue - cost - refund + reversed).toFixed(2))
}
