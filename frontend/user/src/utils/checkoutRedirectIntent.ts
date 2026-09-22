const INTENT_KEY = 'checkout-created-payment-redirect-v1'
const MAX_AGE_MS = 60_000

type RedirectStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

export const createCheckoutRedirectIntent = (storage: RedirectStorage, orderNo: unknown, paymentId: unknown, now = Date.now()) => {
  const order = String(orderNo || '').trim()
  const payment = Number(paymentId)
  if (!order || !Number.isSafeInteger(payment) || payment <= 0) return
  storage.setItem(INTENT_KEY, JSON.stringify({ order, payment, created: now }))
}

export const consumeCheckoutRedirectIntent = (storage: RedirectStorage, orderNo: unknown, paymentId: unknown, now = Date.now()) => {
  const raw = storage.getItem(INTENT_KEY)
  if (!raw) return false
  storage.removeItem(INTENT_KEY)
  try {
    const intent = JSON.parse(raw)
    const age = now - Number(intent.created)
    return age >= 0 && age <= MAX_AGE_MS && intent.order === String(orderNo || '').trim() && intent.payment === Number(paymentId)
  } catch {
    return false
  }
}
