import test from 'node:test'
import assert from 'node:assert/strict'
import { resolveDetailPrice } from '../src/utils/detailPrice.ts'

test('detail price: no sku falls back to product price or product promotion', () => {
  assert.deepEqual(resolveDetailPrice({ productPrice: '10.00' }), { final: '10.00', original: null, kind: '', saved: null })
  assert.deepEqual(
    resolveDetailPrice({ productPrice: '10.00', productPromotion: { price: '8.00', save: '2.00' } }),
    { final: '8.00', original: '10.00', kind: 'promotion', saved: '2.00' },
  )
})

test('detail price: wholesale beats promotion and member', () => {
  const price = resolveDetailPrice({
    skuPrice: '10.00',
    wholesale: { final: '7.00', isMember: false },
    skuPromotion: { price: '8.00', save: '2.00' },
    memberPrice: '9.00',
  })
  assert.deepEqual(price, { final: '7.00', original: '10.00', kind: 'wholesale', saved: 3 })
  assert.equal(resolveDetailPrice({ skuPrice: '10.00', wholesale: { final: '6.50', isMember: true } }).kind, 'member')
})

test('detail price: promotion beats member unless member final is lower', () => {
  assert.deepEqual(
    resolveDetailPrice({ skuPrice: '10.00', skuPromotion: { price: '8.00', save: '2.00' }, memberPrice: '9.00' }),
    { final: '8.00', original: '10.00', kind: 'promotion', saved: '2.00' },
  )
  assert.deepEqual(
    resolveDetailPrice({ skuPrice: '10.00', skuPromotion: { price: '8.00', save: '2.00', memberFinal: '7.50' } }),
    { final: '7.50', original: '10.00', kind: 'member', saved: 2.5 },
  )
})

test('detail price: member price, then plain sku price', () => {
  assert.deepEqual(resolveDetailPrice({ skuPrice: '10.00', memberPrice: '9.00' }), { final: '9.00', original: '10.00', kind: 'member', saved: 1 })
  assert.deepEqual(resolveDetailPrice({ skuPrice: '10.00', productPrice: '12.00' }), { final: '10.00', original: null, kind: '', saved: null })
})
