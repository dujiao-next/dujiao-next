export type CashierState = 'ready' | 'preparing' | 'paused' | 'expired' | 'closed' | 'unavailable'
export interface CashierDetails {
  network: 'tron' | 'binance'
  token: 'USDT'
  contract: string
  recipient: string
  amount: string
  raw_amount: string
  decimals: number
  fiat_amount: string
  fiat_currency: string
  expires_at_ms: number
}

export interface CashierSnapshot {
  state: CashierState
  details: CashierDetails | null
  // Monotonic local deadline; changing the computer clock cannot extend it.
  deadline: number
}

const unavailable = (): CashierSnapshot => ({ state: 'unavailable', details: null, deadline: 0 })
const contracts = {
  tron: 'TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t',
  binance: '0x55d398326f99059ff775485246999027b3197955',
}

export function exactTokenAmount(raw: string, decimals: number): string {
  if (!/^[1-9][0-9]{0,79}$/.test(raw) || ![6, 18].includes(decimals)) throw new Error('Invalid token amount')
  const digits = raw.padStart(decimals + 1, '0')
  const fraction = digits.slice(-decimals).replace(/0+$/, '')
  return digits.slice(0, -decimals) + (fraction ? `.${fraction}` : '')
}

// Reject mismatched/stale replies. Neither gateway URLs nor old generic payment
// payloads are accepted as fallback instructions.
export function parseCashierSnapshot(input: unknown, paymentID: number, started: number, received: number): CashierSnapshot {
  if (!input || typeof input !== 'object' || !Number.isSafeInteger(paymentID) || paymentID <= 0) return unavailable()
  const data = input as Record<string, any>
  if (data.payment_id !== paymentID) return unavailable()
  if (['preparing', 'paused', 'expired', 'closed', 'unavailable'].includes(data.state)) {
    return { state: data.state, details: null, deadline: 0 }
  }
  const d = data.details as CashierDetails | undefined
  if (data.state !== 'ready' || !d || !['tron', 'binance'].includes(d.network) || d.token !== 'USDT') return unavailable()
  const decimals = d.network === 'tron' ? 6 : 18
  if (d.decimals !== decimals || d.contract !== contracts[d.network]) return unavailable()
  if (typeof d.recipient !== 'string' || !(d.network === 'tron' ? /^T[1-9A-HJ-NP-Za-km-z]{33}$/ : /^0x[0-9a-f]{40}$/).test(d.recipient)) return unavailable()
  if (typeof d.raw_amount !== 'string' || typeof d.amount !== 'string') return unavailable()
  try { if (exactTokenAmount(d.raw_amount, decimals) !== d.amount) return unavailable() } catch { return unavailable() }
  if (typeof d.fiat_amount !== 'string' || !/^(0|[1-9][0-9]*)(\.[0-9]{1,2})?$/.test(d.fiat_amount) || !['CNY', 'USD'].includes(d.fiat_currency)) return unavailable()
  if (![data.server_time_ms, data.display_until_ms, d.expires_at_ms].every(Number.isSafeInteger) || !Number.isFinite(started) || !Number.isFinite(received) || received < started) return unavailable()
  const lifetime = Math.min(data.display_until_ms - data.server_time_ms, d.expires_at_ms - data.server_time_ms, 15000)
  // Subtract the entire request duration, conservatively including server time.
  const deadline = started + lifetime
  if (lifetime <= 0 || received >= deadline) return unavailable()
  return { state: 'ready', details: { ...d }, deadline }
}

export function visibleCashierDetails(snapshot: CashierSnapshot, now: number): CashierDetails | null {
  return snapshot.state === 'ready' && Number.isFinite(now) && now < snapshot.deadline ? snapshot.details : null
}

export const isFixedPayment = (payment: unknown): boolean => !!payment && typeof payment === 'object' && (payment as { fixed_payment?: unknown }).fixed_payment === true
