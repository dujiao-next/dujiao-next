<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, RouterView, RouterLink } from 'vue-router'
import {
  LayoutDashboard,
  LogOut,
  ShoppingBag,
  CreditCard,
  Package,
  Users,
  FileText,
  BadgePercent,
  Settings,
  FolderTree,
  Boxes,
  KeyRound,
  Download,
  ListOrdered,
  WalletCards,
  ReceiptText,
  UserRound,
  Wallet,
  History,
  Images,
  Newspaper,
  Ticket,
  Gift,
  SlidersHorizontal,
  ShieldCheck,
  ScrollText,
  ChevronDown,
  ChevronRight,
  ChevronsLeft,
  ChevronsRight,
  Home,
  Sun,
  Moon,
  Link,
  Truck,
  ClipboardCheck,
  Lock,
  Bot,
  Wifi,
  Send,
  Crown,
  Bell,
  ImageIcon,
} from 'lucide-vue-next'
import { Menu } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Sheet, SheetContent, SheetTitle } from '@/components/ui/sheet'
import { RefreshCw } from 'lucide-vue-next'
import SystemUpdateDialog from '@/components/SystemUpdateDialog.vue'
import { useAdminAuthStore } from '@/stores/auth'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { applySiteIcon } from '@/utils/favicon'
import { adminUrl } from '@/utils/adminBase'

interface NavGroupItem {
  label: string
  to: string
  permission?: string
  icon?: any
}

interface NavGroup {
  id: string
  label: string
  icon: any
  items: NavGroupItem[]
}

const NAV_GROUP_EXPANDED_STORAGE_KEY = 'admin_nav_group_expanded'
const SIDEBAR_COLLAPSED_STORAGE_KEY = 'admin_sidebar_collapsed'
const SIDEBAR_AUTO_COLLAPSE_WIDTH = 1280 // xl breakpoint

const readExpandedGroups = () => {
  if (typeof window === 'undefined') {
    return {}
  }
  const raw = localStorage.getItem(NAV_GROUP_EXPANDED_STORAGE_KEY)
  if (!raw) {
    return {}
  }
  try {
    const parsed = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object') {
      return {}
    }
    const result: Record<string, boolean> = {}
    Object.entries(parsed).forEach(([key, value]) => {
      if (typeof value === 'boolean') {
        result[key] = value
      }
    })
    return result
  } catch {
    return {}
  }
}

const { t, locale } = useI18n()
const route = useRoute()
const authStore = useAdminAuthStore()
const isDark = ref(false)
const appVersion = ref('')
const siteUrl = ref('')

// 版本检测与一键升级的完整交互都在 SystemUpdateDialog 内，这里只负责开关
const updateCheckOpen = ref(false)

const navSearch = ref('')
const expandedGroups = ref<Record<string, boolean>>(readExpandedGroups())
const mobileNavOpen = ref(false)
const sidebarCollapsed = ref(false)
const sidebarUserToggled = ref(false) // tracks if user manually toggled

// Close mobile nav on route change
watch(() => route.path, () => {
  mobileNavOpen.value = false
})

const navGroups = computed<NavGroup[]>(() => {
  const groups: NavGroup[] = [
    {
      id: 'products',
      label: t('admin.navGroups.productManagement'),
      icon: Package,
      items: [
        {
          label: t('admin.navItems.productCategories'),
          to: '/categories',
          icon: FolderTree,
          permission: 'GET:/admin/categories',
        },
        {
          label: t('admin.navItems.productList'),
          to: '/products',
          icon: Boxes,
          permission: 'GET:/admin/products',
        },
        {
          label: t('admin.navItems.cardSecrets'),
          to: '/card-secrets',
          icon: KeyRound,
          permission: 'GET:/admin/card-secrets',
        },
        {
          label: t('admin.navItems.cardSecretImports'),
          to: '/card-secret-imports',
          icon: KeyRound,
          permission: 'GET:/admin/card-secrets',
        },
        {
          label: t('admin.navItems.cardSecretExports'),
          to: '/card-secret-exports',
          icon: Download,
          permission: 'GET:/admin/card-secrets',
        },
      ],
    },
    {
      id: 'orders',
      label: t('admin.navGroups.orderManagement'),
      icon: ShoppingBag,
      items: [
        {
          label: t('admin.navItems.orderList'),
          to: '/orders',
          icon: ListOrdered,
          permission: 'GET:/admin/orders',
        },
        {
          label: t('admin.navItems.orderRiskControl'),
          to: '/order-risk-control',
          icon: ShieldCheck,
          permission: 'GET:/admin/settings',
        },
        {
          label: t('admin.navItems.orderRefunds'),
          to: '/order-refunds',
          icon: ReceiptText,
          permission: 'GET:/admin/order-refunds',
        },
      ],
    },
    {
      id: 'payments',
      label: t('admin.navGroups.paymentManagement'),
      icon: CreditCard,
      items: [
        {
          label: t('admin.navItems.paymentChannels'),
          to: '/payment-channels',
          icon: WalletCards,
          permission: 'GET:/admin/payment-channels',
        },
        {
          label: t('admin.navItems.payments'),
          to: '/payments',
          icon: ReceiptText,
          permission: 'GET:/admin/payments',
        },
        {
          label: t('admin.navItems.callbackRoutes'),
          to: '/callback-routes',
          icon: Link,
          permission: 'GET:/admin/settings',
        },
      ],
    },
    {
      id: 'users',
      label: t('admin.navGroups.userManagement'),
      icon: Users,
      items: [
        {
          label: t('admin.navItems.userList'),
          to: '/users',
          icon: UserRound,
          permission: 'GET:/admin/users',
        },
        {
          label: t('admin.navItems.walletManagement'),
          to: '/wallet-recharges',
          icon: Wallet,
          permission: 'GET:/admin/wallet/recharges',
        },
        {
          label: t('admin.navItems.walletConfig'),
          to: '/wallet-config',
          icon: Wallet,
          permission: 'GET:/admin/settings',
        },
        {
          label: t('admin.navItems.userLoginLogs'),
          to: '/user-login-logs',
          icon: History,
          permission: 'GET:/admin/user-login-logs',
        },
        {
          label: t('admin.navItems.memberLevels'),
          to: '/member-levels',
          icon: Crown,
          permission: 'GET:/admin/member-levels',
        },
      ],
    },
    {
      id: 'articles',
      label: t('admin.navGroups.articleManagement'),
      icon: Newspaper,
      items: [
        {
          label: t('admin.navItems.articleList'),
          to: '/posts/blog',
          icon: FileText,
          permission: 'GET:/admin/posts',
        },
        {
          label: t('admin.navItems.postCategories'),
          to: '/posts/categories',
          icon: FolderTree,
          permission: 'GET:/admin/post-categories',
        },
      ],
    },
    {
      id: 'content',
      label: t('admin.navGroups.contentManagement'),
      icon: FileText,
      items: [
        {
          label: t('admin.navItems.banners'),
          to: '/banners',
          icon: Images,
          permission: 'GET:/admin/banners',
        },
        {
          label: t('admin.navItems.media'),
          to: '/media',
          icon: ImageIcon,
          permission: 'GET:/admin/media',
        },
        {
          label: t('admin.navItems.announcementList'),
          to: '/posts/notice',
          icon: Bell,
          permission: 'GET:/admin/posts',
        },
      ],
    },
    {
      id: 'marketing',
      label: t('admin.navGroups.marketingManagement'),
      icon: BadgePercent,
      items: [
        {
          label: t('admin.navItems.coupons'),
          to: '/coupons',
          icon: Ticket,
          permission: 'GET:/admin/coupons',
        },
        {
          label: t('admin.navItems.promotions'),
          to: '/promotions',
          icon: BadgePercent,
          permission: 'GET:/admin/promotions',
        },
        {
          label: t('admin.navItems.wholesalePrices'),
          to: '/wholesale-prices',
          icon: BadgePercent,
          permission: 'GET:/admin/products',
        },
        {
          label: t('admin.navItems.giftCards'),
          to: '/gift-cards',
          icon: Gift,
          permission: 'GET:/admin/gift-cards',
        },
      ],
    },
    {
      id: 'affiliate',
      label: t('admin.navGroups.affiliateManagement'),
      icon: BadgePercent,
      items: [
        {
          label: t('admin.navItems.affiliatesSettings'),
          to: '/affiliates/settings',
          icon: SlidersHorizontal,
          permission: 'GET:/admin/settings/affiliate',
        },
        {
          label: t('admin.navItems.affiliatesUsers'),
          to: '/affiliates/users',
          icon: Users,
          permission: 'GET:/admin/affiliates/users',
        },
        {
          label: t('admin.navItems.affiliatesCommissions'),
          to: '/affiliates/commissions',
          icon: ReceiptText,
          permission: 'GET:/admin/affiliates/commissions',
        },
        {
          label: t('admin.navItems.affiliatesWithdraws'),
          to: '/affiliates/withdraws',
          icon: WalletCards,
          permission: 'GET:/admin/affiliates/withdraws',
        },
      ],
    },
    {
      id: 'reseller',
      label: t('admin.navGroups.resellerManagement'),
      icon: Users,
      items: [
        {
          label: t('admin.navItems.resellerOperations'),
          to: '/resellers/operations',
          icon: LayoutDashboard,
          permission: 'GET:/admin/resellers/operations/overview',
        },
        {
          label: t('admin.navItems.resellerProfiles'),
          to: '/resellers/profiles',
          icon: Users,
          permission: 'GET:/admin/resellers/profiles',
        },
        {
          label: t('admin.navItems.resellerDomains'),
          to: '/resellers/domains',
          icon: Link,
          permission: 'GET:/admin/resellers/domains',
        },
        {
          label: t('admin.navItems.resellerSiteConfigs'),
          to: '/resellers/site-configs',
          icon: Settings,
          permission: 'GET:/admin/resellers/site-configs',
        },
        {
          label: t('admin.navItems.resellerProductSettings'),
          to: '/resellers/product-settings',
          icon: SlidersHorizontal,
          permission: 'GET:/admin/resellers/product-settings',
        },
        {
          label: t('admin.navItems.resellerLedgerEntries'),
          to: '/resellers/ledger-entries',
          icon: ReceiptText,
          permission: 'GET:/admin/resellers/ledger-entries',
        },
        {
          label: t('admin.navItems.resellerBalanceAccounts'),
          to: '/resellers/balance-accounts',
          icon: Wallet,
          permission: 'GET:/admin/resellers/balance-accounts',
        },
        {
          label: t('admin.navItems.resellerWithdraws'),
          to: '/resellers/withdraws',
          icon: WalletCards,
          permission: 'GET:/admin/resellers/withdraws',
        },
      ],
    },
    {
      id: 'integration',
      label: t('admin.navGroups.integrationManagement'),
      icon: Link,
      items: [
        {
          label: t('admin.navItems.siteConnections'),
          to: '/site-connections',
          icon: Link,
          permission: 'GET:/admin/site-connections',
        },
        {
          label: t('admin.navItems.productMappings'),
          to: '/product-mappings',
          icon: Boxes,
          permission: 'GET:/admin/product-mappings',
        },
        {
          label: t('admin.navItems.procurementOrders'),
          to: '/procurement-orders',
          icon: Truck,
          permission: 'GET:/admin/procurement-orders',
        },
        {
          label: t('admin.navItems.reconciliation'),
          to: '/reconciliation',
          icon: ClipboardCheck,
          permission: 'GET:/admin/reconciliation/jobs',
        },
        {
          label: t('admin.navItems.apiCredentials'),
          to: '/api-credentials',
          icon: KeyRound,
          permission: 'GET:/admin/api-credentials',
        },
      ],
    },
    {
      id: 'telegramBot',
      label: t('admin.navGroups.telegramBot'),
      icon: Bot,
      items: [
        {
          label: t('admin.navItems.telegramBotOverview'),
          to: '/telegram-bot',
          icon: Bot,
          permission: 'GET:/admin/settings/telegram-bot',
        },
        {
          label: t('admin.navItems.telegramBotSettings'),
          to: '/telegram-bot/settings',
          icon: SlidersHorizontal,
          permission: 'GET:/admin/settings/telegram-bot',
        },
        {
          label: t('admin.navItems.telegramBotHelpCenter'),
          to: '/telegram-bot/help-center',
          icon: ScrollText,
          permission: 'GET:/admin/settings/telegram-bot',
        },
        {
          label: t('admin.navItems.telegramBotMenuSettings'),
          to: '/telegram-bot/menu',
          icon: ListOrdered,
          permission: 'GET:/admin/settings/telegram-bot',
        },
        {
          label: t('admin.navItems.telegramBotStatus'),
          to: '/telegram-bot/status',
          icon: Wifi,
          permission: 'GET:/admin/settings/telegram-bot',
        },
        {
          label: t('admin.navItems.telegramBotChannelClients'),
          to: '/telegram-bot/channel-clients',
          icon: KeyRound,
          permission: 'GET:/admin/channel-clients',
        },
        {
          label: t('admin.navItems.telegramBotBroadcasts'),
          to: '/telegram-bot/broadcasts',
          icon: Send,
          permission: 'GET:/admin/telegram-bot/broadcasts',
        },
      ],
    },
    {
      id: 'system',
      label: t('admin.navGroups.systemSettings'),
      icon: Settings,
      items: [
        {
          label: t('admin.navItems.siteSettings'),
          to: '/settings',
          icon: SlidersHorizontal,
          permission: 'GET:/admin/settings',
        },
        {
          label: t('admin.navItems.notificationCenter'),
          to: '/settings/notifications',
          icon: Bell,
          permission: 'GET:/admin/settings/notification-center',
        },
        {
          label: t('admin.navItems.authz'),
          to: '/authz',
          icon: ShieldCheck,
          permission: 'GET:/admin/authz/roles',
        },
        {
          label: t('admin.navItems.authzAudit'),
          to: '/authz-audit-logs',
          icon: ScrollText,
          permission: 'GET:/admin/authz/audit-logs',
        },
        {
          label: t('admin.navItems.security'),
          to: '/security',
          icon: Lock,
        },
      ],
    },
  ]

  return groups
    .map((group) => ({
      ...group,
      items: group.items.filter((item) => authStore.hasPermission(item.permission)),
    }))
    .filter((group) => group.items.length > 0)
})

watch(
  navGroups,
  (groups) => {
    const next: Record<string, boolean> = {}
    groups.forEach((group) => {
      next[group.id] = expandedGroups.value[group.id] ?? true
    })
    expandedGroups.value = next
  },
  { immediate: true }
)

watch(
  expandedGroups,
  (value) => {
    if (typeof window === 'undefined') {
      return
    }
    localStorage.setItem(NAV_GROUP_EXPANDED_STORAGE_KEY, JSON.stringify(value))
  },
  { deep: true }
)

const normalizedSearch = computed(() => navSearch.value.trim().toLowerCase())
const dashboardNavLabel = computed(() => t('admin.navItems.dashboard'))

const showDashboardNav = computed(() => {
  const keyword = normalizedSearch.value
  if (!keyword) {
    return true
  }
  const label = dashboardNavLabel.value.toLowerCase()
  const groupLabel = t('admin.navGroups.dashboard').toLowerCase()
  return label.includes(keyword) || groupLabel.includes(keyword)
})

const filteredNavGroups = computed<NavGroup[]>(() => {
  const keyword = normalizedSearch.value
  if (!keyword) {
    return navGroups.value
  }
  const result: NavGroup[] = []
  navGroups.value.forEach((group) => {
    const groupMatched = group.label.toLowerCase().includes(keyword)
    const matchedItems = groupMatched
      ? group.items
      : group.items.filter((item) => item.label.toLowerCase().includes(keyword))
    if (matchedItems.length > 0) {
      result.push({
        ...group,
        items: matchedItems,
      })
    }
  })
  return result
})

const isGroupExpanded = (groupID: string) => {
  if (normalizedSearch.value) {
    return true
  }
  return expandedGroups.value[groupID] !== false
}

const toggleGroup = (groupID: string) => {
  expandedGroups.value[groupID] = !isGroupExpanded(groupID)
}

const splitNavTarget = (to: string) => {
  const [path, query = ''] = to.split('?')
  return { path, query }
}

const navQueryMatches = (query: string) => {
  if (!query) return true
  const params = new URLSearchParams(query)
  for (const [key, value] of params.entries()) {
    const routeValue = route.query[key]
    const normalized = Array.isArray(routeValue) ? routeValue[0] : routeValue
    if (String(normalized ?? '') !== value) return false
  }
  return true
}

const isItemActive = (to: string) => {
  const target = splitNavTarget(to)
  if (target.path === '/') {
    return route.path === '/'
  }
  if (target.query) {
    return route.path === target.path && navQueryMatches(target.query)
  }
  // 精确匹配优先：如果当前路由被其他更具体的导航项精确匹配，则前缀匹配项不应高亮
  if (route.path === target.path) {
    for (const group of navGroups.value) {
      for (const item of group.items) {
        const other = splitNavTarget(item.to)
        if (item.to !== to && other.path === route.path && other.query && navQueryMatches(other.query)) {
          return false
        }
      }
    }
    return true
  }
  if (!route.path.startsWith(`${target.path}/`)) return false
  // 前缀匹配时，检查是否有其他导航项精确匹配当前路由
  for (const group of navGroups.value) {
    for (const item of group.items) {
      const other = splitNavTarget(item.to)
      if (item.to !== to && route.path === other.path && (!other.query || navQueryMatches(other.query))) return false
    }
  }
  return true
}

const applyTheme = (theme: 'light' | 'dark') => {
  isDark.value = theme === 'dark'
  const root = document.documentElement
  if (theme === 'dark') {
    root.classList.add('dark')
  } else {
    root.classList.remove('dark')
  }
  localStorage.setItem('admin_theme', theme)
}

const toggleTheme = () => {
  applyTheme(isDark.value ? 'light' : 'dark')
}

const applyLocale = (value: string) => {
  locale.value = value
  localStorage.setItem('admin_locale', value)
}

const preventMobileNavAutoFocus = (event: Event) => {
  event.preventDefault()
}

const toggleSidebar = () => {
  sidebarCollapsed.value = !sidebarCollapsed.value
  sidebarUserToggled.value = true
  localStorage.setItem(SIDEBAR_COLLAPSED_STORAGE_KEY, String(sidebarCollapsed.value))
}

const handleResize = () => {
  if (sidebarUserToggled.value) return
  sidebarCollapsed.value = window.innerWidth < SIDEBAR_AUTO_COLLAPSE_WIDTH
}

const handleLogout = () => {
  authStore.logout()
  // 必须走 adminUrl()：后台可被挂到运行时任意 web.admin_path 前缀下，
  // 硬编码 '/login' 会跳出后台落到用户前台 SPA 的兜底路由上。
  window.location.href = adminUrl('/login')
}

onMounted(() => {
  const savedTheme = localStorage.getItem('admin_theme')
  if (savedTheme === 'light' || savedTheme === 'dark') {
    applyTheme(savedTheme)
  } else {
    const prefersDark = window.matchMedia?.('(prefers-color-scheme: dark)').matches
    applyTheme(prefersDark ? 'dark' : 'light')
  }

  const savedLocale = localStorage.getItem('admin_locale')
  if (savedLocale) {
    applyLocale(savedLocale)
  }

  // Initialize sidebar collapse state
  const savedCollapsed = localStorage.getItem(SIDEBAR_COLLAPSED_STORAGE_KEY)
  if (savedCollapsed !== null) {
    sidebarCollapsed.value = savedCollapsed === 'true'
    sidebarUserToggled.value = true
  } else {
    // Auto-collapse on medium screens
    sidebarCollapsed.value = window.innerWidth < SIDEBAR_AUTO_COLLAPSE_WIDTH
  }
  window.addEventListener('resize', handleResize)

  // Fetch app version and site URL
  adminAPI.getPublicConfig().then((res) => {
    const payload = res.data?.data
    applySiteIcon(payload?.brand?.site_icon)
    const ver = payload?.app_version
    if (typeof ver === 'string') {
      appVersion.value = ver
    }
    const url = payload?.brand?.site_url
    if (typeof url === 'string' && url) {
      siteUrl.value = url
    }
  }).catch(() => {})
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize)
})
</script>

<template>
  <div class="min-h-screen bg-background text-foreground">
    <div class="flex min-h-screen">
      <!-- Desktop sidebar -->
      <aside
        class="hidden md:flex border-r border-border bg-card flex-col sticky top-0 h-screen overflow-y-auto transition-all duration-300"
        :class="sidebarCollapsed ? 'w-16' : 'w-64'"
      >
        <div class="py-6" :class="sidebarCollapsed ? 'px-2' : 'px-6'">
          <div v-if="!sidebarCollapsed" class="text-xl font-semibold tracking-tight">
            {{ t('admin.brand') }}
          </div>
          <div v-if="!sidebarCollapsed" class="text-xs text-muted-foreground mt-1">{{ t('admin.layout.controlRoom') }}</div>
          <div v-if="sidebarCollapsed" class="text-lg font-semibold tracking-tight text-center">D&J</div>
        </div>
        <div v-if="!sidebarCollapsed" class="px-3 pb-2">
          <Input
            v-model="navSearch"
            class="h-8 text-xs"
            :placeholder="t('admin.navSearch.placeholder')"
          />
        </div>
        <nav class="pb-3 space-y-2 flex-1 overflow-y-auto" :class="sidebarCollapsed ? 'px-1.5' : 'px-3'">
          <!-- Expanded mode -->
          <template v-if="!sidebarCollapsed">
            <RouterLink
              v-if="showDashboardNav"
              to="/"
              class="flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors"
              :class="isItemActive('/') ? 'bg-secondary text-foreground' : 'text-muted-foreground hover:text-foreground hover:bg-secondary/70'"
            >
              <LayoutDashboard class="h-4 w-4 shrink-0" />
              <span>{{ dashboardNavLabel }}</span>
            </RouterLink>
            <div
              v-if="!showDashboardNav && filteredNavGroups.length === 0"
              class="rounded-lg border border-dashed border-border px-3 py-4 text-xs text-muted-foreground"
            >
              {{ t('admin.navSearch.empty') }}
            </div>
            <div v-for="group in filteredNavGroups" :key="group.id" class="space-y-1">
              <button
                type="button"
                class="flex w-full items-center justify-between rounded-lg px-3 py-2 text-sm transition-colors hover:bg-secondary/70"
                :class="isGroupExpanded(group.id) ? 'bg-secondary/40 text-foreground' : 'text-foreground'"
                @click="toggleGroup(group.id)"
              >
                <div class="flex min-w-0 items-center gap-3">
                  <component :is="group.icon" class="h-4 w-4 shrink-0" />
                  <span class="truncate">{{ group.label }}</span>
                </div>
                <component
                  :is="isGroupExpanded(group.id) ? ChevronDown : ChevronRight"
                  class="h-4 w-4 shrink-0 text-muted-foreground"
                />
              </button>
              <div v-show="isGroupExpanded(group.id)" class="space-y-1 pl-9">
                <RouterLink
                  v-for="item in group.items"
                  :key="`${group.id}-${item.to}`"
                  :to="item.to"
                  class="flex items-center gap-2 rounded-lg px-3 py-2 text-sm transition-colors"
                  :class="isItemActive(item.to) ? 'bg-secondary text-foreground' : 'text-muted-foreground hover:bg-secondary/70 hover:text-foreground'"
                >
                  <component v-if="item.icon" :is="item.icon" class="h-3.5 w-3.5 shrink-0" />
                  <span class="truncate">{{ item.label }}</span>
                </RouterLink>
              </div>
            </div>
          </template>
          <!-- Collapsed mode: icon-only -->
          <template v-else>
            <RouterLink
              to="/"
              class="flex items-center justify-center rounded-lg p-2 transition-colors"
              :class="isItemActive('/') ? 'bg-secondary text-foreground' : 'text-muted-foreground hover:text-foreground hover:bg-secondary/70'"
              :title="dashboardNavLabel"
            >
              <LayoutDashboard class="h-4 w-4 shrink-0" />
            </RouterLink>
            <template v-for="group in navGroups" :key="`collapsed-${group.id}`">
              <div class="my-1 border-t border-border/50" />
              <RouterLink
                v-for="item in group.items"
                :key="`collapsed-${group.id}-${item.to}`"
                :to="item.to"
                class="flex items-center justify-center rounded-lg p-2 transition-colors"
                :class="isItemActive(item.to) ? 'bg-secondary text-foreground' : 'text-muted-foreground hover:text-foreground hover:bg-secondary/70'"
                :title="item.label"
              >
                <component v-if="item.icon" :is="item.icon" class="h-4 w-4 shrink-0" />
              </RouterLink>
            </template>
          </template>
        </nav>
        <!-- Collapse toggle button -->
        <div class="border-t border-border">
          <div v-if="!sidebarCollapsed" class="px-6 py-3 text-[11px] text-muted-foreground space-y-1">
            <p>© {{ new Date().getFullYear() }} Dujiao-Next <span v-if="appVersion" class="text-muted-foreground/70">{{ appVersion }}</span></p>
            <a
              href="https://github.com/dujiao-next"
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex items-center gap-1 underline-offset-2 hover:underline"
            >
              <svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
                <path d="M12 .5C5.648.5.5 5.648.5 12c0 5.084 3.292 9.4 7.86 10.922.575.106.784-.25.784-.556 0-.273-.01-1-.016-1.962-3.197.694-3.872-1.54-3.872-1.54-.522-1.326-1.274-1.678-1.274-1.678-1.042-.713.079-.699.079-.699 1.152.081 1.758 1.183 1.758 1.183 1.024 1.755 2.688 1.248 3.343.954.104-.742.401-1.248.73-1.535-2.552-.29-5.236-1.276-5.236-5.678 0-1.254.448-2.28 1.182-3.084-.118-.29-.512-1.457.112-3.04 0 0 .964-.308 3.158 1.178a10.98 10.98 0 0 1 2.876-.387c.976.004 1.96.132 2.878.387 2.192-1.486 3.154-1.178 3.154-1.178.626 1.583.232 2.75.114 3.04.736.804 1.18 1.83 1.18 3.084 0 4.413-2.688 5.384-5.248 5.668.412.354.78 1.052.78 2.12 0 1.53-.014 2.764-.014 3.14 0 .31.206.668.79.554C20.212 21.396 23.5 17.083 23.5 12 23.5 5.648 18.352.5 12 .5Z" />
              </svg>
              <span>https://github.com/dujiao-next</span>
            </a>
          </div>
          <button
            type="button"
            class="flex w-full items-center justify-center py-3 text-muted-foreground hover:text-foreground hover:bg-secondary/70 transition-colors"
            @click="toggleSidebar"
            :title="sidebarCollapsed ? t('admin.layout.expandSidebar') : t('admin.layout.collapseSidebar')"
          >
            <ChevronsLeft v-if="!sidebarCollapsed" class="h-4 w-4" />
            <ChevronsRight v-else class="h-4 w-4" />
          </button>
        </div>
      </aside>

      <!-- Mobile sidebar (Sheet) -->
      <Sheet v-model:open="mobileNavOpen">
        <SheetContent
          v-if="mobileNavOpen"
          side="left"
          class="w-72 p-0 flex flex-col"
          style="animation: none; transform: translateX(0);"
          @open-auto-focus="preventMobileNavAutoFocus"
        >
          <SheetTitle class="sr-only">{{ t('admin.layout.navigation') }}</SheetTitle>
          <div class="px-6 py-6">
            <div class="text-xl font-semibold tracking-tight">
              {{ t('admin.brand') }}
            </div>
            <div class="text-xs text-muted-foreground mt-1">{{ t('admin.layout.controlRoom') }}</div>
          </div>
          <div class="px-3 pb-2">
            <Input
              v-model="navSearch"
              class="h-8 text-xs"
              :placeholder="t('admin.navSearch.placeholder')"
            />
          </div>
          <nav class="px-3 pb-3 space-y-2 flex-1 overflow-y-auto">
            <RouterLink
              v-if="showDashboardNav"
              to="/"
              @click="mobileNavOpen = false"
              class="flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors"
              :class="isItemActive('/') ? 'bg-secondary text-foreground' : 'text-muted-foreground hover:text-foreground hover:bg-secondary/70'"
            >
              <LayoutDashboard class="h-4 w-4 shrink-0" />
              <span>{{ dashboardNavLabel }}</span>
            </RouterLink>
            <div
              v-if="!showDashboardNav && filteredNavGroups.length === 0"
              class="rounded-lg border border-dashed border-border px-3 py-4 text-xs text-muted-foreground"
            >
              {{ t('admin.navSearch.empty') }}
            </div>
            <div v-for="group in filteredNavGroups" :key="`mobile-${group.id}`" class="space-y-1">
              <button
                type="button"
                class="flex w-full items-center justify-between rounded-lg px-3 py-2 text-sm transition-colors hover:bg-secondary/70"
                :class="isGroupExpanded(group.id) ? 'bg-secondary/40 text-foreground' : 'text-foreground'"
                @click="toggleGroup(group.id)"
              >
                <div class="flex min-w-0 items-center gap-3">
                  <component :is="group.icon" class="h-4 w-4 shrink-0" />
                  <span class="truncate">{{ group.label }}</span>
                </div>
                <component
                  :is="isGroupExpanded(group.id) ? ChevronDown : ChevronRight"
                  class="h-4 w-4 shrink-0 text-muted-foreground"
                />
              </button>
              <div v-show="isGroupExpanded(group.id)" class="space-y-1 pl-9">
                <RouterLink
                  v-for="item in group.items"
                  :key="`mobile-${group.id}-${item.to}`"
                  :to="item.to"
                  @click="mobileNavOpen = false"
                  class="flex items-center gap-2 rounded-lg px-3 py-2 text-sm transition-colors"
                  :class="isItemActive(item.to) ? 'bg-secondary text-foreground' : 'text-muted-foreground hover:bg-secondary/70 hover:text-foreground'"
                >
                  <component v-if="item.icon" :is="item.icon" class="h-3.5 w-3.5 shrink-0" />
                  <span class="truncate">{{ item.label }}</span>
                </RouterLink>
              </div>
            </div>
          </nav>
          <div class="px-6 py-4 border-t border-border text-[11px] text-muted-foreground space-y-1">
            <p>© {{ new Date().getFullYear() }} Dujiao-Next <span v-if="appVersion" class="text-muted-foreground/70">{{ appVersion }}</span></p>
          </div>
        </SheetContent>
      </Sheet>

      <div class="flex-1 flex flex-col min-w-0">
        <header class="flex items-center justify-between border-b border-border bg-background px-4 md:px-8 py-3 md:py-4 gap-2">
          <div class="flex items-center gap-3">
            <Button size="icon-sm" variant="ghost" class="md:hidden" @click="mobileNavOpen = true">
              <Menu class="h-5 w-5" />
            </Button>
            <Button
              size="icon-sm"
              variant="ghost"
              class="hidden md:inline-flex"
              @click="toggleSidebar"
              :title="sidebarCollapsed ? t('admin.layout.expandSidebar') : t('admin.layout.collapseSidebar')"
            >
              <ChevronsRight v-if="sidebarCollapsed" class="h-4 w-4" />
              <ChevronsLeft v-else class="h-4 w-4" />
            </Button>
            <div class="text-sm text-muted-foreground hidden sm:block">{{ t('admin.layout.workspace') }}</div>
          </div>
          <div class="flex items-center gap-2">
            <Select
              :model-value="locale"
              @update:modelValue="(value) => { if (value) applyLocale(String(value)) }"
            >
              <SelectTrigger class="h-8 w-[100px] md:w-[140px] text-xs">
                <SelectValue :placeholder="t('admin.common.lang.zhCN')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="zh-CN">{{ t('admin.common.lang.zhCN') }}</SelectItem>
                <SelectItem value="zh-TW">{{ t('admin.common.lang.zhTW') }}</SelectItem>
                <SelectItem value="en-US">{{ t('admin.common.lang.enUS') }}</SelectItem>
              </SelectContent>
            </Select>
            <Button
              size="icon-sm"
              variant="outline"
              :title="t('admin.updateCheck.button')"
              @click="updateCheckOpen = true"
            >
              <RefreshCw class="h-4 w-4" />
            </Button>
            <Button size="icon-sm" variant="outline" @click="toggleTheme">
              <Sun v-if="isDark" class="h-4 w-4" />
              <Moon v-else class="h-4 w-4" />
            </Button>
            <a v-if="siteUrl" :href="siteUrl" target="_blank" rel="noopener noreferrer">
              <Button size="icon-sm" variant="outline" :title="t('admin.layout.homePage')">
                <Home class="h-4 w-4" />
              </Button>
            </a>
            <Button variant="outline" size="sm" class="gap-2" @click="handleLogout">
              <LogOut class="h-4 w-4" />
              <span class="hidden sm:inline">{{ t('admin.common.logout') }}</span>
            </Button>
          </div>
        </header>
        <main class="min-w-0 flex-1 overflow-x-hidden px-4 py-4 md:px-8 md:py-6">
          <div class="min-w-0">
            <RouterView />
          </div>
        </main>
      </div>
    </div>

    <SystemUpdateDialog v-model:open="updateCheckOpen" />
  </div>
</template>
