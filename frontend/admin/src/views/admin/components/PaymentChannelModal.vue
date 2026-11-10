<script setup lang="ts">
import { onMounted, computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminGatewaySecurityTestResult, AdminMemberLevel } from '@/api/types'
import { getLocalizedText } from '@/utils/format'
import { resolveOkpayChannelTypeFromConfig } from '@/utils/paymentChannelDisplay'
import MediaPicker from '@/components/admin/MediaPicker.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { Dialog, DialogScrollContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { MultiSelect } from '@/components/ui/multi-select'

const props = defineProps<{
  modelValue: boolean
  channelId: number | null
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  success: []
}>()

const { t } = useI18n()

const isEditing = computed(() => props.channelId !== null)
const error = ref('')
const showAdvanced = ref(false)
const applyingChannelData = ref(false)
const configJsonPlaceholder = '{ "key": "value" }'
const testingWechatPublicKey = ref(false)
const wechatPublicKeyTestResult = ref<AdminGatewaySecurityTestResult | null>(null)

const form = reactive({
  name: '',
  icon: '',
  provider_type: 'epay',
  channel_type: 'alipay',
  interaction_mode: 'qr',
  fee_rate: '0',
  fixed_fee: '0',
  min_amount: '0',
  max_amount: '0',
  hide_amount_out_range: false,
  payment_types: [] as string[],
  payment_roles: [] as string[],
  member_levels: [] as number[],
  config_json: '',
  is_active: true,
  sort_order: 10,
})

const memberLevels = ref<AdminMemberLevel[]>([])

onMounted(async () => {
  try {
    const mlRes = await adminAPI.getMemberLevels({ page: 1, page_size: 100 })
    memberLevels.value = mlRes.data.data || []
  } catch (error) {
    console.error('Failed to load member levels config:', error)
  }
})

const epayConfig = reactive({
  epay_version: 'v2',
  gateway_url: '',
  merchant_id: '',
  merchant_key: '',
  private_key: '',
  platform_public_key: '',
  notify_url: '',
  return_url: '',
  target_currency: '',
  exchange_rate: '',
})

const paypalConfig = reactive({
  client_id: '',
  client_secret: '',
  base_url: 'https://api-m.sandbox.paypal.com',
  return_url: '',
  cancel_url: '',
  webhook_id: '',
  brand_name: '',
  locale: '',
  target_currency: '',
  exchange_rate: '',
})

const stripeConfig = reactive({
  secret_key: '',
  publishable_key: '',
  webhook_secret: '',
  success_url: '',
  cancel_url: '',
  api_base_url: 'https://api.stripe.com',
  payment_method_types: '',
  target_currency: '',
  exchange_rate: '',
})

const alipayConfig = reactive({
  app_id: '',
  private_key: '',
  alipay_public_key: '',
  gateway_url: 'https://openapi.alipay.com/gateway.do',
  notify_url: '',
  return_url: '',
  sign_type: 'RSA2',
  app_cert_sn: '',
  alipay_root_cert_sn: '',
  target_currency: '',
  exchange_rate: '',
})

const wechatConfig = reactive({
  appid: '',
  mchid: '',
  merchant_serial_no: '',
  merchant_private_key: '',
  api_v3_key: '',
  verification_mode: 'platform_certificate',
  wechatpay_public_key_id: '',
  wechatpay_public_key: '',
  notify_url: '',
  h5_redirect_url: '',
  h5_type: 'WAP',
  h5_wap_url: '',
  h5_wap_name: '',
  target_currency: '',
  exchange_rate: '',
})

const bepusdtConfig = reactive({
  gateway_url: '',
  auth_token: '',
  order_mode: 'transaction',
  trade_type: 'usdt.trc20',
  currencies: '',
  fiat: 'CNY',
  notify_url: '',
  return_url: '',
})

const epusdtConfig = reactive({
  gateway_url: '',
  pid: '',
  secret_key: '',
  order_mode: 'transaction',
  token: 'usdt',
  network: 'tron',
  currency: 'cny',
  notify_url: '',
  return_url: '',
})

const tokenpayConfig = reactive({
  gateway_url: '',
  notify_secret: '',
  currency: '',
  notify_url: '',
  redirect_url: '',
  base_currency: 'CNY',
})

const okpayConfig = reactive({
  gateway_url: 'https://api.okaypay.me/shop',
  merchant_id: '',
  merchant_token: '',
  exchange_rate: '1',
  return_url: '',
  callback_url: '',
  display_name: '',
})

const dujiaopayConfig = reactive({
  api_base_url: 'https://www.dujiaopay.com',
  api_key_id: '',
  api_secret: '',
  webhook_secret: '',
  order_mode: 'transaction',
  allowed_methods: '',
  fiat_currency: 'CNY',
  success_url: '',
  cancel_url: '',
})

const epayChannelOptions = [
  { value: 'wechat', label: 'admin.paymentChannels.channelTypes.wechat' },
  { value: 'alipay', label: 'admin.paymentChannels.channelTypes.alipay' },
  { value: 'qqpay', label: 'admin.paymentChannels.channelTypes.qqpay' },
]

const officialChannelOptions = [
  { value: 'paypal', label: 'admin.paymentChannels.channelTypes.paypal' },
  { value: 'stripe', label: 'admin.paymentChannels.channelTypes.stripe' },
  { value: 'alipay', label: 'admin.paymentChannels.channelTypes.alipay' },
  { value: 'wechat', label: 'admin.paymentChannels.channelTypes.wechat' },
]

const bepusdtChannelOptions = [
  { value: 'usdt-trc20', label: 'admin.paymentChannels.channelTypes.usdtTrc20' },
  { value: 'usdc-trc20', label: 'admin.paymentChannels.channelTypes.usdcTrc20' },
  { value: 'trx', label: 'admin.paymentChannels.channelTypes.trx' },
]

const okpayChannelOptions = [
  { value: 'usdt', label: 'admin.paymentChannels.channelTypes.usdt' },
  { value: 'trx', label: 'admin.paymentChannels.channelTypes.trx' },
]

// DujiaoPay 的 token_id 由管理员手动填写（形如 tron-usdt），不在前端维护固定列表，
// 上游新增链或代币时无需同步改代码。默认值仅用于新建渠道时的初始填充。
const dujiaopayDefaultTokenID = 'tron-usdt'

const channelOptions = [
  ...epayChannelOptions,
  ...officialChannelOptions,
  ...okpayChannelOptions,
]

const paymentTypeOptions = computed(() => [
  { value: 'order', label: t('admin.paymentChannels.paymentTypes.order') },
  { value: 'wallet', label: t('admin.paymentChannels.paymentTypes.wallet') },
])

const paymentRoleOptions = computed(() => [
  { value: 'guest', label: t('admin.paymentChannels.paymentRoles.guest') },
  { value: 'member', label: t('admin.paymentChannels.paymentRoles.member') },
])

const memberLevelOptions = computed(() =>
  memberLevels.value.map((ml) => ({
    label: getLocalizedText(ml.name) || 'Unknown',
    value: ml.id,
  }))
)

const formChannelOptions = computed(() => {
  if (form.provider_type === 'epay') {
    return epayChannelOptions
  }
  if (form.provider_type === 'official') {
    return officialChannelOptions
  }
  if (form.provider_type === 'bepusdt') {
    return bepusdtChannelOptions
  }
  if (form.provider_type === 'okpay') {
    return okpayChannelOptions
  }
  return channelOptions
})

const interactionModeOptions = computed(() => {
  if (form.provider_type === 'epay') {
    return [
      { value: 'qr', label: 'admin.paymentChannels.interactionModes.qr' },
      { value: 'redirect', label: 'admin.paymentChannels.interactionModes.redirect' },
    ]
  }
  if (form.provider_type === 'bepusdt') {
    if (bepusdtConfig.order_mode === 'cashier') {
      return [
        { value: 'redirect', label: 'admin.paymentChannels.interactionModes.redirect' },
      ]
    }
    return [
      { value: 'qr', label: 'admin.paymentChannels.interactionModes.qr' },
      { value: 'redirect', label: 'admin.paymentChannels.interactionModes.redirect' },
    ]
  }
  if (form.provider_type === 'epusdt') {
    return [
      { value: 'redirect', label: 'admin.paymentChannels.interactionModes.redirect' },
    ]
  }
  if (form.provider_type === 'tokenpay') {
    return [
      { value: 'qr', label: 'admin.paymentChannels.interactionModes.qr' },
      { value: 'redirect', label: 'admin.paymentChannels.interactionModes.redirect' },
    ]
  }
  if (form.provider_type === 'okpay') {
    return [
      { value: 'qr', label: 'admin.paymentChannels.interactionModes.qr' },
      { value: 'redirect', label: 'admin.paymentChannels.interactionModes.redirect' },
    ]
  }
  if (form.provider_type === 'dujiaopay') {
    if (dujiaopayConfig.order_mode === 'cashier') {
      return [
        { value: 'redirect', label: 'admin.paymentChannels.interactionModes.redirect' },
      ]
    }
    return [
      { value: 'qr', label: 'admin.paymentChannels.interactionModes.qr' },
      { value: 'redirect', label: 'admin.paymentChannels.interactionModes.redirect' },
    ]
  }
  if (form.provider_type === 'official' && form.channel_type === 'paypal') {
    return [{ value: 'redirect', label: 'admin.paymentChannels.interactionModes.redirect' }]
  }
  if (form.provider_type === 'official' && form.channel_type === 'stripe') {
    return [{ value: 'redirect', label: 'admin.paymentChannels.interactionModes.redirect' }]
  }
  if (form.provider_type === 'official' && form.channel_type === 'alipay') {
    return [
      { value: 'qr', label: 'admin.paymentChannels.interactionModes.qr' },
      { value: 'wap', label: 'admin.paymentChannels.interactionModes.wap' },
      { value: 'page', label: 'admin.paymentChannels.interactionModes.page' },
    ]
  }
  if (form.provider_type === 'official' && form.channel_type === 'wechat') {
    return [
      { value: 'qr', label: 'admin.paymentChannels.interactionModes.qr' },
      { value: 'redirect', label: 'admin.paymentChannels.interactionModes.redirect' },
    ]
  }
  return [
    { value: 'qr', label: 'admin.paymentChannels.interactionModes.qr' },
    { value: 'redirect', label: 'admin.paymentChannels.interactionModes.redirect' },
  ]
})

const pickDefaultInteractionMode = () => {
  const options = interactionModeOptions.value
  if (options.length === 0) {
    return 'qr'
  }
  return options[0]?.value || 'qr'
}

// --- Reset functions ---

const resetEpayConfig = () => {
  epayConfig.epay_version = 'v2'
  epayConfig.gateway_url = ''
  epayConfig.merchant_id = ''
  epayConfig.merchant_key = ''
  epayConfig.private_key = ''
  epayConfig.platform_public_key = ''
  epayConfig.notify_url = ''
  epayConfig.return_url = ''
  epayConfig.target_currency = ''
  epayConfig.exchange_rate = ''
}

const resetPaypalConfig = () => {
  paypalConfig.client_id = ''
  paypalConfig.client_secret = ''
  paypalConfig.base_url = 'https://api-m.sandbox.paypal.com'
  paypalConfig.return_url = ''
  paypalConfig.cancel_url = ''
  paypalConfig.webhook_id = ''
  paypalConfig.brand_name = ''
  paypalConfig.locale = ''
  paypalConfig.target_currency = ''
  paypalConfig.exchange_rate = ''
}

const resetStripeConfig = () => {
  stripeConfig.secret_key = ''
  stripeConfig.publishable_key = ''
  stripeConfig.webhook_secret = ''
  stripeConfig.success_url = ''
  stripeConfig.cancel_url = ''
  stripeConfig.api_base_url = 'https://api.stripe.com'
  stripeConfig.payment_method_types = ''
  stripeConfig.target_currency = ''
  stripeConfig.exchange_rate = ''
}

const resetAlipayConfig = () => {
  alipayConfig.app_id = ''
  alipayConfig.private_key = ''
  alipayConfig.alipay_public_key = ''
  alipayConfig.gateway_url = 'https://openapi.alipay.com/gateway.do'
  alipayConfig.notify_url = ''
  alipayConfig.return_url = ''
  alipayConfig.sign_type = 'RSA2'
  alipayConfig.app_cert_sn = ''
  alipayConfig.alipay_root_cert_sn = ''
  alipayConfig.target_currency = ''
  alipayConfig.exchange_rate = ''
}

const resetWechatConfig = () => {
  wechatConfig.appid = ''
  wechatConfig.mchid = ''
  wechatConfig.merchant_serial_no = ''
  wechatConfig.merchant_private_key = ''
  wechatConfig.api_v3_key = ''
  wechatConfig.verification_mode = 'platform_certificate'
  wechatConfig.wechatpay_public_key_id = ''
  wechatConfig.wechatpay_public_key = ''
  wechatConfig.notify_url = ''
  wechatConfig.h5_redirect_url = ''
  wechatConfig.h5_type = 'WAP'
  wechatConfig.h5_wap_url = ''
  wechatConfig.h5_wap_name = ''
  wechatConfig.target_currency = ''
  wechatConfig.exchange_rate = ''
}

const resetBepusdtConfig = () => {
  bepusdtConfig.gateway_url = ''
  bepusdtConfig.auth_token = ''
  bepusdtConfig.order_mode = 'transaction'
  bepusdtConfig.trade_type = 'usdt.trc20'
  bepusdtConfig.currencies = ''
  bepusdtConfig.fiat = 'CNY'
  bepusdtConfig.notify_url = 'https://api.yourdomain.com/api/v1/payments/callback'
  bepusdtConfig.return_url = 'https://yourdomain.com/pay'
}

const resetEpusdtConfig = () => {
  epusdtConfig.gateway_url = ''
  epusdtConfig.pid = ''
  epusdtConfig.secret_key = ''
  epusdtConfig.order_mode = 'transaction'
  epusdtConfig.token = 'usdt'
  epusdtConfig.network = 'tron'
  epusdtConfig.currency = 'cny'
  epusdtConfig.notify_url = 'https://api.yourdomain.com/api/v1/payments/callback'
  epusdtConfig.return_url = 'https://yourdomain.com/pay'
}

const resetTokenpayConfig = () => {
  tokenpayConfig.gateway_url = ''
  tokenpayConfig.notify_secret = ''
  tokenpayConfig.currency = 'USDT'
  tokenpayConfig.notify_url = 'https://api.yourdomain.com/api/v1/payments/callback'
  tokenpayConfig.redirect_url = 'https://yourdomain.com/pay'
  tokenpayConfig.base_currency = 'CNY'
}

const resetOkpayConfig = () => {
  okpayConfig.gateway_url = 'https://api.okaypay.me/shop'
  okpayConfig.merchant_id = ''
  okpayConfig.merchant_token = ''
  okpayConfig.exchange_rate = '1'
  okpayConfig.return_url = 'https://yourdomain.com/pay'
  okpayConfig.callback_url = 'https://api.yourdomain.com/api/v1/payments/callback'
  okpayConfig.display_name = ''
}

const resetDujiaoPayConfig = () => {
  dujiaopayConfig.api_base_url = 'https://www.dujiaopay.com'
  dujiaopayConfig.api_key_id = ''
  dujiaopayConfig.api_secret = ''
  dujiaopayConfig.webhook_secret = ''
  dujiaopayConfig.order_mode = 'transaction'
  dujiaopayConfig.allowed_methods = ''
  dujiaopayConfig.fiat_currency = 'CNY'
  dujiaopayConfig.success_url = 'https://yourdomain.com/pay'
  dujiaopayConfig.cancel_url = 'https://yourdomain.com/pay'
}

const resetAllConfigs = () => {
  resetEpayConfig()
  resetPaypalConfig()
  resetStripeConfig()
  resetAlipayConfig()
  resetWechatConfig()
  resetBepusdtConfig()
  resetEpusdtConfig()
  resetTokenpayConfig()
  resetOkpayConfig()
  resetDujiaoPayConfig()
}

// --- Apply functions ---

const applyEpayConfig = (raw: Record<string, unknown>) => {
  const version = String(raw.epay_version || '').toLowerCase()
  epayConfig.epay_version = version === 'v1' ? 'v1' : 'v2'
  epayConfig.gateway_url = String(raw.gateway_url || '')
  epayConfig.merchant_id = String(raw.merchant_id || '')
  epayConfig.merchant_key = String(raw.merchant_key || '')
  epayConfig.private_key = String(raw.private_key || '')
  epayConfig.platform_public_key = String(raw.platform_public_key || '')
  epayConfig.notify_url = String(raw.notify_url || '')
  epayConfig.return_url = String(raw.return_url || '')
  epayConfig.target_currency = String(raw.target_currency || '')
  epayConfig.exchange_rate = String(raw.exchange_rate || '')
}

const applyPaypalConfig = (raw: Record<string, unknown>) => {
  paypalConfig.client_id = String(raw.client_id || '')
  paypalConfig.client_secret = String(raw.client_secret || '')
  paypalConfig.base_url = String(raw.base_url || 'https://api-m.sandbox.paypal.com')
  paypalConfig.return_url = String(raw.return_url || '')
  paypalConfig.cancel_url = String(raw.cancel_url || '')
  paypalConfig.webhook_id = String(raw.webhook_id || '')
  paypalConfig.brand_name = String(raw.brand_name || '')
  paypalConfig.locale = String(raw.locale || '')
  paypalConfig.target_currency = String(raw.target_currency || '')
  paypalConfig.exchange_rate = String(raw.exchange_rate || '')
}

const applyStripeConfig = (raw: Record<string, unknown>) => {
  stripeConfig.secret_key = String(raw.secret_key || '')
  stripeConfig.publishable_key = String(raw.publishable_key || '')
  stripeConfig.webhook_secret = String(raw.webhook_secret || '')
  stripeConfig.success_url = String(raw.success_url || '')
  stripeConfig.cancel_url = String(raw.cancel_url || '')
  stripeConfig.api_base_url = String(raw.api_base_url || 'https://api.stripe.com')
  const methodTypes = Array.isArray(raw.payment_method_types)
    ? (raw.payment_method_types as unknown[]).map((item) => String(item || '').trim()).filter(Boolean)
    : []
  stripeConfig.payment_method_types = methodTypes.length > 0 ? methodTypes.join(',') : ''
  stripeConfig.target_currency = String(raw.target_currency || '')
  stripeConfig.exchange_rate = String(raw.exchange_rate || '')
}

const applyAlipayConfig = (raw: Record<string, unknown>) => {
  alipayConfig.app_id = String(raw.app_id || '')
  alipayConfig.private_key = String(raw.private_key || '')
  alipayConfig.alipay_public_key = String(raw.alipay_public_key || '')
  alipayConfig.gateway_url = String(raw.gateway_url || 'https://openapi.alipay.com/gateway.do')
  alipayConfig.notify_url = String(raw.notify_url || '')
  alipayConfig.return_url = String(raw.return_url || '')
  alipayConfig.sign_type = String(raw.sign_type || 'RSA2').toUpperCase()
  alipayConfig.app_cert_sn = String(raw.app_cert_sn || '')
  alipayConfig.alipay_root_cert_sn = String(raw.alipay_root_cert_sn || '')
  alipayConfig.target_currency = String(raw.target_currency || '')
  alipayConfig.exchange_rate = String(raw.exchange_rate || '')
}

const applyWechatConfig = (raw: Record<string, unknown>) => {
  wechatConfig.appid = String(raw.appid || '')
  wechatConfig.mchid = String(raw.mchid || '')
  wechatConfig.merchant_serial_no = String(raw.merchant_serial_no || '')
  wechatConfig.merchant_private_key = String(raw.merchant_private_key || '')
  wechatConfig.api_v3_key = String(raw.api_v3_key || '')
  const verificationMode = String(raw.verification_mode || 'platform_certificate').trim().toLowerCase()
  wechatConfig.verification_mode = ['platform_certificate', 'wechatpay_public_key', 'combined'].includes(verificationMode)
    ? verificationMode
    : 'platform_certificate'
  wechatConfig.wechatpay_public_key_id = String(raw.wechatpay_public_key_id || '')
  wechatConfig.wechatpay_public_key = String(raw.wechatpay_public_key || '')
  wechatConfig.notify_url = String(raw.notify_url || '')
  wechatConfig.h5_redirect_url = String(raw.h5_redirect_url || '')
  wechatConfig.h5_type = String(raw.h5_type || 'WAP').toUpperCase()
  wechatConfig.h5_wap_url = String(raw.h5_wap_url || '')
  wechatConfig.h5_wap_name = String(raw.h5_wap_name || '')
  wechatConfig.target_currency = String(raw.target_currency || '')
  wechatConfig.exchange_rate = String(raw.exchange_rate || '')
}

const applyBepusdtConfig = (raw: Record<string, unknown>) => {
  bepusdtConfig.gateway_url = String(raw.gateway_url || '')
  bepusdtConfig.auth_token = String(raw.auth_token || '')
  bepusdtConfig.order_mode = String(raw.order_mode || 'transaction') === 'cashier' ? 'cashier' : 'transaction'
  bepusdtConfig.trade_type = bepusdtConfig.order_mode === 'cashier' ? '' : String(raw.trade_type || 'usdt.trc20')
  bepusdtConfig.currencies = bepusdtConfig.order_mode === 'cashier' ? String(raw.currencies || '') : ''
  bepusdtConfig.fiat = String(raw.fiat || 'CNY')
  bepusdtConfig.notify_url = String(raw.notify_url || '')
  bepusdtConfig.return_url = String(raw.return_url || '')
}

const applyEpusdtConfig = (raw: Record<string, unknown>) => {
  epusdtConfig.gateway_url = String(raw.gateway_url || '')
  epusdtConfig.pid = String(raw.pid || '')
  epusdtConfig.secret_key = String(raw.secret_key || '')
  epusdtConfig.order_mode = String(raw.order_mode || 'transaction') === 'cashier' ? 'cashier' : 'transaction'
  epusdtConfig.token = epusdtConfig.order_mode === 'cashier' ? '' : String(raw.token || 'usdt')
  epusdtConfig.network = epusdtConfig.order_mode === 'cashier' ? '' : String(raw.network || 'tron')
  epusdtConfig.currency = String(raw.currency || 'cny')
  epusdtConfig.notify_url = String(raw.notify_url || '')
  epusdtConfig.return_url = String(raw.return_url || '')
}

const applyTokenpayConfig = (raw: Record<string, unknown>) => {
  tokenpayConfig.gateway_url = String(raw.gateway_url || '')
  tokenpayConfig.notify_secret = String(raw.notify_secret || '')
  tokenpayConfig.currency = String(raw.currency || 'USDT')
  tokenpayConfig.notify_url = String(raw.notify_url || '')
  tokenpayConfig.redirect_url = String(raw.redirect_url || '')
  tokenpayConfig.base_currency = String(raw.base_currency || 'CNY')
}

const applyOkpayConfig = (raw: Record<string, unknown>) => {
  okpayConfig.gateway_url = String(raw.gateway_url || 'https://api.okaypay.me/shop')
  okpayConfig.merchant_id = String(raw.merchant_id || '')
  okpayConfig.merchant_token = String(raw.merchant_token || '')
  okpayConfig.exchange_rate = String(raw.exchange_rate || '1')
  okpayConfig.return_url = String(raw.return_url || '')
  okpayConfig.callback_url = String(raw.callback_url || '')
  okpayConfig.display_name = String(raw.display_name || '')
}

const applyDujiaoPayConfig = (raw: Record<string, unknown>) => {
  dujiaopayConfig.api_base_url = String(raw.api_base_url || 'https://www.dujiaopay.com')
  dujiaopayConfig.api_key_id = String(raw.api_key_id || '')
  dujiaopayConfig.api_secret = String(raw.api_secret || '')
  dujiaopayConfig.webhook_secret = String(raw.webhook_secret || '')
  dujiaopayConfig.order_mode = String(raw.order_mode || 'transaction') === 'cashier' ? 'cashier' : 'transaction'
  dujiaopayConfig.allowed_methods =
    dujiaopayConfig.order_mode === 'cashier' ? String(raw.allowed_methods || '') : ''
  dujiaopayConfig.fiat_currency = String(raw.fiat_currency || 'CNY').toUpperCase()
  dujiaopayConfig.success_url = String(raw.success_url || '')
  dujiaopayConfig.cancel_url = String(raw.cancel_url || '')
}

// --- Build functions ---

const buildEpayConfig = () => {
  const config: Record<string, unknown> = {}
  const setIfNotEmpty = (key: string, value: string) => {
    const trimmed = String(value || '').trim()
    if (trimmed !== '') {
      config[key] = trimmed
    }
  }
  setIfNotEmpty('epay_version', epayConfig.epay_version)
  setIfNotEmpty('gateway_url', epayConfig.gateway_url)
  setIfNotEmpty('merchant_id', epayConfig.merchant_id)
  setIfNotEmpty('notify_url', epayConfig.notify_url)
  setIfNotEmpty('return_url', epayConfig.return_url)
  if (epayConfig.epay_version === 'v1') {
    setIfNotEmpty('merchant_key', epayConfig.merchant_key)
  } else {
    setIfNotEmpty('private_key', epayConfig.private_key)
    setIfNotEmpty('platform_public_key', epayConfig.platform_public_key)
  }
  setIfNotEmpty('target_currency', epayConfig.target_currency)
  setIfNotEmpty('exchange_rate', epayConfig.exchange_rate)
  return config
}

const buildPaypalConfig = () => {
  const config: Record<string, unknown> = {}
  const setIfNotEmpty = (key: string, value: string) => {
    const trimmed = String(value || '').trim()
    if (trimmed !== '') {
      config[key] = trimmed
    }
  }
  setIfNotEmpty('client_id', paypalConfig.client_id)
  setIfNotEmpty('client_secret', paypalConfig.client_secret)
  setIfNotEmpty('base_url', paypalConfig.base_url)
  setIfNotEmpty('return_url', paypalConfig.return_url)
  setIfNotEmpty('cancel_url', paypalConfig.cancel_url)
  setIfNotEmpty('webhook_id', paypalConfig.webhook_id)
  setIfNotEmpty('brand_name', paypalConfig.brand_name)
  setIfNotEmpty('locale', paypalConfig.locale)
  setIfNotEmpty('target_currency', paypalConfig.target_currency)
  setIfNotEmpty('exchange_rate', paypalConfig.exchange_rate)
  return config
}

const buildStripeConfig = () => {
  const config: Record<string, unknown> = {}
  const setIfNotEmpty = (key: string, value: string) => {
    const trimmed = String(value || '').trim()
    if (trimmed !== '') {
      config[key] = trimmed
    }
  }
  setIfNotEmpty('secret_key', stripeConfig.secret_key)
  setIfNotEmpty('publishable_key', stripeConfig.publishable_key)
  setIfNotEmpty('webhook_secret', stripeConfig.webhook_secret)
  setIfNotEmpty('success_url', stripeConfig.success_url)
  setIfNotEmpty('cancel_url', stripeConfig.cancel_url)
  setIfNotEmpty('api_base_url', stripeConfig.api_base_url)
  const methodTypes = String(stripeConfig.payment_method_types || '')
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
  config.payment_method_types = methodTypes
  setIfNotEmpty('target_currency', stripeConfig.target_currency)
  setIfNotEmpty('exchange_rate', stripeConfig.exchange_rate)
  return config
}

const buildAlipayConfig = () => {
  const config: Record<string, unknown> = {}
  const setIfNotEmpty = (key: string, value: string) => {
    const trimmed = String(value || '').trim()
    if (trimmed !== '') {
      config[key] = trimmed
    }
  }
  setIfNotEmpty('app_id', alipayConfig.app_id)
  setIfNotEmpty('private_key', alipayConfig.private_key)
  setIfNotEmpty('alipay_public_key', alipayConfig.alipay_public_key)
  setIfNotEmpty('gateway_url', alipayConfig.gateway_url)
  setIfNotEmpty('notify_url', alipayConfig.notify_url)
  setIfNotEmpty('return_url', alipayConfig.return_url)
  setIfNotEmpty('sign_type', alipayConfig.sign_type)
  setIfNotEmpty('app_cert_sn', alipayConfig.app_cert_sn)
  setIfNotEmpty('alipay_root_cert_sn', alipayConfig.alipay_root_cert_sn)
  setIfNotEmpty('target_currency', alipayConfig.target_currency)
  setIfNotEmpty('exchange_rate', alipayConfig.exchange_rate)
  return config
}

const buildWechatConfig = () => {
  const config: Record<string, unknown> = {}
  const setIfNotEmpty = (key: string, value: string) => {
    const trimmed = String(value || '').trim()
    if (trimmed !== '') {
      config[key] = trimmed
    }
  }
  setIfNotEmpty('appid', wechatConfig.appid)
  setIfNotEmpty('mchid', wechatConfig.mchid)
  setIfNotEmpty('merchant_serial_no', wechatConfig.merchant_serial_no)
  setIfNotEmpty('merchant_private_key', wechatConfig.merchant_private_key)
  setIfNotEmpty('api_v3_key', wechatConfig.api_v3_key)
  setIfNotEmpty('verification_mode', wechatConfig.verification_mode)
  if (wechatConfig.verification_mode !== 'platform_certificate') {
    setIfNotEmpty('wechatpay_public_key_id', wechatConfig.wechatpay_public_key_id)
    setIfNotEmpty('wechatpay_public_key', wechatConfig.wechatpay_public_key)
  }
  setIfNotEmpty('notify_url', wechatConfig.notify_url)
  setIfNotEmpty('h5_redirect_url', wechatConfig.h5_redirect_url)
  setIfNotEmpty('h5_type', wechatConfig.h5_type)
  setIfNotEmpty('h5_wap_url', wechatConfig.h5_wap_url)
  setIfNotEmpty('h5_wap_name', wechatConfig.h5_wap_name)
  setIfNotEmpty('target_currency', wechatConfig.target_currency)
  setIfNotEmpty('exchange_rate', wechatConfig.exchange_rate)
  return config
}

const buildBepusdtConfig = () => {
  const config: Record<string, unknown> = {}

  // Required fields
  config.gateway_url = String(bepusdtConfig.gateway_url || '').trim()
  config.auth_token = String(bepusdtConfig.auth_token || '').trim()
  config.order_mode = String(bepusdtConfig.order_mode || 'transaction') === 'cashier' ? 'cashier' : 'transaction'

  // notify_url and return_url: ensure always have value
  const notifyUrl = String(bepusdtConfig.notify_url || '').trim()
  const returnUrl = String(bepusdtConfig.return_url || '').trim()

  config.notify_url = notifyUrl || 'https://api.yourdomain.com/api/v1/payments/callback'
  config.return_url = returnUrl || 'https://yourdomain.com/pay'

  // Optional fields
  const tradeType = String(bepusdtConfig.trade_type || '').trim()
  if (config.order_mode !== 'cashier' && tradeType !== '') {
    config.trade_type = tradeType
  }
  const currencies = String(bepusdtConfig.currencies || '').trim().toUpperCase().replace(/\s+/g, '')
  if (config.order_mode === 'cashier' && currencies !== '') {
    config.currencies = currencies
  }
  const fiat = String(bepusdtConfig.fiat || '').trim()
  if (fiat !== '') {
    config.fiat = fiat
  }

  return config
}

// BEpusdt Special: channel_type identifies the provider; trade_type lives in config_json.
const resolveBepusdtChannelType = () => 'bepusdt'

const buildEpusdtConfig = () => {
  const config: Record<string, unknown> = {}
  config.gateway_url = String(epusdtConfig.gateway_url || '').trim()
  config.pid = String(epusdtConfig.pid || '').trim()
  config.secret_key = String(epusdtConfig.secret_key || '').trim()
  config.order_mode = String(epusdtConfig.order_mode || 'transaction') === 'cashier' ? 'cashier' : 'transaction'
  config.token = config.order_mode === 'cashier' ? '' : String(epusdtConfig.token || '').trim().toLowerCase()
  config.network = config.order_mode === 'cashier' ? '' : String(epusdtConfig.network || '').trim().toLowerCase()
  config.currency = String(epusdtConfig.currency || '').trim().toLowerCase()
  config.notify_url = String(epusdtConfig.notify_url || '').trim()
  config.return_url = String(epusdtConfig.return_url || '').trim()
  return config
}

const buildTokenpayConfig = () => {
  const config: Record<string, unknown> = {}
  const setIfNotEmpty = (key: string, value: string) => {
    const trimmed = String(value || '').trim()
    if (trimmed !== '') {
      config[key] = trimmed
    }
  }
  setIfNotEmpty('gateway_url', tokenpayConfig.gateway_url)
  setIfNotEmpty('notify_secret', tokenpayConfig.notify_secret)
  setIfNotEmpty('currency', tokenpayConfig.currency)
  setIfNotEmpty('notify_url', tokenpayConfig.notify_url)
  setIfNotEmpty('redirect_url', tokenpayConfig.redirect_url)
  setIfNotEmpty('base_currency', tokenpayConfig.base_currency)
  return config
}

const buildOkpayConfig = () => {
  const config: Record<string, unknown> = {}
  const setIfNotEmpty = (key: string, value: string) => {
    const trimmed = String(value || '').trim()
    if (trimmed !== '') {
      config[key] = trimmed
    }
  }
  setIfNotEmpty('gateway_url', okpayConfig.gateway_url)
  setIfNotEmpty('merchant_id', okpayConfig.merchant_id)
  setIfNotEmpty('merchant_token', okpayConfig.merchant_token)
  setIfNotEmpty('exchange_rate', okpayConfig.exchange_rate)
  setIfNotEmpty('return_url', okpayConfig.return_url)
  setIfNotEmpty('callback_url', okpayConfig.callback_url)
  setIfNotEmpty('display_name', okpayConfig.display_name)
  if (form.channel_type === 'usdt') {
    config.coin = 'USDT'
  } else if (form.channel_type === 'trx') {
    config.coin = 'TRX'
  }
  return config
}

const buildDujiaoPayConfig = () => {
  const config: Record<string, unknown> = {}
  const setIfNotEmpty = (key: string, value: string) => {
    const trimmed = String(value || '').trim()
    if (trimmed !== '') {
      config[key] = trimmed
    }
  }
  setIfNotEmpty('api_base_url', dujiaopayConfig.api_base_url)
  setIfNotEmpty('api_key_id', dujiaopayConfig.api_key_id)
  setIfNotEmpty('api_secret', dujiaopayConfig.api_secret)
  setIfNotEmpty('webhook_secret', dujiaopayConfig.webhook_secret)
  setIfNotEmpty('fiat_currency', dujiaopayConfig.fiat_currency.toUpperCase())
  setIfNotEmpty('success_url', dujiaopayConfig.success_url)
  setIfNotEmpty('cancel_url', dujiaopayConfig.cancel_url)
  config.order_mode = String(dujiaopayConfig.order_mode || 'transaction') === 'cashier' ? 'cashier' : 'transaction'
  if (config.order_mode === 'cashier') {
    const methods = String(dujiaopayConfig.allowed_methods || '')
      .split(',')
      .map((item) => item.trim().toLowerCase())
      .filter((item) => item !== '')
    if (methods.length > 0) {
      config.allowed_methods = methods.join(',')
    }
  } else if (form.channel_type) {
    config.token_id = String(form.channel_type).trim().toLowerCase()
  }
  return config
}

// token_id 由管理员手动输入，提交前统一规范化，避免大小写或空格写进 channel_type。
const resolveDujiaopayChannelType = () =>
  dujiaopayConfig.order_mode === 'cashier'
    ? 'dujiaopay'
    : String(form.channel_type || '').trim().toLowerCase()

// --- Watchers for provider_type / channel_type ---

watch(
  () => form.provider_type,
  (value) => {
    if (applyingChannelData.value) {
      return
    }
    if (value === 'epay') {
      const allowed = epayChannelOptions.map((option) => option.value)
      if (!allowed.includes(form.channel_type)) {
        form.channel_type = allowed[0] || 'wechat'
      }
    } else if (value === 'official') {
      const allowed = officialChannelOptions.map((option) => option.value)
      if (!allowed.includes(form.channel_type)) {
        form.channel_type = allowed[0] || 'paypal'
      }
    } else if (value === 'bepusdt') {
      form.channel_type = 'bepusdt'
    } else if (value === 'epusdt') {
      form.channel_type = 'epusdt'
    } else if (value === 'okpay') {
      const allowed = okpayChannelOptions.map((option) => option.value)
      if (!allowed.includes(form.channel_type)) {
        form.channel_type = allowed[0] || 'usdt'
      }
    } else if (value === 'dujiaopay') {
      if (dujiaopayConfig.order_mode === 'cashier') {
        form.channel_type = 'dujiaopay'
      } else if (String(form.channel_type || '').trim() === '' || form.channel_type === 'dujiaopay') {
        // token_id 由管理员手动输入，这里只在为空或残留 cashier 值时填一个默认值。
        form.channel_type = dujiaopayDefaultTokenID
      }
    } else if (value === 'tokenpay') {
      form.channel_type = 'usdt'
    }
    const allowedInteractionModes = interactionModeOptions.value.map((item) => item.value)
    if (!allowedInteractionModes.includes(form.interaction_mode)) {
      form.interaction_mode = pickDefaultInteractionMode()
    }
  }
)

watch(
  () => form.channel_type,
  () => {
    if (applyingChannelData.value) {
      return
    }
    const allowed = interactionModeOptions.value.map((item) => item.value)
    if (!allowed.includes(form.interaction_mode)) {
      form.interaction_mode = pickDefaultInteractionMode()
    }
  }
)

watch(
  () => bepusdtConfig.order_mode,
  () => {
    if (form.provider_type !== 'bepusdt' || applyingChannelData.value) {
      return
    }
    if (bepusdtConfig.order_mode === 'cashier') {
      bepusdtConfig.trade_type = ''
      form.channel_type = 'bepusdt'
    } else {
      bepusdtConfig.currencies = ''
      if (String(bepusdtConfig.trade_type || '').trim() === '') {
        bepusdtConfig.trade_type = 'usdt.trc20'
      }
      form.channel_type = 'bepusdt'
    }
    const allowed = interactionModeOptions.value.map((item) => item.value)
    if (!allowed.includes(form.interaction_mode)) {
      form.interaction_mode = pickDefaultInteractionMode()
    }
  }
)

watch(
  () => epusdtConfig.order_mode,
  () => {
    if (form.provider_type !== 'epusdt' || applyingChannelData.value) {
      return
    }
    if (epusdtConfig.order_mode === 'cashier') {
      epusdtConfig.token = ''
      epusdtConfig.network = ''
      return
    }
    if (String(epusdtConfig.token || '').trim() === '') {
      epusdtConfig.token = 'usdt'
    }
    if (String(epusdtConfig.network || '').trim() === '') {
      epusdtConfig.network = 'tron'
    }
  }
)

watch(
  () => dujiaopayConfig.order_mode,
  () => {
    if (form.provider_type !== 'dujiaopay' || applyingChannelData.value) {
      return
    }
    if (dujiaopayConfig.order_mode === 'cashier') {
      form.channel_type = 'dujiaopay'
    } else {
      dujiaopayConfig.allowed_methods = ''
      if (String(form.channel_type || '').trim() === '' || form.channel_type === 'dujiaopay') {
        form.channel_type = dujiaopayDefaultTokenID
      }
    }
    const allowedModes = interactionModeOptions.value.map((item) => item.value)
    if (!allowedModes.includes(form.interaction_mode)) {
      form.interaction_mode = pickDefaultInteractionMode()
    }
  }
)

// --- Reset form for create mode ---
function resetFormForCreate() {
  error.value = ''
  wechatPublicKeyTestResult.value = null
  showAdvanced.value = false
  applyingChannelData.value = true
  form.name = ''
  form.icon = ''
  form.provider_type = 'epay'
  form.channel_type = 'alipay'
  form.interaction_mode = 'qr'
  form.fee_rate = '0'
  form.fixed_fee = '0'
  form.min_amount = '0'
  form.max_amount = '0'
  form.hide_amount_out_range = false
  form.payment_types = []
  form.payment_roles = []
  form.member_levels = []
  form.config_json = ''
  form.is_active = true
  form.sort_order = 10
  resetAllConfigs()
  applyingChannelData.value = false
}

// --- Watch modelValue to reset form when dialog opens in create mode ---
watch(
  () => props.modelValue,
  (open) => {
    if (open && props.channelId === null) {
      resetFormForCreate()
    }
  }
)

// --- Watch channelId to load data or reset ---

watch(
  () => props.channelId,
  async (id) => {
    error.value = ''
    wechatPublicKeyTestResult.value = null
    showAdvanced.value = false
    if (id === null) {
      // Create mode: reset form
      resetFormForCreate()
    } else {
      // Edit mode: fetch channel details
      try {
        applyingChannelData.value = true
        const response = await adminAPI.getPaymentChannel(id)
        const channel = response.data.data
        form.name = channel.name
        form.icon = channel.icon || ''
        form.provider_type = channel.provider_type
        form.channel_type = channel.channel_type
        form.interaction_mode = channel.interaction_mode
        form.fee_rate = channel.fee_rate !== undefined && channel.fee_rate !== null ? String(channel.fee_rate) : '0'
        form.fixed_fee = channel.fixed_fee !== undefined && channel.fixed_fee !== null ? String(channel.fixed_fee) : '0'
        form.min_amount = channel.min_amount !== undefined && channel.min_amount !== null ? String(channel.min_amount) : '0'
        form.max_amount = channel.max_amount !== undefined && channel.max_amount !== null ? String(channel.max_amount) : '0'
        form.hide_amount_out_range = Boolean(channel.hide_amount_out_range)
        form.payment_types = Array.isArray(channel.payment_types) ? channel.payment_types : []
        form.payment_roles = Array.isArray(channel.payment_roles) ? channel.payment_roles : []
        form.member_levels = Array.isArray(channel.member_levels) ? channel.member_levels : []
        form.config_json = channel.config_json ? JSON.stringify(channel.config_json, null, 2) : ''
        form.is_active = !!channel.is_active
        form.sort_order = channel.sort_order || 0
        if (channel.config_json && typeof channel.config_json === 'object') {
          applyEpayConfig(channel.config_json)
          applyPaypalConfig(channel.config_json)
          applyStripeConfig(channel.config_json)
          applyAlipayConfig(channel.config_json)
          applyWechatConfig(channel.config_json)
          applyBepusdtConfig(channel.config_json)
          applyEpusdtConfig(channel.config_json)
          applyTokenpayConfig(channel.config_json)
          applyOkpayConfig(channel.config_json)
          applyDujiaoPayConfig(channel.config_json)
          if (channel.provider_type === 'okpay' && !String(form.channel_type || '').trim()) {
            form.channel_type = resolveOkpayChannelTypeFromConfig(channel.config_json)
          }
        } else {
          resetAllConfigs()
        }
      } catch (err: any) {
        error.value = err?.message || t('admin.paymentChannels.errors.fetchFailed')
      } finally {
        applyingChannelData.value = false
        const allowed = interactionModeOptions.value.map((item) => item.value)
        if (!allowed.includes(form.interaction_mode)) {
          form.interaction_mode = pickDefaultInteractionMode()
        }
      }
    }
  }
)

// --- Submit ---

const handleWechatPublicKeyTest = async () => {
  if (!props.channelId || testingWechatPublicKey.value) {
    return
  }
  error.value = ''
  wechatPublicKeyTestResult.value = null
  testingWechatPublicKey.value = true
  try {
    const response = await adminAPI.testWechatPayPublicKey(props.channelId)
    wechatPublicKeyTestResult.value = response.data.data as AdminGatewaySecurityTestResult
  } catch (err: any) {
    error.value = err?.message || t('admin.paymentChannels.modal.wechatPublicKeyTestFailed')
  } finally {
    testingWechatPublicKey.value = false
  }
}

const handleSubmit = async () => {
  error.value = ''
  let configJson: Record<string, unknown> = {}
  let explicitlyNullConfigKeys: string[] = []
  if (form.config_json && form.config_json.trim() !== '') {
    try {
      const parsed = JSON.parse(form.config_json)
      if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
        configJson = parsed
        explicitlyNullConfigKeys = Object.entries(configJson)
          .filter(([, value]) => value === null)
          .map(([key]) => key)
      }
    } catch (err) {
      error.value = t('admin.paymentChannels.errors.invalidConfig')
      return
    }
  }

  if (form.provider_type === 'epay') {
    if (epayConfig.epay_version === 'v1') {
      delete configJson.private_key
      delete configJson.platform_public_key
    } else {
      delete configJson.merchant_key
    }
    delete configJson.target_currency
    delete configJson.exchange_rate
    configJson = {
      ...configJson,
      ...buildEpayConfig(),
    }
  } else if (form.provider_type === 'official' && form.channel_type === 'paypal') {
    delete configJson.target_currency
    delete configJson.exchange_rate
    configJson = {
      ...configJson,
      ...buildPaypalConfig(),
    }
  } else if (form.provider_type === 'official' && form.channel_type === 'stripe') {
    delete configJson.target_currency
    delete configJson.exchange_rate
    configJson = {
      ...configJson,
      ...buildStripeConfig(),
    }
  } else if (form.provider_type === 'official' && form.channel_type === 'alipay') {
    delete configJson.target_currency
    delete configJson.exchange_rate
    configJson = {
      ...configJson,
      ...buildAlipayConfig(),
    }
  } else if (form.provider_type === 'official' && form.channel_type === 'wechat') {
    delete configJson.target_currency
    delete configJson.exchange_rate
    configJson = {
      ...configJson,
      ...buildWechatConfig(),
    }
  } else if (form.provider_type === 'bepusdt') {
    // currencies 一律删除，build 在 cashier 且非空时重新写入，清空即保存为不限制
    delete configJson.currencies
    if (bepusdtConfig.order_mode === 'cashier') {
      delete configJson.trade_type
    }
    configJson = {
      ...configJson,
      ...buildBepusdtConfig(),
    }
  } else if (form.provider_type === 'epusdt') {
    configJson = {
      ...configJson,
      ...buildEpusdtConfig(),
    }
  } else if (form.provider_type === 'tokenpay') {
    configJson = {
      ...configJson,
      ...buildTokenpayConfig(),
    }
  } else if (form.provider_type === 'okpay') {
    delete configJson.exchange_rate
    configJson = {
      ...configJson,
      ...buildOkpayConfig(),
    }
  } else if (form.provider_type === 'dujiaopay') {
    // chain 一律删除，由后端按 token_id 重新推导，避免切换 token 后残留旧链；
    // allowed_methods 一律删除，build 在 cashier 且非空时重新写入，清空即保存为不限制
    delete configJson.chain
    delete configJson.allowed_methods
    if (dujiaopayConfig.order_mode === 'cashier') {
      delete configJson.token_id
    }
    configJson = {
      ...configJson,
      ...buildDujiaoPayConfig(),
    }
  }

  // 专用表单构建器可能会重写同名字段；高级 JSON 中的显式 null
  // 始终具有最高优先级，用于区分“清空密钥”和“保留掩码值”。
  explicitlyNullConfigKeys.forEach((key) => {
    configJson[key] = null
  })

  const payload = {
    name: form.name,
    icon: form.icon || '',
    provider_type: form.provider_type,
    channel_type:
      form.provider_type === 'tokenpay'
        ? 'usdt'
        : form.provider_type === 'bepusdt'
          ? resolveBepusdtChannelType()
          : form.provider_type === 'epusdt'
            ? 'epusdt'
            : form.provider_type === 'dujiaopay'
              ? resolveDujiaopayChannelType()
          : form.channel_type,
    interaction_mode: form.interaction_mode,
    fee_rate: String(form.fee_rate || '0').trim(),
    fixed_fee: String(form.fixed_fee || '0').trim(),
    min_amount: String(form.min_amount || '0').trim(),
    max_amount: String(form.max_amount || '0').trim(),
    hide_amount_out_range: form.hide_amount_out_range,
    payment_types: form.payment_types,
    payment_roles: form.payment_roles,
    member_levels: form.member_levels,
    config_json: configJson,
    is_active: form.is_active,
    sort_order: form.sort_order,
  }

  if (isEditing.value && props.channelId) {
    await adminAPI.updatePaymentChannel(props.channelId, payload)
  } else {
    await adminAPI.createPaymentChannel(payload)
  }
  emit('update:modelValue', false)
  emit('success')
}

const closeModal = () => {
  emit('update:modelValue', false)
}
</script>

<template>
  <Dialog :open="modelValue" @update:open="(value) => { if (!value) closeModal() }">
    <DialogScrollContent class="w-[calc(100vw-1rem)] max-w-3xl p-4 sm:p-6" @interact-outside="(e) => e.preventDefault()">
      <DialogHeader>
        <DialogTitle>{{ isEditing ? t('admin.paymentChannels.modal.editTitle') : t('admin.paymentChannels.modal.createTitle') }}</DialogTitle>
      </DialogHeader>
      <form class="space-y-4" @submit.prevent="handleSubmit">
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2 [&>*]:min-w-0">
          <div class="min-w-0">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.name') }}</label>
            <Input v-model="form.name" required :placeholder="t('admin.paymentChannels.modal.namePlaceholder')" />
          </div>
          <div class="min-w-0">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.icon') }}</label>
            <MediaPicker v-model="form.icon" scene="common" />
          </div>
          <div class="min-w-0">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.providerType') }}</label>
            <Select v-model="form.provider_type">
              <SelectTrigger class="h-9 w-full">
                <SelectValue :placeholder="t('admin.paymentChannels.providerTypes.official')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="official">{{ t('admin.paymentChannels.providerTypes.official') }}</SelectItem>
                <SelectItem value="dujiaopay">
                  <span class="flex w-full items-center justify-between gap-2">
                    <span>{{ t('admin.paymentChannels.providerTypes.dujiaopay') }}</span>
                    <span class="shrink-0 rounded border border-emerald-500/30 bg-emerald-500/10 px-1.5 py-0.5 text-[10px] font-semibold leading-none text-emerald-600 dark:text-emerald-400">
                      {{ t('admin.paymentChannels.providerOfficialCertified') }}
                    </span>
                  </span>
                </SelectItem>
                <SelectItem value="epay">{{ t('admin.paymentChannels.providerTypes.epay') }}</SelectItem>
                <SelectItem value="bepusdt">{{ t('admin.paymentChannels.providerTypes.bepusdt') }}</SelectItem>
                <SelectItem value="epusdt">{{ t('admin.paymentChannels.providerTypes.epusdt') }}</SelectItem>
                <SelectItem value="okpay">{{ t('admin.paymentChannels.providerTypes.okpay') }}</SelectItem>
                <SelectItem value="tokenpay">{{ t('admin.paymentChannels.providerTypes.tokenpay') }}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div v-if="form.provider_type !== 'tokenpay' && form.provider_type !== 'bepusdt' && form.provider_type !== 'epusdt' && form.provider_type !== 'dujiaopay'" class="min-w-0">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.channelType') }}</label>
            <Select v-model="form.channel_type">
              <SelectTrigger class="h-9 w-full">
                <SelectValue :placeholder="t('admin.paymentChannels.channelTypes.wechat')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="option in formChannelOptions" :key="option.value" :value="option.value">
                  {{ t(option.label) }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="min-w-0">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.interactionMode') }}</label>
            <Select v-model="form.interaction_mode">
              <SelectTrigger class="h-9 w-full">
                <SelectValue :placeholder="t('admin.paymentChannels.interactionModes.qr')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="option in interactionModeOptions" :key="option.value" :value="option.value">
                  {{ t(option.label) }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="min-w-0">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.sortOrder') }}</label>
            <Input v-model.number="form.sort_order" type="number" placeholder="10" />
          </div>
          <div class="min-w-0">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.feeRate') }}</label>
            <Input v-model="form.fee_rate" type="number" step="0.01" min="0" max="100" :placeholder="t('admin.paymentChannels.modal.feeRatePlaceholder')" />
          </div>
          <div class="min-w-0">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.fixedFee') }}</label>
            <Input v-model="form.fixed_fee" type="number" step="0.01" min="0" :placeholder="t('admin.paymentChannels.modal.fixedFeePlaceholder')" />
          </div>
          <div class="min-w-0">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.minAmount') }}</label>
            <Input v-model="form.min_amount" type="number" step="0.01" min="0" :placeholder="t('admin.paymentChannels.modal.minAmountPlaceholder')" />
          </div>
          <div class="min-w-0">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.maxAmount') }}</label>
            <Input v-model="form.max_amount" type="number" step="0.01" min="0" :placeholder="t('admin.paymentChannels.modal.maxAmountPlaceholder')" />
          </div>
          <div class="mt-2 flex flex-col gap-2 sm:mt-6 sm:flex-row sm:items-center">
            <Switch v-model="form.hide_amount_out_range" />
            <Label class="text-xs text-muted-foreground">{{ t('admin.paymentChannels.modal.hideAmountOutRange') }}</Label>
          </div>
          <div class="min-w-0 md:col-span-2">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.paymentTypes') }}</label>
            <MultiSelect
              v-model="form.payment_types"
              :options="paymentTypeOptions"
              :placeholder="t('admin.paymentChannels.modal.paymentTypesPlaceholder')"
            />
          </div>
          <div class="min-w-0 md:col-span-2">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.paymentRoles') }}</label>
            <MultiSelect
              v-model="form.payment_roles"
              :options="paymentRoleOptions"
              :placeholder="t('admin.paymentChannels.modal.paymentRolesPlaceholder')"
            />
          </div>
          <div class="min-w-0 md:col-span-2">
            <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.memberLevels') }}</label>
            <MultiSelect
              v-model="form.member_levels"
              :options="memberLevelOptions"
              :placeholder="t('admin.paymentChannels.modal.memberLevelsPlaceholder')"
              :disabled="memberLevels.length === 0"
            />
          </div>
          <div class="mt-2 flex flex-col gap-2 sm:mt-6 sm:flex-row sm:items-center">
            <Switch v-model="form.is_active" />
            <Label class="text-xs text-muted-foreground">{{ t('admin.common.enabled') }}</Label>
          </div>
        </div>

        <div v-if="form.provider_type === 'epay'" class="min-w-0 rounded-xl border border-border bg-muted/20 p-4 overflow-hidden">
          <div class="text-sm font-semibold text-foreground mb-3">{{ t('admin.paymentChannels.modal.epaySection') }}</div>
          <div class="grid grid-cols-1 gap-4 md:grid-cols-2 [&>*]:min-w-0">
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.epayVersion') }}</label>
              <Select v-model="epayConfig.epay_version">
                <SelectTrigger class="h-9 w-full">
                  <SelectValue placeholder="v1" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="v1">v1</SelectItem>
                  <SelectItem value="v2">v2</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.gatewayUrl') }}</label>
              <Input v-model="epayConfig.gateway_url" :placeholder="t('admin.paymentChannels.modal.gatewayUrlPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.merchantId') }}</label>
              <Input v-model="epayConfig.merchant_id" :placeholder="t('admin.paymentChannels.modal.merchantIdPlaceholder')" />
            </div>
            <div v-if="epayConfig.epay_version === 'v1'" class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.merchantKey') }}</label>
              <Input v-model="epayConfig.merchant_key" :placeholder="t('admin.paymentChannels.modal.merchantKeyPlaceholder')" />
            </div>
            <div v-else class="min-w-0 md:col-span-2">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.privateKey') }}</label>
              <Textarea v-model="epayConfig.private_key" rows="4" :placeholder="t('admin.paymentChannels.modal.privateKeyPlaceholder')" />
            </div>
            <div v-if="epayConfig.epay_version === 'v2'" class="min-w-0 md:col-span-2">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.platformPublicKey') }}</label>
              <Textarea v-model="epayConfig.platform_public_key" rows="4" :placeholder="t('admin.paymentChannels.modal.platformPublicKeyPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.notifyUrl') }}</label>
              <Input v-model="epayConfig.notify_url" :placeholder="t('admin.paymentChannels.modal.notifyUrlPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.returnUrl') }}</label>
              <Input v-model="epayConfig.return_url" :placeholder="t('admin.paymentChannels.modal.returnUrlPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.targetCurrency') }}</label>
              <Input v-model="epayConfig.target_currency" :placeholder="t('admin.paymentChannels.modal.targetCurrencyPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.exchangeRate') }}</label>
              <Input v-model="epayConfig.exchange_rate" :placeholder="t('admin.paymentChannels.modal.exchangeRatePlaceholder')" />
            </div>
          </div>
          <div class="mt-3 text-xs text-muted-foreground">{{ t('admin.paymentChannels.modal.epayHint') }}</div>
        </div>

        <div v-if="form.provider_type === 'official' && form.channel_type === 'paypal'" class="min-w-0 rounded-xl border border-border bg-muted/20 p-4 overflow-hidden">
          <div class="text-sm font-semibold text-foreground mb-3">{{ t('admin.paymentChannels.modal.paypalSection') }}</div>
          <div class="grid grid-cols-1 gap-4 md:grid-cols-2 [&>*]:min-w-0">
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.paypalClientId') }}</label>
              <Input v-model="paypalConfig.client_id" :placeholder="t('admin.paymentChannels.modal.paypalClientIdPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.paypalClientSecret') }}</label>
              <Input v-model="paypalConfig.client_secret" :placeholder="t('admin.paymentChannels.modal.paypalClientSecretPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.paypalBaseUrl') }}</label>
              <Input v-model="paypalConfig.base_url" :placeholder="t('admin.paymentChannels.modal.paypalBaseUrlPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.returnUrl') }}</label>
              <Input v-model="paypalConfig.return_url" :placeholder="t('admin.paymentChannels.modal.returnUrlPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.paypalCancelUrl') }}</label>
              <Input v-model="paypalConfig.cancel_url" :placeholder="t('admin.paymentChannels.modal.paypalCancelUrlPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.paypalWebhookId') }}</label>
              <Input v-model="paypalConfig.webhook_id" :placeholder="t('admin.paymentChannels.modal.paypalWebhookIdPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.paypalBrandName') }}</label>
              <Input v-model="paypalConfig.brand_name" :placeholder="t('admin.paymentChannels.modal.paypalBrandNamePlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.paypalLocale') }}</label>
              <Input v-model="paypalConfig.locale" :placeholder="t('admin.paymentChannels.modal.paypalLocalePlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.paypalTargetCurrency') }}</label>
              <Input v-model="paypalConfig.target_currency" :placeholder="t('admin.paymentChannels.modal.paypalTargetCurrencyPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.paypalExchangeRate') }}</label>
              <Input v-model="paypalConfig.exchange_rate" :placeholder="t('admin.paymentChannels.modal.paypalExchangeRatePlaceholder')" />
            </div>
          </div>
          <div class="mt-3 text-xs text-muted-foreground">{{ t('admin.paymentChannels.modal.paypalHint') }}</div>
        </div>

        <div v-if="form.provider_type === 'official' && form.channel_type === 'stripe'" class="min-w-0 rounded-xl border border-border bg-muted/20 p-4 overflow-hidden">
          <div class="text-sm font-semibold text-foreground mb-3">{{ t('admin.paymentChannels.modal.stripeSection') }}</div>
          <div class="grid grid-cols-1 gap-4 md:grid-cols-2 [&>*]:min-w-0">
            <div class="min-w-0 md:col-span-2">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.stripeSecretKey') }}</label>
              <Input v-model="stripeConfig.secret_key" :placeholder="t('admin.paymentChannels.modal.stripeSecretKeyPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.stripePublishableKey') }}</label>
              <Input v-model="stripeConfig.publishable_key" :placeholder="t('admin.paymentChannels.modal.stripePublishableKeyPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.stripeWebhookSecret') }}</label>
              <Input v-model="stripeConfig.webhook_secret" :placeholder="t('admin.paymentChannels.modal.stripeWebhookSecretPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.stripeSuccessUrl') }}</label>
              <Input v-model="stripeConfig.success_url" :placeholder="t('admin.paymentChannels.modal.stripeSuccessUrlPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.stripeCancelUrl') }}</label>
              <Input v-model="stripeConfig.cancel_url" :placeholder="t('admin.paymentChannels.modal.stripeCancelUrlPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.stripeApiBaseUrl') }}</label>
              <Input v-model="stripeConfig.api_base_url" :placeholder="t('admin.paymentChannels.modal.stripeApiBaseUrlPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.stripePaymentMethodTypes') }}</label>
              <Input v-model="stripeConfig.payment_method_types" :placeholder="t('admin.paymentChannels.modal.stripePaymentMethodTypesPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.targetCurrency') }}</label>
              <Input v-model="stripeConfig.target_currency" :placeholder="t('admin.paymentChannels.modal.targetCurrencyPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.exchangeRate') }}</label>
              <Input v-model="stripeConfig.exchange_rate" :placeholder="t('admin.paymentChannels.modal.exchangeRatePlaceholder')" />
            </div>
          </div>
          <div class="mt-3 text-xs text-muted-foreground">{{ t('admin.paymentChannels.modal.stripeHint') }}</div>
        </div>

        <div v-if="form.provider_type === 'official' && form.channel_type === 'wechat'" class="min-w-0 rounded-xl border border-border bg-muted/20 p-4 overflow-hidden">
          <div class="text-sm font-semibold text-foreground mb-3">{{ t('admin.paymentChannels.modal.wechatSection') }}</div>
          <div class="grid grid-cols-1 gap-4 md:grid-cols-2 [&>*]:min-w-0">
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.wechatAppId') }}</label>
              <Input v-model="wechatConfig.appid" :placeholder="t('admin.paymentChannels.modal.wechatAppIdPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.wechatMerchantId') }}</label>
              <Input v-model="wechatConfig.mchid" :placeholder="t('admin.paymentChannels.modal.wechatMerchantIdPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.wechatMerchantSerialNo') }}</label>
              <Input v-model="wechatConfig.merchant_serial_no" :placeholder="t('admin.paymentChannels.modal.wechatMerchantSerialNoPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.wechatApiV3Key') }}</label>
              <Input v-model="wechatConfig.api_v3_key" :placeholder="t('admin.paymentChannels.modal.wechatApiV3KeyPlaceholder')" />
            </div>
            <div class="min-w-0 md:col-span-2">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.wechatVerificationMode') }}</label>
              <Select v-model="wechatConfig.verification_mode">
                <SelectTrigger class="h-9 w-full">
                  <SelectValue :placeholder="t('admin.paymentChannels.modal.wechatVerificationModePlatformCertificate')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="platform_certificate">{{ t('admin.paymentChannels.modal.wechatVerificationModePlatformCertificate') }}</SelectItem>
                  <SelectItem value="wechatpay_public_key">{{ t('admin.paymentChannels.modal.wechatVerificationModePublicKey') }}</SelectItem>
                  <SelectItem value="combined">{{ t('admin.paymentChannels.modal.wechatVerificationModeCombined') }}</SelectItem>
                </SelectContent>
              </Select>
              <p class="text-xs text-muted-foreground mt-1">{{ t('admin.paymentChannels.modal.wechatVerificationModeHint') }}</p>
            </div>
            <div v-if="wechatConfig.verification_mode !== 'platform_certificate'" class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.wechatPublicKeyId') }}</label>
              <Input v-model="wechatConfig.wechatpay_public_key_id" :placeholder="t('admin.paymentChannels.modal.wechatPublicKeyIdPlaceholder')" />
            </div>
            <div v-if="wechatConfig.verification_mode !== 'platform_certificate'" class="min-w-0 md:col-span-2">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.wechatPublicKey') }}</label>
              <Textarea v-model="wechatConfig.wechatpay_public_key" rows="4" :placeholder="t('admin.paymentChannels.modal.wechatPublicKeyPlaceholder')" />
            </div>
            <div class="min-w-0 md:col-span-2">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.wechatMerchantPrivateKey') }}</label>
              <Textarea v-model="wechatConfig.merchant_private_key" rows="4" :placeholder="t('admin.paymentChannels.modal.wechatMerchantPrivateKeyPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.wechatNotifyUrl') }}</label>
              <Input v-model="wechatConfig.notify_url" :placeholder="t('admin.paymentChannels.modal.wechatNotifyUrlPlaceholder')" />
              <p class="text-xs text-muted-foreground mt-1">{{ t('admin.paymentChannels.modal.wechatNotifyUrlHint') }}</p>
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.wechatH5RedirectUrl') }}</label>
              <Input v-model="wechatConfig.h5_redirect_url" :placeholder="t('admin.paymentChannels.modal.wechatH5RedirectUrlPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.wechatH5Type') }}</label>
              <Input v-model="wechatConfig.h5_type" :placeholder="t('admin.paymentChannels.modal.wechatH5TypePlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.wechatH5WapUrl') }}</label>
              <Input v-model="wechatConfig.h5_wap_url" :placeholder="t('admin.paymentChannels.modal.wechatH5WapUrlPlaceholder')" />
            </div>
            <div class="min-w-0 md:col-span-2">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.wechatH5WapName') }}</label>
              <Input v-model="wechatConfig.h5_wap_name" :placeholder="t('admin.paymentChannels.modal.wechatH5WapNamePlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.targetCurrency') }}</label>
              <Input v-model="wechatConfig.target_currency" :placeholder="t('admin.paymentChannels.modal.targetCurrencyPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.exchangeRate') }}</label>
              <Input v-model="wechatConfig.exchange_rate" :placeholder="t('admin.paymentChannels.modal.exchangeRatePlaceholder')" />
            </div>
          </div>
          <div class="mt-4 rounded-lg border border-border bg-background/70 p-3">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <div class="text-sm font-medium text-foreground">{{ t('admin.paymentChannels.modal.wechatPublicKeyTestTitle') }}</div>
                <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.paymentChannels.modal.wechatPublicKeyTestHint') }}</p>
              </div>
              <Button
                type="button"
                variant="outline"
                class="w-full shrink-0 sm:w-auto"
                :disabled="!isEditing || wechatConfig.verification_mode === 'platform_certificate' || testingWechatPublicKey"
                @click="handleWechatPublicKeyTest"
              >
                {{ testingWechatPublicKey ? t('admin.paymentChannels.modal.wechatPublicKeyTesting') : t('admin.paymentChannels.modal.wechatPublicKeyTest') }}
              </Button>
            </div>
            <p v-if="!isEditing" class="mt-2 text-xs text-amber-600 dark:text-amber-400">
              {{ t('admin.paymentChannels.modal.wechatPublicKeyTestSaveFirst') }}
            </p>
            <p v-else-if="wechatConfig.verification_mode === 'platform_certificate'" class="mt-2 text-xs text-amber-600 dark:text-amber-400">
              {{ t('admin.paymentChannels.modal.wechatPublicKeyTestModeRequired') }}
            </p>
            <div v-if="wechatPublicKeyTestResult" class="mt-3 rounded-md border border-emerald-500/30 bg-emerald-500/10 p-3 text-xs text-emerald-700 dark:text-emerald-300">
              <div class="font-medium">{{ t('admin.paymentChannels.modal.wechatPublicKeyTestSuccess') }}</div>
              <div class="mt-2 grid gap-1 sm:grid-cols-2">
                <span>{{ t('admin.paymentChannels.modal.wechatPublicKeyTestSerial') }}</span>
                <span class="break-all font-mono sm:text-right">{{ wechatPublicKeyTestResult.response_serial }}</span>
                <span>{{ t('admin.paymentChannels.modal.wechatPublicKeyTestRequestSignature') }}</span>
                <span class="sm:text-right">{{ wechatPublicKeyTestResult.request_signature_accepted ? t('admin.paymentChannels.modal.wechatPublicKeyTestPassed') : t('admin.paymentChannels.modal.wechatPublicKeyTestFailed') }}</span>
                <span>{{ t('admin.paymentChannels.modal.wechatPublicKeyTestResponseSignature') }}</span>
                <span class="sm:text-right">{{ wechatPublicKeyTestResult.response_signature_valid ? t('admin.paymentChannels.modal.wechatPublicKeyTestPassed') : t('admin.paymentChannels.modal.wechatPublicKeyTestFailed') }}</span>
                <span>{{ t('admin.paymentChannels.modal.wechatPublicKeyTestEcho') }}</span>
                <span class="sm:text-right">{{ wechatPublicKeyTestResult.echo_message_matched ? t('admin.paymentChannels.modal.wechatPublicKeyTestPassed') : t('admin.paymentChannels.modal.wechatPublicKeyTestFailed') }}</span>
              </div>
            </div>
          </div>
          <div class="mt-3 text-xs text-muted-foreground">{{ t('admin.paymentChannels.modal.wechatHint') }}</div>
        </div>

        <div v-if="form.provider_type === 'official' && form.channel_type === 'alipay'" class="min-w-0 rounded-xl border border-border bg-muted/20 p-4 overflow-hidden">
          <div class="text-sm font-semibold text-foreground mb-3">{{ t('admin.paymentChannels.modal.alipaySection') }}</div>
          <div class="grid grid-cols-1 gap-4 md:grid-cols-2 [&>*]:min-w-0">
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.alipayAppId') }}</label>
              <Input v-model="alipayConfig.app_id" :placeholder="t('admin.paymentChannels.modal.alipayAppIdPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.alipaySignType') }}</label>
              <Input v-model="alipayConfig.sign_type" :placeholder="t('admin.paymentChannels.modal.alipaySignTypePlaceholder')" />
            </div>
            <div class="min-w-0 md:col-span-2">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.alipayPrivateKey') }}</label>
              <Textarea v-model="alipayConfig.private_key" rows="4" :placeholder="t('admin.paymentChannels.modal.alipayPrivateKeyPlaceholder')" />
            </div>
            <div class="min-w-0 md:col-span-2">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.alipayPublicKey') }}</label>
              <Textarea v-model="alipayConfig.alipay_public_key" rows="4" :placeholder="t('admin.paymentChannels.modal.alipayPublicKeyPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.alipayGatewayUrl') }}</label>
              <Input v-model="alipayConfig.gateway_url" :placeholder="t('admin.paymentChannels.modal.alipayGatewayUrlPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.alipayNotifyUrl') }}</label>
              <Input v-model="alipayConfig.notify_url" :placeholder="t('admin.paymentChannels.modal.alipayNotifyUrlPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.alipayReturnUrl') }}</label>
              <Input v-model="alipayConfig.return_url" :placeholder="t('admin.paymentChannels.modal.alipayReturnUrlPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.alipayAppCertSn') }}</label>
              <Input v-model="alipayConfig.app_cert_sn" :placeholder="t('admin.paymentChannels.modal.alipayAppCertSnPlaceholder')" />
            </div>
            <div class="min-w-0 md:col-span-2">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.alipayRootCertSn') }}</label>
              <Input v-model="alipayConfig.alipay_root_cert_sn" :placeholder="t('admin.paymentChannels.modal.alipayRootCertSnPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.targetCurrency') }}</label>
              <Input v-model="alipayConfig.target_currency" :placeholder="t('admin.paymentChannels.modal.targetCurrencyPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.exchangeRate') }}</label>
              <Input v-model="alipayConfig.exchange_rate" :placeholder="t('admin.paymentChannels.modal.exchangeRatePlaceholder')" />
            </div>
          </div>
          <div class="mt-3 text-xs text-muted-foreground">{{ t('admin.paymentChannels.modal.alipayHint') }}</div>
        </div>

        <div v-if="form.provider_type === 'bepusdt'" class="min-w-0 rounded-xl border border-border bg-muted/20 p-4 overflow-hidden">
          <div class="text-sm font-semibold text-foreground mb-3">{{ t('admin.paymentChannels.modal.bepusdtSection') }}</div>
          <div class="grid grid-cols-1 gap-4 md:grid-cols-2 [&>*]:min-w-0">
            <div class="min-w-0 md:col-span-2">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.bepusdtGatewayUrl') }}</label>
              <Input v-model="bepusdtConfig.gateway_url" :placeholder="t('admin.paymentChannels.modal.bepusdtGatewayUrlPlaceholder')" />
            </div>
            <div class="min-w-0 md:col-span-2">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.bepusdtAuthToken') }}</label>
              <Input v-model="bepusdtConfig.auth_token" :placeholder="t('admin.paymentChannels.modal.bepusdtAuthTokenPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.bepusdtOrderMode') }}</label>
              <Select v-model="bepusdtConfig.order_mode">
                <SelectTrigger class="h-9 w-full">
                  <SelectValue :placeholder="t('admin.paymentChannels.modal.bepusdtOrderModeTransaction')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="transaction">{{ t('admin.paymentChannels.modal.bepusdtOrderModeTransaction') }}</SelectItem>
                  <SelectItem value="cashier">{{ t('admin.paymentChannels.modal.bepusdtOrderModeCashier') }}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div v-if="bepusdtConfig.order_mode !== 'cashier'" class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.bepusdtTradeType') }}</label>
              <Input v-model="bepusdtConfig.trade_type" :placeholder="t('admin.paymentChannels.modal.bepusdtTradeTypePlaceholder')" />
            </div>
            <div v-if="bepusdtConfig.order_mode === 'cashier'" class="min-w-0 md:col-span-2">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.bepusdtCurrencies') }}</label>
              <Input v-model="bepusdtConfig.currencies" :placeholder="t('admin.paymentChannels.modal.bepusdtCurrenciesPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.bepusdtFiat') }}</label>
              <Input v-model="bepusdtConfig.fiat" :placeholder="t('admin.paymentChannels.modal.bepusdtFiatPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.bepusdtNotifyUrl') }}</label>
              <Input v-model="bepusdtConfig.notify_url" :placeholder="t('admin.paymentChannels.modal.bepusdtNotifyUrlPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.bepusdtReturnUrl') }}</label>
              <Input v-model="bepusdtConfig.return_url" :placeholder="t('admin.paymentChannels.modal.bepusdtReturnUrlPlaceholder')" />
            </div>
          </div>
          <div class="mt-3 text-xs text-muted-foreground">{{ t('admin.paymentChannels.modal.bepusdtHint') }}</div>
        </div>

        <div v-if="form.provider_type === 'epusdt'" class="min-w-0 rounded-xl border border-border bg-muted/20 p-4 overflow-hidden">
          <div class="text-sm font-semibold text-foreground mb-3">{{ t('admin.paymentChannels.modal.epusdtSection') }}</div>
          <div class="grid grid-cols-1 gap-4 md:grid-cols-2 [&>*]:min-w-0">
            <div class="min-w-0 md:col-span-2">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.epusdtGatewayUrl') }}</label>
              <Input v-model="epusdtConfig.gateway_url" :placeholder="t('admin.paymentChannels.modal.epusdtGatewayUrlPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.epusdtPid') }}</label>
              <Input v-model="epusdtConfig.pid" :placeholder="t('admin.paymentChannels.modal.epusdtPidPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.epusdtSecretKey') }}</label>
              <Input v-model="epusdtConfig.secret_key" :placeholder="t('admin.paymentChannels.modal.epusdtSecretKeyPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.epusdtOrderMode') }}</label>
              <Select v-model="epusdtConfig.order_mode">
                <SelectTrigger class="h-9 w-full">
                  <SelectValue :placeholder="t('admin.paymentChannels.modal.epusdtOrderModeTransaction')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="transaction">{{ t('admin.paymentChannels.modal.epusdtOrderModeTransaction') }}</SelectItem>
                  <SelectItem value="cashier">{{ t('admin.paymentChannels.modal.epusdtOrderModeCashier') }}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div v-if="epusdtConfig.order_mode !== 'cashier'" class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.epusdtToken') }}</label>
              <Input v-model="epusdtConfig.token" required :placeholder="t('admin.paymentChannels.modal.epusdtTokenPlaceholder')" />
            </div>
            <div v-if="epusdtConfig.order_mode !== 'cashier'" class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.epusdtNetwork') }}</label>
              <Input v-model="epusdtConfig.network" required :placeholder="t('admin.paymentChannels.modal.epusdtNetworkPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.epusdtCurrency') }}</label>
              <Input v-model="epusdtConfig.currency" :placeholder="t('admin.paymentChannels.modal.epusdtCurrencyPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.epusdtNotifyUrl') }}</label>
              <Input v-model="epusdtConfig.notify_url" :placeholder="t('admin.paymentChannels.modal.epusdtNotifyUrlPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.epusdtReturnUrl') }}</label>
              <Input v-model="epusdtConfig.return_url" :placeholder="t('admin.paymentChannels.modal.epusdtReturnUrlPlaceholder')" />
            </div>
          </div>
          <div class="mt-3 text-xs text-muted-foreground">{{ t('admin.paymentChannels.modal.epusdtHint') }}</div>
        </div>

        <div v-if="form.provider_type === 'tokenpay'" class="min-w-0 rounded-xl border border-border bg-muted/20 p-4 overflow-hidden">
          <div class="text-sm font-semibold text-foreground mb-3">{{ t('admin.paymentChannels.modal.tokenpaySection') }}</div>
          <div class="grid grid-cols-1 gap-4 md:grid-cols-2 [&>*]:min-w-0">
            <div class="min-w-0 md:col-span-2">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.tokenpayGatewayUrl') }}</label>
              <Input v-model="tokenpayConfig.gateway_url" :placeholder="t('admin.paymentChannels.modal.tokenpayGatewayUrlPlaceholder')" />
            </div>
            <div class="min-w-0 md:col-span-2">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.tokenpayNotifySecret') }}</label>
              <Input v-model="tokenpayConfig.notify_secret" :placeholder="t('admin.paymentChannels.modal.tokenpayNotifySecretPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.tokenpayCurrency') }}</label>
              <Input v-model="tokenpayConfig.currency" :placeholder="t('admin.paymentChannels.modal.tokenpayCurrencyPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.tokenpayBaseCurrency') }}</label>
              <Input v-model="tokenpayConfig.base_currency" :placeholder="t('admin.paymentChannels.modal.tokenpayBaseCurrencyPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.tokenpayNotifyUrl') }}</label>
              <Input v-model="tokenpayConfig.notify_url" :placeholder="t('admin.paymentChannels.modal.tokenpayNotifyUrlPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.tokenpayRedirectUrl') }}</label>
              <Input v-model="tokenpayConfig.redirect_url" :placeholder="t('admin.paymentChannels.modal.tokenpayRedirectUrlPlaceholder')" />
            </div>
          </div>
          <div class="mt-3 text-xs text-muted-foreground">{{ t('admin.paymentChannels.modal.tokenpayHint') }}</div>
        </div>

        <div v-if="form.provider_type === 'okpay'" class="min-w-0 rounded-xl border border-border bg-muted/20 p-4 overflow-hidden">
          <div class="text-sm font-semibold text-foreground mb-3">{{ t('admin.paymentChannels.modal.okpaySection') }}</div>
          <div class="grid grid-cols-1 gap-4 md:grid-cols-2 [&>*]:min-w-0">
            <div class="min-w-0 md:col-span-2">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.okpayGatewayUrl') }}</label>
              <Input v-model="okpayConfig.gateway_url" :placeholder="t('admin.paymentChannels.modal.okpayGatewayUrlPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.okpayMerchantId') }}</label>
              <Input v-model="okpayConfig.merchant_id" :placeholder="t('admin.paymentChannels.modal.okpayMerchantIdPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.okpayMerchantToken') }}</label>
              <Input v-model="okpayConfig.merchant_token" :placeholder="t('admin.paymentChannels.modal.okpayMerchantTokenPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.okpayExchangeRate') }}</label>
              <Input v-model="okpayConfig.exchange_rate" type="number" step="0.00000001" min="0.00000001" :placeholder="t('admin.paymentChannels.modal.okpayExchangeRatePlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.okpayCallbackUrl') }}</label>
              <Input v-model="okpayConfig.callback_url" :placeholder="t('admin.paymentChannels.modal.okpayCallbackUrlPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.okpayReturnUrl') }}</label>
              <Input v-model="okpayConfig.return_url" :placeholder="t('admin.paymentChannels.modal.okpayReturnUrlPlaceholder')" />
            </div>
            <div class="min-w-0 md:col-span-2">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.okpayDisplayName') }}</label>
              <Input v-model="okpayConfig.display_name" :placeholder="t('admin.paymentChannels.modal.okpayDisplayNamePlaceholder')" />
            </div>
          </div>
          <div class="mt-3 text-xs text-muted-foreground">{{ t('admin.paymentChannels.modal.okpayHint') }}</div>
        </div>

        <div v-if="form.provider_type === 'dujiaopay'" class="min-w-0 rounded-xl border border-border bg-muted/20 p-4 overflow-hidden">
          <div class="text-sm font-semibold text-foreground mb-3">{{ t('admin.paymentChannels.modal.dujiaopaySection') }}</div>
          <div class="grid grid-cols-1 gap-4 md:grid-cols-2 [&>*]:min-w-0">
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.dujiaopayOrderMode') }}</label>
              <Select v-model="dujiaopayConfig.order_mode">
                <SelectTrigger class="h-9 w-full">
                  <SelectValue :placeholder="t('admin.paymentChannels.modal.dujiaopayOrderModeTransaction')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="transaction">{{ t('admin.paymentChannels.modal.dujiaopayOrderModeTransaction') }}</SelectItem>
                  <SelectItem value="cashier">{{ t('admin.paymentChannels.modal.dujiaopayOrderModeCashier') }}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div v-if="dujiaopayConfig.order_mode !== 'cashier'" class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.dujiaopayFixedMethod') }}</label>
              <Input v-model="form.channel_type" :placeholder="t('admin.paymentChannels.modal.dujiaopayFixedMethodPlaceholder')" />
            </div>
            <div v-if="dujiaopayConfig.order_mode === 'cashier'" class="min-w-0 md:col-span-2">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.dujiaopayAllowedMethods') }}</label>
              <Input v-model="dujiaopayConfig.allowed_methods" :placeholder="t('admin.paymentChannels.modal.dujiaopayAllowedMethodsPlaceholder')" />
            </div>
            <div class="min-w-0 md:col-span-2">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.dujiaopayApiBaseUrl') }}</label>
              <Input v-model="dujiaopayConfig.api_base_url" :placeholder="t('admin.paymentChannels.modal.dujiaopayApiBaseUrlPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.dujiaopayApiKeyId') }}</label>
              <Input v-model="dujiaopayConfig.api_key_id" :placeholder="t('admin.paymentChannels.modal.dujiaopayApiKeyIdPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.dujiaopayFiatCurrency') }}</label>
              <Input v-model="dujiaopayConfig.fiat_currency" :placeholder="t('admin.paymentChannels.modal.dujiaopayFiatCurrencyPlaceholder')" />
            </div>
            <div class="min-w-0 md:col-span-2">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.dujiaopayApiSecret') }}</label>
              <Textarea v-model="dujiaopayConfig.api_secret" rows="3" :placeholder="t('admin.paymentChannels.modal.dujiaopayApiSecretPlaceholder')" />
            </div>
            <div class="min-w-0 md:col-span-2">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.dujiaopayWebhookSecret') }}</label>
              <Textarea v-model="dujiaopayConfig.webhook_secret" rows="3" :placeholder="t('admin.paymentChannels.modal.dujiaopayWebhookSecretPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.dujiaopaySuccessUrl') }}</label>
              <Input v-model="dujiaopayConfig.success_url" :placeholder="t('admin.paymentChannels.modal.dujiaopaySuccessUrlPlaceholder')" />
            </div>
            <div class="min-w-0">
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">{{ t('admin.paymentChannels.modal.dujiaopayCancelUrl') }}</label>
              <Input v-model="dujiaopayConfig.cancel_url" :placeholder="t('admin.paymentChannels.modal.dujiaopayCancelUrlPlaceholder')" />
            </div>
          </div>
          <div class="mt-3 text-xs text-muted-foreground">{{ t('admin.paymentChannels.modal.dujiaopayHint') }}</div>
        </div>

        <div>
          <div class="mb-1.5 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <label class="block text-xs font-medium text-muted-foreground">{{ t('admin.paymentChannels.modal.configJson') }}</label>
            <button type="button" class="text-xs text-muted-foreground hover:text-foreground" @click="showAdvanced = !showAdvanced">
              {{ showAdvanced ? t('admin.paymentChannels.modal.advancedHide') : t('admin.paymentChannels.modal.advancedShow') }}
            </button>
          </div>
          <template v-if="showAdvanced">
            <Textarea v-model="form.config_json" rows="8" class="font-mono text-xs" :placeholder="configJsonPlaceholder" />
            <p class="mt-1.5 text-xs text-muted-foreground">{{ t('admin.paymentChannels.modal.advancedClearHint') }}</p>
          </template>
        </div>

        <div v-if="error" class="rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive">
          {{ error }}
        </div>

        <div class="flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
          <Button class="w-full sm:w-auto" type="button" variant="outline" @click="closeModal">{{ t('admin.common.cancel') }}</Button>
          <Button class="w-full sm:w-auto" type="submit">{{ t('admin.common.save') }}</Button>
        </div>
      </form>
    </DialogScrollContent>
  </Dialog>
</template>
