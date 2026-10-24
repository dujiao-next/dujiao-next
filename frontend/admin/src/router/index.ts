import { createRouter, createWebHistory } from 'vue-router'
import { useAdminAuthStore } from '@/stores/auth'
import { ADMIN_BASE } from '@/utils/adminBase'

const routes = [
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/Login.vue'),
  },
  {
    path: '/',
    name: 'dashboard',
    component: () => import('@/layouts/AdminLayout.vue'),
    meta: { requiresAuth: true },
    children: [
      {
        path: '',
        name: 'dashboard-home',
        component: () => import('@/views/Dashboard.vue'),
      },
      {
        path: 'forbidden',
        name: 'forbidden',
        component: () => import('@/views/Forbidden.vue'),
      },
      {
        path: 'products',
        name: 'products',
        component: () => import('@/views/admin/Products.vue'),
        meta: { permission: 'GET:/admin/products' },
      },
      {
        path: 'categories',
        name: 'categories',
        component: () => import('@/views/admin/Categories.vue'),
        meta: { permission: 'GET:/admin/categories' },
      },
      {
        path: 'card-secrets',
        name: 'card-secrets',
        component: () => import('@/views/admin/CardSecrets.vue'),
        meta: { permission: 'GET:/admin/card-secrets' },
      },
      {
        path: 'card-secret-imports',
        name: 'card-secret-imports',
        component: () => import('@/views/admin/CardSecretImports.vue'),
        meta: { permission: 'GET:/admin/card-secrets' },
      },
      {
        path: 'card-secret-exports',
        name: 'card-secret-exports',
        component: () => import('@/views/admin/CardSecretExports.vue'),
        meta: { permission: 'GET:/admin/card-secrets' },
      },
      {
        path: 'gift-cards',
        name: 'gift-cards',
        component: () => import('@/views/admin/GiftCards.vue'),
        meta: { permission: 'GET:/admin/gift-cards' },
      },
      {
        path: 'invite-codes',
        name: 'invite-codes',
        component: () => import('@/views/admin/InviteCodes.vue'),
        meta: { permission: 'GET:/admin/invite-codes' },
      },
      {
        path: 'orders',
        name: 'orders',
        component: () => import('@/views/admin/Orders.vue'),
        meta: { permission: 'GET:/admin/orders' },
      },
      {
        path: 'order-risk-control',
        name: 'order-risk-control',
        component: () => import('@/views/admin/OrderRiskControl.vue'),
        meta: { permission: 'GET:/admin/settings' },
      },
      {
        path: 'order-refunds',
        name: 'order-refunds',
        component: () => import('@/views/admin/OrderRefunds.vue'),
        meta: { permission: 'GET:/admin/order-refunds' },
      },
      {
        path: 'payments',
        name: 'payments',
        component: () => import('@/views/admin/Payments.vue'),
        meta: { permission: 'GET:/admin/payments' },
      },
      {
        path: 'wallet-recharges',
        name: 'wallet-recharges',
        component: () => import('@/views/admin/WalletRecharges.vue'),
        meta: { permission: 'GET:/admin/wallet/recharges' },
      },
      {
        path: 'wallet-config',
        name: 'wallet-config',
        component: () => import('@/views/admin/Wallet.vue'),
        meta: { permission: 'GET:/admin/settings' },
      },
      {
        path: 'payment-channels',
        name: 'payment-channels',
        component: () => import('@/views/admin/PaymentChannels.vue'),
        meta: { permission: 'GET:/admin/payment-channels' },
      },
      {
        path: 'callback-routes',
        name: 'callback-routes',
        component: () => import('@/views/admin/CallbackRoutes.vue'),
        meta: { permission: 'GET:/admin/settings' },
      },
      {
        path: 'users',
        name: 'users',
        component: () => import('@/views/admin/Users.vue'),
        meta: { permission: 'GET:/admin/users' },
      },
      {
        path: 'user-login-logs',
        name: 'user-login-logs',
        component: () => import('@/views/admin/UserLoginLogs.vue'),
        meta: { permission: 'GET:/admin/user-login-logs' },
      },
      {
        path: 'users/:id',
        name: 'user-detail',
        component: () => import('@/views/admin/UserDetail.vue'),
        meta: { permission: 'GET:/admin/users/:id' },
      },
      {
        path: 'posts/categories',
        name: 'postCategories',
        component: () => import('@/views/admin/PostCategories.vue'),
        meta: { permission: 'GET:/admin/post-categories' },
      },
      {
        path: 'posts',
        redirect: '/posts/blog',
      },
      {
        path: 'posts/:type(blog|notice)',
        name: 'posts',
        component: () => import('@/views/admin/Posts.vue'),
        meta: { permission: 'GET:/admin/posts' },
      },
      {
        path: 'banners',
        name: 'banners',
        component: () => import('@/views/admin/Banners.vue'),
        meta: { permission: 'GET:/admin/banners' },
      },
      {
        path: 'media',
        name: 'media',
        component: () => import('@/views/admin/Media.vue'),
        meta: { permission: 'GET:/admin/media' },
      },
      {
        path: 'coupons',
        name: 'coupons',
        component: () => import('@/views/admin/Coupons.vue'),
        meta: { permission: 'GET:/admin/coupons' },
      },
      {
        path: 'promotions',
        name: 'promotions',
        component: () => import('@/views/admin/Promotions.vue'),
        meta: { permission: 'GET:/admin/promotions' },
      },
      {
        path: 'wholesale-prices',
        name: 'wholesale-prices',
        component: () => import('@/views/admin/WholesalePrices.vue'),
        meta: { permission: 'GET:/admin/products' },
      },
      {
        path: 'member-levels',
        name: 'member-levels',
        component: () => import('@/views/admin/MemberLevels.vue'),
        meta: { permission: 'GET:/admin/member-levels' },
      },
      {
        path: 'settings',
        name: 'settings',
        component: () => import('@/views/admin/Settings.vue'),
        meta: { permission: 'GET:/admin/settings' },
      },
      {
        path: 'settings/notifications',
        name: 'notifications',
        component: () => import('@/views/admin/Notifications.vue'),
        meta: { permission: 'GET:/admin/settings/notification-center' },
      },
      {
        path: 'security',
        name: 'security',
        component: () => import('@/views/admin/Security.vue'),
      },
      {
        path: 'affiliates/settings',
        name: 'affiliates-settings',
        component: () => import('@/views/admin/AffiliateSettings.vue'),
        meta: { permission: 'GET:/admin/settings/affiliate' },
      },
      {
        path: 'affiliates/users',
        name: 'affiliates-users',
        component: () => import('@/views/admin/AffiliateUsers.vue'),
        meta: { permission: 'GET:/admin/affiliates/users' },
      },
      {
        path: 'affiliates/commissions',
        name: 'affiliates-commissions',
        component: () => import('@/views/admin/AffiliateCommissions.vue'),
        meta: { permission: 'GET:/admin/affiliates/commissions' },
      },
      {
        path: 'affiliates/withdraws',
        name: 'affiliates-withdraws',
        component: () => import('@/views/admin/AffiliateWithdraws.vue'),
        meta: { permission: 'GET:/admin/affiliates/withdraws' },
      },
      {
        path: 'resellers/operations',
        name: 'resellers-operations',
        component: () => import('@/views/admin/ResellerOperationsDashboard.vue'),
        meta: { permission: 'GET:/admin/resellers/operations/overview' },
      },
      {
        path: 'resellers/profiles',
        name: 'resellers-profiles',
        component: () => import('@/views/admin/ResellerProfiles.vue'),
        meta: { permission: 'GET:/admin/resellers/profiles' },
      },
      {
        path: 'resellers/profiles/:id',
        name: 'resellers-profile-detail',
        component: () => import('@/views/admin/ResellerProfileDetail.vue'),
        meta: { permission: 'GET:/admin/resellers/profiles/:id' },
      },
      {
        path: 'resellers/domains',
        name: 'resellers-domains',
        component: () => import('@/views/admin/ResellerDomains.vue'),
        meta: { permission: 'GET:/admin/resellers/domains' },
      },
      {
        path: 'resellers/site-configs',
        name: 'resellers-site-configs',
        component: () => import('@/views/admin/ResellerSiteConfigs.vue'),
        meta: { permission: 'GET:/admin/resellers/site-configs' },
      },
      {
        path: 'resellers/product-settings',
        name: 'resellers-product-settings',
        component: () => import('@/views/admin/ResellerProductSettings.vue'),
        meta: { permission: 'GET:/admin/resellers/product-settings' },
      },
      {
        path: 'resellers/ledger-entries',
        name: 'resellers-ledger-entries',
        component: () => import('@/views/admin/ResellerLedgerEntries.vue'),
        meta: { permission: 'GET:/admin/resellers/ledger-entries' },
      },
      {
        path: 'resellers/balance-accounts',
        name: 'resellers-balance-accounts',
        component: () => import('@/views/admin/ResellerBalanceAccounts.vue'),
        meta: { permission: 'GET:/admin/resellers/balance-accounts' },
      },
      {
        path: 'resellers/withdraws',
        name: 'resellers-withdraws',
        component: () => import('@/views/admin/ResellerWithdraws.vue'),
        meta: { permission: 'GET:/admin/resellers/withdraws' },
      },
      {
        path: 'authz',
        name: 'authz',
        component: () => import('@/views/admin/Authz.vue'),
        meta: { permission: 'GET:/admin/authz/roles' },
      },
      {
        path: 'authz-audit-logs',
        name: 'authz-audit-logs',
        component: () => import('@/views/admin/AuthzAuditLogs.vue'),
        meta: { permission: 'GET:/admin/authz/audit-logs' },
      },
      {
        path: 'site-connections',
        name: 'site-connections',
        component: () => import('@/views/admin/SiteConnections.vue'),
        meta: { permission: 'GET:/admin/site-connections' },
      },
      {
        path: 'product-mappings',
        name: 'product-mappings',
        component: () => import('@/views/admin/ProductMappings.vue'),
        meta: { permission: 'GET:/admin/product-mappings' },
      },
      {
        path: 'procurement-orders',
        name: 'procurement-orders',
        component: () => import('@/views/admin/ProcurementOrders.vue'),
        meta: { permission: 'GET:/admin/procurement-orders' },
      },
      {
        path: 'reconciliation',
        name: 'reconciliation',
        component: () => import('@/views/admin/Reconciliation.vue'),
        meta: { permission: 'GET:/admin/reconciliation/jobs' },
      },
      {
        path: 'api-credentials',
        name: 'api-credentials',
        component: () => import('@/views/admin/ApiCredentials.vue'),
        meta: { permission: 'GET:/admin/api-credentials' },
      },
      {
        path: 'telegram-bot',
        name: 'telegram-bot',
        component: () => import('@/views/admin/TelegramBot.vue'),
        meta: { permission: 'GET:/admin/settings/telegram-bot' },
      },
      {
        path: 'telegram-bot/settings',
        name: 'telegram-bot-settings',
        component: () => import('@/views/admin/TelegramBotSettings.vue'),
        meta: { permission: 'GET:/admin/settings/telegram-bot' },
      },
      {
        path: 'telegram-bot/help-center',
        name: 'telegram-bot-help-center',
        component: () => import('@/views/admin/TelegramBotHelpCenter.vue'),
        meta: { permission: 'GET:/admin/settings/telegram-bot' },
      },
      {
        path: 'telegram-bot/menu',
        name: 'telegram-bot-menu-settings',
        component: () => import('@/views/admin/TelegramBotMenuSettings.vue'),
        meta: { permission: 'GET:/admin/settings/telegram-bot' },
      },
      {
        path: 'telegram-bot/status',
        name: 'telegram-bot-status',
        component: () => import('@/views/admin/TelegramBotStatus.vue'),
        meta: { permission: 'GET:/admin/settings/telegram-bot' },
      },
      {
        path: 'telegram-bot/channel-clients',
        name: 'telegram-bot-channel-clients',
        component: () => import('@/views/admin/TelegramBotChannelClients.vue'),
        meta: { permission: 'GET:/admin/channel-clients' },
      },
      {
        path: 'telegram-bot/broadcasts',
        name: 'telegram-bot-broadcasts',
        component: () => import('@/views/admin/TelegramBotBroadcasts.vue'),
        meta: { permission: 'GET:/admin/telegram-bot/broadcasts' },
      },
      {
        path: 'telegram-bot/broadcasts/create',
        name: 'telegram-bot-broadcast-create',
        component: () => import('@/views/admin/TelegramBotBroadcastCreate.vue'),
        meta: { permission: 'GET:/admin/telegram-bot/broadcasts' },
      },
      {
        path: 'telegram-bot/broadcasts/:id',
        name: 'telegram-bot-broadcast-detail',
        component: () => import('@/views/admin/TelegramBotBroadcastDetail.vue'),
        meta: { permission: 'GET:/admin/telegram-bot/broadcasts' },
      },
      {
        path: 'compliance-required',
        name: 'compliance-required',
        component: () => import('@/views/admin/ComplianceRequired.vue'),
      },
    ],
  },
]

// base 由 utils/adminBase 统一解析：fullstack 模式下读后端注入的 <base href>，
// 否则退回构建期的 import.meta.env.BASE_URL。原生 <a href> 走同一份前缀（adminUrl）。
const router = createRouter({
  history: createWebHistory(ADMIN_BASE || '/'),
  routes,
})

const PAYMENT_PROTECTED_ROUTE_NAMES = new Set<string>([
  'payments',
  'payment-channels',
  'wallet-config',
  'wallet-recharges',
  'reconciliation',
  'affiliates-withdraws',
  'affiliates-commissions',
  'resellers-operations',
  'resellers-ledger-entries',
  'resellers-balance-accounts',
  'resellers-withdraws',
])

router.beforeEach(async (to) => {
  const authStore = useAdminAuthStore()

  if (to.meta.requiresAuth && !authStore.token) {
    return { path: '/login' }
  }

  if (to.path === '/login' && authStore.token) {
    return { path: '/' }
  }

  if (to.meta.requiresAuth && authStore.token && !authStore.permissionsLoaded) {
    try {
      await authStore.loadAuthz()
    } catch {
      authStore.logout()
      return { path: '/login' }
    }
  }

  const requiredPermission = typeof to.meta.permission === 'string' ? to.meta.permission : ''
  if (requiredPermission && !authStore.hasPermission(requiredPermission)) {
    if (to.path !== '/forbidden') {
      return {
        path: '/forbidden',
        query: {
          from: to.fullPath,
        },
      }
    }
  }

  // 合规声明拦截：受保护路由 + 非超管 + 未确认 → 跳转提示页
  if (PAYMENT_PROTECTED_ROUTE_NAMES.has(String(to.name))) {
    const { useComplianceStore } = await import('@/stores/compliance')
    const compliance = useComplianceStore()
    if (!compliance.loaded) {
      try {
        await compliance.fetchStatus()
      } catch {
        /* 服务异常时不阻塞导航，由页面 wrapper 兜底 */
      }
    }
    if (!compliance.acknowledged && !authStore.isSuper) {
      return { name: 'compliance-required' }
    }
    // 超管未确认：放行，由页面 wrapper 弹窗拦截
  }

  return true
})

export default router
