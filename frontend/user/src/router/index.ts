import { createRouter, createWebHistory } from 'vue-router'
import { useUserAuthStore } from '../stores/userAuth'
import { useAppStore } from '../stores/app'
import { useTelegramMiniAppStore } from '../stores/telegramMiniApp'
import { captureAffiliateFromRoute } from '../utils/affiliate'
import { templateView } from '../templates/registry'
import { GOOGLE_REDIRECT_FRONTEND_CALLBACK_PATH } from '../utils/googleRedirect'
import Products from '../views/Products.vue'
import { prefetchProductDetail } from '../utils/productDetailPrefetch'

type RouteComponentLoader = () => Promise<unknown>

const homeViewLoader: RouteComponentLoader = () => import('../views/Home.vue')
const productsViewLoader: RouteComponentLoader = () => import('../views/Products.vue')
const productDetailViewLoader: RouteComponentLoader = () => import('../views/ProductDetail.vue')
const cartViewLoader: RouteComponentLoader = () => import('../views/Cart.vue')
const checkoutViewLoader: RouteComponentLoader = () => import('../views/Checkout.vue')
const paymentViewLoader: RouteComponentLoader = () => import('../views/Payment.vue')
const blogViewLoader: RouteComponentLoader = () => import('../views/Blog.vue')
const noticeViewLoader: RouteComponentLoader = () => import('../views/Notice.vue')
const loginViewLoader: RouteComponentLoader = () => import('../views/auth/Login.vue')
const registerViewLoader: RouteComponentLoader = () => import('../views/auth/Register.vue')
const guestOrdersViewLoader: RouteComponentLoader = () => import('../views/GuestOrders.vue')
const personalCenterViewLoader: RouteComponentLoader = () => import('../views/PersonalCenter.vue')
const resellerLayoutLoader: RouteComponentLoader = () => import('../views/reseller/ResellerConsoleLayout.vue')

const routeWarmupLoaders: RouteComponentLoader[] = [
    guestOrdersViewLoader,
    personalCenterViewLoader,
    productDetailViewLoader,
    checkoutViewLoader,
    paymentViewLoader,
    loginViewLoader,
]

const authRouteWarmupLoaders: RouteComponentLoader[] = [registerViewLoader]

let hasScheduledRouteWarmup = false

const shouldWarmupRoutes = () => {
    if (typeof window === 'undefined' || typeof navigator === 'undefined') {
        return false
    }

    const connection = (navigator as Navigator & {
        connection?: {
            saveData?: boolean
            effectiveType?: string
        }
    }).connection

    if (connection?.saveData) {
        return false
    }

    return connection?.effectiveType !== 'slow-2g' && connection?.effectiveType !== '2g'
}

const scheduleIdleTask = (task: () => void) => {
    if (typeof window === 'undefined') {
        return
    }

    if ('requestIdleCallback' in window && typeof window.requestIdleCallback === 'function') {
        window.requestIdleCallback(task, { timeout: 1500 })
        return
    }

    window.setTimeout(task, 600)
}

const runRouteWarmupQueue = (loaders: RouteComponentLoader[]) => {
    const nextLoader = loaders.shift()
    if (!nextLoader || typeof window === 'undefined') {
        return
    }

    void nextLoader()
        .catch(() => undefined)
        .finally(() => {
            window.setTimeout(() => {
                scheduleIdleTask(() => runRouteWarmupQueue(loaders))
            }, 400)
        })
}

export const warmupCommonRoutes = () => {
    if (hasScheduledRouteWarmup || !shouldWarmupRoutes()) {
        return
    }

    hasScheduledRouteWarmup = true

    const startWarmup = () => {
        scheduleIdleTask(() => runRouteWarmupQueue([...routeWarmupLoaders]))
    }

    if (document.readyState === 'complete') {
        startWarmup()
        return
    }

    window.addEventListener('load', startWarmup, { once: true })
}

const router = createRouter({
    history: createWebHistory(import.meta.env.BASE_URL),
    scrollBehavior(to, _from, savedPosition) {
        if (savedPosition) {
            return savedPosition
        }
        if (to.hash) {
            return { el: to.hash, top: 80 }
        }
        return { top: 0 }
    },
    routes: [
        {
            path: '/',
            name: 'products',
            component: Products,
        },
        {
            path: '/products',
            name: 'product-catalog',
            component: () => {
                const appStore = useAppStore()
                return appStore.config?.template_mode === 'list'
                    ? templateView('Home', homeViewLoader)()
                    : templateView('Products', productsViewLoader)()
            },
        },
        {
            path: '/categories/:slug',
            name: 'category-products',
            component: () => {
                const appStore = useAppStore()
                return appStore.config?.template_mode === 'list'
                    ? templateView('Home', homeViewLoader)()
                    : templateView('Products', productsViewLoader)()
            },
        },
        {
            path: '/products/:slug',
            name: 'product-detail',
            component: productDetailViewLoader,
        },
        {
            path: '/cart',
            name: 'cart',
            component: templateView('Cart', cartViewLoader),
        },
        {
            path: '/checkout',
            name: 'checkout',
            component: templateView('Checkout', checkoutViewLoader),
        },
        {
            path: '/pay',
            name: 'payment',
            component: templateView('Payment', paymentViewLoader),
        },
        {
            path: '/me',
            name: 'personal-center',
            component: templateView('PersonalCenter', personalCenterViewLoader),
            props: { section: 'overview' },
            meta: { requiresUserAuth: true }
        },
        {
            path: '/me/profile',
            name: 'personal-center-profile',
            component: templateView('PersonalCenter', personalCenterViewLoader),
            props: { section: 'profile' },
            meta: { requiresUserAuth: true }
        },
        {
            path: '/me/security',
            name: 'personal-center-security',
            component: templateView('PersonalCenter', personalCenterViewLoader),
            props: { section: 'security' },
            meta: { requiresUserAuth: true }
        },
        {
            path: '/me/orders',
            name: 'personal-center-orders',
            component: templateView('PersonalCenter', personalCenterViewLoader),
            props: { section: 'orders' },
            meta: { requiresUserAuth: true }
        },
        {
            path: '/me/wallet',
            name: 'personal-center-wallet',
            component: templateView('PersonalCenter', personalCenterViewLoader),
            props: { section: 'wallet' },
            meta: { requiresUserAuth: true }
        },
        {
            path: '/me/gift-cards',
            name: 'personal-center-gift-cards',
            component: templateView('PersonalCenter', personalCenterViewLoader),
            props: { section: 'giftCard' },
            meta: { requiresUserAuth: true }
        },
        {
            path: '/me/api',
            name: 'personal-center-api',
            redirect: '/me',
            meta: { requiresUserAuth: true }
        },
        {
            path: '/me/affiliate',
            name: 'personal-center-affiliate',
            redirect: '/me',
            meta: { requiresUserAuth: true }
        },
        {
            path: '/me/reseller',
            name: 'personal-center-reseller',
            redirect: '/me',
            meta: { requiresUserAuth: true }
        },
        {
            path: '/reseller',
            component: resellerLayoutLoader,
            meta: { requiresUserAuth: true, resellerConsole: true },
            children: [
                { path: '', name: 'reseller-dashboard', component: () => import('../views/reseller/ResellerDashboard.vue') },
                { path: 'apply', name: 'reseller-apply', component: () => import('../views/reseller/ResellerApply.vue') },
                { path: 'domains', name: 'reseller-domains', component: () => import('../views/reseller/ResellerDomains.vue') },
                { path: 'site', name: 'reseller-site', component: () => import('../views/reseller/ResellerSiteConfig.vue') },
                { path: 'products', name: 'reseller-products', component: () => import('../views/reseller/ResellerProducts.vue') },
                { path: 'orders', name: 'reseller-orders', component: () => import('../views/reseller/ResellerOrders.vue') },
                { path: 'orders/:order_no', name: 'reseller-order-detail', component: () => import('../views/reseller/ResellerOrderDetail.vue') },
                { path: 'finance', name: 'reseller-finance', component: () => import('../views/reseller/ResellerFinance.vue') },
                { path: 'ledger', name: 'reseller-ledger', component: () => import('../views/reseller/ResellerLedger.vue') },
                { path: 'withdraws', name: 'reseller-withdraws', component: () => import('../views/reseller/ResellerWithdraws.vue') },
            ],
        },
        {
            path: '/orders/:order_no',
            name: 'order-detail',
            component: templateView('OrderDetail', () => import('../views/OrderDetail.vue')),
            meta: { requiresUserAuth: true }
        },
        {
            path: '/recharge-orders/:recharge_no',
            name: 'recharge-order-detail',
            component: templateView('RechargeOrderDetail', () => import('../views/RechargeOrderDetail.vue')),
            meta: { requiresUserAuth: true }
        },
        {
            path: '/guest/orders',
            name: 'guest-orders',
            component: templateView('GuestOrders', guestOrdersViewLoader),
        },
        {
            path: '/guest/orders/:order_no',
            name: 'guest-order-detail',
            component: templateView('GuestOrderDetail', () => import('../views/GuestOrderDetail.vue')),
        },
        {
            path: '/blog',
            name: 'blog',
            component: templateView('Blog', blogViewLoader),
        },
        {
            path: '/blog/:slug',
            name: 'blog-detail',
            component: templateView('BlogDetail', () => import('../views/BlogDetail.vue')),
        },
        {
            path: '/notice',
            name: 'notice',
            component: templateView('Notice', noticeViewLoader),
        },
        {
            path: '/about',
            name: 'about',
            component: templateView('About', () => import('../views/About.vue')),
        },
        {
            path: '/terms',
            name: 'terms',
            component: templateView('Legal', () => import('../views/Legal.vue')),
            props: { type: 'terms' }
        },
        {
            path: '/privacy',
            name: 'privacy',
            component: templateView('Legal', () => import('../views/Legal.vue')),
            props: { type: 'privacy' }
        },
        {
            path: '/auth/login',
            name: 'user-login',
            component: templateView('auth/Login', loginViewLoader),
            meta: { userGuest: true }
        },
        {
            path: '/auth/register',
            name: 'user-register',
            component: templateView('auth/Register', registerViewLoader),
            meta: { userGuest: true }
        },
        {
            path: '/auth/forgot',
            name: 'user-forgot',
            component: templateView('auth/Forgot', () => import('../views/auth/Forgot.vue')),
            meta: { userGuest: true }
        },
        {
            path: '/auth/telegram/callback',
            name: 'user-telegram-callback',
            component: templateView('auth/TelegramCallback', () => import('../views/auth/TelegramCallback.vue')),
        },
        {
            path: GOOGLE_REDIRECT_FRONTEND_CALLBACK_PATH,
            name: 'user-google-callback',
            component: templateView('auth/GoogleCallback', () => import('../views/auth/GoogleCallback.vue')),
        },
        {
            path: '/:pathMatch(.*)*',
            name: 'not-found',
            component: templateView('NotFound', () => import('../views/NotFound.vue')),
        },
    ],
})

// Navigation Guard
router.beforeEach(async (to, _from, next) => {
    const userAuthStore = useUserAuthStore()
    const appStore = useAppStore()
    void captureAffiliateFromRoute(to)
    if (to.name === 'product-detail') {
        void prefetchProductDetail(String(to.params.slug || ''))
    }

    // 路由组件会根据站点配置选择模板与列表模式。导航解析前只等待配置，
    // Vue 本身仍会立即挂载，因此不会重新引入全屏 loading 或挂载阻塞。
    if (!appStore.config) {
        if (to.name === 'products' || to.name === 'product-detail') {
            void appStore.loadConfig()
        } else {
            await appStore.loadConfig()
        }
    }

    if (to.meta.requiresUserAuth) {
        if (!userAuthStore.isAuthenticated) {
            const redirect = encodeURIComponent(to.fullPath)
            next(`/auth/login?redirect=${redirect}`)
        } else if (to.meta.resellerConsole && !appStore.canAccessResellerConsole) {
            next('/me/orders')
        } else {
            next()
        }
    }
    else if (to.meta.userGuest) {
        if (userAuthStore.isAuthenticated) {
            next('/me/orders')
        } else {
            next()
        }
    }
    else {
        next()
    }
})

// Update SEO on route change
router.afterEach(() => {
    const appStore = useAppStore()
    const telegramMiniAppStore = useTelegramMiniAppStore()
    appStore.applySEO()
    telegramMiniAppStore.syncRouteBackButton(router.currentRoute.value.path, () => {
        if (window.history.length > 1) {
            router.back()
            return
        }
        void router.push('/')
    })
    if (router.currentRoute.value.name === 'user-login' && shouldWarmupRoutes()) {
        scheduleIdleTask(() => runRouteWarmupQueue([...authRouteWarmupLoaders]))
    }
})

export default router
