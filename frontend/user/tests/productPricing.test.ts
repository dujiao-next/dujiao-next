import test from 'node:test'
import assert from 'node:assert/strict'
import {
  resolveMemberPriceAmount,
  resolveMemberLevelPriceCents,
  resolveUserPriceCents,
} from '../src/utils/productPricing.ts'

const product = {
  id: 18,
  price_amount: '100.00',
  skus: [
    { id: 19, price_amount: '100.00', is_active: true },
    { id: 20, price_amount: '200.00', is_active: true },
  ],
}

test('专属价低于会员价时显示专属价', () => {
  const userPrices = [{ product_id: 18, sku_id: 0, price_amount: '90.00' }]
  // 会员 8 折 = 80，专属价 90 更高 → 展示会员价
  assert.equal(resolveMemberPriceAmount(product, 19, '100.00', 3, 80, userPrices), '80.00')
  // 专属价 70 更低 → 展示专属价
  const cheaper = [{ product_id: 18, sku_id: 0, price_amount: '70.00' }]
  assert.equal(resolveMemberPriceAmount(product, 19, '100.00', 3, 80, cheaper), '70.00')
})

test('会员价比专属价低时不使用专属价', () => {
  const userPrices = [{ product_id: 18, sku_id: 0, price_amount: '90.00' }]
  assert.equal(resolveMemberPriceAmount(product, 19, '100.00', 3, 80, userPrices), '80.00')
})

test('没有会员等级时回退到专属价', () => {
  const userPrices = [{ product_id: 18, sku_id: 0, price_amount: '90.00' }]
  assert.equal(resolveMemberPriceAmount(product, 19, '100.00', 0, 0, userPrices), '90.00')
})

test('专属价必须严格低于基础价才生效', () => {
  const equalPrice = [{ product_id: 18, sku_id: 0, price_amount: '100.00' }]
  assert.equal(resolveUserPriceCents(product, 19, equalPrice), null)
  const abovePrice = [{ product_id: 18, sku_id: 0, price_amount: '120.00' }]
  assert.equal(resolveUserPriceCents(product, 19, abovePrice), null)
  const lower = [{ product_id: 18, sku_id: 0, price_amount: '95.00' }]
  assert.equal(resolveUserPriceCents(product, 19, lower), 9500)
})

test('SKU 级专属价优先于商品级，且不跨商品匹配', () => {
  const userPrices = [
    { product_id: 18, sku_id: 0, price_amount: '95.00' },
    { product_id: 18, sku_id: 19, price_amount: '90.00' },
    { product_id: 99, sku_id: 0, price_amount: '1.00' },
  ]
  assert.equal(resolveUserPriceCents(product, 19, userPrices), 9000)
  // SKU 20 没有 SKU 级价，回退商品级 95
  assert.equal(resolveUserPriceCents(product, 20, userPrices), 9500)
  assert.equal(resolveUserPriceCents({ ...product, id: 77 }, 19, userPrices), null)
})

test('会员等级覆盖价优先于折扣率', () => {
  const withOverride = {
    ...product,
    member_prices: [{ member_level_id: 3, sku_id: 0, price_amount: '75.00' }],
  }
  assert.equal(resolveMemberLevelPriceCents(withOverride, 19, 10000, 3, 80), 7500)
  // 覆盖价不低于基础价时不生效，也不回退到折扣率
  const highOverride = {
    ...product,
    member_prices: [{ member_level_id: 3, sku_id: 0, price_amount: '150.00' }],
  }
  assert.equal(resolveMemberLevelPriceCents(highOverride, 19, 10000, 3, 80), null)
})

test('专属价高于传入的活动价/批发价时不使用专属价', () => {
  // 原价 100，活动价 80，专属价 90：后端下单收 80，详情页应回退到活动价
  const userPrices = [{ product_id: 18, sku_id: 0, price_amount: '90.00' }]
  assert.equal(resolveMemberPriceAmount(product, 19, '80.00', 0, 0, userPrices), null)
  // 专属价 70 仍低于活动价 80 → 使用专属价
  const cheaper = [{ product_id: 18, sku_id: 0, price_amount: '70.00' }]
  assert.equal(resolveMemberPriceAmount(product, 19, '80.00', 0, 0, cheaper), '70.00')
})

test('无效输入返回 null', () => {
  assert.equal(resolveMemberPriceAmount(product, 19, '0', 3, 80, []), null)
  assert.equal(resolveMemberPriceAmount(product, 19, undefined, 3, 80, []), null)
  assert.equal(resolveMemberPriceAmount(product, 19, '100.00', 3, 100, []), null)
  assert.equal(resolveUserPriceCents(product, 19, []), null)
  assert.equal(resolveUserPriceCents(product, 19, undefined), null)
})
