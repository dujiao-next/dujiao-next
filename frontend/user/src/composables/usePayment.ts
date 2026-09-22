import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { NavigationFailureType, isNavigationFailure, useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { guestOrderAPI, paymentAPI, userOrderAPI, walletAPI } from '../api'
import { useAppStore } from '../stores/app'
import { useTelegramMiniAppStore } from '../stores/telegramMiniApp'
import { orderStatusLabel } from '../utils/status'
import { fulfillmentTypeLabel } from '../utils/fulfillment'
import { debounceAsync } from '../utils/debounce'
import { copyText } from '../utils/clipboard'
import { amountToCents, centsToAmount } from '../utils/money'
import { buildSkuDisplayTextFromSnapshot } from '../utils/sku'
import {
  getCachedPaymentRestorePolicy,
  getPaymentResetPolicy,
  isCustomerSurchargePayment,
  resolvePaymentInteractionLabelKey,
  resolvePaymentLinkNavigationTarget,
  resolvePaymentPresentationMode,
  resolvePaymentResultTitleKey,
  shouldAutoOpenPaymentLink,
  type PaymentResetReason,
} from '../utils/paymentResumePolicy'
import QRCode from 'qrcode'
import { type PageAlert } from '../utils/alerts'
import { loadGuestOrderAuth, saveGuestOrderAuth } from '../utils/guestOrderAuth'
import { useConfirmDialog } from './useConfirmDialog'
import { toast } from './useToast'

/**
 * 支付页共享逻辑（classic + vault 双模板共用）。
 * 完整保留原 views/Payment.vue 的行为，仅抽离为 composable。
 */
export function usePayment() {
  const route = useRoute()
  const router = useRouter()
  const appStore = useAppStore()
  const telegramMiniAppStore = useTelegramMiniAppStore()
  const { t } = useI18n()
  const { confirm: showConfirm } = useConfirmDialog()

  const loading = ref(true)
  const submitting = ref(false)
  const order = ref<any>(null)
  const paymentResult = ref<any>(null)
  const error = ref('')
  const selectedChannelId = ref<number | null>(null)
  const copied = ref(false)
  const walletAddressCopied = ref(false)
  const capturing = ref(false)
  const redirecting = ref(false)
  const redirected = ref(false)
  const openedPayWindow = ref(false)
  const latestLoaded = ref(false)
  const cachedPayment = ref<any>(null)
  const guestAuth = ref({
    email: '',
    order_password: '',
  })
  const guestAuthError = ref('')
  const pollTimer = ref<number | null>(null)
  const countdownTimer = ref<number | null>(null)
  const now = ref(appStore.getServerTime())
  const copiedTimer = ref<number | null>(null)
  const walletAddressCopiedTimer = ref<number | null>(null)
  const redirectTimer = ref<number | null>(null)
  const walletLoading = ref(false)
  const walletBalance = ref('0')
  const useBalance = ref(false)
  const orderPaymentChannels = ref<any[]>([])
  const orderPaymentChannelsLoaded = ref(false)
  const orderPaymentChannelsRequestId = ref(0)

  const routeQueryValueToString = (value: unknown): string => {
    if (Array.isArray(value)) {
      for (const item of value) {
        const text = String(item ?? '').trim()
        if (text !== '') return text
      }
      return ''
    }
    return String(value ?? '').trim()
  }

  const readRouteQueryValue = (key: string): string => {
    const normalizedKey = String(key || '').trim().toLowerCase()
    if (normalizedKey === '') return ''

    const query = route.query as Record<string, unknown>
    const candidates = [key, normalizedKey, `amp;${key}`, `amp;${normalizedKey}`]
    for (const candidate of candidates) {
      const value = routeQueryValueToString(query[candidate])
      if (value !== '') return value
    }

    for (const [rawKey, rawValue] of Object.entries(query)) {
      const cleanedKey = String(rawKey || '').trim().toLowerCase().replace(/^(amp;)+/, '')
      if (cleanedKey !== normalizedKey) continue
      const value = routeQueryValueToString(rawValue)
      if (value !== '') return value
    }
    return ''
  }

  const readRouteQueryFlag = (key: string): boolean => {
    const value = readRouteQueryValue(key).toLowerCase()
    return value === '1' || value === 'true' || value === 'yes'
  }

  const paymentReturnMarkers = ['epay_return', 'alipay_return', 'wechat_return', 'epusdt_return', 'bepusdt_return', 'tokenpay_return', 'okpay_return', 'pp_return', 'stripe_return']
  const rechargeBizType = computed(() => readRouteQueryValue('biz_type').toLowerCase())
  const rechargeNoQuery = computed(() => {
    const rechargeNo = readRouteQueryValue('recharge_no')
    if (rechargeNo !== '') return rechargeNo
    const orderNo = readRouteQueryValue('order_no')
    if (/^WR/i.test(orderNo)) return orderNo
    return ''
  })
  const isRechargeReturn = computed(() => rechargeBizType.value === 'recharge' || /^WR/i.test(rechargeNoQuery.value))
  const isGuest = computed(() => readRouteQueryFlag('guest'))
  const changePaymentMethodRequested = computed(() => readRouteQueryFlag('change_method'))
  const orderNoQuery = computed(() => {
    const orderNo = readRouteQueryValue('order_no')
    if (orderNo !== '') return orderNo
    return readRouteQueryValue('out_trade_no')
  })
  const orderNoResolved = computed(() => {
    return order.value?.order_no || orderNoQuery.value || ''
  })
  const backLink = computed(() => (isGuest.value ? '/guest/orders' : '/me/orders'))
  const hasGuestAuth = computed(() => Boolean(guestAuth.value.email && guestAuth.value.order_password))
  const showGuestAuthForm = computed(() => isGuest.value && (!hasGuestAuth.value || guestAuthError.value))
  const walletOnlyPayment = computed(() => !!appStore.config?.wallet_only_payment)
  const showBalanceOption = computed(() => !isGuest.value)

  const filterChannelsByOrder = (list: any[]) => {
    if (!Array.isArray(list)) return []
    let filtered = list.filter((channel: any) => {
      const providerType = String(channel?.provider_type || '').toLowerCase()
      const channelType = String(channel?.channel_type || '').toLowerCase()
      if (providerType === 'epay') {
        return ['wechat', 'wxpay', 'alipay', 'qqpay'].includes(channelType)
      }
      return true
    })
    // 按订单中商品允许的支付渠道过滤
    const allowedIds = order.value?.allowed_payment_channel_ids
    if (Array.isArray(allowedIds) && allowedIds.length > 0) {
      const allowedSet = new Set(allowedIds.map(Number))
      filtered = filtered.filter((ch: any) => allowedSet.has(Number(ch?.id)))
    }
    return filtered
  }

  const configReady = computed(() => !appStore.loading && (!!appStore.config || (!isGuest.value && orderPaymentChannelsLoaded.value)))
  const channels = computed(() => {
    if (!isGuest.value && orderPaymentChannelsLoaded.value) {
      return filterChannelsByOrder(orderPaymentChannels.value)
    }
    const fallback = appStore.config?.payment_channels
    return filterChannelsByOrder(Array.isArray(fallback) ? fallback : [])
  })

  const normalizeID = (value: unknown) => String(value ?? '').trim()

  const findChannelByID = (channelID: unknown) => {
    const target = normalizeID(channelID)
    if (target === '') return null
    return channels.value.find((item: any) => normalizeID(item?.id) === target) || null
  }

  const selectedChannel = computed(() => findChannelByID(selectedChannelId.value))

  const cachedChannel = computed(() => findChannelByID(cachedPayment.value?.channel_id))

  const selectedChannelName = computed(() => resolveChannelName(selectedChannel.value, selectedChannel.value?.channel_type))

  const cachedChannelName = computed(() => resolveChannelName(cachedChannel.value, cachedPayment.value?.channel_type, cachedPayment.value?.channel_name))

  const resultChannel = computed(() => findChannelByID(paymentResult.value?.channel_id))

  const resultChannelName = computed(() => resolveChannelName(resultChannel.value, paymentResult.value?.channel_type, paymentResult.value?.channel_name))

  const currentPaymentID = () => {
    const paymentID = Number(paymentResult.value?.payment_id || paymentResult.value?.id || 0)
    return Number.isFinite(paymentID) && paymentID > 0 ? paymentID : 0
  }

  const paymentProviderType = computed(() => String(paymentResult.value?.provider_type || resultChannel.value?.provider_type || '').toLowerCase())

  const paymentChannelType = computed(() => String(paymentResult.value?.channel_type || resultChannel.value?.channel_type || '').toLowerCase())

  const interactionMode = computed(() => String(paymentResult.value?.interaction_mode || '').trim().toLowerCase())
  const paymentPresentationMode = computed(() => resolvePaymentPresentationMode(interactionMode.value))

  const interactionLabel = computed(() => {
    const mode = interactionMode.value
    if (!mode) return '-'
    const labelKey = resolvePaymentInteractionLabelKey(mode)
    if (labelKey) return t(labelKey)
    return mode
  })

  const paymentResultTitle = computed(() => t(resolvePaymentResultTitleKey(interactionMode.value)))
  const paymentGuideTitle = computed(() => paymentPresentationMode.value === 'redirect' ? t('payment.redirectTitle') : t('payment.qrTitle'))
  const paymentGuideTip = computed(() => paymentPresentationMode.value === 'redirect' ? t('payment.redirectTip') : t('payment.qrTip'))

  const showPayLink = computed(() => {
    return paymentPresentationMode.value === 'redirect' || Boolean(payLink.value)
  })
  const isTelegramMiniApp = computed(() => telegramMiniAppStore.isMiniApp && telegramMiniAppStore.isReady)
  const showTelegramPayHint = computed(() => isTelegramMiniApp.value && Boolean(payLink.value))
  const payLinkOpenedTip = computed(() => (
    isTelegramMiniApp.value ? t('payment.redirectOpenedTelegram') : t('payment.redirectOpened')
  ))

  const payLink = computed(() => String(paymentResult.value?.pay_url || '').trim())
  const qrCodeContent = computed(() => String(paymentResult.value?.qr_code || '').trim())
  const cryptoWalletAddress = computed(() => String(paymentResult.value?.wallet_address || '').trim())
  const cryptoChainAmount = computed(() => String(paymentResult.value?.chain_amount || '').trim())
  const cryptoChain = computed(() => String(paymentResult.value?.chain || '').trim())
  const cryptoTokenID = computed(() => String(paymentResult.value?.token_id || '').trim())
  const cryptoTokenLabel = computed(() => {
    const tokenID = cryptoTokenID.value
    if (!tokenID) return ''
    const parts = tokenID.split('-').filter(Boolean)
    return String(parts[parts.length - 1] || tokenID).toUpperCase()
  })
  const cryptoTokenDetail = computed(() => {
    if (!cryptoTokenID.value) return ''
    return cryptoTokenID.value.toUpperCase() === cryptoTokenLabel.value ? '' : cryptoTokenID.value
  })
  const formatCryptoChain = (value: string) => {
    const normalized = value.trim().toLowerCase()
    const labels: Record<string, string> = {
      tron: 'TRON',
      trc20: 'TRON',
      base: 'Base',
      ethereum: 'Ethereum',
      eth: 'Ethereum',
      bsc: 'BNB Smart Chain',
      polygon: 'Polygon',
    }
    return labels[normalized] || value
  }
  const cryptoPaymentDetails = computed(() => {
    const details: Array<{ key: string; label: string; value: string; detail?: string }> = []
    if (cryptoTokenLabel.value) {
      details.push({
        key: 'token',
        label: t('payment.cryptoToken'),
        value: cryptoTokenLabel.value,
        detail: cryptoTokenDetail.value,
      })
    }
    if (cryptoChain.value) {
      details.push({
        key: 'chain',
        label: t('payment.cryptoChain'),
        value: formatCryptoChain(cryptoChain.value),
      })
    }
    if (cryptoChainAmount.value) {
      details.push({
        key: 'amount',
        label: t('payment.cryptoAmount'),
        value: cryptoChainAmount.value,
      })
    }
    if (cryptoWalletAddress.value) {
      details.push({
        key: 'wallet_address',
        label: t('payment.walletAddress'),
        value: cryptoWalletAddress.value,
      })
    }
    return details
  })
  const hasCryptoPaymentDetails = computed(() => cryptoPaymentDetails.value.length > 0)
  const qrFallbackContent = computed(() => {
    if (paymentPresentationMode.value !== 'qr') return ''
    if (qrCodeContent.value) return ''
    return payLink.value
  })
  const qrDisplayContent = computed(() => qrCodeContent.value || qrFallbackContent.value)
  const qrUsingPayLinkFallback = computed(() => Boolean(!qrCodeContent.value && qrFallbackContent.value))
  const showQRCode = computed(() => paymentPresentationMode.value === 'qr' && Boolean(qrDisplayContent.value))

  const qrImageUrl = ref('')
  const qrRenderVersion = ref(0)

  const renderQRCodeImage = async () => {
    const qr = qrDisplayContent.value
    const currentVersion = qrRenderVersion.value + 1
    qrRenderVersion.value = currentVersion
    if (!qr) {
      qrImageUrl.value = ''
      return
    }
    if (qr.startsWith('data:image/')) {
      qrImageUrl.value = qr
      return
    }
    const isImageURL = /^https?:\/\/.+\.(png|jpe?g|gif|webp|svg)(\?.*)?$/i.test(qr)
    if (isImageURL) {
      qrImageUrl.value = qr
      return
    }
    try {
      const dataURL = await QRCode.toDataURL(qr, {
        width: 220,
        margin: 1,
        errorCorrectionLevel: 'M',
      })
      if (currentVersion !== qrRenderVersion.value) return
      qrImageUrl.value = dataURL
    } catch (err) {
      if (currentVersion !== qrRenderVersion.value) return
      qrImageUrl.value = ''
    }
  }

  watch(
    () => qrDisplayContent.value,
    () => {
      void renderQRCodeImage()
    },
    { immediate: true }
  )

  watch(walletOnlyPayment, (v) => {
    if (v) useBalance.value = true
  }, { immediate: true })

  const expiresAtMs = computed(() => {
    const raw = paymentResult.value?.expires_at || order.value?.expires_at
    if (!raw) return null
    const ts = new Date(raw).getTime()
    if (Number.isNaN(ts)) return null
    return ts
  })

  const orderExpired = computed(() => {
    if (!order.value?.expires_at) return false
    const ts = new Date(order.value.expires_at).getTime()
    if (Number.isNaN(ts)) return false
    return ts <= appStore.getServerTime()
  })
  const orderCanceled = computed(() => order.value?.status === 'canceled')
  const paymentAlert = computed<PageAlert | null>(() => {
    if (redirecting.value) {
      return {
        level: 'success' as const,
        message: t('payment.redirecting'),
      }
    }
    if (orderCanceled.value) {
      return {
        level: 'error' as const,
        message: t('payment.orderCanceled'),
      }
    }
    if (orderExpired.value) {
      return {
        level: 'error' as const,
        message: t('payment.orderExpired'),
      }
    }
    if (error.value) {
      return {
        level: 'error' as const,
        message: error.value,
      }
    }
    return null
  })
  const remainingMs = computed(() => {
    if (!expiresAtMs.value) return null
    return expiresAtMs.value - now.value
  })

  const countdownExpired = computed(() => remainingMs.value !== null && remainingMs.value <= 0)

  const countdownText = computed(() => {
    if (remainingMs.value === null) return '-'
    if (remainingMs.value <= 0) return t('payment.countdownExpired')
    const totalSeconds = Math.max(0, Math.floor(remainingMs.value / 1000))
    const minutes = Math.floor(totalSeconds / 60)
    const seconds = totalSeconds % 60
    return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
  })

  const showCountdown = computed(() => Boolean(expiresAtMs.value && order.value?.status === 'pending_payment'))
  const showResultView = computed(() => Boolean(paymentResult.value && order.value && order.value.status === 'pending_payment' && !orderExpired.value && !orderCanceled.value))
  const pollingActive = computed(() => pollTimer.value !== null)
  const orderItems = computed(() => (Array.isArray(order.value?.items) ? order.value.items : []))
  const customerFeeApplied = computed(() => isCustomerSurchargePayment(paymentResult.value) && (amountToCents(paymentResult.value?.fee_amount) || 0) > 0)
  // payable_amount/amount/online_pay_amount 均为 payment.Amount 或其派生值——换汇渠道
  // （如 Stripe 配置了 target_currency+exchange_rate）下这是结算币种数值（如 GBP），
  // 不是订单币种（CNY）。展示时必须搭配后端返回的 paymentResult.currency，不能直接
  // 套用 order.currency，否则会把结算币种数值显示成订单币种。
  //
  // fee_amount 不受影响：calculatePaymentAmounts 按订单币种（CNY）算出手续费后，
  // applyProviderPayment 只用 AmountSent/CurrencySent 覆盖 payment.Amount/Currency，
  // 从不改写 payment.FeeAmount，所以 fee_amount 永远是订单币种数值，必须继续用
  // order.currency 展示，不能套用 payableAmountCurrency，否则会把 CNY 手续费误标成 GBP。
  const payableAmountCurrency = computed(() => {
    const currency = String(paymentResult.value?.currency || '').trim()
    return currency || order.value?.currency
  })
  const customerFeeAmountDisplay = computed(() => {
    if (!customerFeeApplied.value) return ''
    return formatMoney(String(paymentResult.value?.fee_amount || '0'), order.value?.currency)
  })
  const payableAmountDisplay = computed(() => {
    if (paymentResult.value?.payable_amount !== undefined && paymentResult.value?.payable_amount !== null && paymentResult.value?.payable_amount !== '') {
      return formatMoney(String(paymentResult.value.payable_amount), payableAmountCurrency.value)
    }
    if (paymentResult.value?.amount !== undefined && paymentResult.value?.amount !== null && paymentResult.value?.amount !== '') {
      return formatMoney(String(paymentResult.value.amount), payableAmountCurrency.value)
    }
    if (paymentResult.value?.online_pay_amount !== undefined && paymentResult.value?.online_pay_amount !== null && paymentResult.value?.online_pay_amount !== '') {
      return formatMoney(String(paymentResult.value.online_pay_amount), payableAmountCurrency.value)
    }
    return formatMoney(String(order.value?.total_amount ?? ''), order.value?.currency)
  })
  const walletBalanceDisplay = computed(() => formatMoney(walletBalance.value, order.value?.currency))
  const expectedWalletPaidCents = computed(() => {
    if (!showBalanceOption.value || !useBalance.value) return 0
    const balance = amountToCents(walletBalance.value)
    const total = amountToCents(order.value?.total_amount)
    if (balance === null || total === null) return 0
    return Math.min(balance, total)
  })
  const expectedOnlinePayCents = computed(() => {
    const total = amountToCents(order.value?.total_amount)
    if (total === null) return 0
    return Math.max(total - expectedWalletPaidCents.value, 0)
  })
  const expectedWalletPaidDisplay = computed(() => formatMoney(centsToAmount(expectedWalletPaidCents.value), order.value?.currency))
  const expectedOnlinePayDisplay = computed(() => formatMoney(centsToAmount(expectedOnlinePayCents.value), order.value?.currency))
  const requiresOnlineChannel = computed(() => {
    if (isGuest.value) return true
    if (!useBalance.value) return true
    return expectedOnlinePayCents.value > 0
  })
  const channelLimitMeta = (channel?: any) => {
    const minCents = amountToCents(String(channel?.min_amount ?? ''))
    const maxCents = amountToCents(String(channel?.max_amount ?? ''))
    return {
      minCents,
      maxCents,
      hasMin: minCents !== null && minCents > 0,
      hasMax: maxCents !== null && maxCents > 0,
      hideAmountOutRange: Boolean(channel?.hide_amount_out_range),
    }
  }
  const isChannelDisabledForAmount = (channel?: any) => {
    if (!requiresOnlineChannel.value) return false
    const targetAmount = expectedOnlinePayCents.value
    if (targetAmount <= 0) return false

    const meta = channelLimitMeta(channel)
    if (!meta.hasMin && !meta.hasMax) return false

    const lessThanMin = meta.hasMin && meta.minCents !== null && targetAmount < meta.minCents
    const greaterThanMax = meta.hasMax && meta.maxCents !== null && targetAmount > meta.maxCents
    if (!lessThanMin && !greaterThanMax) return false

    return !meta.hideAmountOutRange
  }
  const channelAmountLimitHint = (channel?: any) => {
    const meta = channelLimitMeta(channel)
    if (meta.hasMin && meta.hasMax && meta.minCents !== null && meta.maxCents !== null) {
      return t('payment.channelAmountLimitHint', {
        min: formatMoney(centsToAmount(meta.minCents), order.value?.currency),
        max: formatMoney(centsToAmount(meta.maxCents), order.value?.currency),
      })
    }
    if (meta.hasMin && meta.minCents !== null) {
      return t('payment.channelAmountMinHint', {
        min: formatMoney(centsToAmount(meta.minCents), order.value?.currency),
      })
    }
    if (meta.hasMax && meta.maxCents !== null) {
      return t('payment.channelAmountMaxHint', {
        max: formatMoney(centsToAmount(meta.maxCents), order.value?.currency),
      })
    }
    return ''
  }
  const selectedChannelAmountHint = computed(() => {
    const channel = findChannelByID(selectedChannelId.value)
    if (!channel) return ''
    if (!isChannelDisabledForAmount(channel)) return ''
    return channelAmountLimitHint(channel)
  })
  const canSubmitPayment = computed(() => {
    if (submitting.value) return false
    if (walletOnlyPayment.value && expectedOnlinePayCents.value > 0) return false
    if (!walletOnlyPayment.value && requiresOnlineChannel.value && !selectedChannelId.value) return false
    if (requiresOnlineChannel.value && selectedChannelAmountHint.value) return false
    if (orderExpired.value || orderCanceled.value) return false
    return true
  })
  const paymentWalletPaidDisplay = computed(() => {
    if (paymentResult.value?.wallet_paid_amount === undefined || paymentResult.value?.wallet_paid_amount === null || paymentResult.value?.wallet_paid_amount === '') {
      return '-'
    }
    return formatMoney(String(paymentResult.value.wallet_paid_amount), order.value?.currency)
  })
  const paymentOnlinePayDisplay = computed(() => {
    if (paymentResult.value?.online_pay_amount === undefined || paymentResult.value?.online_pay_amount === null || paymentResult.value?.online_pay_amount === '') {
      return '-'
    }
    return formatMoney(String(paymentResult.value.online_pay_amount), order.value?.currency)
  })

  const loadOrderPaymentChannels = async () => {
    if (isGuest.value) {
      orderPaymentChannels.value = []
      orderPaymentChannelsLoaded.value = false
      return
    }
    if (!orderNoResolved.value) {
      orderPaymentChannels.value = []
      orderPaymentChannelsLoaded.value = false
      return
    }
    if (!requiresOnlineChannel.value) {
      orderPaymentChannels.value = []
      orderPaymentChannelsLoaded.value = true
      return
    }
    const amount = centsToAmount(expectedOnlinePayCents.value)
    const amountCents = amountToCents(amount)
    if (amountCents === null || amountCents <= 0) {
      orderPaymentChannels.value = []
      orderPaymentChannelsLoaded.value = true
      return
    }

    const requestID = ++orderPaymentChannelsRequestId.value
    try {
      const response = await userOrderAPI.getPaymentChannels({
        order_no: orderNoResolved.value,
        amount,
      })
      if (requestID !== orderPaymentChannelsRequestId.value) return
      const channels = response.data.data
      orderPaymentChannels.value = Array.isArray(channels) ? channels : []
      orderPaymentChannelsLoaded.value = true
    } catch {
      if (requestID !== orderPaymentChannelsRequestId.value) return
      orderPaymentChannels.value = []
      orderPaymentChannelsLoaded.value = false
    }
  }

  const debouncedLoadOrderPaymentChannels = debounceAsync(loadOrderPaymentChannels, 250)

  const loadWallet = async () => {
    if (isGuest.value) return
    walletLoading.value = true
    try {
      const response = await walletAPI.account()
      walletBalance.value = String(response.data.data?.balance || '0')
    } catch {
      walletBalance.value = '0'
    } finally {
      walletLoading.value = false
    }
  }

  const loadOrder = async (options?: { silent?: boolean }) => {
    const silent = options?.silent && !!order.value
    if (!silent) {
      loading.value = true
    }
    try {
      if (isGuest.value) {
        if (!hasGuestAuth.value) {
          order.value = null
          guestAuthError.value = t('payment.guestAuthRequired')
          return
        }
        if (!orderNoQuery.value) {
          order.value = null
          orderPaymentChannels.value = []
          orderPaymentChannelsLoaded.value = false
          return
        }
        const response = await guestOrderAPI.detail(orderNoQuery.value, {
          email: guestAuth.value.email,
          order_password: guestAuth.value.order_password,
        }, { silentBusinessError: true })
        order.value = response.data.data
        guestAuthError.value = ''
      } else {
        if (!orderNoQuery.value) {
          order.value = null
          orderPaymentChannels.value = []
          orderPaymentChannelsLoaded.value = false
          return
        }
        const response = await userOrderAPI.detail(orderNoQuery.value, { silentBusinessError: true })
        order.value = response.data.data
      }
    } catch (err) {
      if (!silent) {
        order.value = null
        orderPaymentChannels.value = []
        orderPaymentChannelsLoaded.value = false
        if (isGuest.value) {
          guestAuthError.value = t('payment.guestAuthInvalid')
        }
      }
    } finally {
      try {
        if (order.value) {
          if (orderCanceled.value) {
            if (!silent) {
              error.value = t('payment.orderCanceled')
            }
            cachedPayment.value = null
            return
          }
          if (orderExpired.value) {
            if (!silent) {
              error.value = t('payment.orderExpired')
            }
            cachedPayment.value = null
            return
          }
          if (!changePaymentMethodRequested.value && !paymentResult.value && !latestLoaded.value && order.value.status === 'pending_payment') {
            latestLoaded.value = true
            await loadLatestPayment()
          }
        }
      } finally {
        if (!silent) {
          loading.value = false
        }
      }
    }
  }

  const debouncedLoadOrder = debounceAsync(loadOrder, 250)

  const startCountdown = () => {
    if (!expiresAtMs.value || countdownTimer.value) return
    if (order.value?.status !== 'pending_payment') return
    now.value = appStore.getServerTime()
    countdownTimer.value = window.setInterval(() => {
      now.value = appStore.getServerTime()
    }, 1000)
  }

  const stopCountdown = () => {
    if (!countdownTimer.value) return
    window.clearInterval(countdownTimer.value)
    countdownTimer.value = null
  }

  const shouldCaptureCurrentPayment = () => {
    if (!currentPaymentID()) return false
    if (!order.value || order.value.status !== 'pending_payment') return false
    return paymentProviderType.value === 'official' && paymentChannelType.value === 'wechat'
  }

  const captureCurrentPayment = async (options?: { silent?: boolean }) => {
    if (capturing.value || !shouldCaptureCurrentPayment()) return
    capturing.value = true
    if (!options?.silent) {
      error.value = ''
    }
    try {
      const paymentID = currentPaymentID()
      if (!paymentID) return
      if (isGuest.value) {
        if (!hasGuestAuth.value) {
          if (!options?.silent) {
            guestAuthError.value = t('payment.guestAuthRequired')
          }
          return
        }
        await guestOrderAPI.capturePayment(paymentID, {
          email: guestAuth.value.email,
          order_password: guestAuth.value.order_password,
        })
      } else {
        await paymentAPI.capture(paymentID)
      }
    } catch (err: any) {
      if (!options?.silent) {
        error.value = err?.message || t('payment.captureFailed')
      }
    } finally {
      capturing.value = false
    }
  }

  const startPolling = () => {
    if (pollTimer.value) return
    pollTimer.value = window.setInterval(async () => {
      await captureCurrentPayment({ silent: true })
      await debouncedLoadOrder({ silent: true })
    }, 5000)
  }

  const stopPolling = () => {
    if (!pollTimer.value) return
    window.clearInterval(pollTimer.value)
    pollTimer.value = null
  }

  const handleCopyPayLink = async () => {
    if (!payLink.value) return
    try {
      await copyText(payLink.value)
      copied.value = true
      if (copiedTimer.value) {
        window.clearTimeout(copiedTimer.value)
      }
      copiedTimer.value = window.setTimeout(() => {
        copied.value = false
        copiedTimer.value = null
      }, 1500)
    } catch (err: any) {
      error.value = err?.message || t('payment.copyFailed')
    }
  }

  const handleCopyWalletAddress = async () => {
    if (!cryptoWalletAddress.value) return
    try {
      await copyText(cryptoWalletAddress.value)
      walletAddressCopied.value = true
      if (walletAddressCopiedTimer.value) {
        window.clearTimeout(walletAddressCopiedTimer.value)
      }
      walletAddressCopiedTimer.value = window.setTimeout(() => {
        walletAddressCopied.value = false
        walletAddressCopiedTimer.value = null
      }, 1500)
    } catch (err: any) {
      error.value = err?.message || t('payment.copyFailed')
    }
  }

  const openPayLinkInCompatibleWindow = (automatic = false) => {
    if (!payLink.value) return
    if (isTelegramMiniApp.value) {
      telegramMiniAppStore.openLink(payLink.value)
    } else if (resolvePaymentLinkNavigationTarget(automatic) === 'current-tab') {
      // 支付请求返回后已脱离原始点击事件，浏览器通常会拦截此时创建的新窗口。
      // 自动收银台跳转使用当前标签页，既可靠也能保留当前 sessionStorage。
      window.location.assign(payLink.value)
      return
    } else {
      // 先创建同源空白页，让浏览器按规范复制当前标签页的 sessionStorage；
      // 随后立即切断 opener 再跳转到支付站。这样第三方回跳仍能恢复游客订单，
      // 同时不给外部收银台保留反向控制原页面的能力。
      const paymentWindow = window.open('', '_blank')
      if (!paymentWindow) return
      paymentWindow.opener = null
      paymentWindow.location.replace(payLink.value)
    }
    openedPayWindow.value = true
  }

  const handleOpenPayLink = () => {
    error.value = ''
    openPayLinkInCompatibleWindow()
  }

  const loadLatestPayment = async () => {
    if (!order.value || order.value.status !== 'pending_payment') return
    if (paymentResult.value) return
    if (isGuest.value && !hasGuestAuth.value) return
    if (!orderNoResolved.value) return
    try {
      let response
      if (isGuest.value) {
        response = await guestOrderAPI.latestPayment({
          order_no: orderNoResolved.value,
          email: guestAuth.value.email,
          order_password: guestAuth.value.order_password,
        })
      } else {
        response = await paymentAPI.latest({ order_no: orderNoResolved.value })
      }
      const data = response.data.data
      if (data && (data.pay_url || data.qr_code)) {
        cachedPayment.value = data
        paymentResult.value = data
        selectedChannelId.value = data.channel_id || null
        startPolling()
        void captureCurrentPayment({ silent: true })
        startCountdown()
        // 恢复已有待支付记录时只展示状态和操作按钮，绝不再次自动进入外部收银台。
        // 自动跳转仅发生在用户刚刚主动创建支付的那一次。
      }
    } catch (err) {
      // 没有历史支付记录时忽略错误
    }
  }

  const buildPayRouteQuery = () => {
    const query: Record<string, string> = {}
    const resolvedOrderNo = String(order.value?.order_no || orderNoQuery.value || '').trim()
    if (resolvedOrderNo !== '') {
      query.order_no = resolvedOrderNo
    }
    if (isGuest.value) {
      query.guest = '1'
    }
    return query
  }

  const buildRechargeReturnRouteQuery = () => {
    const query: Record<string, string> = {}
    const resolvedRechargeNo = String(rechargeNoQuery.value || '').trim()
    if (resolvedRechargeNo !== '') {
      query.recharge_no = resolvedRechargeNo
    }
    for (const marker of paymentReturnMarkers) {
      const value = readRouteQueryValue(marker)
      if (value !== '') {
        query[marker] = value
      }
    }
    for (const key of ['token', 'payer_id', 'PayerID', 'session_id']) {
      const value = readRouteQueryValue(key)
      if (value !== '') {
        query[key] = value
      }
    }
    return query
  }

  const redirectToWalletRecharge = async () => {
    const resolvedRechargeNo = String(rechargeNoQuery.value || '').trim()
    if (resolvedRechargeNo === '') return
    await router.replace({
      name: 'personal-center-wallet',
      query: buildRechargeReturnRouteQuery(),
    })
  }

  const capturePaypalIfNeeded = async () => {
    if (capturing.value) return
    const paymentID = currentPaymentID()
    if (!paymentID) return
    if (!(paymentProviderType.value === 'official' && paymentChannelType.value === 'paypal')) return
    const returnFlag = readRouteQueryValue('pp_return').toLowerCase()
    const token = readRouteQueryValue('token')
    const payerId = readRouteQueryValue('payer_id') || readRouteQueryValue('PayerID')
    if (returnFlag !== '1' && token === '' && payerId === '') return
    if (!orderNoResolved.value || !order.value || order.value.status !== 'pending_payment') return

    capturing.value = true
    error.value = ''
    try {
      if (isGuest.value) {
        if (!hasGuestAuth.value) {
          guestAuthError.value = t('payment.guestAuthRequired')
          return
        }
        await guestOrderAPI.capturePayment(paymentID, {
          email: guestAuth.value.email,
          order_password: guestAuth.value.order_password,
        })
      } else {
        await paymentAPI.capture(paymentID)
      }
      await debouncedLoadOrder({ silent: true })
      await router.replace({
        path: route.path,
        query: buildPayRouteQuery(),
      })
    } catch (err: any) {
      error.value = err?.message || t('payment.captureFailed')
    } finally {
      capturing.value = false
    }
  }

  const captureStripeIfNeeded = async () => {
    if (capturing.value) return
    const paymentID = currentPaymentID()
    if (!paymentID) return
    if (!(paymentProviderType.value === 'official' && paymentChannelType.value === 'stripe')) return
    const returnFlag = readRouteQueryValue('stripe_return').toLowerCase()
    const sessionID = readRouteQueryValue('session_id')
    if (returnFlag !== '1' && sessionID === '') return
    if (!orderNoResolved.value || !order.value || order.value.status !== 'pending_payment') return

    capturing.value = true
    error.value = ''
    try {
      if (isGuest.value) {
        if (!hasGuestAuth.value) {
          guestAuthError.value = t('payment.guestAuthRequired')
          return
        }
        await guestOrderAPI.capturePayment(paymentID, {
          email: guestAuth.value.email,
          order_password: guestAuth.value.order_password,
        })
      } else {
        await paymentAPI.capture(paymentID)
      }
      await debouncedLoadOrder({ silent: true })
      await router.replace({
        path: route.path,
        query: buildPayRouteQuery(),
      })
    } catch (err: any) {
      error.value = err?.message || t('payment.captureFailed')
    } finally {
      capturing.value = false
    }
  }

  const syncPaymentReturnIfNeeded = async () => {
    // 支持所有支付方式的回调同步跳转
    const hasReturn = paymentReturnMarkers.some(marker => readRouteQueryValue(marker).toLowerCase() === '1')

    if (!hasReturn) return
    if (!orderNoQuery.value) return

    try {
      await debouncedLoadOrder({ silent: true })
      await loadLatestPayment()
    } finally {
      await router.replace({
        path: route.path,
        query: buildPayRouteQuery(),
      })
    }
  }

  const performPayment = async () => {
    error.value = ''
    if (!orderNoResolved.value) {
      error.value = t('payment.orderNotFound')
      return
    }
    if (requiresOnlineChannel.value && !selectedChannelId.value) {
      error.value = t('payment.selectChannelError')
      return
    }
    if (requiresOnlineChannel.value && selectedChannelAmountHint.value) {
      error.value = selectedChannelAmountHint.value
      return
    }
    if (orderCanceled.value) {
      error.value = t('payment.orderCanceled')
      return
    }
    if (orderExpired.value) {
      error.value = t('payment.orderExpired')
      return
    }
    if (requiresOnlineChannel.value && cachedPayment.value && selectedChannelId.value && selectedChannelId.value === cachedPayment.value.channel_id) {
      paymentResult.value = cachedPayment.value
      openedPayWindow.value = false
      startPolling()
      void captureCurrentPayment({ silent: true })
      startCountdown()
      window.scrollTo({ top: 0, behavior: 'smooth' })
      // 此处是用户主动再次点击「提交订单并支付」，不同于页面加载时恢复支付。
      // 已有支付记录也应立即打开收银台，避免桌面端只能手动点击链接。
      if (shouldAutoOpenPaymentLink(paymentResult.value)) {
        openPayLinkInCompatibleWindow(true)
      }
      return
    }
    submitting.value = true
    try {
      if (isGuest.value) {
        if (!hasGuestAuth.value) {
          error.value = t('payment.guestAuthRequired')
          return
        }
        const response = await guestOrderAPI.createPayment({
          email: guestAuth.value.email,
          order_password: guestAuth.value.order_password,
          order_no: orderNoResolved.value,
          channel_id: selectedChannelId.value,
        })
        paymentResult.value = response.data.data
        if (paymentResult.value?.pay_url || paymentResult.value?.qr_code) {
          cachedPayment.value = paymentResult.value
        }
        openedPayWindow.value = false
        startPolling()
        void captureCurrentPayment({ silent: true })
      } else {
        const payload: any = {
          order_no: orderNoResolved.value,
          use_balance: useBalance.value,
        }
        if (requiresOnlineChannel.value && selectedChannelId.value) {
          payload.channel_id = selectedChannelId.value
        }
        const response = await paymentAPI.create(payload)
        const created = response.data.data || {}
        if (created.order_paid && !created.payment_id) {
          paymentResult.value = null
          cachedPayment.value = null
          selectedChannelId.value = null
          useBalance.value = false
          stopPolling()
          stopCountdown()
          await Promise.all([
            debouncedLoadOrder({ silent: true }),
            loadWallet(),
          ])
          redirectToOrderDetail()
          return
        }
        paymentResult.value = created
        if (paymentResult.value?.pay_url || paymentResult.value?.qr_code) {
          cachedPayment.value = paymentResult.value
        }
        openedPayWindow.value = false
        startPolling()
        void captureCurrentPayment({ silent: true })
        await loadWallet()
      }
      window.scrollTo({ top: 0, behavior: 'smooth' })
      if (shouldAutoOpenPaymentLink(paymentResult.value)) {
        openPayLinkInCompatibleWindow(true)
      }
    } catch (err: any) {
      error.value = err.message || t('payment.createFailed')
    } finally {
      submitting.value = false
    }
  }

  const handlePayment = debounceAsync(performPayment, 200)

  const shouldRedirect = (status?: string) => {
    if (!status) return false
    return ['paid', 'fulfilling', 'partially_delivered', 'delivered', 'completed'].includes(status)
  }

  const resetRedirectState = () => {
    redirecting.value = false
    redirected.value = false
  }

  const redirectToOrderDetail = () => {
    if (redirected.value || redirecting.value) return
    const resolvedOrderNo = String(order.value?.order_no || orderNoQuery.value || '').trim()
    if (!resolvedOrderNo) return

    const target = isGuest.value
      ? { name: 'guest-order-detail', params: { order_no: resolvedOrderNo } }
      : { name: 'order-detail', params: { order_no: resolvedOrderNo } }
    const fallbackPath = isGuest.value
      ? `/guest/orders/${encodeURIComponent(resolvedOrderNo)}`
      : `/orders/${encodeURIComponent(resolvedOrderNo)}`
    const resolvedTarget = router.resolve(target)
    if (!resolvedTarget.matched.length) {
      window.location.assign(fallbackPath)
      return
    }

    redirected.value = true
    redirecting.value = true

    if (redirectTimer.value !== null) {
      window.clearTimeout(redirectTimer.value)
      redirectTimer.value = null
    }

    redirectTimer.value = window.setTimeout(async () => {
      try {
        const failure = await router.push(target)
        if (failure && !isNavigationFailure(failure, NavigationFailureType.duplicated)) {
          resetRedirectState()
          window.location.assign(fallbackPath)
        }
      } catch (_err) {
        resetRedirectState()
        window.location.assign(fallbackPath)
      } finally {
        if (redirectTimer.value !== null) {
          window.clearTimeout(redirectTimer.value)
          redirectTimer.value = null
        }
      }
    }, 600)
  }

  const resetPayment = (reason: PaymentResetReason = 'generic') => {
    const resetPolicy = getPaymentResetPolicy(reason)
    if (resetPolicy.stopActivePaymentWatch) {
      stopPolling()
      stopCountdown()
      debouncedLoadOrder.cancel()
    }
    paymentResult.value = null
    error.value = ''
    openedPayWindow.value = false
    resetRedirectState()
    latestLoaded.value = !resetPolicy.resumeLatestPayment
    if (resetPolicy.clearSelectedChannel) {
      selectedChannelId.value = null
    }
  }

  const resetPaymentRouteState = () => {
    stopPolling()
    stopCountdown()
    resetPayment('route_change')
    cachedPayment.value = null
    selectedChannelId.value = null
    order.value = null
    orderPaymentChannels.value = []
    orderPaymentChannelsLoaded.value = false
  }

  const restoreCachedPayment = () => {
    if (!cachedPayment.value) return
    const restorePolicy = getCachedPaymentRestorePolicy()
    paymentResult.value = cachedPayment.value
    selectedChannelId.value = cachedPayment.value.channel_id || null
    openedPayWindow.value = false
    if (restorePolicy.startActivePaymentWatch) {
      startPolling()
      void captureCurrentPayment({ silent: true })
      startCountdown()
    }
    if (restorePolicy.autoOpenPayLink && shouldAutoOpenPaymentLink(paymentResult.value)) {
      openPayLinkInCompatibleWindow(true)
    }
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  const handleChangePaymentMethod = () => {
    resetPayment('change_payment_method')
  }

  const formatDate = (raw?: string) => {
    if (!raw) return ''
    const date = new Date(raw)
    if (Number.isNaN(date.getTime())) return raw
    return date.toLocaleString()
  }

  const statusLabel = (status: string) => orderStatusLabel(t, status)

  const formatMoney = (amount?: string, currency?: string) => {
    if (amount === null || amount === undefined || amount === '') return '-'
    if (currency === null || currency === undefined || currency === '') {
      return String(amount)
    }
    return `${amount} ${currency}`
  }

  const hasDiscountAmount = (amount?: string) => {
    if (amount === null || amount === undefined || amount === '') return false
    const valueCents = amountToCents(amount)
    return valueCents !== null && valueCents > 0
  }

  const formatDiscountMoney = (amount?: string, currency?: string) => {
    return hasDiscountAmount(amount) ? `-${formatMoney(amount, currency)}` : formatMoney(amount, currency)
  }

  const getLocalizedText = (jsonData: any) => {
    if (!jsonData) return ''
    const locale = appStore.locale
    return jsonData[locale] || jsonData['zh-CN'] || jsonData['en-US'] || ''
  }

  const orderItemSkuText = (item: any) => {
    return buildSkuDisplayTextFromSnapshot(item?.sku_snapshot, {
      locale: appStore.locale,
      fallback: t('productDetail.skuFallback'),
    })
  }

  const fulfillmentTypeLabelText = (type: string) => fulfillmentTypeLabel(t, type, 'orderDetail')

  const channelTypeLabel = (value?: string) => {
    const map: Record<string, string> = {
      wechat: t('payment.channelTypes.wechat'),
      wxpay: t('payment.channelTypes.wxpay'),
      alipay: t('payment.channelTypes.alipay'),
      qqpay: t('payment.channelTypes.qqpay'),
      paypal: t('payment.channelTypes.paypal'),
      stripe: t('payment.channelTypes.stripe'),
      usdt: t('payment.channelTypes.usdt'),
      'usdt-trc20': t('payment.channelTypes.usdtTrc20'),
      'usdc-trc20': t('payment.channelTypes.usdcTrc20'),
      trx: t('payment.channelTypes.trx'),
    }
    if (!value) return '-'
    return map[value] || value
  }

  const resolveChannelName = (channel?: any, fallbackChannelType?: unknown, apiChannelName?: unknown) => {
    if (channel?.name) return channel.name
    const name = String(apiChannelName || '').trim()
    if (name) return name
    const channelType = String(fallbackChannelType || '').trim()
    if (channelType) return channelTypeLabel(channelType)
    return '-'
  }

  onMounted(() => {
    if (isRechargeReturn.value && rechargeNoQuery.value) {
      void redirectToWalletRecharge()
      return
    }
    if (!orderNoQuery.value) return
    guestAuth.value = loadGuestOrderAuth()
    loadOrder()
    void loadWallet()
    if (!appStore.config || !Array.isArray(appStore.config?.payment_channels)) {
      appStore.loadConfig(true)
    }
  })

  watch(
    () => order.value?.status,
    (status) => {
      if (!status) return
      if (status === 'canceled') {
        error.value = t('payment.orderCanceled')
        stopPolling()
        stopCountdown()
        return
      }
      if (status === 'pending_payment') {
        startPolling()
        startCountdown()
        return
      }
      stopPolling()
      stopCountdown()
      if (shouldRedirect(status)) {
        redirectToOrderDetail()
      }
    }
  )

  watch(
    () => [isGuest.value, orderNoQuery.value],
    async ([, orderNo], [, previousOrderNo]) => {
      if (!previousOrderNo || orderNo === previousOrderNo) return
      resetPaymentRouteState()
      if (!orderNo) return
      await loadOrder()
      void loadWallet()
    }
  )

  watch(
    () => [isGuest.value, orderNoResolved.value, requiresOnlineChannel.value, expectedOnlinePayCents.value, order.value?.status],
    () => {
      void debouncedLoadOrderPaymentChannels()
    },
    { immediate: true }
  )

  watch(
    () => [currentPaymentID(), paymentProviderType.value, paymentChannelType.value, route.fullPath, order.value?.status],
    () => {
      void capturePaypalIfNeeded()
      void captureStripeIfNeeded()
      void syncPaymentReturnIfNeeded()
    },
    { immediate: true }
  )

  watch(
    () => [channels.value, expectedOnlinePayCents.value, requiresOnlineChannel.value],
    () => {
      if (!selectedChannelId.value) return
      const selected = findChannelByID(selectedChannelId.value)
      if (!selected || isChannelDisabledForAmount(selected)) {
        selectedChannelId.value = null
      }
    },
    { deep: true }
  )

  watch(expiresAtMs, (value) => {
    stopCountdown()
    if (!value) return
    if (order.value?.status !== 'pending_payment') return
    startCountdown()
  })

  watch(remainingMs, (value) => {
    if (value === null) return
    if (value <= 0) {
      stopCountdown()
    }
  })

  onUnmounted(() => {
    stopPolling()
    stopCountdown()
    if (redirectTimer.value) {
      window.clearTimeout(redirectTimer.value)
      redirectTimer.value = null
    }
    if (copiedTimer.value) {
      window.clearTimeout(copiedTimer.value)
      copiedTimer.value = null
    }
    if (walletAddressCopiedTimer.value) {
      window.clearTimeout(walletAddressCopiedTimer.value)
      walletAddressCopiedTimer.value = null
    }
    debouncedLoadOrder.cancel()
    debouncedLoadOrderPaymentChannels.cancel()
  })

  const handleGuestAuthSubmit = async () => {
    guestAuthError.value = ''
    if (!hasGuestAuth.value) {
      guestAuthError.value = t('payment.guestAuthRequired')
      return
    }
    saveGuestOrderAuth({
      email: guestAuth.value.email,
      order_password: guestAuth.value.order_password,
    })
    await debouncedLoadOrder()
  }

  const handleRefresh = async () => {
    await captureCurrentPayment()
    await Promise.all([
      debouncedLoadOrder(),
      loadWallet(),
      debouncedLoadOrderPaymentChannels(),
    ])
  }

  const canCancelOrder = computed(() => order.value?.status === 'pending_payment')
  const cancelOrder = async () => {
    if (!canCancelOrder.value || !orderNoResolved.value) return
    const confirmed = await showConfirm({
      title: t('orderDetail.cancel'),
      message: t('orderDetail.cancelConfirm'),
      confirmText: t('common.confirm'),
      cancelText: t('common.cancel'),
      variant: 'danger',
    })
    if (!confirmed) return
    try {
      if (isGuest.value) {
        await guestOrderAPI.cancel(orderNoResolved.value, guestAuth.value)
      } else {
        await userOrderAPI.cancel(orderNoResolved.value)
      }
      stopPolling()
      paymentResult.value = null
      cachedPayment.value = null
      await debouncedLoadOrder()
    } catch {
      toast.error(t('orderDetail.cancelFailed'))
    }
  }

  return {
    // state
    loading,
    submitting,
    order,
    paymentResult,
    selectedChannelId,
    copied,
    walletAddressCopied,
    openedPayWindow,
    cachedPayment,
    guestAuth,
    guestAuthError,
    walletLoading,
    walletBalance,
    useBalance,
    // routing / flags
    backLink,
    showGuestAuthForm,
    walletOnlyPayment,
    showBalanceOption,
    configReady,
    channels,
    // channel resolution
    selectedChannel,
    selectedChannelName,
    cachedChannelName,
    resultChannelName,
    interactionLabel,
    paymentResultTitle,
    paymentGuideTitle,
    paymentGuideTip,
    showPayLink,
    showTelegramPayHint,
    payLinkOpenedTip,
    // crypto / qr
    cryptoWalletAddress,
    cryptoPaymentDetails,
    hasCryptoPaymentDetails,
    qrUsingPayLinkFallback,
    showQRCode,
    qrImageUrl,
    // status
    orderExpired,
    orderCanceled,
    paymentAlert,
    countdownExpired,
    countdownText,
    showCountdown,
    showResultView,
    pollingActive,
    orderItems,
    // amounts
    customerFeeApplied,
    customerFeeAmountDisplay,
    payableAmountDisplay,
    walletBalanceDisplay,
    expectedWalletPaidDisplay,
    expectedOnlinePayDisplay,
    expectedOnlinePayCents,
    requiresOnlineChannel,
    paymentWalletPaidDisplay,
    paymentOnlinePayDisplay,
    // channel limits
    isChannelDisabledForAmount,
    channelAmountLimitHint,
    canSubmitPayment,
    // formatters
    formatDate,
    statusLabel,
    formatMoney,
    hasDiscountAmount,
    formatDiscountMoney,
    getLocalizedText,
    orderItemSkuText,
    fulfillmentTypeLabelText,
    // actions
    handleCopyPayLink,
    handleCopyWalletAddress,
    handleOpenPayLink,
    restoreCachedPayment,
    handleChangePaymentMethod,
    handlePayment,
    handleGuestAuthSubmit,
    handleRefresh,
    canCancelOrder,
    cancelOrder,
  }
}
