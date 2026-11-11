import type { TranslateFn } from '@/utils/status'

type PaymentChannelConfig = Record<string, unknown> | null | undefined

const readConfigString = (config: PaymentChannelConfig, key: string) => {
  const value = config?.[key]
  return typeof value === 'string' || typeof value === 'number' ? String(value).trim() : ''
}

export const resolveOkpayConfiguredCoin = (config: PaymentChannelConfig) => {
  return readConfigString(config, 'coin').toUpperCase()
}

export const resolveOkpayChannelTypeFromConfig = (config: PaymentChannelConfig) => {
  switch (resolveOkpayConfiguredCoin(config).toLowerCase()) {
    case 'usdt':
      return 'usdt'
    case 'trx':
      return 'trx'
    default:
      return ''
  }
}

export const providerTypeLabel = (t: TranslateFn, value?: string) => {
  const map: Record<string, string> = {
    official: t('admin.paymentChannels.providerTypes.official'),
    epay: t('admin.paymentChannels.providerTypes.epay'),
    bepusdt: t('admin.paymentChannels.providerTypes.bepusdt'),
    epusdt: t('admin.paymentChannels.providerTypes.epusdt'),
    tokenpay: t('admin.paymentChannels.providerTypes.tokenpay'),
    wallet: t('admin.paymentChannels.providerTypes.wallet'),
  }
  if (!value) return '-'
  return map[value] || value
}

export const channelTypeLabel = (t: TranslateFn, value?: string) => {
  const map: Record<string, string> = {
    wechat: t('admin.paymentChannels.channelTypes.wechat'),
    alipay: t('admin.paymentChannels.channelTypes.alipay'),
    qqpay: t('admin.paymentChannels.channelTypes.qqpay'),
    paypal: t('admin.paymentChannels.channelTypes.paypal'),
    stripe: t('admin.paymentChannels.channelTypes.stripe'),
    usdt: t('admin.paymentChannels.channelTypes.usdt'),
    'usdt-trc20': t('admin.paymentChannels.channelTypes.usdtTrc20'),
    'usdc-trc20': t('admin.paymentChannels.channelTypes.usdcTrc20'),
    trx: t('admin.paymentChannels.channelTypes.trx'),
    bepusdt: t('admin.paymentChannels.channelTypes.bepusdtCashier'),
    epusdt: t('admin.paymentChannels.channelTypes.epusdtCashier'),
    balance: t('admin.paymentChannels.channelTypes.balance'),
  }
  if (!value) return '-'
  return map[value] || value
}

export const paymentChannelTypeLabel = (t: TranslateFn, payment: { display_channel_type?: string; channel_type?: string }) => {
  const value = String(payment.display_channel_type || payment.channel_type || '').trim()
  return channelTypeLabel(t, value)
}

export const formatFeeRate = (channel: { fee_rate?: number | string; fixed_fee?: number | string }) => {
  const feeRate = channel.fee_rate
  const fixedFee = channel.fixed_fee

  let display = '-'
  if (feeRate !== undefined && feeRate !== null && feeRate !== '') {
    const rateParsed = Number(feeRate)
    if (!Number.isNaN(rateParsed)) {
      display = `${rateParsed.toFixed(2)}%`
    }
  }

  if (fixedFee !== undefined && fixedFee !== null && fixedFee !== '') {
    const fixedParsed = Number(fixedFee)
    if (!Number.isNaN(fixedParsed) && fixedParsed > 0) {
      if (display === '-') {
        display = fixedParsed.toFixed(2)
      } else {
        display += ` + ${fixedParsed.toFixed(2)}`
      }
    }
  }

  return display
}
