import { createRouter, createWebHistory } from 'vue-router'
import { useUserAuthStore } from '../stores/userAuth'
import { useAppStore } from '../stores/app'
import { useTelegramMiniAppStore } from '../stores/telegramMiniApp'
import { captureAffiliateFromRoute } from '../utils/affiliate'
import { templateView } from '../templates/registry'
import { GOOGLE_REDIRECT_FRONTEND_CALLBACK_PATH } from '../utils/googleRedirect'

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
const resellerLayoutLoader: RouteComponentLoader = () => import('../views/reseller/ResellerConsoleLayout.vue')

const routeWarmupLoaders: RouteComponentLoader[] = [
    productsViewLoader,
    productDetailViewLoader,
    cartViewLoader,
    checkoutViewLoader,
    paymentViewLoader,
    blogViewLoader,
    noticeViewLoader,
    loginViewLoader,
]

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
        window.setTimeout(() => {
            scheduleIdleTask(() => runRouteWarmupQueue([...routeWarmupLoaders]))
        }, 1200)
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
            name: 'home',
            component: templateView('Home', homeViewLoader),
        },
        {
            path: '/products',
            name: 'products',
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
            component: templateView('ProductDetail', productDetailViewLoader),
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
            component: templateView('PersonalCenter', () => import('../views/PersonalCenter.vue')),
            props: { section: 'overview' },
            meta: { requiresUserAuth: true }
        },
        {
            path: '/me/profile',
            name: 'personal-center-profile',
            component: templateView('PersonalCenter', () => import('../views/PersonalCenter.vue')),
            props: { section: 'profile' },
            meta: { requiresUserAuth: true }
        },
        {
            path: '/me/security',
            name: 'personal-center-security',
            component: templateView('PersonalCenter', () => import('../views/PersonalCenter.vue')),
            props: { section: 'security' },
            meta: { requiresUserAuth: true }
        },
        {
            path: '/me/orders',
            name: 'personal-center-orders',
            component: templateView('PersonalCenter', () => import('../views/PersonalCenter.vue')),
            props: { section: 'orders' },
            meta: { requiresUserAuth: true }
        },
        {
            path: '/me/wallet',
            name: 'personal-center-wallet',
            component: templateView('PersonalCenter', () => import('../views/PersonalCenter.vue')),
            props: { section: 'wallet' },
            meta: { requiresUserAuth: true }
        },
        {
            path: '/me/gift-cards',
            name: 'personal-center-gift-cards',
            component: templateView('PersonalCenter', () => import('../views/PersonalCenter.vue')),
            props: { section: 'giftCard' },
            meta: { requiresUserAuth: true }
        },
        {
            path: '/me/api',
            name: 'personal-center-api',
            component: templateView('PersonalCenter', () => import('../views/PersonalCenter.vue')),
            props: { section: 'api' },
            meta: { requiresUserAuth: true }
        },
        {
            path: '/me/affiliate',
            name: 'personal-center-affiliate',
            component: templateView('PersonalCenter', () => import('../views/PersonalCenter.vue')),
            props: { section: 'affiliate' },
            meta: { requiresUserAuth: true }
        },
        {
            path: '/me/reseller',
            name: 'personal-center-reseller',
            redirect: '/reseller',
            meta: { requiresUserAuth: true, resellerConsole: true }
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
            component: templateView('GuestOrders', () => import('../views/GuestOrders.vue')),
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
            // 分类页与 /blog/:slug 不冲突：这里是三段，详情是两段；
            // 且静态段 category 的匹配优先级高于动态段。
            path: '/blog/category/:slug',
            name: 'blog-category',
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
            component: templateView('auth/Register', () => import('../views/auth/Register.vue')),
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

    // Ensure config is loaded before checking template mode
    if (!appStore.config) {
        await appStore.loadConfig()
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
})

export default router
