import { computed, onMounted, ref, type Component } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Banknote, Home, ShoppingBag, Wallet, Gift, ShieldCheck, UserCircle, Megaphone, Key } from 'lucide-vue-next'
import { orderStatusLabel, orderStatusVariant } from '../utils/status'
import type { PageAlert } from '../utils/alerts'
import { useAppStore } from '../stores/app'
import { useUserProfileStore } from '../stores/userProfile'
import type { PublicMemberLevel } from '../api'
import { useLocalized } from './useProduct'
import { formatSkuSpecValues } from '../utils/sku'

export type PersonalSection = 'overview' | 'profile' | 'security' | 'orders' | 'wallet' | 'giftCard' | 'affiliate' | 'reseller' | 'api'

export interface PersonalSectionItem {
  key: PersonalSection
  label: string
  icon: Component
}

/**
 * 个人中心壳逻辑（classic + vault 壳共用）：账户头部、侧栏导航、概览数据、初始化。
 */
export function usePersonalCenter(sectionGetter: () => PersonalSection) {
  const router = useRouter()
  const { t, locale } = useI18n()
  const appStore = useAppStore()
  const userProfileStore = useUserProfileStore()

  const sectionItems: PersonalSectionItem[] = [
    { key: 'overview', label: 'personalCenter.tabs.overview', icon: Home },
    { key: 'orders', label: 'personalCenter.tabs.orders', icon: ShoppingBag },
    { key: 'wallet', label: 'personalCenter.tabs.wallet', icon: Wallet },
    { key: 'affiliate', label: 'personalCenter.tabs.affiliate', icon: Megaphone },
    { key: 'reseller', label: 'personalCenter.tabs.reseller', icon: Banknote },
    { key: 'giftCard', label: 'personalCenter.tabs.giftCard', icon: Gift },
    { key: 'security', label: 'personalCenter.tabs.security', icon: ShieldCheck },
    { key: 'api', label: 'personalCenter.tabs.api', icon: Key },
    { key: 'profile', label: 'personalCenter.tabs.profile', icon: UserCircle },
  ]

  const sectionRouteMap: Record<PersonalSection, string> = {
    overview: '/me',
    profile: '/me/profile',
    security: '/me/security',
    orders: '/me/orders',
    wallet: '/me/wallet',
    affiliate: '/me/affiliate',
    reseller: '/me/reseller',
    giftCard: '/me/gift-cards',
    api: '/me/api',
  }

  const canAccessResellerConsole = computed(() => appStore.canAccessResellerConsole)
  const visibleSectionItems = computed(() => {
    return sectionItems.filter((item) => item.key !== 'reseller' || canAccessResellerConsole.value)
  })
  const currentSection = computed<PersonalSection>(() => {
    if (sectionGetter() === 'reseller' && !canAccessResellerConsole.value) {
      return 'overview'
    }
    return sectionGetter()
  })
  const globalAlert = ref<PageAlert | null>(null)

  const displayInitial = computed(() => {
    const name = userProfileStore.displayName || ''
    const normalized = name.trim()
    if (!normalized) return 'U'
    return normalized.slice(0, 1).toUpperCase()
  })

  const switchSection = (section: PersonalSection) => {
    router.push(sectionRouteMap[section])
  }

  const statusLabel = (status?: string) => orderStatusLabel(t, status)
  const statusVariant = (status?: string) => orderStatusVariant(status)

  // vault 模板：BadgeTone → vault pill 类名（classic 不使用）
  const statusPillClass = (status?: string) => {
    const map: Record<string, string> = {
      success: 'pill-done',
      warning: 'pill-low',
      danger: 'pill-sale',
      info: 'pill-stock',
      accent: 'pill-sale',
      neutral: 'pill-out',
    }
    return map[orderStatusVariant(status)] || 'pill-out'
  }

  const formatMoney = (amount?: string, currency?: string) => {
    if (!amount) return '-'
    if (!currency) return amount
    return `${amount} ${currency}`
  }

  const formatDate = (raw?: string) => {
    if (!raw) return '-'
    const date = new Date(raw)
    if (Number.isNaN(date.getTime())) return raw
    return date.toLocaleString()
  }

  const emailVerifiedLabel = computed(() => {
    if (userProfileStore.profile?.email_verified_at) {
      return t('personalCenter.overview.emailVerified')
    }
    return t('personalCenter.overview.emailUnverified')
  })

  const emailVerifiedVariant = computed<'success' | 'warning'>(() => {
    return userProfileStore.profile?.email_verified_at ? 'success' : 'warning'
  })

  const discountText = computed(() => {
    const lvl = userProfileStore.currentLevel
    if (lvl && lvl.discount_rate < 100) {
      return t('personalCenter.memberLevel.discountOff', { n: lvl.discount_rate / 10 })
    }
    return t('personalCenter.memberLevel.noDiscount')
  })

  const isImagePath = (icon: string | undefined | null) => icon?.startsWith('/uploads/') || icon?.startsWith('http')

  const levelName = (level: PublicMemberLevel | null | undefined) => {
    if (!level) return t('personalCenter.memberLevel.defaultLevel')
    const loc = locale.value as string
    return level.name[loc] || level.name['zh-CN'] || level.name['en'] || level.slug || t('personalCenter.memberLevel.defaultLevel')
  }

  // 我的专属价：独立于会员等级，没有任何专属价时列表为空，由模板决定不渲染。
  const { formatPrice, siteCurrency } = useLocalized()
  const exclusivePriceItems = computed(() => {
    const loc = locale.value as string
    return userProfileStore.myProductPrices.map((row) => {
      const titleMap = row.product_title || {}
      const title = titleMap[loc] || titleMap['zh-CN'] || titleMap['en-US'] || titleMap['zh-TW'] || Object.values(titleMap)[0] || ''
      const skuText = row.sku_id > 0
        ? formatSkuSpecValues(row.sku_spec_values, loc) || row.sku_code || ''
        : ''
      const active = row.product_active === true && !!row.product_slug
      return {
        key: `${row.product_id}-${row.sku_id}`,
        title: title || `#${row.product_id}`,
        skuText,
        priceText: formatPrice(row.price_amount, siteCurrency.value),
        to: active ? `/products/${row.product_slug}` : '',
        unavailable: !active,
      }
    })
  })

  // 概览只先展示前 N 条，其余点击展开，避免专属价很多时把最近订单挤到页面底部。
  const EXCLUSIVE_PREVIEW_COUNT = 5
  const exclusivePriceExpanded = ref(false)
  const hasMoreExclusivePrices = computed(() => exclusivePriceItems.value.length > EXCLUSIVE_PREVIEW_COUNT)
  const visibleExclusivePriceItems = computed(() => {
    if (exclusivePriceExpanded.value || !hasMoreExclusivePrices.value) return exclusivePriceItems.value
    return exclusivePriceItems.value.slice(0, EXCLUSIVE_PREVIEW_COUNT)
  })
  const toggleExclusivePrices = () => {
    exclusivePriceExpanded.value = !exclusivePriceExpanded.value
  }

  const initialize = async () => {
    globalAlert.value = null
    const [profileOk] = await Promise.all([
      userProfileStore.loadProfile(),
      userProfileStore.loadRecentOrders(5),
      userProfileStore.loadMemberLevels(),
      userProfileStore.loadMyProductPrices(true),
    ])
    if (!profileOk) {
      globalAlert.value = {
        level: 'error',
        message: userProfileStore.profileError || t('personalCenter.common.loadFailed'),
      }
    }
  }

  onMounted(() => {
    initialize()
  })

  return {
    userProfileStore,
    canAccessResellerConsole,
    visibleSectionItems,
    currentSection,
    globalAlert,
    displayInitial,
    switchSection,
    statusLabel,
    statusVariant,
    statusPillClass,
    formatMoney,
    formatDate,
    emailVerifiedLabel,
    emailVerifiedVariant,
    discountText,
    isImagePath,
    levelName,
    exclusivePriceItems,
    visibleExclusivePriceItems,
    hasMoreExclusivePrices,
    exclusivePriceExpanded,
    toggleExclusivePrices,
  }
}
