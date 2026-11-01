import test from 'node:test'
import assert from 'node:assert/strict'
import { exactTokenAmount, parseCashierSnapshot, visibleCashierDetails } from '../src/utils/fixedCashier.ts'

const quote = (network: 'tron' | 'binance' = 'binance') => ({
 payment_id: 7, state: 'ready', server_time_ms: 1000000, display_until_ms: 1015000,
 details: { network, token: 'USDT', contract: network === 'tron' ? 'TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t' : '0x55d398326f99059ff775485246999027b3197955',
 recipient: network === 'tron' ? 'TBXSw8fM4jpQkGc6zZjsVABFpVN7UvXPdV' : '0x1111111111111111111111111111111111111111',
 raw_amount: network === 'tron' ? '14000100' : '14000100000000000000', decimals: network === 'tron' ? 6 : 18,
 amount: '14.0001', fiat_amount: '100.00', fiat_currency: 'CNY', expires_at_ms: 1600000 }
})

test('exact token digits survive amounts beyond Number precision', () => {
 assert.equal(exactTokenAmount('9007199254740993123456789', 18), '9007199.254740993123456789')
 assert.equal(exactTokenAmount('1', 18), '0.000000000000000001')
 assert.equal(exactTokenAmount('14000100', 6), '14.0001')
 for (const bad of ['1e18', '0', '-1', '1.0', ' 100', '001']) assert.throws(() => exactTokenAmount(bad, 18))
})

test('both networks provide the same exact display and copy value with a short lease', () => {
 for (const network of ['tron', 'binance'] as const) {
  const state = parseCashierSnapshot(quote(network), 7, 100, 350)
  assert.equal(visibleCashierDetails(state, 15099)?.amount, '14.0001')
  assert.equal(visibleCashierDetails(state, 15100), null)
 }
})

test('paused, failed, mismatched or slow responses cannot preserve payment details', () => {
 for (const state of ['paused', 'preparing', 'expired', 'closed', 'unavailable', 'success', '']) {
  const parsed = parseCashierSnapshot({ ...quote(), state }, 7, 100, 101)
  assert.equal(visibleCashierDetails(parsed, 101), null)
 }
 assert.equal(parseCashierSnapshot(quote(), 8, 100, 101).details, null)
 assert.equal(parseCashierSnapshot(quote(), 7, 100, 15100).details, null)
 assert.equal(parseCashierSnapshot({ ...quote(), display_until_ms: 999999 }, 7, 100, 101).details, null)
})

test('tampered instructions and gateway fallback fields are rejected', () => {
 const mutations = [
  { network: 'ethereum' }, { token: 'USDC' }, { decimals: 6 }, { recipient: 'https://attacker.example/qr.png' },
  { contract: '0x2222222222222222222222222222222222222222' }, { amount: '14.00' }, { raw_amount: 14000100000000000000 },
  { amount: '14.0001<script>' }, { fiat_currency: 'USDT' }, { expires_at_ms: '1600000' }
 ]
 for (const mutation of mutations) {
  const data = quote()
  assert.equal(parseCashierSnapshot({ ...data, details: { ...data.details, ...mutation } }, 7, 100, 101).details, null)
 }
 assert.equal(parseCashierSnapshot({ payment_id: 7, wallet_address: 'attacker', chain_amount: '1', qr_code: 'attacker' }, 7, 100, 101).details, null)
})
