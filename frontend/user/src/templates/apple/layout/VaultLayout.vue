<template>
  <div class="vault-scope">
    <!-- 顶栏（Apple Store 风格：44px / 移动 48px） -->
    <header
      class="st-header"
      :class="{
        'is-scrolled': scrolled,
        'is-home': isHome,
        'is-article': isArticle,
      }"
    >
      <div class="st-header-inner">
        <RouterLink class="st-brand" to="/" :title="brandName">
          <img v-if="brandLogo" :src="brandLogo" :alt="brandName" class="st-brand-logo" />
          <span v-else class="truncate">{{ brandName }}</span>
        </RouterLink>

        <nav class="st-nav" @mouseleave="scheduleCloseFlyout">
          <template v-for="item in menuItems" :key="item.key">
            <RouterLink
              v-if="item.type === 'route'"
              :to="item.path"
              class="st-nav-link"
              @mouseenter="openFlyoutFor(item)"
              @focus="openFlyoutFor(item)"
            >{{ item.label }}</RouterLink>
            <a v-else :href="item.path" :target="item.target" rel="noopener noreferrer" class="st-nav-link">{{ item.label }}</a>
          </template>
        </nav>

        <div class="st-actions">
          <RouterLink class="st-icon" to="/products" :aria-label="t('nav.products')"><Search /></RouterLink>

          <div class="st-lang max-[1023px]:hidden" ref="langEl">
            <button class="st-icon" type="button" :aria-label="t('navbar.selectLanguage')" @click="toggleLang"><Languages /></button>
            <div v-if="langOpen" class="st-menu">
              <button v-for="lang in languages" :key="lang.code" :class="{ 'is-active': appStore.locale === lang.code }" @click="changeLanguage(lang.code)">
                {{ lang.name }}
                <span v-if="appStore.locale === lang.code">•</span>
              </button>
            </div>
          </div>

          <button class="st-icon max-[1023px]:hidden" type="button" :aria-label="t('resellerConsole.common.toggleTheme')" @click="toggleTheme">
            <Sun v-if="theme === 'dark'" />
            <Moon v-else />
          </button>

          <RouterLink class="st-icon st-cart" to="/cart" :aria-label="t('navbar.cart')">
            <ShoppingCart />
            <span v-if="cartCount > 0" class="st-badge">{{ cartCount > 99 ? '99+' : cartCount }}</span>
          </RouterLink>

          <RouterLink v-if="!userAuthStore.isAuthenticated" class="st-login max-[1023px]:hidden" to="/auth/login">{{ t('navbar.login') }}</RouterLink>
          <template v-else>
            <RouterLink class="st-icon max-[1023px]:hidden" to="/me" :aria-label="t('navbar.personalCenter')"><User /></RouterLink>
            <button class="st-icon max-[1023px]:hidden" type="button" :aria-label="t('navbar.logout')" @click="userAuthStore.logout()"><LogOut /></button>
          </template>

          <button class="st-icon st-menu-btn" type="button" :aria-label="t('navbar.more')" @click="toggleMore">
            <Menu v-if="!moreOpen" />
            <X v-else />
          </button>
        </div>
      </div>

      <!-- 桌面端下拉面板（Apple megamenu） -->
      <Transition name="st-flyout">
        <div
          v-if="openFlyout === 'products'"
          class="st-flyout"
          @mouseenter="cancelCloseFlyout"
          @mouseleave="scheduleCloseFlyout"
        >
          <div class="st-container st-flyout-inner">
            <div class="st-flyout-col">
              <p class="st-flyout-title">{{ t('products.allCategories') }}</p>
              <RouterLink
                v-for="(cat, i) in flyoutCategories"
                :key="cat.slug"
                class="st-flyout-link"
                :style="{ animationDelay: `${i * 18}ms` }"
                :to="`/categories/${cat.slug}`"
                @click="closeFlyout"
              >{{ cat.name }}</RouterLink>
            </div>
            <div class="st-flyout-col">
              <p class="st-flyout-title">{{ t('vault.footer.shop') }}</p>
              <RouterLink class="st-flyout-link" :style="{ animationDelay: '40ms' }" to="/products" @click="closeFlyout">{{ t('nav.products') }}</RouterLink>
              <RouterLink class="st-flyout-link" :style="{ animationDelay: '58ms' }" to="/blog" @click="closeFlyout">{{ t('nav.blog') }}</RouterLink>
              <RouterLink class="st-flyout-link" :style="{ animationDelay: '76ms' }" to="/me" @click="closeFlyout">{{ t('navbar.personalCenter') }}</RouterLink>
            </div>
          </div>
        </div>
      </Transition>

      <!-- 移动端抽屉 -->
      <Transition name="st-drawer">
      <div v-if="moreOpen" class="st-drawer">
        <template v-for="item in menuItems" :key="`m-${item.key}`">
          <RouterLink v-if="item.type === 'route'" :to="item.path" class="st-drawer-link" @click="moreOpen = false">{{ item.label }}</RouterLink>
          <a v-else :href="item.path" :target="item.target" rel="noopener noreferrer" class="st-drawer-link" @click="moreOpen = false">{{ item.label }}</a>
        </template>
        <RouterLink v-if="!userAuthStore.isAuthenticated" to="/guest/orders" class="st-drawer-link" @click="moreOpen = false">{{ t('navbar.guestOrders') }}</RouterLink>
        <RouterLink v-if="userAuthStore.isAuthenticated" to="/me" class="st-drawer-link" @click="moreOpen = false">{{ t('navbar.personalCenter') }}</RouterLink>
        <RouterLink v-if="!userAuthStore.isAuthenticated" to="/auth/login" class="st-drawer-link" @click="moreOpen = false">{{ t('navbar.login') }}</RouterLink>
        <button v-if="userAuthStore.isAuthenticated" type="button" class="st-drawer-link w-full text-left" @click="userAuthStore.logout(); moreOpen = false">{{ t('navbar.logout') }}</button>

        <span class="st-drawer-label">{{ t('resellerConsole.common.toggleTheme') }}</span>
        <div class="st-drawer-lang">
          <button type="button" :class="{ 'is-active': theme !== 'dark' }" @click="toggleTheme">☀</button>
          <button type="button" :class="{ 'is-active': theme === 'dark' }" @click="toggleTheme">☾</button>
        </div>

        <span class="st-drawer-label">{{ t('navbar.selectLanguage') }}</span>
        <div class="st-drawer-lang">
          <button v-for="lang in languages" :key="`ml-${lang.code}`" type="button" :class="{ 'is-active': appStore.locale === lang.code }" @click="changeLanguage(lang.code); moreOpen = false">{{ lang.name }}</button>
        </div>
      </div>
      </Transition>
    </header>

    <!-- 下拉幕布（苹果的 globalnav-curtain：半透明材质 + 20px 模糊） -->
    <Transition name="st-curtain">
      <div v-if="openFlyout" class="st-curtain" @click="closeFlyout"></div>
    </Transition>

    <!-- 页面内容 -->
    <main class="flex-1">
      <slot />
    </main>

    <!-- 页脚（Apple 风格：浅灰底、小字、多列） -->
    <footer class="st-footer">
      <div class="st-container st-footer-inner">
        <div class="st-footer-brand">
          <RouterLink class="st-footer-logo" to="/">{{ brandName }}</RouterLink>
          <p v-if="brandDescription" class="st-footer-desc">{{ brandDescription }}</p>
        </div>

        <div class="st-footer-col">
          <h4>{{ t('vault.footer.shop') }}</h4>
          <RouterLink to="/products">{{ t('products.allCategories') }}</RouterLink>
          <RouterLink v-if="noticeEnabled" to="/notice">{{ t('nav.notice') }}</RouterLink>
          <RouterLink v-if="blogEnabled" to="/blog">{{ t('nav.blog') }}</RouterLink>
          <RouterLink to="/me">{{ t('navbar.personalCenter') }}</RouterLink>
        </div>

        <div class="st-footer-col">
          <h4>{{ t('vault.footer.support') }}</h4>
          <RouterLink v-if="aboutEnabled" to="/about">{{ t('nav.about') }}</RouterLink>
          <RouterLink v-if="!userAuthStore.isAuthenticated" to="/guest/orders">{{ t('navbar.guestOrders') }}</RouterLink>
          <a v-if="contact?.telegram" :href="contact.telegram" target="_blank" rel="noopener noreferrer">Telegram</a>
          <a v-if="contact?.whatsapp" :href="contact.whatsapp" target="_blank" rel="noopener noreferrer">WhatsApp</a>
          <a v-if="contact?.email" :href="`mailto:${contact.email}`">{{ t('footer.email') }}</a>
          <a v-if="contact?.support_url" :href="contact.support_url" :target="contact.support_url.startsWith('https://') ? '_blank' : undefined" rel="noopener noreferrer">{{ t('footer.supportUrl') }}</a>
        </div>

        <div class="st-footer-col">
          <h4>{{ t('vault.footer.legal') }}</h4>
          <RouterLink to="/terms">{{ t('footer.terms') }}</RouterLink>
          <RouterLink to="/privacy">{{ t('footer.privacy') }}</RouterLink>
          <a v-for="link in footerLinks" :key="link.name" :href="link.url || 'javascript:void(0)'" :target="link.url ? '_blank' : undefined" rel="noopener noreferrer">{{ link.name }}</a>
        </div>
      </div>

      <div class="st-container st-footer-bottom">
        <span>© {{ year }} {{ brandName }}</span>
        <span class="st-footer-langs">简体中文 · 繁體 · English</span>
        <a href="https://github.com/dujiao-next" target="_blank" rel="noopener noreferrer" aria-label="Dujiao-Next on GitHub">Dujiao-Next</a>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import {
  Search, Moon, Sun, ShoppingCart, Languages, Menu, X, User, LogOut, LayoutGrid,
} from 'lucide-vue-next'
import { useAppStore } from '../../../stores/app'
import { useCartStore } from '../../../stores/cart'
import { useUserAuthStore } from '../../../stores/userAuth'
import { useNavConfig, type NavItem } from '../../../composables/useNavConfig'
import { categoryAPI } from '../../../api'
import { useTheme } from '../../../utils/theme'
import { getImageUrl } from '../../../utils/image'
import { getLocalizedText } from '../../../utils/resellerSiteConfig'
// 本地自托管字体（替代 Google Fonts CDN），仅 vault 模板加载
import '@fontsource/rubik/latin-400.css'
import '@fontsource/rubik/latin-500.css'
import '@fontsource/rubik/latin-600.css'
import '@fontsource/rubik/latin-700.css'
import '@fontsource/rubik/latin-800.css'
import '@fontsource/nunito-sans/latin-400.css'
import '@fontsource/nunito-sans/latin-500.css'
import '@fontsource/nunito-sans/latin-600.css'
import '@fontsource/nunito-sans/latin-700.css'
import '../styles/vault.css'
import '../styles/store.css'

const { t } = useI18n()
const appStore = useAppStore()
const cartStore = useCartStore()
const userAuthStore = useUserAuthStore()
const { theme, toggleTheme } = useTheme()

const langOpen = ref(false)
const moreOpen = ref(false)
const langEl = ref<HTMLElement | null>(null)
const moreEl = ref<HTMLElement | null>(null)

const year = new Date().getFullYear()

const brandName = computed(() => String(appStore.config?.brand?.site_name || '').trim() || 'D&J Studio')
const brandLogo = computed(() => {
  const raw = String(appStore.config?.brand?.site_logo || '').trim()
  return raw ? getImageUrl(raw) : ''
})
const brandDescription = computed(() => {
  const desc = appStore.config?.brand?.site_description
  if (desc && typeof desc === 'object') {
    const val = (desc as Record<string, string>)[appStore.locale] || (desc as Record<string, string>)['zh-CN'] || ''
    return typeof val === 'string' ? val.trim() : ''
  }
  return ''
})

const { isListMode, blogEnabled, noticeEnabled, aboutEnabled, secondaryNavItems } = useNavConfig()

const menuItems = computed<NavItem[]>(() => {
  const items: NavItem[] = []
  // 列表模式下首页就是商品列表，/products 与首页同页，不再单列一项
  if (!isListMode.value) {
    items.push({ key: 'products', path: '/products', label: t('products.allCategories'), icon: LayoutGrid, type: 'route', target: '_self' })
  }
  // 内置（博客/公告/关于）+ 后台自定义导航项
  items.push(...secondaryNavItems.value)
  return items
})

/** 后台配置的自定义页脚链接 */
const footerLinks = computed(() => {
  const links = appStore.config?.footer_links
  if (!Array.isArray(links)) return []
  return links
    .map((item: { name?: unknown; url?: unknown }) => ({
      name: typeof item?.name === 'string' ? item.name.trim() : getLocalizedText(item?.name as Record<string, string>, appStore.locale),
      url: String(item?.url || '').trim(),
    }))
    .filter((item) => item.name)
})

const contact = computed(() => appStore.config?.contact as { telegram?: string; whatsapp?: string; email?: string; support_url?: string } | undefined)

const cartCount = computed(() => cartStore.totalItems)

const languages = [
  { code: 'zh-CN', name: '简体中文' },
  { code: 'zh-TW', name: '繁體中文' },
  { code: 'en-US', name: 'English' },
]

const changeLanguage = (code: string) => {
  appStore.setLocale(code)
  langOpen.value = false
}

const route = useRoute()
const isHome = computed(() => route.path === '/')
const isArticle = computed(() => /^\/blog\/[^/]+$/.test(route.path))
const scrolled = ref(false)

/* ---------- 桌面端下拉面板 ---------- */
type FlyoutCategory = { slug: string; name: string }
const openFlyout = ref<string | null>(null)
const flyoutCategories = ref<FlyoutCategory[]>([])
let flyoutTimer: ReturnType<typeof setTimeout> | null = null

const cancelCloseFlyout = () => {
  if (flyoutTimer) {
    clearTimeout(flyoutTimer)
    flyoutTimer = null
  }
}

const scheduleCloseFlyout = () => {
  cancelCloseFlyout()
  flyoutTimer = setTimeout(() => { openFlyout.value = null }, 160)
}

const closeFlyout = () => {
  cancelCloseFlyout()
  openFlyout.value = null
}

/** 只有「全部商品」有下拉，其余导航保持普通链接 */
const openFlyoutFor = (item: NavItem) => {
  cancelCloseFlyout()
  openFlyout.value = item.key === 'products' ? 'products' : null
}

const loadFlyoutCategories = async () => {
  try {
    const res = await categoryAPI.list()
    const list = Array.isArray(res.data?.data) ? res.data.data : []
    flyoutCategories.value = list
      .filter((c: any) => !c.parent_id)
      .slice(0, 8)
      .map((c: any) => ({ slug: c.slug, name: getLocalizedText(c.name, appStore.locale) }))
  } catch {
    /* 分类拉取失败时面板只显示固定链接 */
  }
}
const onScroll = () => { scrolled.value = window.scrollY > 4 }

const toggleLang = () => { langOpen.value = !langOpen.value; moreOpen.value = false }
const toggleMore = () => { moreOpen.value = !moreOpen.value; langOpen.value = false }

const onEscape = (e: KeyboardEvent) => {
  if (e.key === 'Escape') closeFlyout()
}

const onDocClick = (e: MouseEvent) => {
  const target = e.target as Node
  if (langOpen.value && langEl.value && !langEl.value.contains(target)) langOpen.value = false
  if (moreOpen.value && moreEl.value && !moreEl.value.contains(target)) moreOpen.value = false
}

onMounted(() => {
  document.addEventListener('click', onDocClick)
  void loadFlyoutCategories()
  document.addEventListener('keydown', onEscape)
  window.addEventListener('scroll', onScroll, { passive: true })
  onScroll()
  // Teleport 到 body 的浮层（Toast / ConfirmDialog / 公告弹窗 / Select 下拉）在
  // .vault-scope 之外，靠 body 上的这个 class 拿到 vault 配色，详见 styles/vault.css
  document.body.classList.add('vault-tokens')
})
onUnmounted(() => {
  document.removeEventListener('click', onDocClick)
  window.removeEventListener('scroll', onScroll)
  document.removeEventListener('keydown', onEscape)
  cancelCloseFlyout()
  document.body.classList.remove('vault-tokens')
})
</script>
