<template>
  <div class="atelier-scope">
    <!-- 顶部柔和极光氛围光晕 -->
    <div class="prism-aurora" aria-hidden="true"></div>

    <!-- 顶部居中悬浮玻璃岛导航（Floating Island Dock） -->
    <div class="pointer-events-none fixed inset-x-0 top-0 z-50 px-3 pt-3 sm:px-6 sm:pt-3.5">
      <header
        class="prism-dock pointer-events-auto mx-auto flex h-[54px] w-full max-w-[1160px] items-center justify-between gap-3 rounded-full px-3 backdrop-blur-xl backdrop-saturate-150 sm:px-4"
      >
        <!-- 左：棱镜品牌标志 + 站点名 + 主导航 -->
        <div class="flex min-w-0 items-center gap-4 lg:gap-5">
          <RouterLink
            to="/"
            class="inline-flex min-w-0 shrink-0 items-center gap-2.5 text-[15px] font-bold tracking-[-0.025em] text-foreground transition-opacity hover:opacity-85"
            :title="brandSiteName"
          >
            <img
              v-if="brandLogo"
              :src="brandLogo"
              :alt="brandSiteName"
              class="h-6 max-w-[132px] shrink-0 object-contain"
            />
            <span
              v-else
              class="grid h-6 w-6 shrink-0 place-items-center rounded-[7px] bg-gradient-to-br from-indigo-500 to-cyan-400 text-[12px] font-extrabold tracking-tight text-white shadow-xs"
            >
              {{ brandInitial }}
            </span>
            <span class="max-w-[38vw] truncate sm:max-w-[12rem]">{{ brandSiteName }}</span>
          </RouterLink>

          <nav class="hidden min-w-0 items-center gap-0.5 overflow-x-auto scrollbar-hide lg:flex" :aria-label="t('nav.home')">
            <template v-for="item in primaryNavItems" :key="item.key">
              <RouterLink
                v-if="item.type === 'route'"
                :to="item.path"
                class="inline-flex h-8 shrink-0 items-center whitespace-nowrap rounded-full px-3 text-[13px] font-medium text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
                :active-class="item.path === '/' ? undefined : '!bg-secondary !font-semibold !text-foreground'"
                :exact-active-class="item.path === '/' ? '!bg-secondary !font-semibold !text-foreground' : undefined"
              >
                {{ item.label }}
              </RouterLink>
              <a
                v-else
                :href="item.path"
                :target="item.target"
                rel="noopener noreferrer"
                class="inline-flex h-8 shrink-0 items-center whitespace-nowrap rounded-full px-3 text-[13px] font-medium text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
              >
                {{ item.label }}
              </a>
            </template>
          </nav>
        </div>

        <!-- 右：快捷检索触发器 + 游客查单 + 主题/语言 + 购物车 + 账户入口 -->
        <div class="flex shrink-0 items-center gap-1.5">
          <!-- ⌘K 快捷检索按钮（桌面） -->
          <button
            type="button"
            class="hidden h-8 items-center gap-2 rounded-full border bg-secondary/80 px-2.5 text-xs font-medium text-muted-foreground transition-colors hover:border-primary/40 hover:text-foreground md:inline-flex"
            :aria-label="t('atelier.quickSearch')"
            @click="triggerQuickSearch"
          >
            <Search class="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
            <span class="hidden xl:inline">{{ t('atelier.quickSearch') }}</span>
            <kbd class="rounded border bg-card px-1.5 py-0.5 font-mono text-[10px] leading-none text-foreground/80">
              {{ shortcutLabel }}
            </kbd>
          </button>

          <RouterLink
            v-if="!userAuthStore.isAuthenticated"
            to="/guest/orders"
            class="hidden h-8 items-center gap-1.5 rounded-full px-2.5 text-[12.5px] font-medium text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground lg:inline-flex"
          >
            <ClipboardList class="h-3.5 w-3.5 shrink-0" />
            <span>{{ t('navbar.guestOrders') }}</span>
          </RouterLink>

          <button
            type="button"
            class="grid h-8 w-8 place-items-center rounded-full text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
            :aria-label="t('resellerConsole.common.toggleTheme')"
            @click="toggleTheme"
          >
            <Sun v-if="theme === 'dark'" class="h-4 w-4" />
            <Moon v-else class="h-4 w-4" />
          </button>

          <!-- 语言切换（桌面） -->
          <Popover v-model:open="langOpen">
            <PopoverTrigger as-child>
              <button
                type="button"
                class="hidden h-8 items-center gap-1 rounded-full px-2 text-xs font-medium text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground lg:inline-flex"
                :aria-label="t('navbar.selectLanguage')"
              >
                <Languages class="h-3.5 w-3.5" />
                <span class="uppercase">{{ currentLocaleLabel }}</span>
              </button>
            </PopoverTrigger>
            <PopoverContent align="end" class="w-40 rounded-[14px] border bg-card p-1.5 shadow-[var(--at-shadow-float)]">
              <div class="border-b px-2.5 pb-1.5 pt-1 text-[11px] font-semibold text-muted-foreground">
                {{ t('navbar.selectLanguage') }}
              </div>
              <div class="mt-1 space-y-0.5">
                <button
                  v-for="lang in languages"
                  :key="lang.code"
                  type="button"
                  class="flex w-full items-center justify-between rounded-[9px] px-2.5 py-2 text-left text-[13px] transition-colors hover:bg-secondary"
                  :class="appStore.locale === lang.code ? 'font-semibold text-primary' : 'text-muted-foreground'"
                  @click="changeLanguage(lang.code)"
                >
                  <span>{{ lang.name }}</span>
                  <span v-if="appStore.locale === lang.code" class="h-1.5 w-1.5 rounded-full bg-primary"></span>
                </button>
              </div>
            </PopoverContent>
          </Popover>

          <!-- 购物车胶囊（桌面端显示） -->
          <RouterLink
            to="/cart"
            class="relative hidden h-8 items-center gap-1.5 rounded-full border bg-secondary/70 px-2.5 text-xs font-semibold text-foreground transition-colors hover:border-primary/40 hover:bg-secondary lg:inline-flex"
            :aria-label="t('navbar.cart')"
            :title="t('navbar.cart')"
          >
            <ShoppingCart class="h-3.5 w-3.5" />
            <span
              v-if="cartCount > 0"
              class="inline-grid h-4 min-w-4 place-items-center rounded-full bg-primary px-1 font-mono text-[10px] font-bold leading-none text-primary-foreground"
              :class="{ 'theme-bounce-in': cartBounce }"
            >
              {{ cartCount > 99 ? '99+' : cartCount }}
            </span>
          </RouterLink>

          <!-- 登录 / 个人中心 -->
          <template v-if="userAuthStore.isAuthenticated">
            <RouterLink
              to="/me"
              class="hidden h-8 items-center gap-1.5 rounded-full bg-primary px-3.5 text-xs font-semibold text-primary-foreground transition-opacity hover:opacity-90 lg:inline-flex"
            >
              <User class="h-3.5 w-3.5" />
              <span>{{ t('navbar.personalCenter') }}</span>
            </RouterLink>
            <button
              type="button"
              class="hidden h-8 w-8 place-items-center rounded-full text-muted-foreground transition-colors hover:bg-destructive/10 hover:text-destructive lg:grid"
              :aria-label="t('navbar.logout')"
              :title="t('navbar.logout')"
              @click="userAuthStore.logout()"
            >
              <LogOut class="h-3.5 w-3.5" />
            </button>
          </template>
          <RouterLink
            v-else
            to="/auth/login"
            class="hidden h-8 items-center justify-center rounded-full bg-primary px-4 text-xs font-semibold text-primary-foreground shadow-xs transition-opacity hover:opacity-90 lg:inline-flex"
          >
            {{ t('navbar.login') }}
          </RouterLink>

          <!-- 移动端更多菜单 -->
          <button
            type="button"
            class="grid h-8 w-8 place-items-center rounded-full text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground lg:hidden"
            :aria-label="t('navbar.more')"
            @click="showMobileMenu = !showMobileMenu"
          >
            <Menu class="h-4 w-4" />
          </button>
        </div>
      </header>
    </div>

    <!-- 移动端抽屉 -->
    <Teleport to="body">
      <Transition
        enter-active-class="transition duration-250 ease-out"
        enter-from-class="opacity-0"
        enter-to-class="opacity-100"
        leave-active-class="transition duration-200 ease-in"
        leave-from-class="opacity-100"
        leave-to-class="opacity-0"
      >
        <div
          v-if="showMobileMenu"
          class="fixed inset-0 z-[60] bg-black/50 backdrop-blur-xs lg:hidden"
          @click="showMobileMenu = false"
        ></div>
      </Transition>

      <Transition
        enter-active-class="transition duration-250 ease-out"
        enter-from-class="translate-x-full"
        enter-to-class="translate-x-0"
        leave-active-class="transition duration-200 ease-in"
        leave-from-class="translate-x-0"
        leave-to-class="translate-x-full"
      >
        <div
          v-if="showMobileMenu"
          class="fixed bottom-0 right-0 top-0 z-[70] w-72 max-w-[82vw] overflow-y-auto border-l bg-card p-5 shadow-2xl lg:hidden"
        >
          <div class="mb-4 flex items-center justify-between border-b pb-3">
            <span class="font-mono text-xs font-semibold tracking-wider text-muted-foreground uppercase">{{ t('navbar.more') }}</span>
            <button
              type="button"
              class="grid h-8 w-8 place-items-center rounded-full bg-secondary text-foreground"
              @click="showMobileMenu = false"
            >
              <X class="h-4 w-4" />
            </button>
          </div>

          <div class="space-y-1">
            <button
              type="button"
              class="flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-left text-sm font-medium text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
              @click="showMobileMenu = false; triggerQuickSearch()"
            >
              <Search class="h-4 w-4 shrink-0 opacity-75" />
              <span>{{ t('atelier.quickSearch') }}</span>
            </button>

            <template v-for="item in secondaryNavItems" :key="item.key">
              <RouterLink
                v-if="item.type === 'route'"
                :to="item.path"
                class="flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
                active-class="!bg-primary/10 !font-semibold !text-primary"
                @click="showMobileMenu = false"
              >
                <component :is="item.icon" class="h-4 w-4 shrink-0 opacity-75" />
                <span>{{ item.label }}</span>
              </RouterLink>
              <a
                v-else
                :href="item.path"
                :target="item.target"
                rel="noopener noreferrer"
                class="flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
                @click="showMobileMenu = false"
              >
                <component :is="item.icon" class="h-4 w-4 shrink-0 opacity-75" />
                <span>{{ item.label }}</span>
              </a>
            </template>

            <RouterLink
              v-if="!userAuthStore.isAuthenticated"
              to="/guest/orders"
              class="flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
              @click="showMobileMenu = false"
            >
              <ClipboardList class="h-4 w-4 shrink-0 opacity-75" />
              <span>{{ t('navbar.guestOrders') }}</span>
            </RouterLink>

            <button
              v-if="userAuthStore.isAuthenticated"
              type="button"
              class="flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-left text-sm font-medium text-destructive transition-colors hover:bg-destructive/10"
              @click="userAuthStore.logout(); showMobileMenu = false"
            >
              <LogOut class="h-4 w-4 shrink-0 opacity-75" />
              <span>{{ t('navbar.logout') }}</span>
            </button>
          </div>

          <div class="mt-5 border-t pt-4">
            <span class="px-3 text-xs font-semibold text-muted-foreground">{{ t('navbar.selectLanguage') }}</span>
            <div class="mt-2 space-y-1">
              <button
                v-for="lang in languages"
                :key="lang.code"
                type="button"
                class="flex w-full items-center justify-between rounded-xl px-3 py-2 text-sm transition-colors hover:bg-secondary"
                :class="appStore.locale === lang.code ? 'font-semibold text-primary' : 'text-muted-foreground'"
                @click="changeLanguage(lang.code); showMobileMenu = false"
              >
                <span>{{ lang.name }}</span>
                <span v-if="appStore.locale === lang.code" class="h-1.5 w-1.5 rounded-full bg-primary"></span>
              </button>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <!-- 主内容区域 -->
    <main class="relative z-10 flex-1 pb-14 lg:pb-0">
      <slot />
    </main>

    <!-- 精密科技三栏页脚 -->
    <footer class="relative z-10 border-t bg-card/85 text-muted-foreground backdrop-blur-md">
      <div class="mx-auto w-full max-w-[1240px] px-4 py-12 sm:px-6">
        <div
          class="grid grid-cols-1 gap-9 border-b pb-9 md:gap-10"
          :class="hasContact ? 'md:grid-cols-[1.6fr_1fr_1.2fr]' : 'md:grid-cols-[1.6fr_1fr]'"
        >
          <!-- 品牌简介 -->
          <div>
            <RouterLink to="/" class="inline-flex items-center gap-2.5 text-[15px] font-bold tracking-[-0.025em] text-foreground">
              <img
                v-if="brandLogo"
                :src="brandLogo"
                :alt="brandSiteName"
                class="h-6 max-w-[150px] object-contain"
              />
              <span
                v-else
                class="grid h-6 w-6 place-items-center rounded-[7px] bg-gradient-to-br from-indigo-500 to-cyan-400 text-[12px] font-extrabold text-white"
              >
                {{ brandInitial }}
              </span>
              <span>{{ brandSiteName }}</span>
            </RouterLink>
            <p class="mt-3 max-w-[38ch] text-[13.5px] leading-relaxed text-muted-foreground">
              {{ brandDescription || t('footer.description') }}
            </p>
          </div>

          <!-- 快速导航 -->
          <div>
            <h4 class="mb-3.5 font-mono text-[11px] font-semibold tracking-[0.08em] text-muted-foreground uppercase">
              {{ t('footer.quickLinks') }}
            </h4>
            <ul class="grid grid-cols-2 gap-x-4 gap-y-2.5 text-[13.5px]">
              <li v-for="item in quickLinks" :key="item.path">
                <RouterLink :to="item.path" class="transition-colors hover:text-foreground">
                  {{ t(item.label) }}
                </RouterLink>
              </li>
              <li v-if="!userAuthStore.isAuthenticated">
                <RouterLink to="/guest/orders" class="transition-colors hover:text-foreground">
                  {{ t('navbar.guestOrders') }}
                </RouterLink>
              </li>
            </ul>
          </div>

          <!-- 联系入口 -->
          <div v-if="hasContact">
            <h4 class="mb-3.5 font-mono text-[11px] font-semibold tracking-[0.08em] text-muted-foreground uppercase">
              {{ t('footer.contact') }}
            </h4>
            <div class="flex flex-wrap gap-2">
              <a
                v-if="contact?.telegram"
                :href="contact.telegram"
                target="_blank"
                rel="noopener noreferrer"
                class="inline-flex h-8.5 items-center gap-2 rounded-[10px] border bg-background px-3 text-[12.5px] font-medium text-foreground/85 transition-colors hover:border-primary/40 hover:bg-secondary hover:text-foreground"
              >
                <Send class="h-3.5 w-3.5 text-primary" />
                <span>Telegram</span>
              </a>
              <a
                v-if="contact?.whatsapp"
                :href="contact.whatsapp"
                target="_blank"
                rel="noopener noreferrer"
                class="inline-flex h-8.5 items-center gap-2 rounded-[10px] border bg-background px-3 text-[12.5px] font-medium text-foreground/85 transition-colors hover:border-primary/40 hover:bg-secondary hover:text-foreground"
              >
                <MessageCircle class="h-3.5 w-3.5 text-primary" />
                <span>WhatsApp</span>
              </a>
              <a
                v-if="contact?.email"
                :href="`mailto:${contact.email}`"
                class="inline-flex h-8.5 items-center gap-2 rounded-[10px] border bg-background px-3 text-[12.5px] font-medium text-foreground/85 transition-colors hover:border-primary/40 hover:bg-secondary hover:text-foreground"
              >
                <Mail class="h-3.5 w-3.5 text-primary" />
                <span>{{ t('footer.email') }}</span>
              </a>
              <a
                v-if="contact?.support_url"
                :href="contact.support_url"
                :target="contact.support_url.startsWith('https://') ? '_blank' : undefined"
                rel="noopener noreferrer"
                class="inline-flex h-8.5 items-center gap-2 rounded-[10px] border bg-background px-3 text-[12.5px] font-medium text-foreground/85 transition-colors hover:border-primary/40 hover:bg-secondary hover:text-foreground"
              >
                <Headphones class="h-3.5 w-3.5 text-primary" />
                <span>{{ t('footer.supportUrl') }}</span>
              </a>
            </div>
          </div>
        </div>

        <!-- 底部版权与条款 -->
        <div class="flex flex-col items-center justify-between gap-3 pt-6 text-xs text-muted-foreground sm:flex-row">
          <div class="flex flex-wrap items-center justify-center gap-x-3 gap-y-1 sm:justify-start">
            <span>&copy; {{ currentYear }} {{ brandSiteName }}. {{ t('footer.rights') }}</span>
            <a
              href="https://github.com/dujiao-next"
              target="_blank"
              rel="noopener noreferrer"
              aria-label="Dujiao-Next on GitHub"
              class="inline-flex items-center gap-1 transition-colors hover:text-foreground"
            >
              <Github class="h-3.5 w-3.5" />
              <span>Dujiao-Next</span>
            </a>
          </div>

          <div class="flex flex-wrap items-center justify-center gap-x-4 gap-y-1 sm:justify-end">
            <RouterLink to="/privacy" class="transition-colors hover:text-foreground">
              {{ t('footer.privacy') || 'Privacy Policy' }}
            </RouterLink>
            <RouterLink to="/terms" class="transition-colors hover:text-foreground">
              {{ t('footer.terms') || 'Terms of Service' }}
            </RouterLink>
            <a
              v-for="link in footerLinks"
              :key="link.name"
              :href="link.url || 'javascript:void(0)'"
              :target="link.url ? '_blank' : undefined"
              rel="noopener noreferrer"
              class="transition-colors hover:text-foreground"
            >
              {{ link.name }}
            </a>
          </div>
        </div>
      </div>
    </footer>

    <BackToTop />
    <MobileBottomNav v-if="route.name !== 'product-detail'" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  ClipboardList,
  Github,
  Headphones,
  Languages,
  LogOut,
  Mail,
  Menu,
  MessageCircle,
  Moon,
  Search,
  Send,
  ShoppingCart,
  Sun,
  User,
  X,
} from 'lucide-vue-next'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import BackToTop from '../../../components/BackToTop.vue'
import MobileBottomNav from '../../../components/MobileBottomNav.vue'
import { useAppStore } from '../../../stores/app'
import { useCartStore } from '../../../stores/cart'
import { useUserAuthStore } from '../../../stores/userAuth'
import { useNavConfig } from '../../../composables/useNavConfig'
import { useFooterConfig } from '../../../composables/useFooterConfig'
import { useTheme } from '../../../utils/theme'
import '../styles/atelier.css'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const appStore = useAppStore()
const cartStore = useCartStore()
const userAuthStore = useUserAuthStore()
const { theme, toggleTheme } = useTheme()
const { isListMode, primaryNavItems, secondaryNavItems } = useNavConfig()
const {
  currentYear,
  brandSiteName,
  brandInitial,
  brandLogo,
  brandDescription,
  contact,
  hasContact,
  quickLinks,
  footerLinks,
} = useFooterConfig({ includeNotice: true })

const showMobileMenu = ref(false)
const langOpen = ref(false)
const cartBounce = ref(false)

const isMac = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/i.test(navigator.platform || navigator.userAgent)
const shortcutLabel = isMac ? '⌘K' : 'Ctrl K'

const languages = [
  { code: 'zh-CN', name: '简体中文' },
  { code: 'zh-TW', name: '繁體中文' },
  { code: 'en-US', name: 'English' },
]

const currentLocaleLabel = computed(() => {
  if (appStore.locale === 'en-US') return 'EN'
  return appStore.locale === 'zh-TW' ? '繁' : '简'
})

const changeLanguage = (code: string) => {
  appStore.setLocale(code)
  langOpen.value = false
}

const cartCount = computed(() => cartStore.totalItems)

const isEditableElement = (target: EventTarget | null): boolean => {
  if (!(target instanceof HTMLElement)) return false
  if (target.isContentEditable) return true
  const tag = target.tagName
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT'
}

/** 聚焦目录搜索中枢；若当前不在目录页则通过 history.state 携带意图，待目录挂载完成后再聚焦 */
const triggerQuickSearch = async () => {
  const catalogRouteNames = new Set(['home', 'products', 'category-products'])
  if (!catalogRouteNames.has(String(route.name || ''))) {
    await router.push({
      path: isListMode.value ? '/' : '/products',
      state: { focusSearch: true },
    })
    return
  }
  window.dispatchEvent(new CustomEvent('atelier:focus-search'))
}

const onGlobalKeydown = (e: KeyboardEvent) => {
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
    // 结算/支付页或正在表单内输入时不劫持快捷键，避免误跳导致已填表单丢失
    const routeName = String(route.name || '')
    if (routeName === 'checkout' || routeName === 'payment' || isEditableElement(e.target)) {
      return
    }
    e.preventDefault()
    void triggerQuickSearch()
  }
}

watch(cartCount, (newVal, oldVal) => {
  if (newVal > oldVal) {
    cartBounce.value = true
    window.setTimeout(() => { cartBounce.value = false }, 400)
  }
})

onMounted(() => {
  // Teleport 到 body 的浮层（弹窗/Toast/Select）依靠 body.atelier-tokens 继承 Prism 令牌
  document.body.classList.add('atelier-tokens')
  window.addEventListener('keydown', onGlobalKeydown)
})

onUnmounted(() => {
  document.body.classList.remove('atelier-tokens')
  window.removeEventListener('keydown', onGlobalKeydown)
})
</script>

<style scoped>
.scrollbar-hide {
  -ms-overflow-style: none;
  scrollbar-width: none;
}
.scrollbar-hide::-webkit-scrollbar {
  display: none;
}
</style>
