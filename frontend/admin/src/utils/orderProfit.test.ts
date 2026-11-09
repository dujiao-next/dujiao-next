import { describe, expect, it } from 'vitest'
import type { AdminOrder, AdminOrderItem } from '@/api/types'
import { buildItemRefundAllocation, calcItemProfit } from './orderProfit'

const makeItem = (id: number, total: number, cost: number, quantity = 1): AdminOrderItem =>
  ({ id, total_price: total, cost_price: cost, quantity, coupon_discount_amount: 0 }) as unknown as AdminOrderItem

const makeOrder = (patch: Partial<AdminOrder>): AdminOrder => ({ id: 1, total_amount: 0, refunded_amount: 0, ...patch }) as unknown as AdminOrder

const profitOf = (order: AdminOrder, item: AdminOrderItem, reversesCost: boolean) =>
  calcItemProfit(order, item, buildItemRefundAllocation(order), reversesCost)

// 每个订单都是「父订单 + 子订单」，父订单的 items 由子订单聚合而来（id 相同）。
const familyOf = (children: AdminOrder[], patch: Partial<AdminOrder>) => {
  const items = children.flatMap((child) => child.items || [])
  return makeOrder({ ...patch, items, children })
}

describe('订单详情利润', () => {
  it('issue #386：价格 50、成本 30、全额退款 50，开启冲回成本后利润为 0', () => {
    const item = makeItem(1, 50, 30)
    const child = makeOrder({ id: 2, total_amount: 50, refunded_amount: 0, items: [item] })
    const order = familyOf([child], { total_amount: 50, refunded_amount: 50 })
    expect(profitOf(order, item, true)).toBe(0)
  })

  it('关闭冲回成本时保持原口径：成本视为已发生，利润为 -30', () => {
    const item = makeItem(1, 50, 30)
    const child = makeOrder({ id: 2, total_amount: 50, items: [item] })
    const order = familyOf([child], { total_amount: 50, refunded_amount: 50 })
    expect(profitOf(order, item, false)).toBe(-30)
  })

  it('没有退款时两种口径一致', () => {
    const item = makeItem(1, 50, 30)
    const order = makeOrder({ total_amount: 50, items: [item] })
    expect(profitOf(order, item, true)).toBe(20)
    expect(profitOf(order, item, false)).toBe(20)
  })

  it('部分退款按比例冲回成本：退 20，利润 = 50-30-20+12 = 12', () => {
    const item = makeItem(1, 50, 30)
    const order = makeOrder({ total_amount: 50, refunded_amount: 20, items: [item] })
    expect(profitOf(order, item, true)).toBe(12)
    expect(profitOf(order, item, false)).toBe(0)
  })

  it('成本乘以数量：2 件、成本 15、实付 100，全额退款后利润为 0', () => {
    const item = makeItem(1, 100, 15, 2)
    const order = makeOrder({ total_amount: 100, refunded_amount: 100, items: [item] })
    expect(profitOf(order, item, true)).toBe(0)
  })

  it('实付为 0 的订单项整单退款时也要冲回成本', () => {
    // A：实付 50、成本 30；B：原价 10 被定向优惠券抵成 0、成本 6。
    // 整单退 50：仪表盘成本基数 36 全部冲回，利润 0；订单详情两边必须一致。
    const a = makeItem(1, 50, 30)
    const b = { ...makeItem(2, 10, 6), coupon_discount_amount: 10 } as unknown as AdminOrderItem
    const order = familyOf(
      [
        makeOrder({ id: 11, total_amount: 50, items: [a] }),
        makeOrder({ id: 12, total_amount: 0, items: [b] }),
      ],
      { total_amount: 50, refunded_amount: 50 },
    )
    // A 分到 50，利润 50-30-50+30 = 0
    expect(profitOf(order, a, true)).toBe(0)
    // B 实付 0 分不到退款，但同样要冲回成本：0-6-0+6 = 0
    expect(profitOf(order, b, true)).toBe(0)
    // 关闭冲回成本时 B 保持成本已发生
    expect(profitOf(order, b, false)).toBe(-6)
  })

  it('实付为 0 且未退款时不冲回成本', () => {
    const b = { ...makeItem(2, 10, 6), coupon_discount_amount: 10 } as unknown as AdminOrderItem
    const order = makeOrder({ total_amount: 0, refunded_amount: 0, items: [b] })
    expect(profitOf(order, b, true)).toBe(-6)
  })
})

describe('父子订单退款分摊', () => {
  it('父订单退款不会写入子订单 refunded_amount，分摊按 item.id 覆盖子订单项', () => {
    const a = makeItem(1, 60, 30)
    const b = makeItem(2, 40, 10)
    const order = familyOf(
      [
        makeOrder({ id: 11, total_amount: 60, refunded_amount: 0, items: [a] }),
        makeOrder({ id: 12, total_amount: 40, refunded_amount: 0, items: [b] }),
      ],
      { total_amount: 100, refunded_amount: 100 },
    )
    const allocation = buildItemRefundAllocation(order)
    expect(allocation.get(1)).toBe(60)
    expect(allocation.get(2)).toBe(40)
    expect(profitOf(order, a, true)).toBe(0)
    expect(profitOf(order, b, true)).toBe(0)
    expect(profitOf(order, a, false)).toBe(-30)
  })

  it('直接退子订单：金额精确落到该子订单的订单项，不被其他子订单稀释', () => {
    const a = makeItem(1, 60, 30)
    const b = makeItem(2, 40, 10)
    const order = familyOf(
      [
        makeOrder({ id: 11, total_amount: 60, refunded_amount: 60, items: [a] }),
        makeOrder({ id: 12, total_amount: 40, refunded_amount: 0, items: [b] }),
      ],
      { total_amount: 100, refunded_amount: 60 },
    )
    const allocation = buildItemRefundAllocation(order)
    expect(allocation.get(1)).toBe(60)
    expect(allocation.get(2) || 0).toBe(0)
    expect(profitOf(order, a, true)).toBe(0)
    expect(profitOf(order, b, true)).toBe(30)
  })

  it('分摊总和等于已退款金额，尾差不丢失', () => {
    const items = [makeItem(1, 33.33, 10), makeItem(2, 33.33, 10), makeItem(3, 33.34, 10)]
    const order = familyOf(
      items.map((item, index) => makeOrder({ id: 10 + index, total_amount: Number(item.total_price), items: [item] })),
      { total_amount: 100, refunded_amount: 50 },
    )
    const total = [...buildItemRefundAllocation(order).values()].reduce((sum, value) => sum + value, 0)
    expect(Number(total.toFixed(2))).toBe(50)
  })

  it('没有退款时分摊为空', () => {
    const order = makeOrder({ total_amount: 50, items: [makeItem(1, 50, 30)] })
    expect(buildItemRefundAllocation(order).size).toBe(0)
  })
})
