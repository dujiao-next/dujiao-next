<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import RichEditor from '@/components/RichEditor.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { notifyError, notifySuccess } from '@/utils/notify'
import { applySiteIcon } from '@/utils/favicon'
import { getImageUrl } from '@/utils/image'
import { orderEmailSceneKeys } from '@/utils/orderEmailTemplates'
import MediaPicker from '@/components/admin/MediaPicker.vue'
import SettingsSMTPTab from './components/SettingsSMTPTab.vue'
import SettingsCaptchaTab from './components/SettingsCaptchaTab.vue'
import SettingsOrderEmailTemplateTab from './components/SettingsOrderEmailTemplateTab.vue'
import SettingsNavigationTab from './components/SettingsNavigationTab.vue'
import SettingsHomeAnnouncementTab from './components/SettingsHomeAnnouncementTab.vue'
import SettingsUpstreamSyncTab from './components/SettingsUpstreamSyncTab.vue'

const { t } = useI18n()
const loading = ref(false)
const smtpTabRef = ref<InstanceType<typeof SettingsSMTPTab>>()
const captchaTabRef = ref<InstanceType<typeof SettingsCaptchaTab>>()
const orderEmailTemplateTabRef = ref<InstanceType<typeof SettingsOrderEmailTemplateTab>>()
const navigationTabRef = ref<InstanceType<typeof SettingsNavigationTab>>()
const homeAnnouncementTabRef = ref<InstanceType<typeof SettingsHomeAnnouncementTab>>()
const upstreamSyncTabRef = ref<InstanceType<typeof SettingsUpstreamSyncTab>>()
const siteIconPickerRef = ref<InstanceType<typeof MediaPicker> | null>(null)
const siteLogoPickerRef = ref<InstanceType<typeof MediaPicker> | null>(null)
const supportedLanguages = ['zh-CN', 'zh-TW', 'en-US'] as const
type SupportedLanguage = (typeof supportedLanguages)[number]
type SiteScriptPosition = 'head' | 'body_end'
type SiteScriptItem = {
  name: string
  enabled: boolean
  position: SiteScriptPosition
  code: string
}

const siteScriptsMaxCount = 20
const footerLinksMaxCount = 20

const registrationForm = reactive({
  registration_enabled: true,
  email_verification_enabled: true,
  email_domain_allowlist_enabled: false,
  allowed_email_domains_text: '',
})
const orderPaymentExpireMinutes = ref(15)
type FooterLinkItem = {
  name: string
  url: string
}
const createFooterLinkItem = (): FooterLinkItem => ({
  name: '',
  url: '',
})
const currentLang = ref<SupportedLanguage>('zh-CN')
const currentTab = ref('basic')

const languages = computed(() => [
  { code: 'zh-CN' as SupportedLanguage, name: t('admin.common.lang.zhCN') },
  { code: 'zh-TW' as SupportedLanguage, name: t('admin.common.lang.zhTW') },
  { code: 'en-US' as SupportedLanguage, name: t('admin.common.lang.enUS') },
])

const tabs = computed(() => [
  { label: t('admin.settings.tabs.basic'), value: 'basic' },
  { label: t('admin.settings.tabs.template'), value: 'template' },
  { label: t('admin.settings.tabs.navigation'), value: 'navigation' },
  { label: t('admin.settings.tabs.about'), value: 'about' },
  { label: t('admin.settings.tabs.legal'), value: 'legal' },
  { label: t('admin.settings.tabs.homeAnnouncement'), value: 'home_announcement' },
  { label: t('admin.settings.tabs.smtp'), value: 'smtp' },
  { label: t('admin.settings.tabs.orderEmailTemplate'), value: 'order_email_template' },
  { label: t('admin.settings.tabs.captcha'), value: 'captcha' },
  { label: t('admin.settings.tabs.telegram'), value: 'telegram' },
  { label: t('admin.settings.tabs.google'), value: 'google' },
  { label: t('admin.settings.tabs.github'), value: 'github' },
  { label: t('admin.settings.tabs.dashboard'), value: 'dashboard' },
  { label: t('admin.settings.tabs.upstreamSync'), value: 'upstream_sync' },
])

const fallbackCurrencyOptions = [
  'CNY', 'USD', 'EUR', 'GBP', 'JPY', 'KRW', 'HKD', 'TWD', 'SGD', 'AUD',
  'CAD', 'CHF', 'NZD', 'SEK', 'NOK', 'DKK', 'AED', 'SAR', 'MYR', 'THB',
  'PHP', 'IDR', 'VND', 'INR', 'RUB', 'TRY', 'ZAR', 'BRL', 'MXN', 'ARS',
]

const currencyOptions = computed(() => {
  const values: string[] = []
  if (typeof Intl !== 'undefined' && typeof (Intl as Record<string, unknown>).supportedValuesOf === 'function') {
    const candidate = (Intl as unknown as Record<string, unknown> & { supportedValuesOf: (key: string) => unknown }).supportedValuesOf('currency')
    if (Array.isArray(candidate)) {
      values.push(...candidate.map((item: unknown) => String(item || '').trim().toUpperCase()))
    }
  }
  values.push(...fallbackCurrencyOptions)
  const unique = Array.from(new Set(values.filter((item) => /^[A-Z]{3}$/.test(item))))
  const filtered = unique.filter((item) => item !== 'CNY').sort()
  return ['CNY', ...filtered]
})

const createLocalizedField = () => ({ 'zh-CN': '', 'zh-TW': '', 'en-US': '' } as Record<SupportedLanguage, string>)
const createSiteScriptItem = (): SiteScriptItem => ({
  name: '',
  enabled: true,
  position: 'head',
  code: '',
})

const normalizeSiteScriptPosition = (raw: unknown): SiteScriptPosition => {
  return raw === 'body_end' ? 'body_end' : 'head'
}

const normalizeSiteScriptEnabled = (raw: unknown): boolean => {
  if (typeof raw === 'boolean') return raw
  if (typeof raw === 'number') return raw !== 0
  if (typeof raw === 'string') {
    const value = raw.trim().toLowerCase()
    return value === '1' || value === 'true' || value === 'yes' || value === 'on'
  }
  return false
}

const normalizeSiteScripts = (raw: unknown): SiteScriptItem[] => {
  if (!Array.isArray(raw)) return []
  return raw
    .map((item) => {
      if (!item || typeof item !== 'object') return null
      const value = item as Record<string, unknown>
      return {
        name: typeof value.name === 'string' ? value.name : '',
        enabled: normalizeSiteScriptEnabled(value.enabled),
        position: normalizeSiteScriptPosition(value.position),
        code: typeof value.code === 'string' ? value.code : '',
      } as SiteScriptItem
    })
    .filter((item): item is SiteScriptItem => !!item)
    .slice(0, siteScriptsMaxCount)
}

const normalizeFooterLinks = (raw: unknown): FooterLinkItem[] => {
  if (!Array.isArray(raw)) return []
  return raw
    .map((item) => {
      if (!item || typeof item !== 'object') return null
      const value = item as Record<string, unknown>
      return {
        name: typeof value.name === 'string' ? value.name : '',
        url: typeof value.url === 'string' ? value.url : '',
      } as FooterLinkItem
    })
    .filter((item): item is FooterLinkItem => !!item && item.name.trim() !== '')
    .slice(0, footerLinksMaxCount)
}

const normalizeLocalizedField = (raw: unknown): Record<SupportedLanguage, string> => {
  const normalized = createLocalizedField()
  if (!raw || typeof raw !== 'object') {
    return normalized
  }
  const record = raw as Record<string, unknown>
  supportedLanguages.forEach((lang) => {
    const value = record[lang]
    normalized[lang] = typeof value === 'string' ? value : ''
  })
  return normalized
}

const isLocalizedFieldNotEmpty = (value: Record<SupportedLanguage, string>) => {
  return Object.values(value).some((item) => item.trim() !== '')
}

const form = reactive({
  brand: {
    site_name: '',
    site_url: '',
    // site_icon 是浏览器 favicon；site_logo 是前台页面 Logo，二者必须独立保存和清除。
    site_icon: '',
    site_logo: '',
    site_description: createLocalizedField(),
  },
  currency: 'CNY',
  order_max_refund_days: 30,
  contact: {
    telegram: '',
    whatsapp: '',
  },
  seo: {
    title: createLocalizedField(),
    keywords: createLocalizedField(),
    description: createLocalizedField(),
  },
  about: {
    hero: {
      title: createLocalizedField(),
      subtitle: createLocalizedField(),
    },
    introduction: createLocalizedField(),
    services: {
      title: createLocalizedField(),
      items: [] as Array<Record<SupportedLanguage, string>>,
    },
    contact: {
      title: createLocalizedField(),
      text: createLocalizedField(),
    },
  },
  legal: {
    terms: createLocalizedField(),
    privacy: createLocalizedField(),
  },
  scripts: [] as SiteScriptItem[],
  footer_links: [] as FooterLinkItem[],
  storefront_template: 'classic' as 'classic' | 'vault',
  template_mode: 'card' as 'card' | 'list',
})

const smtpData = reactive({
  enabled: false,
  host: '',
  port: 587,
  username: '',
  password: '',
  has_password: false,
  from: '',
  from_name: '',
  use_tls: true,
  use_ssl: false,
  order_notification_enabled: true,
  verify_code: {
    expire_minutes: 10,
    send_interval_seconds: 60,
    max_attempts: 5,
    length: 6,
  },
})

const captchaData = reactive({
  provider: 'none',
  scenes: {
    login: false,
    register_send_code: false,
    reset_send_code: false,
    guest_create_order: false,
    gift_card_redeem: false,
  },
  image: {
    length: 5,
    width: 240,
    height: 80,
    noise_count: 2,
    show_line: 2,
    expire_seconds: 300,
    max_store: 10240,
  },
  turnstile: {
    site_key: '',
    secret_key: '',
    has_secret: false,
    verify_url: 'https://challenges.cloudflare.com/turnstile/v0/siteverify',
    timeout_ms: 2000,
  },
})

const telegramForm = reactive({
  enabled: false,
  bot_username: '',
  bot_token: '',
  has_bot_token: false,
  mini_app_url: '',
  login_expire_seconds: 300,
  replay_ttl_seconds: 300,
  client_secret: '',
  has_client_secret: false,
  oidc_redirect_uri: '',
  mode: '' as string,
})

const googleForm = reactive({
  enabled: false,
  client_id: '',
})

const githubForm = reactive({
  enabled: false,
  client_id: '',
  client_secret: '',
  client_secret_configured: false,
})

const createOrderEmailLocalizedTemplate = () => ({ subject: '', body: '' })
const createOrderEmailSceneTemplate = () => ({
  'zh-CN': createOrderEmailLocalizedTemplate(),
  'zh-TW': createOrderEmailLocalizedTemplate(),
  'en-US': createOrderEmailLocalizedTemplate(),
})
const orderEmailTemplateData = reactive({
  templates: {
    default: createOrderEmailSceneTemplate(),
    paid: createOrderEmailSceneTemplate(),
    delivered: createOrderEmailSceneTemplate(),
    delivered_with_content: createOrderEmailSceneTemplate(),
    refunded: createOrderEmailSceneTemplate(),
    partially_refunded: createOrderEmailSceneTemplate(),
  },
  guest_tip: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' } as Record<typeof supportedLanguages[number], string>,
})

const dashboardForm = reactive({
  accounting: {
    refund_reverses_cost: false,
  },
  alert: {
    low_stock_threshold: 5,
    out_of_stock_products_threshold: 1,
    pending_payment_orders_threshold: 20,
    payments_failed_threshold: 10,
  },
  ranking: {
    top_products_limit: 5,
    top_channels_limit: 5,
  },
})

const getCurrentLangName = () => {
  return languages.value.find((item) => item.code === currentLang.value)?.name || t('admin.common.lang.zhCN')
}

const normalizeNumber = (value: unknown, fallback: number) => {
  if (value === null || value === undefined || value === '') return fallback
  const parsed = Number(value)
  if (Number.isNaN(parsed)) return fallback
  return parsed
}

const splitAllowedEmailDomains = (raw: string): string[] => {
  const seen = new Set<string>()
  const result: string[] = []
  raw
    .split(/[\s,，;；]+/)
    .map((item) => item.trim().replace(/^@+/, '').toLowerCase())
    .filter(Boolean)
    .forEach((domain) => {
      if (seen.has(domain)) return
      seen.add(domain)
      result.push(domain)
    })
  return result
}

const joinAllowedEmailDomains = (raw: unknown): string => {
  if (!Array.isArray(raw)) return ''
  return raw.map((item) => String(item || '').trim()).filter(Boolean).join('\n')
}

const clampNumber = (value: unknown, min: number, max: number, fallback: number) => {
  const parsed = normalizeNumber(value, fallback)
  if (parsed < min) return min
  if (parsed > max) return max
  return parsed
}

const notifyErrorIfNeeded = (err: unknown, fallback: string) => {
  const known = err as Error & { __notified?: boolean }
  if (known?.__notified) {
    return
  }
  notifyError(known?.message || fallback)
}

const fetchSettings = async () => {
  loading.value = true
  try {
    const [siteRes, orderRes, smtpRes, captchaRes, telegramRes, googleRes, githubRes, dashboardRes, registrationRes, orderEmailTmplRes] = await Promise.all([
      adminAPI.getSettings({ key: 'site_config' }),
      adminAPI.getSettings({ key: 'order_config' }),
      adminAPI.getSMTPSettings(),
      adminAPI.getCaptchaSettings(),
      adminAPI.getTelegramAuthSettings(),
      adminAPI.getGoogleAuthSettings(),
      adminAPI.getGitHubAuthSettings(),
      adminAPI.getSettings({ key: 'dashboard_config' }),
      adminAPI.getSettings({ key: 'registration_config' }),
      adminAPI.getOrderEmailTemplateSettings(),
    ])

    if (siteRes.data && siteRes.data.data) {
      const data = siteRes.data.data as Record<string, unknown>
      const brand = data.brand as Record<string, unknown> | undefined
      if (brand) {
        form.brand.site_name = String(brand.site_name || '')
        form.brand.site_url = String(brand.site_url || '')
        form.brand.site_icon = String(brand.site_icon || '')
        form.brand.site_logo = String(brand.site_logo || '')
        form.brand.site_description = normalizeLocalizedField(brand.site_description)
      }
      {
        const rawCurrency = String(data.currency || 'CNY').trim().toUpperCase()
        form.currency = /^[A-Z]{3}$/.test(rawCurrency) ? rawCurrency : 'CNY'
      }
      if (data.contact) {
        Object.assign(form.contact, data.contact)
      }
      const seo = data.seo as Record<string, unknown> | undefined
      if (seo) {
        ;['title', 'keywords', 'description'].forEach((field) => {
          if (seo[field]) {
            Object.assign(form.seo[field as keyof typeof form.seo], seo[field])
          }
        })
      }
      const about = data.about as Record<string, unknown> | undefined
      if (about) {
        const hero = about.hero as Record<string, unknown> | undefined
        if (hero) {
          form.about.hero.title = normalizeLocalizedField(hero.title)
          form.about.hero.subtitle = normalizeLocalizedField(hero.subtitle)
        }
        form.about.introduction = normalizeLocalizedField(about.introduction)

        const services = about.services as Record<string, unknown> | undefined
        if (services) {
          form.about.services.title = normalizeLocalizedField(services.title)
          const serviceItems = Array.isArray(services.items)
            ? services.items
                .map((item: unknown) => normalizeLocalizedField(item))
                .filter((item: Record<SupportedLanguage, string>) => isLocalizedFieldNotEmpty(item))
                .slice(0, 12)
            : []
          form.about.services.items.splice(0, form.about.services.items.length, ...serviceItems)
        } else {
          form.about.services.items.splice(0, form.about.services.items.length)
        }

        const aboutContact = about.contact as Record<string, unknown> | undefined
        if (aboutContact) {
          form.about.contact.title = normalizeLocalizedField(aboutContact.title)
          form.about.contact.text = normalizeLocalizedField(aboutContact.text)
        }
      }

      const legal = data.legal as Record<string, unknown> | undefined
      if (legal) {
        ;['terms', 'privacy'].forEach((field) => {
          if (legal[field]) {
            Object.assign(form.legal[field as keyof typeof form.legal], legal[field])
          }
        })
      }

      const scripts = normalizeSiteScripts(data.scripts)
      form.scripts.splice(0, form.scripts.length, ...scripts)

      const footerLinks = normalizeFooterLinks(data.footer_links)
      form.footer_links.splice(0, form.footer_links.length, ...footerLinks)

      const rawTemplateMode = String(data.template_mode || 'card').trim()
      form.template_mode = rawTemplateMode === 'list' ? 'list' : 'card'

      const rawStorefrontTemplate = String(data.storefront_template || 'classic').trim()
      form.storefront_template = rawStorefrontTemplate === 'vault' ? 'vault' : 'classic'
    }

    if (orderRes.data && orderRes.data.data) {
      const orderData = orderRes.data.data as Record<string, unknown>
      form.order_max_refund_days = clampNumber(orderData.max_refund_days, 0, 3650, 30)
      orderPaymentExpireMinutes.value = clampNumber(orderData.payment_expire_minutes, 1, 10080, 15)
    } else {
      form.order_max_refund_days = 30
      orderPaymentExpireMinutes.value = 15
    }

    if (smtpRes.data && smtpRes.data.data) {
      const smtp = smtpRes.data.data as Record<string, unknown>
      smtpData.enabled = !!smtp.enabled
      smtpData.host = String(smtp.host || '')
      smtpData.port = normalizeNumber(smtp.port, 587)
      smtpData.username = String(smtp.username || '')
      smtpData.password = ''
      smtpData.has_password = !!smtp.has_password
      smtpData.from = String(smtp.from || '')
      smtpData.from_name = String(smtp.from_name || '')
      smtpData.use_tls = !!smtp.use_tls
      smtpData.use_ssl = !!smtp.use_ssl
      smtpData.order_notification_enabled = smtp.order_notification_enabled !== false
      const verifyCode = smtp.verify_code as Record<string, unknown> | undefined
      smtpData.verify_code.expire_minutes = normalizeNumber(verifyCode?.expire_minutes, 10)
      smtpData.verify_code.send_interval_seconds = normalizeNumber(verifyCode?.send_interval_seconds, 60)
      smtpData.verify_code.max_attempts = normalizeNumber(verifyCode?.max_attempts, 5)
      smtpData.verify_code.length = normalizeNumber(verifyCode?.length, 6)
    }


    if (captchaRes.data && captchaRes.data.data) {
      const captcha = captchaRes.data.data as Record<string, unknown>
      captchaData.provider = String(captcha.provider || 'none')
      const captchaScenes = captcha.scenes as Record<string, unknown> | undefined
      captchaData.scenes.login = !!captchaScenes?.login
      captchaData.scenes.register_send_code = !!captchaScenes?.register_send_code
      captchaData.scenes.reset_send_code = !!captchaScenes?.reset_send_code
      captchaData.scenes.guest_create_order = !!captchaScenes?.guest_create_order
      captchaData.scenes.gift_card_redeem = !!captchaScenes?.gift_card_redeem

      const captchaImage = captcha.image as Record<string, unknown> | undefined
      captchaData.image.length = normalizeNumber(captchaImage?.length, 5)
      captchaData.image.width = normalizeNumber(captchaImage?.width, 240)
      captchaData.image.height = normalizeNumber(captchaImage?.height, 80)
      captchaData.image.noise_count = normalizeNumber(captchaImage?.noise_count, 2)
      captchaData.image.show_line = normalizeNumber(captchaImage?.show_line, 2)
      captchaData.image.expire_seconds = normalizeNumber(captchaImage?.expire_seconds, 300)
      captchaData.image.max_store = normalizeNumber(captchaImage?.max_store, 10240)

      const captchaTurnstile = captcha.turnstile as Record<string, unknown> | undefined
      captchaData.turnstile.site_key = String(captchaTurnstile?.site_key || '')
      captchaData.turnstile.secret_key = ''
      captchaData.turnstile.has_secret = !!captchaTurnstile?.has_secret
      captchaData.turnstile.verify_url = String(captchaTurnstile?.verify_url || 'https://challenges.cloudflare.com/turnstile/v0/siteverify')
      captchaData.turnstile.timeout_ms = normalizeNumber(captchaTurnstile?.timeout_ms, 2000)
    }

    if (telegramRes.data && telegramRes.data.data) {
      const telegram = telegramRes.data.data as Record<string, unknown>
      telegramForm.enabled = !!telegram.enabled
      telegramForm.bot_username = String(telegram.bot_username || '')
      telegramForm.bot_token = ''
      telegramForm.has_bot_token = !!telegram.has_bot_token
      telegramForm.mini_app_url = String(telegram.mini_app_url || '')
      telegramForm.login_expire_seconds = normalizeNumber(telegram.login_expire_seconds, 300)
      telegramForm.replay_ttl_seconds = normalizeNumber(telegram.replay_ttl_seconds, 300)
      telegramForm.client_secret = ''
      telegramForm.has_client_secret = !!telegram.has_client_secret
      telegramForm.oidc_redirect_uri = String(telegram.oidc_redirect_uri || '')
      telegramForm.mode = String(telegram.mode || '')
    }

    if (googleRes.data && googleRes.data.data) {
      const google = googleRes.data.data as Record<string, unknown>
      googleForm.enabled = !!google.enabled
      googleForm.client_id = String(google.client_id || '')
    }

    if (githubRes.data?.data) {
      const github = githubRes.data.data as Record<string, unknown>
      githubForm.enabled = !!github.enabled
      githubForm.client_id = String(github.client_id || '')
      githubForm.client_secret = ''
      githubForm.client_secret_configured = !!github.client_secret_configured
    }

    if (dashboardRes.data && dashboardRes.data.data) {
      const dashboard = dashboardRes.data.data as Record<string, unknown>
      const dashAccounting = dashboard.accounting as Record<string, unknown> | undefined
      const dashAlert = dashboard.alert as Record<string, unknown> | undefined
      const dashRanking = dashboard.ranking as Record<string, unknown> | undefined
      dashboardForm.accounting.refund_reverses_cost = dashAccounting?.refund_reverses_cost === true
      dashboardForm.alert.low_stock_threshold = clampNumber(dashAlert?.low_stock_threshold, 1, 500, 5)
      dashboardForm.alert.out_of_stock_products_threshold = clampNumber(dashAlert?.out_of_stock_products_threshold, 1, 10000, 1)
      dashboardForm.alert.pending_payment_orders_threshold = clampNumber(dashAlert?.pending_payment_orders_threshold, 1, 100000, 20)
      dashboardForm.alert.payments_failed_threshold = clampNumber(dashAlert?.payments_failed_threshold, 1, 100000, 10)
      dashboardForm.ranking.top_products_limit = clampNumber(dashRanking?.top_products_limit, 1, 20, 5)
      dashboardForm.ranking.top_channels_limit = clampNumber(dashRanking?.top_channels_limit, 1, 20, 5)
    }

    if (registrationRes.data && registrationRes.data.data) {
      const regData = registrationRes.data.data as Record<string, unknown>
      registrationForm.registration_enabled = regData.registration_enabled !== false
      registrationForm.email_verification_enabled = regData.email_verification_enabled !== false
      registrationForm.email_domain_allowlist_enabled = regData.email_domain_allowlist_enabled === true
      registrationForm.allowed_email_domains_text = joinAllowedEmailDomains(regData.allowed_email_domains)
    }

    if (orderEmailTmplRes.data && orderEmailTmplRes.data.data) {
      const tmplData = orderEmailTmplRes.data.data as Record<string, unknown>
      const templates = tmplData.templates as Record<string, unknown> | undefined
      if (templates) {
        orderEmailSceneKeys.forEach((key) => {
          const scene = templates[key] as Record<string, unknown> | undefined
          if (scene) {
            supportedLanguages.forEach((lang) => {
              const langData = scene[lang] as Record<string, unknown> | undefined
              if (langData) {
                orderEmailTemplateData.templates[key][lang].subject = String(langData.subject || '')
                orderEmailTemplateData.templates[key][lang].body = String(langData.body || '')
              }
            })
          }
        })
      }
      const guestTip = tmplData.guest_tip as Record<string, unknown> | undefined
      if (guestTip) {
        supportedLanguages.forEach((lang) => {
          orderEmailTemplateData.guest_tip[lang] = String(guestTip[lang] || '')
        })
      }
    }

  } catch (err) {
    notifyErrorIfNeeded(err, t('admin.settings.alerts.saveFailed'))
  } finally {
    loading.value = false
  }
}

const saveRegistrationSettings = async () => {
  await adminAPI.updateSettings({
    key: 'registration_config',
    value: {
      registration_enabled: registrationForm.registration_enabled,
      email_verification_enabled: registrationForm.email_verification_enabled,
      email_domain_allowlist_enabled: registrationForm.email_domain_allowlist_enabled,
      allowed_email_domains: splitAllowedEmailDomains(registrationForm.allowed_email_domains_text),
    },
  })
}

const saveSiteSettings = async () => {
  const payload = {
    key: 'site_config',
    value: {
      brand: form.brand,
      currency: String(form.currency || 'CNY').trim().toUpperCase(),
      contact: form.contact,
      seo: form.seo,
      about: form.about,
      legal: form.legal,
      scripts: form.scripts,
      footer_links: form.footer_links,
      storefront_template: form.storefront_template,
      template_mode: form.template_mode,
    },
  }
  await adminAPI.updateSettings(payload)
  applySiteIcon(form.brand.site_icon)
}

const openSiteIconPicker = () => {
  siteIconPickerRef.value?.openPicker()
}

const openSiteLogoPicker = () => {
  siteLogoPickerRef.value?.openPicker()
}

const clearSiteIcon = () => {
  form.brand.site_icon = ''
}

const clearSiteLogo = () => {
  form.brand.site_logo = ''
}

const saveOrderSettings = async () => {
  const normalizedMaxRefundDays = clampNumber(form.order_max_refund_days, 0, 3650, 30)
  const normalizedPaymentExpireMinutes = clampNumber(orderPaymentExpireMinutes.value, 1, 10080, 15)
  form.order_max_refund_days = normalizedMaxRefundDays
  orderPaymentExpireMinutes.value = normalizedPaymentExpireMinutes
  await adminAPI.updateSettings({
    key: 'order_config',
    value: {
      payment_expire_minutes: normalizedPaymentExpireMinutes,
      max_refund_days: normalizedMaxRefundDays,
    },
  })
}

const addAboutServiceItem = () => {
  if (form.about.services.items.length >= 12) {
    notifyError(t('admin.settings.about.maxServicesHint'))
    return
  }
  form.about.services.items.push(createLocalizedField())
}

const removeAboutServiceItem = (index: number) => {
  form.about.services.items.splice(index, 1)
}

const addSiteScriptItem = () => {
  if (form.scripts.length >= siteScriptsMaxCount) {
    notifyError(t('admin.settings.scripts.maxScriptsHint', { max: siteScriptsMaxCount }))
    return
  }
  form.scripts.push(createSiteScriptItem())
}

const removeSiteScriptItem = (index: number) => {
  form.scripts.splice(index, 1)
}

const addFooterLinkItem = () => {
  if (form.footer_links.length >= footerLinksMaxCount) {
    notifyError(t('admin.settings.footerLinks.maxHint', { max: footerLinksMaxCount }))
    return
  }
  form.footer_links.push(createFooterLinkItem())
}

const removeFooterLinkItem = (index: number) => {
  form.footer_links.splice(index, 1)
}


const saveTelegramAuthSettings = async () => {
  const payload: Record<string, unknown> = {
    enabled: telegramForm.enabled,
    bot_username: telegramForm.bot_username,
    mini_app_url: telegramForm.mini_app_url,
    login_expire_seconds: Number(telegramForm.login_expire_seconds),
    replay_ttl_seconds: Number(telegramForm.replay_ttl_seconds),
    oidc_redirect_uri: telegramForm.oidc_redirect_uri.trim(),
  }
  if (telegramForm.bot_token.trim() !== '') {
    payload.bot_token = telegramForm.bot_token.trim()
  }
  if (telegramForm.client_secret.trim() !== '') {
    payload.client_secret = telegramForm.client_secret.trim()
  }

  const res = await adminAPI.updateTelegramAuthSettings(payload)
  const data = res.data?.data as Record<string, unknown> | undefined
  telegramForm.bot_token = ''
  telegramForm.has_bot_token = !!data?.has_bot_token || telegramForm.has_bot_token
  telegramForm.client_secret = ''
  telegramForm.has_client_secret = !!data?.has_client_secret || telegramForm.has_client_secret
  telegramForm.mode = String(data?.mode || telegramForm.mode)
  telegramForm.oidc_redirect_uri = String(data?.oidc_redirect_uri ?? telegramForm.oidc_redirect_uri)
}

const saveGoogleAuthSettings = async () => {
  const res = await adminAPI.updateGoogleAuthSettings({
    enabled: googleForm.enabled,
    client_id: googleForm.client_id.trim(),
  })
  const data = res.data?.data as Record<string, unknown> | undefined
  googleForm.enabled = !!data?.enabled
  googleForm.client_id = String(data?.client_id || '')
}

const saveGitHubAuthSettings = async () => {
  const payload: Record<string, unknown> = { enabled: githubForm.enabled, client_id: githubForm.client_id.trim() }
  if (githubForm.client_secret.trim()) payload.client_secret = githubForm.client_secret.trim()
  const res = await adminAPI.updateGitHubAuthSettings(payload)
  const data = res.data?.data as Record<string, unknown> | undefined
  githubForm.client_secret = ''
  githubForm.client_secret_configured = !!data?.client_secret_configured
}

const saveDashboardSettings = async () => {
  const normalized = {
    accounting: {
      refund_reverses_cost: dashboardForm.accounting.refund_reverses_cost === true,
    },
    alert: {
      low_stock_threshold: clampNumber(dashboardForm.alert.low_stock_threshold, 1, 500, 5),
      out_of_stock_products_threshold: clampNumber(dashboardForm.alert.out_of_stock_products_threshold, 1, 10000, 1),
      pending_payment_orders_threshold: clampNumber(dashboardForm.alert.pending_payment_orders_threshold, 1, 100000, 20),
      payments_failed_threshold: clampNumber(dashboardForm.alert.payments_failed_threshold, 1, 100000, 10),
    },
    ranking: {
      top_products_limit: clampNumber(dashboardForm.ranking.top_products_limit, 1, 20, 5),
      top_channels_limit: clampNumber(dashboardForm.ranking.top_channels_limit, 1, 20, 5),
    },
  }

  dashboardForm.accounting.refund_reverses_cost = normalized.accounting.refund_reverses_cost
  dashboardForm.alert.low_stock_threshold = normalized.alert.low_stock_threshold
  dashboardForm.alert.out_of_stock_products_threshold = normalized.alert.out_of_stock_products_threshold
  dashboardForm.alert.pending_payment_orders_threshold = normalized.alert.pending_payment_orders_threshold
  dashboardForm.alert.payments_failed_threshold = normalized.alert.payments_failed_threshold
  dashboardForm.ranking.top_products_limit = normalized.ranking.top_products_limit
  dashboardForm.ranking.top_channels_limit = normalized.ranking.top_channels_limit

  const payload = {
    key: 'dashboard_config',
    value: normalized,
  }
  await adminAPI.updateSettings(payload)
}

const saveSettings = async () => {
  if (currentTab.value === 'smtp') {
    await smtpTabRef.value?.save()
    return
  }
  if (currentTab.value === 'order_email_template') {
    await orderEmailTemplateTabRef.value?.save()
    return
  }
  if (currentTab.value === 'captcha') {
    await captchaTabRef.value?.save()
    return
  }
  if (currentTab.value === 'navigation') {
    await navigationTabRef.value?.save()
    return
  }
  if (currentTab.value === 'home_announcement') {
    await homeAnnouncementTabRef.value?.save()
    return
  }
  if (currentTab.value === 'upstream_sync') {
    await upstreamSyncTabRef.value?.save()
    return
  }
  loading.value = true
  try {
    if (currentTab.value === 'telegram') {
      await saveTelegramAuthSettings()
    } else if (currentTab.value === 'google') {
      await saveGoogleAuthSettings()
    } else if (currentTab.value === 'github') {
      await saveGitHubAuthSettings()
    } else if (currentTab.value === 'dashboard') {
      await saveDashboardSettings()
    } else {
      await saveRegistrationSettings()
      await saveOrderSettings()
      await saveSiteSettings()
    }
    notifySuccess(t('admin.settings.alerts.saveSuccess'))
  } catch (err) {
    notifyErrorIfNeeded(err, t('admin.settings.alerts.saveFailed'))
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchSettings()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
      <div>
        <h1 class="text-2xl font-semibold">{{ t('admin.settings.title') }}</h1>
        <p class="mt-1 text-sm text-muted-foreground">{{ t('admin.settings.subtitle') }}</p>
      </div>
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center">
        <div class="flex max-w-full overflow-x-auto rounded-lg border border-border bg-card p-1">
          <button
            v-for="lang in languages"
            :key="lang.code"
            class="shrink-0 rounded-md px-3 py-1.5 text-xs font-medium transition-colors"
            :class="currentLang === lang.code ? 'bg-secondary text-foreground' : 'text-muted-foreground hover:text-foreground'"
            @click="currentLang = lang.code"
          >
            {{ lang.name }}
          </button>
        </div>
        <Button size="sm" class="w-full sm:w-auto" :disabled="loading || smtpTabRef?.submitting || smtpTabRef?.smtpTesting || captchaTabRef?.submitting || orderEmailTemplateTabRef?.submitting || navigationTabRef?.submitting || homeAnnouncementTabRef?.submitting || upstreamSyncTabRef?.submitting" @click="saveSettings">
          <span v-if="loading" class="h-3 w-3 animate-spin rounded-full border-2 border-primary/30 border-t-primary"></span>
          {{ loading ? t('admin.settings.actions.saving') : t('admin.settings.actions.save') }}
        </Button>
      </div>
    </div>

    <Tabs v-model="currentTab" class="flex flex-col gap-6">
      <TabsList class="h-auto flex-wrap gap-1">
        <TabsTrigger v-for="tab in tabs" :key="tab.value" :value="tab.value">{{ tab.label }}</TabsTrigger>
      </TabsList>

      <TabsContent value="basic" :forceMount="true" v-show="currentTab === 'basic'" class="space-y-6 mt-0">
      <div class="rounded-xl border border-border bg-card">
        <div class="border-b border-border bg-muted/40 px-6 py-4">
          <h2 class="text-lg font-semibold">{{ t('admin.settings.registration.title') }}</h2>
          <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.registration.subtitle') }}</p>
        </div>
        <div class="space-y-4 p-6">
          <div class="flex flex-col gap-3 rounded-lg border border-border bg-muted/20 px-4 py-3 sm:flex-row sm:items-center">
            <Switch id="registration-enabled" v-model="registrationForm.registration_enabled" />
            <div>
              <Label for="registration-enabled" class="text-sm font-medium">{{ t('admin.settings.registration.registrationEnabled') }}</Label>
              <p class="text-xs text-muted-foreground">{{ t('admin.settings.registration.registrationEnabledDesc') }}</p>
            </div>
          </div>
          <div class="flex flex-col gap-3 rounded-lg border border-border bg-muted/20 px-4 py-3 sm:flex-row sm:items-center">
            <Switch id="email-verification-enabled" v-model="registrationForm.email_verification_enabled" />
            <div>
              <Label for="email-verification-enabled" class="text-sm font-medium">{{ t('admin.settings.registration.emailVerificationEnabled') }}</Label>
              <p class="text-xs text-muted-foreground">{{ t('admin.settings.registration.emailVerificationEnabledDesc') }}</p>
            </div>
          </div>
          <div class="flex flex-col gap-3 rounded-lg border border-border bg-muted/20 px-4 py-3 sm:flex-row sm:items-center">
            <Switch id="email-domain-allowlist-enabled" v-model="registrationForm.email_domain_allowlist_enabled" />
            <div>
              <Label for="email-domain-allowlist-enabled" class="text-sm font-medium">{{ t('admin.settings.registration.emailDomainAllowlistEnabled') }}</Label>
              <p class="text-xs text-muted-foreground">{{ t('admin.settings.registration.emailDomainAllowlistEnabledDesc') }}</p>
            </div>
          </div>
          <div v-if="registrationForm.email_domain_allowlist_enabled" class="space-y-2 rounded-lg border border-border bg-muted/20 px-4 py-3">
            <Label for="allowed-email-domains" class="text-sm font-medium">{{ t('admin.settings.registration.allowedEmailDomains') }}</Label>
            <Textarea
              id="allowed-email-domains"
              v-model="registrationForm.allowed_email_domains_text"
              rows="4"
              :placeholder="t('admin.settings.registration.allowedEmailDomainsPlaceholder')"
            />
            <p class="text-xs text-muted-foreground">{{ t('admin.settings.registration.allowedEmailDomainsDesc') }}</p>
          </div>
        </div>
      </div>

      <div class="rounded-xl border border-border bg-card">
        <div class="border-b border-border bg-muted/40 px-6 py-4">
          <h2 class="text-lg font-semibold">{{ t('admin.settings.order.title') }}</h2>
          <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.order.subtitle') }}</p>
        </div>
        <div class="grid grid-cols-1 gap-6 p-6 md:grid-cols-2">
          <div class="space-y-2">
            <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.order.paymentExpireMinutes') }}</label>
            <Input
              v-model.number="orderPaymentExpireMinutes"
              type="number"
              min="1"
              max="10080"
              :placeholder="t('admin.settings.order.paymentExpireMinutesPlaceholder')"
            />
            <p class="text-xs text-muted-foreground">{{ t('admin.settings.order.paymentExpireMinutesTip') }}</p>
          </div>
          <div class="space-y-2">
            <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.order.maxRefundDays') }}</label>
            <Input
              v-model.number="form.order_max_refund_days"
              type="number"
              min="0"
              max="3650"
              :placeholder="t('admin.settings.order.maxRefundDaysPlaceholder')"
            />
            <p class="text-xs text-muted-foreground">{{ t('admin.settings.order.maxRefundDaysTip') }}</p>
          </div>
        </div>
      </div>

      <div class="rounded-xl border border-border bg-card">
        <div class="border-b border-border bg-muted/40 px-6 py-4">
          <h2 class="text-lg font-semibold">{{ t('admin.settings.brand.title') }}</h2>
          <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.brand.subtitle') }}</p>
        </div>
        <div class="grid grid-cols-1 gap-6 p-6 md:grid-cols-2">
          <div class="space-y-2">
            <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.brand.siteName') }}</label>
            <Input v-model="form.brand.site_name" :placeholder="t('admin.settings.brand.siteNamePlaceholder')" />
          </div>
          <div class="space-y-2">
            <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.brand.currency') }}</label>
            <Select v-model="form.currency">
              <SelectTrigger class="h-10 w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem v-for="item in currencyOptions" :key="item" :value="item">{{ item }}</SelectItem>
              </SelectContent>
            </Select>
            <p class="text-xs text-muted-foreground">{{ t('admin.settings.brand.currencyTip') }}</p>
          </div>
          <div class="space-y-2">
            <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.brand.siteUrl') }}</label>
            <Input v-model="form.brand.site_url" :placeholder="t('admin.settings.brand.siteUrlPlaceholder')" />
          </div>
          <div class="space-y-2">
            <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.brand.siteIcon') }}</label>
            <div class="flex items-center gap-3">
              <button
                type="button"
                class="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-lg border border-border bg-muted/20 transition-colors hover:border-primary/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                :title="t('admin.settings.brand.siteIconTip')"
                @click="openSiteIconPicker"
              >
                <img v-if="form.brand.site_icon" :src="getImageUrl(form.brand.site_icon)" class="h-full w-full object-contain" alt="" />
                <span v-else class="text-[10px] font-semibold text-muted-foreground">ICO</span>
              </button>
              <Button type="button" variant="outline" size="sm" @click="openSiteIconPicker">
                {{ t('admin.settings.brand.siteIconSelect') }}
              </Button>
              <Button v-if="form.brand.site_icon" type="button" variant="ghost" size="sm" @click="clearSiteIcon">
                {{ t('admin.common.delete') }}
              </Button>
            </div>
            <p class="text-xs text-muted-foreground">{{ t('admin.settings.brand.siteIconTip') }}</p>
            <MediaPicker ref="siteIconPickerRef" v-model="form.brand.site_icon" scene="common" dialog-only />
          </div>
          <!-- Logo 使用独立的 picker 和清除动作，避免误操作影响浏览器 favicon。 -->
          <div class="space-y-2">
            <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.brand.siteLogo') }}</label>
            <div class="flex items-center gap-3">
              <button
                type="button"
                class="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-lg border border-border bg-muted/20 transition-colors hover:border-primary/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                :title="t('admin.settings.brand.siteLogoTip')"
                @click="openSiteLogoPicker"
              >
                <img v-if="form.brand.site_logo" :src="getImageUrl(form.brand.site_logo)" class="h-full w-full object-contain" alt="" />
                <span v-else class="text-[10px] font-semibold text-muted-foreground">LOGO</span>
              </button>
              <Button type="button" variant="outline" size="sm" @click="openSiteLogoPicker">
                {{ t('admin.settings.brand.siteLogoSelect') }}
              </Button>
              <Button v-if="form.brand.site_logo" type="button" variant="ghost" size="sm" @click="clearSiteLogo">
                {{ t('admin.common.delete') }}
              </Button>
            </div>
            <p class="text-xs text-muted-foreground">{{ t('admin.settings.brand.siteLogoTip') }}</p>
            <MediaPicker ref="siteLogoPickerRef" v-model="form.brand.site_logo" scene="common" dialog-only />
          </div>
          <div class="space-y-2 md:col-span-2">
            <div class="flex items-center justify-between">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.brand.siteDescription') }}</label>
              <span class="rounded bg-muted px-2 py-1 text-xs text-muted-foreground">{{ currentLang }}</span>
            </div>
            <Input v-model="form.brand.site_description[currentLang]" :placeholder="t('admin.settings.brand.siteDescriptionPlaceholder')" />
            <p class="text-xs text-muted-foreground">{{ t('admin.settings.brand.siteDescriptionTip') }}</p>
          </div>
        </div>
      </div>

      <div class="rounded-xl border border-border bg-card">
        <div class="flex flex-col gap-3 border-b border-border bg-muted/40 px-6 py-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 class="text-lg font-semibold">{{ t('admin.settings.seo.title') }}</h2>
            <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.seo.subtitle', { lang: getCurrentLangName() }) }}</p>
          </div>
          <span class="rounded bg-muted px-2 py-1 text-xs text-muted-foreground">{{ currentLang }}</span>
        </div>
        <div class="space-y-6 p-6">
          <div class="space-y-2">
            <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.seo.siteTitle') }}</label>
            <Input v-model="form.seo.title[currentLang]" :placeholder="t('admin.settings.seo.siteTitlePlaceholder')" />
          </div>
          <div class="space-y-2">
            <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.seo.keywords') }}</label>
            <Input v-model="form.seo.keywords[currentLang]" :placeholder="t('admin.settings.seo.keywordsPlaceholder')" />
          </div>
          <div class="space-y-2">
            <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.seo.description') }}</label>
            <Textarea v-model="form.seo.description[currentLang]" rows="3" :placeholder="t('admin.settings.seo.descriptionPlaceholder')" />
          </div>
        </div>
      </div>

      <div class="rounded-xl border border-border bg-card">
        <div class="border-b border-border bg-muted/40 px-6 py-4">
          <h2 class="text-lg font-semibold">{{ t('admin.settings.contact.title') }}</h2>
          <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.contact.subtitle') }}</p>
        </div>
        <div class="grid grid-cols-1 gap-6 p-6 md:grid-cols-2">
          <div class="space-y-2">
            <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.contact.telegram') }}</label>
            <Input v-model="form.contact.telegram" :placeholder="t('admin.settings.contact.telegramPlaceholder')" />
          </div>
          <div class="space-y-2">
            <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.contact.whatsapp') }}</label>
            <Input v-model="form.contact.whatsapp" :placeholder="t('admin.settings.contact.whatsappPlaceholder')" />
          </div>
        </div>
      </div>

      <div class="rounded-xl border border-border bg-card">
        <div class="flex flex-col gap-3 border-b border-border bg-muted/40 px-6 py-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 class="text-lg font-semibold">{{ t('admin.settings.footerLinks.title') }}</h2>
            <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.footerLinks.subtitle') }}</p>
          </div>
          <Button type="button" size="sm" variant="outline" class="w-full sm:w-auto" @click="addFooterLinkItem">
            {{ t('admin.settings.footerLinks.add') }}
          </Button>
        </div>
        <div class="space-y-4 p-6">
          <div v-if="form.footer_links.length === 0" class="rounded-lg border border-dashed border-border bg-muted/10 px-3 py-6 text-center text-xs text-muted-foreground">
            {{ t('admin.settings.footerLinks.empty') }}
          </div>

          <div v-for="(link, index) in form.footer_links" :key="`footer-link-${index}`" class="flex flex-col gap-3 sm:flex-row sm:items-center">
            <div class="grid flex-1 grid-cols-1 gap-3 md:grid-cols-2">
              <Input v-model="link.name" :placeholder="t('admin.settings.footerLinks.namePlaceholder')" />
              <Input v-model="link.url" :placeholder="t('admin.settings.footerLinks.urlPlaceholder')" />
            </div>
            <Button type="button" size="sm" variant="destructive" class="w-full sm:w-auto" @click="removeFooterLinkItem(index)">
              {{ t('admin.common.delete') }}
            </Button>
          </div>
        </div>
      </div>

      <div class="rounded-xl border border-border bg-card">
        <div class="flex flex-col gap-3 border-b border-border bg-muted/40 px-6 py-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 class="text-lg font-semibold">{{ t('admin.settings.scripts.title') }}</h2>
            <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.scripts.subtitle') }}</p>
          </div>
          <Button type="button" size="sm" variant="outline" class="w-full sm:w-auto" @click="addSiteScriptItem">
            {{ t('admin.settings.scripts.addScript') }}
          </Button>
        </div>

        <div class="space-y-4 p-6">
          <p class="rounded-lg border border-border bg-muted/20 px-3 py-2 text-xs text-muted-foreground">
            {{ t('admin.settings.scripts.injectTip') }}
          </p>

          <div v-if="form.scripts.length === 0" class="rounded-lg border border-dashed border-border bg-muted/10 px-3 py-6 text-center text-xs text-muted-foreground">
            {{ t('admin.settings.scripts.empty') }}
          </div>

          <div v-for="(script, index) in form.scripts" :key="`site-script-${index}`" class="space-y-4 rounded-lg border border-border bg-muted/10 p-4">
            <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <h3 class="text-sm font-semibold">{{ t('admin.settings.scripts.scriptItem', { index: index + 1 }) }}</h3>
              <Button type="button" size="sm" variant="destructive" class="w-full sm:w-auto" @click="removeSiteScriptItem(index)">
                {{ t('admin.common.delete') }}
              </Button>
            </div>

            <div class="grid grid-cols-1 gap-4 md:grid-cols-3">
              <div class="space-y-2 md:col-span-2">
                <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.scripts.name') }}</label>
                <Input v-model="script.name" :placeholder="t('admin.settings.scripts.namePlaceholder')" />
              </div>

              <div class="space-y-2">
                <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.scripts.position') }}</label>
                <Select v-model="script.position">
                  <SelectTrigger class="h-10 w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="head">{{ t('admin.settings.scripts.positionHead') }}</SelectItem>
                    <SelectItem value="body_end">{{ t('admin.settings.scripts.positionBodyEnd') }}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            <Label class="flex items-center gap-2 text-sm text-muted-foreground cursor-pointer">
              <Switch v-model="script.enabled" />
              {{ t('admin.settings.scripts.enabled') }}
            </Label>

            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.scripts.code') }}</label>
              <Textarea v-model="script.code" rows="7" class="font-mono text-xs" :placeholder="t('admin.settings.scripts.codePlaceholder')" />
            </div>
          </div>
        </div>
      </div>
      </TabsContent>

      <!-- Template Mode Tab -->
      <TabsContent value="template" :forceMount="true" v-show="currentTab === 'template'" class="space-y-6 mt-0">
      <!-- Storefront theme: classic / vault -->
      <div class="rounded-xl border border-border bg-card">
        <div class="flex flex-col gap-3 border-b border-border bg-muted/40 px-6 py-4">
          <div>
            <h2 class="text-lg font-semibold">{{ t('admin.settings.template.storefrontTitle') }}</h2>
            <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.template.storefrontSubtitle') }}</p>
          </div>
        </div>
        <div class="px-6 py-6">
          <RadioGroup v-model="form.storefront_template" class="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <!-- Classic -->
            <Label
              class="relative flex cursor-pointer flex-col items-center gap-3 rounded-xl border-2 p-6 transition-all"
              :class="form.storefront_template === 'classic'
                ? 'border-primary bg-primary/5 ring-1 ring-primary/20'
                : 'border-border hover:border-muted-foreground/30'"
            >
              <RadioGroupItem value="classic" class="sr-only" />
              <div class="flex h-16 w-16 items-center justify-center rounded-xl" :class="form.storefront_template === 'classic' ? 'bg-primary/10 text-primary' : 'bg-muted text-muted-foreground'">
                <svg class="h-8 w-8" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
                  <circle cx="12" cy="12" r="4" />
                  <path stroke-linecap="round" d="M12 2v2.5M12 19.5V22M2 12h2.5M19.5 12H22M4.9 4.9l1.8 1.8M17.3 17.3l1.8 1.8M19.1 4.9l-1.8 1.8M6.7 17.3l-1.8 1.8" />
                </svg>
              </div>
              <div class="text-center">
                <div class="font-semibold" :class="form.storefront_template === 'classic' ? 'text-primary' : ''">{{ t('admin.settings.template.classicMode') }}</div>
                <div class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.template.classicModeDesc') }}</div>
              </div>
              <div v-if="form.storefront_template === 'classic'" class="absolute right-3 top-3">
                <svg class="h-5 w-5 text-primary" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                </svg>
              </div>
            </Label>

            <!-- Vault -->
            <Label
              class="relative flex cursor-pointer flex-col items-center gap-3 rounded-xl border-2 p-6 transition-all"
              :class="form.storefront_template === 'vault'
                ? 'border-primary bg-primary/5 ring-1 ring-primary/20'
                : 'border-border hover:border-muted-foreground/30'"
            >
              <RadioGroupItem value="vault" class="sr-only" />
              <div class="flex h-16 w-16 items-center justify-center rounded-xl" :class="form.storefront_template === 'vault' ? 'bg-primary/10 text-primary' : 'bg-muted text-muted-foreground'">
                <svg class="h-8 w-8" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M12 3l7 3v5c0 4.5-3 7.6-7 9-4-1.4-7-4.5-7-9V6l7-3z" />
                  <circle cx="12" cy="11" r="2.2" />
                  <path stroke-linecap="round" d="M12 13.2V16" />
                </svg>
              </div>
              <div class="text-center">
                <div class="font-semibold" :class="form.storefront_template === 'vault' ? 'text-primary' : ''">{{ t('admin.settings.template.vaultMode') }}</div>
                <div class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.template.vaultModeDesc') }}</div>
              </div>
              <div v-if="form.storefront_template === 'vault'" class="absolute right-3 top-3">
                <svg class="h-5 w-5 text-primary" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                </svg>
              </div>
            </Label>
          </RadioGroup>
        </div>
      </div>

      <!-- Product layout: card / list -->
      <div class="rounded-xl border border-border bg-card">
        <div class="flex flex-col gap-3 border-b border-border bg-muted/40 px-6 py-4">
          <div>
            <h2 class="text-lg font-semibold">{{ t('admin.settings.template.layoutTitle') }}</h2>
            <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.template.layoutSubtitle') }}</p>
          </div>
        </div>
        <div class="px-6 py-6 space-y-6">
          <RadioGroup v-model="form.template_mode" class="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <!-- Card Mode -->
            <Label
              class="relative flex cursor-pointer flex-col items-center gap-3 rounded-xl border-2 p-6 transition-all"
              :class="form.template_mode === 'card'
                ? 'border-primary bg-primary/5 ring-1 ring-primary/20'
                : 'border-border hover:border-muted-foreground/30'"
            >
              <RadioGroupItem value="card" class="sr-only" />
              <div class="flex h-16 w-16 items-center justify-center rounded-xl" :class="form.template_mode === 'card' ? 'bg-primary/10 text-primary' : 'bg-muted text-muted-foreground'">
                <svg class="h-8 w-8" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
                  <rect x="3" y="3" width="7" height="7" rx="1" />
                  <rect x="14" y="3" width="7" height="7" rx="1" />
                  <rect x="3" y="14" width="7" height="7" rx="1" />
                  <rect x="14" y="14" width="7" height="7" rx="1" />
                </svg>
              </div>
              <div class="text-center">
                <div class="font-semibold" :class="form.template_mode === 'card' ? 'text-primary' : ''">{{ t('admin.settings.template.cardMode') }}</div>
                <div class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.template.cardModeDesc') }}</div>
              </div>
              <div v-if="form.template_mode === 'card'" class="absolute right-3 top-3">
                <svg class="h-5 w-5 text-primary" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                </svg>
              </div>
            </Label>

            <!-- List Mode -->
            <Label
              class="relative flex cursor-pointer flex-col items-center gap-3 rounded-xl border-2 p-6 transition-all"
              :class="form.template_mode === 'list'
                ? 'border-primary bg-primary/5 ring-1 ring-primary/20'
                : 'border-border hover:border-muted-foreground/30'"
            >
              <RadioGroupItem value="list" class="sr-only" />
              <div class="flex h-16 w-16 items-center justify-center rounded-xl" :class="form.template_mode === 'list' ? 'bg-primary/10 text-primary' : 'bg-muted text-muted-foreground'">
                <svg class="h-8 w-8" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
                  <path stroke-linecap="round" d="M3 6h18M3 12h18M3 18h18" />
                  <circle cx="3" cy="6" r="1" fill="currentColor" />
                  <circle cx="3" cy="12" r="1" fill="currentColor" />
                  <circle cx="3" cy="18" r="1" fill="currentColor" />
                </svg>
              </div>
              <div class="text-center">
                <div class="font-semibold" :class="form.template_mode === 'list' ? 'text-primary' : ''">{{ t('admin.settings.template.listMode') }}</div>
                <div class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.template.listModeDesc') }}</div>
              </div>
              <div v-if="form.template_mode === 'list'" class="absolute right-3 top-3">
                <svg class="h-5 w-5 text-primary" fill="currentColor" viewBox="0 0 20 20">
                  <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
                </svg>
              </div>
            </Label>
          </RadioGroup>
        </div>
      </div>
      </TabsContent>

      <TabsContent value="about" :forceMount="true" v-show="currentTab === 'about'" class="space-y-6 mt-0">
      <div class="rounded-xl border border-border bg-card">
        <div class="flex flex-col gap-3 border-b border-border bg-muted/40 px-6 py-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 class="text-lg font-semibold">{{ t('admin.settings.about.title') }}</h2>
            <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.about.subtitle', { lang: getCurrentLangName() }) }}</p>
          </div>
          <span class="rounded bg-muted px-2 py-1 text-xs text-muted-foreground">{{ currentLang }}</span>
        </div>

        <div class="space-y-6 p-6">
          <div class="rounded-xl border border-border">
            <div class="border-b border-border bg-muted/30 px-4 py-3">
              <h3 class="text-sm font-semibold">{{ t('admin.settings.about.heroTitle') }}</h3>
            </div>
            <div class="grid grid-cols-1 gap-4 p-4 md:grid-cols-2">
              <div class="space-y-2">
                <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.about.heroMainTitle') }}</label>
                <Input v-model="form.about.hero.title[currentLang]" :placeholder="t('admin.settings.about.heroMainTitlePlaceholder')" />
              </div>
              <div class="space-y-2">
                <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.about.heroSubtitle') }}</label>
                <Input v-model="form.about.hero.subtitle[currentLang]" :placeholder="t('admin.settings.about.heroSubtitlePlaceholder')" />
              </div>
            </div>
          </div>

          <div class="rounded-xl border border-border">
            <div class="border-b border-border bg-muted/30 px-4 py-3">
              <h3 class="text-sm font-semibold">{{ t('admin.settings.about.introductionTitle') }}</h3>
              <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.about.introductionSubtitle') }}</p>
            </div>
            <div class="p-4">
              <Textarea v-model="form.about.introduction[currentLang]" rows="5" :placeholder="t('admin.settings.about.introductionPlaceholder')" />
            </div>
          </div>

          <div class="rounded-xl border border-border">
            <div class="flex flex-col gap-3 border-b border-border bg-muted/30 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
              <h3 class="text-sm font-semibold">{{ t('admin.settings.about.servicesTitle') }}</h3>
              <Button type="button" size="sm" variant="outline" class="w-full sm:w-auto" @click="addAboutServiceItem">
                {{ t('admin.settings.about.addServiceItem') }}
              </Button>
            </div>
            <div class="space-y-4 p-4">
              <div class="space-y-2">
                <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.about.servicesBlockTitle') }}</label>
                <Input v-model="form.about.services.title[currentLang]" :placeholder="t('admin.settings.about.servicesBlockTitlePlaceholder')" />
              </div>

              <div v-if="form.about.services.items.length === 0" class="rounded-lg border border-dashed border-border bg-muted/10 px-3 py-4 text-xs text-muted-foreground">
                {{ t('admin.settings.about.servicesEmpty') }}
              </div>

              <div v-for="(item, index) in form.about.services.items" :key="`about-service-${index}`" class="rounded-lg border border-border bg-muted/10 p-3">
                <div class="mb-2 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                  <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.about.serviceItem', { index: index + 1 }) }}</label>
                  <Button type="button" size="sm" variant="destructive" class="w-full sm:w-auto" @click="removeAboutServiceItem(index)">
                    {{ t('admin.common.delete') }}
                  </Button>
                </div>
                <Input v-model="item[currentLang]" :placeholder="t('admin.settings.about.serviceItemPlaceholder')" />
              </div>
            </div>
          </div>

          <div class="rounded-xl border border-border">
            <div class="border-b border-border bg-muted/30 px-4 py-3">
              <h3 class="text-sm font-semibold">{{ t('admin.settings.about.contactTitle') }}</h3>
              <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.about.contactSubtitle') }}</p>
            </div>
            <div class="space-y-4 p-4">
              <div class="space-y-2">
                <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.about.contactBlockTitle') }}</label>
                <Input v-model="form.about.contact.title[currentLang]" :placeholder="t('admin.settings.about.contactBlockTitlePlaceholder')" />
              </div>
              <div class="space-y-2">
                <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.about.contactText') }}</label>
                <Textarea v-model="form.about.contact.text[currentLang]" rows="4" :placeholder="t('admin.settings.about.contactTextPlaceholder')" />
              </div>
            </div>
          </div>
        </div>
      </div>
      </TabsContent>

      <TabsContent value="legal" :forceMount="true" v-show="currentTab === 'legal'" class="space-y-6 mt-0">
      <div class="rounded-xl border border-border bg-card">
        <div class="flex flex-col gap-3 border-b border-border bg-muted/40 px-6 py-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 class="text-lg font-semibold">{{ t('admin.settings.legal.termsTitle') }}</h2>
            <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.legal.termsSubtitle', { lang: getCurrentLangName() }) }}</p>
          </div>
          <span class="rounded bg-muted px-2 py-1 text-xs text-muted-foreground">{{ currentLang }}</span>
        </div>
        <div class="p-0">
          <RichEditor :key="`terms-${currentLang}`" v-model="form.legal.terms[currentLang]" :placeholder="t('admin.settings.legal.termsPlaceholder')" />
        </div>
      </div>

      <div class="rounded-xl border border-border bg-card">
        <div class="flex flex-col gap-3 border-b border-border bg-muted/40 px-6 py-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 class="text-lg font-semibold">{{ t('admin.settings.legal.privacyTitle') }}</h2>
            <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.legal.privacySubtitle', { lang: getCurrentLangName() }) }}</p>
          </div>
          <span class="rounded bg-muted px-2 py-1 text-xs text-muted-foreground">{{ currentLang }}</span>
        </div>
        <div class="p-0">
          <RichEditor :key="`privacy-${currentLang}`" v-model="form.legal.privacy[currentLang]" :placeholder="t('admin.settings.legal.privacyPlaceholder')" />
        </div>
      </div>
      </TabsContent>

      <TabsContent value="home_announcement" :forceMount="true" v-show="currentTab === 'home_announcement'" class="mt-0">
        <SettingsHomeAnnouncementTab ref="homeAnnouncementTabRef" :current-lang="currentLang" @saved="fetchSettings" />
      </TabsContent>

      <TabsContent value="smtp" :forceMount="true" v-show="currentTab === 'smtp'" class="mt-0">
        <SettingsSMTPTab ref="smtpTabRef" :data="smtpData" @saved="fetchSettings" />
      </TabsContent>

      <TabsContent value="order_email_template" :forceMount="true" v-show="currentTab === 'order_email_template'" class="mt-0">
        <SettingsOrderEmailTemplateTab ref="orderEmailTemplateTabRef" :data="orderEmailTemplateData" :current-lang="currentLang" @saved="fetchSettings" />
      </TabsContent>

      <TabsContent value="captcha" :forceMount="true" v-show="currentTab === 'captcha'" class="mt-0">
        <SettingsCaptchaTab ref="captchaTabRef" :data="captchaData" @saved="fetchSettings" />
      </TabsContent>

      <TabsContent value="navigation" :forceMount="true" v-show="currentTab === 'navigation'" class="mt-0">
        <SettingsNavigationTab ref="navigationTabRef" :current-lang="currentLang" @saved="fetchSettings" />
      </TabsContent>

      <TabsContent value="telegram" :forceMount="true" v-show="currentTab === 'telegram'" class="space-y-6 mt-0">
      <div class="rounded-xl border border-border bg-card">
        <div class="border-b border-border bg-muted/40 px-6 py-4">
          <h2 class="text-lg font-semibold">{{ t('admin.settings.telegram.title') }}</h2>
          <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.telegram.subtitle') }}</p>
        </div>

        <div class="space-y-6 p-6">
          <div class="flex flex-col gap-3 rounded-lg border border-border bg-muted/20 px-4 py-3 sm:flex-row sm:items-center">
            <Switch id="telegram-auth-enabled" v-model="telegramForm.enabled" />
            <Label for="telegram-auth-enabled" class="text-sm font-medium">{{ t('admin.settings.telegram.enabled') }}</Label>
          </div>

          <div class="grid grid-cols-1 gap-6 md:grid-cols-2">
            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.telegram.botUsername') }}</label>
              <Input v-model="telegramForm.bot_username" :placeholder="t('admin.settings.telegram.botUsernamePlaceholder')" />
            </div>
            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.telegram.botToken') }}</label>
              <Input v-model="telegramForm.bot_token" type="password" :placeholder="t('admin.settings.telegram.botTokenPlaceholder')" />
              <p class="text-xs text-muted-foreground">
                {{ telegramForm.has_bot_token ? t('admin.settings.telegram.botTokenHintKeep') : t('admin.settings.telegram.botTokenHintEmpty') }}
              </p>
            </div>
            <div class="space-y-2 md:col-span-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.telegram.modeLabel') }}</label>
              <p class="text-sm font-medium">
                {{ telegramForm.mode === 'oidc' ? t('admin.settings.telegram.modeOidc') : (telegramForm.mode === 'widget' ? t('admin.settings.telegram.modeWidget') : t('admin.settings.telegram.modeDisabled')) }}
              </p>
            </div>
            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.telegram.clientSecret') }}</label>
              <Input v-model="telegramForm.client_secret" type="password" :placeholder="t('admin.settings.telegram.clientSecretPlaceholder')" />
              <p class="text-xs text-muted-foreground">
                {{ telegramForm.has_client_secret ? t('admin.settings.telegram.clientSecretHintKeep') : t('admin.settings.telegram.clientSecretHintEmpty') }}
              </p>
            </div>
            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.telegram.oidcRedirectURI') }}</label>
              <Input v-model="telegramForm.oidc_redirect_uri" :placeholder="t('admin.settings.telegram.oidcRedirectURIPlaceholder')" />
              <p class="text-xs text-muted-foreground">
                {{ t('admin.settings.telegram.oidcRedirectURIHint') }}
              </p>
            </div>
            <div class="space-y-2 md:col-span-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.telegram.miniAppURL') }}</label>
              <Input v-model="telegramForm.mini_app_url" :placeholder="t('admin.settings.telegram.miniAppURLPlaceholder')" />
              <p class="text-xs text-muted-foreground">
                {{ t('admin.settings.telegram.miniAppURLHint') }}
              </p>
            </div>
            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.telegram.loginExpireSeconds') }}</label>
              <Input v-model.number="telegramForm.login_expire_seconds" type="number" min="30" max="86400" />
            </div>
            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.telegram.replayTTLSeconds') }}</label>
              <Input v-model.number="telegramForm.replay_ttl_seconds" type="number" min="60" max="86400" />
            </div>
          </div>
        </div>
      </div>
      </TabsContent>

      <TabsContent value="google" :forceMount="true" v-show="currentTab === 'google'" class="space-y-6 mt-0">
        <div class="rounded-xl border border-border bg-card">
          <div class="border-b border-border bg-muted/40 px-6 py-4">
            <h2 class="text-lg font-semibold">{{ t('admin.settings.google.title') }}</h2>
            <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.google.subtitle') }}</p>
          </div>

          <div class="space-y-6 p-6">
            <div class="flex flex-col gap-3 rounded-lg border border-border bg-muted/20 px-4 py-3 sm:flex-row sm:items-center">
              <Switch id="google-auth-enabled" v-model="googleForm.enabled" />
              <Label for="google-auth-enabled" class="text-sm font-medium">{{ t('admin.settings.google.enabled') }}</Label>
            </div>

            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.google.clientID') }}</label>
              <Input v-model="googleForm.client_id" :placeholder="t('admin.settings.google.clientIDPlaceholder')" />
              <p class="text-xs text-muted-foreground">{{ t('admin.settings.google.clientIDHint') }}</p>
            </div>

            <div class="rounded-lg border border-blue-200 bg-blue-50 px-4 py-3 text-xs leading-5 text-blue-800 dark:border-blue-900/60 dark:bg-blue-950/30 dark:text-blue-200">
              <p>{{ t('admin.settings.google.credentialHint') }}</p>
              <p class="mt-1">{{ t('admin.settings.google.originHint') }}</p>
              <p class="mt-1">{{ t('admin.settings.google.redirectHint') }}</p>
            </div>
          </div>
        </div>
      </TabsContent>

      <TabsContent value="github" :forceMount="true" v-show="currentTab === 'github'" class="space-y-6 mt-0">
        <div class="rounded-xl border border-border bg-card p-6 space-y-5">
          <h2 class="text-lg font-semibold">{{ t('admin.settings.github.title') }}</h2>
          <div class="flex items-center gap-3"><Switch id="github-auth-enabled" v-model="githubForm.enabled" /><Label for="github-auth-enabled">{{ t('admin.settings.github.enabled') }}</Label></div>
          <div class="space-y-2"><Label>{{ t('admin.settings.github.clientID') }}</Label><Input v-model="githubForm.client_id" autocomplete="off" /></div>
          <div class="space-y-2"><Label>{{ t('admin.settings.github.clientSecret') }}</Label><Input v-model="githubForm.client_secret" type="password" autocomplete="new-password" :placeholder="githubForm.client_secret_configured ? t('admin.settings.github.secretConfigured') : t('admin.settings.github.secretRequired')" /></div>
          <p class="text-xs text-muted-foreground">{{ t('admin.settings.github.callbackHint') }}</p>
        </div>
      </TabsContent>

      <TabsContent value="upstream_sync" :forceMount="true" v-show="currentTab === 'upstream_sync'" class="mt-0">
        <SettingsUpstreamSyncTab ref="upstreamSyncTabRef" />
      </TabsContent>

      <TabsContent value="dashboard" :forceMount="true" v-show="currentTab === 'dashboard'" class="space-y-6 mt-0">
      <div class="rounded-xl border border-border bg-card">
        <div class="border-b border-border bg-muted/40 px-6 py-4">
          <h2 class="text-lg font-semibold">{{ t('admin.settings.dashboard.title') }}</h2>
          <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.settings.dashboard.subtitle') }}</p>
        </div>

        <div class="space-y-6 p-6">
          <div class="rounded-xl border border-border">
            <div class="border-b border-border bg-muted/30 px-4 py-3">
              <h3 class="text-sm font-semibold">{{ t('admin.settings.dashboard.accounting.title') }}</h3>
            </div>
            <div class="p-4">
              <div class="flex flex-col gap-3 rounded-lg border border-border bg-muted/20 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
                <div class="space-y-1">
                  <Label for="dashboard-refund-reverses-cost" class="text-sm font-medium">{{ t('admin.settings.dashboard.accounting.refundReversesCost') }}</Label>
                  <p class="text-xs text-muted-foreground">{{ t('admin.settings.dashboard.accounting.refundReversesCostHint') }}</p>
                </div>
                <Switch id="dashboard-refund-reverses-cost" v-model="dashboardForm.accounting.refund_reverses_cost" />
              </div>
            </div>
          </div>

          <div class="rounded-xl border border-border">
            <div class="border-b border-border bg-muted/30 px-4 py-3">
              <h3 class="text-sm font-semibold">{{ t('admin.settings.dashboard.alert.title') }}</h3>
            </div>
            <div class="grid grid-cols-1 gap-4 p-4 md:grid-cols-2">
              <div class="space-y-2">
                <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.dashboard.alert.lowStockThreshold') }}</label>
                <Input v-model.number="dashboardForm.alert.low_stock_threshold" type="number" min="1" max="500" />
                <p class="text-xs text-muted-foreground">{{ t('admin.settings.dashboard.alert.lowStockThresholdHint') }}</p>
              </div>
              <div class="space-y-2">
                <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.dashboard.alert.outOfStockThreshold') }}</label>
                <Input v-model.number="dashboardForm.alert.out_of_stock_products_threshold" type="number" min="1" max="10000" />
                <p class="text-xs text-muted-foreground">{{ t('admin.settings.dashboard.alert.outOfStockThresholdHint') }}</p>
              </div>
              <div class="space-y-2">
                <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.dashboard.alert.pendingOrderThreshold') }}</label>
                <Input v-model.number="dashboardForm.alert.pending_payment_orders_threshold" type="number" min="1" max="100000" />
                <p class="text-xs text-muted-foreground">{{ t('admin.settings.dashboard.alert.pendingOrderThresholdHint') }}</p>
              </div>
              <div class="space-y-2">
                <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.dashboard.alert.paymentFailedThreshold') }}</label>
                <Input v-model.number="dashboardForm.alert.payments_failed_threshold" type="number" min="1" max="100000" />
                <p class="text-xs text-muted-foreground">{{ t('admin.settings.dashboard.alert.paymentFailedThresholdHint') }}</p>
              </div>
            </div>
          </div>

          <div class="rounded-xl border border-border">
            <div class="border-b border-border bg-muted/30 px-4 py-3">
              <h3 class="text-sm font-semibold">{{ t('admin.settings.dashboard.ranking.title') }}</h3>
            </div>
            <div class="grid grid-cols-1 gap-4 p-4 md:grid-cols-2">
              <div class="space-y-2">
                <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.dashboard.ranking.topProductsLimit') }}</label>
                <Input v-model.number="dashboardForm.ranking.top_products_limit" type="number" min="1" max="20" />
                <p class="text-xs text-muted-foreground">{{ t('admin.settings.dashboard.ranking.topProductsLimitHint') }}</p>
              </div>
              <div class="space-y-2">
                <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.dashboard.ranking.topChannelsLimit') }}</label>
                <Input v-model.number="dashboardForm.ranking.top_channels_limit" type="number" min="1" max="20" />
                <p class="text-xs text-muted-foreground">{{ t('admin.settings.dashboard.ranking.topChannelsLimitHint') }}</p>
              </div>
            </div>
          </div>
        </div>
      </div>
      </TabsContent>

    </Tabs>
  </div>
</template>
