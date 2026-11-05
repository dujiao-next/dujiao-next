export type TranslateFn = (...args: any[]) => string

export const orderStatusLabel = (t: TranslateFn, status?: string) => {
  if (!status) return '-'
  const map: Record<string, string> = {
    pending_payment: t('order.status.pending_payment'),
    paid: t('order.status.paid'),
    fulfilling: t('order.status.fulfilling'),
    partially_delivered: t('order.status.partially_delivered'),
    partially_refunded: t('order.status.partially_refunded'),
    delivered: t('order.status.delivered'),
    completed: t('order.status.completed'),
    canceled: t('order.status.canceled'),
    refunded: t('order.status.refunded'),
  }
  return map[status] || status
}

export const orderStatusClass = (status?: string) => {
  switch (status) {
    case 'pending_payment':
      return 'text-amber-700 border-amber-200 bg-amber-50'
    case 'paid':
      return 'text-emerald-700 border-emerald-200 bg-emerald-50'
    case 'partially_delivered':
      return 'text-orange-700 border-orange-200 bg-orange-50'
    case 'partially_refunded':
      return 'text-orange-700 border-orange-200 bg-orange-50'
    case 'delivered':
    case 'completed':
      return 'text-slate-800 border-slate-200 bg-slate-50'
    case 'canceled':
      return 'text-slate-500 border-slate-200 bg-slate-50'
    case 'refunded':
      return 'text-blue-700 border-blue-200 bg-blue-50'
    default:
      return 'text-slate-600 border-slate-200 bg-slate-50'
  }
}

export const paymentStatusLabel = (t: TranslateFn, status?: string) => {
  if (!status) return '-'
  const map: Record<string, string> = {
    initiated: t('payment.status.initiated'),
    pending: t('payment.status.pending'),
    success: t('payment.status.success'),
    failed: t('payment.status.failed'),
    expired: t('payment.status.expired'),
  }
  return map[status] || status
}

// 支付异常码未配置翻译时原样显示，方便排查新增的异常码
export const paymentExceptionLabel = (t: TranslateFn, code?: string) => {
  if (!code) return '-'
  const key = `admin.payments.exceptions.${code}`
  const translated = t(key)
  return translated === key ? code : translated
}

export const paymentStatusClass = (status?: string) => {
  switch (status) {
    case 'success':
      return 'text-emerald-700 border-emerald-200 bg-emerald-50'
    case 'pending':
      return 'text-amber-700 border-amber-200 bg-amber-50'
    case 'failed':
      return 'text-rose-700 border-rose-200 bg-rose-50'
    case 'expired':
      return 'text-slate-500 border-slate-200 bg-slate-50'
    default:
      return 'text-slate-600 border-slate-200 bg-slate-50'
  }
}

export const userStatusLabel = (t: TranslateFn, status?: string) => {
  if (!status) return '-'
  const map: Record<string, string> = {
    active: t('admin.users.status.active'),
    disabled: t('admin.users.status.disabled'),
  }
  return map[status] || status
}

export const userStatusClass = (status?: string) => {
  if (status === 'active') return 'text-emerald-700 border-emerald-200 bg-emerald-50'
  if (status === 'disabled') return 'text-rose-700 border-rose-200 bg-rose-50'
  return 'text-slate-600 border-slate-200 bg-slate-50'
}
