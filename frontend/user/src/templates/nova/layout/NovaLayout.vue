<template>
  <div class="nova-scope">
    <!-- ==================== 顶栏 ==================== -->
    <header
      class="sticky top-0 z-50 border-b transition-[background-color,border-color] duration-300"
      :class="stuck ? 'nova-glass border-border' : 'border-transparent'"
    >
      <div class="mx-auto flex h-[72px] w-full max-w-[1200px] items-center gap-3 px-4 sm:gap-6 sm:px-6">
        <!-- 品牌 -->
        <RouterLink
          class="inline-flex min-w-0 flex-none items-center gap-2.5 text-[17px] font-semibold tracking-[-0.03em] text-foreground"
          to="/"
          :title="brandName"
        >
          <img v-if="brandLogo" :src="brandLogo" :alt="brandName" class="h-8 max-w-[120px] object-contain sm:max-w-[160px]" />
          <template v-else>
            <span class="grid h-8 w-8 flex-none place-items-center rounded-[10px] bg-foreground text-[13px] font-semibold text-background">
              {{ brandInitial }}
            </span>
            <span class="truncate">{{ brandName }}</span>
          </template>
        </RouterLink>

        <!-- 主导航：nova 用干净文字链 + 极轻的悬浮底，不用描边胶囊 -->
        <nav class="mx-auto flex items-center gap-1 max-[900px]:hidden">
          <template v-for="item in menuItems" :key="item.key">
            <RouterLink
              v-if="item.type === 'route'"
              :to="item.path"
              class="whitespace-nowrap rounded-full px-3.5 py-2 text-[14px] font-medium text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
              active-class="!bg-secondary !text-foreground"
            >{{ item.label }}</RouterLink>
            <a
              v-else
              :href="item.path"
              :target="item.target"
              rel="noopener noreferrer"
              class="whitespace-nowrap rounded-full px-3.5 py-2 text-[14px] font-medium text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
            >{{ item.label }}</a>
          </template>
        </nav>

        <!-- 右侧操作 -->
        <div class="ml-auto flex items-center gap-1.5">
          <!-- 语言 -->
          <div class="relative max-[900px]:hidden" ref="langEl">
            <button
              class="grid h-9 w-9 flex-none place-items-center rounded-full text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
              type="button"
              :aria-label="t('navbar.selectLanguage')"
              @click="toggleLang"
            >
              <Languages class="h-[18px] w-[18px]" />
            </button>
            <div v-if="langOpen" class="nova-menu">
              <span class="nova-menu__label">{{ t('navbar.selectLanguage') }}</span>
              <button
                v-for="lang in languages"
                :key="lang.code"
                class="nova-menu__item justify-between"
                :class="appStore.locale === lang.code ? 'text-[color:var(--accent)]' : 'text-muted-foreground'"
                @click="changeLanguage(lang.code)"
              >
                {{ lang.name }}
                <span v-if="appStore.locale === lang.code" class="h-[7px] w-[7px] rounded-full bg-[color:var(--accent)]"></span>
              </button>
            </div>
          </div>

          <!-- 主题 -->
          <button
            class="grid h-9 w-9 flex-none place-items-center rounded-full text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground max-[900px]:hidden"
            type="button"
            :aria-label="t('resellerConsole.common.toggleTheme')"
            @click="toggleTheme"
          >
            <Sun v-if="theme === 'dark'" class="h-[18px] w-[18px]" />
            <Moon v-else class="h-[18px] w-[18px]" />
          </button>

          <!-- 购物车 -->
          <RouterLink
            class="relative inline-flex h-9 flex-none items-center gap-2 rounded-full border border-[color:var(--border)] px-3.5 text-[13.5px] font-medium text-foreground transition-colors hover:border-[color:var(--border-strong)] hover:bg-secondary"
            to="/cart"
            :aria-label="t('navbar.cart')"
          >
            <ShoppingCart class="h-[17px] w-[17px]" />
            <span class="max-[420px]:hidden">{{ t('navbar.cart') }}</span>
            <span
              v-if="cartCount > 0"
              class="grid h-[19px] min-w-[19px] place-items-center rounded-full bg-[color:var(--accent)] px-[5px] text-[11px] font-semibold text-[color:var(--on-accent)]"
            >{{ cartCount > 99 ? '99+' : cartCount }}</span>
          </RouterLink>

          <!-- 账户 -->
          <RouterLink
            v-if="userAuthStore.isAuthenticated"
            class="inline-flex h-9 items-center gap-2 rounded-full bg-[color:var(--accent)] px-4 text-[13.5px] font-medium text-[color:var(--on-accent)] transition-colors hover:bg-[color:var(--accent-strong)] max-[900px]:hidden"
            to="/me"
          >{{ t('navbar.personalCenter') }}</RouterLink>
          <RouterLink
            v-else
            class="inline-flex h-9 items-center gap-2 rounded-full bg-[color:var(--accent)] px-4 text-[13.5px] font-medium text-[color:var(--on-accent)] transition-colors hover:bg-[color:var(--accent-strong)] max-[900px]:hidden"
            to="/auth/login"
          >{{ t('navbar.login') }}</RouterLink>

          <!-- 移动端更多 -->
          <div class="relative hidden max-[900px]:block" ref="moreEl">
            <button
              class="grid h-9 w-9 flex-none place-items-center rounded-full text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
              type="button"
              :aria-label="t('navbar.more')"
              @click="toggleMore"
            >
              <X v-if="moreOpen" class="pointer-events-none h-[18px] w-[18px]" />
              <Menu v-else class="pointer-events-none h-[18px] w-[18px]" />
            </button>
            <div v-if="moreOpen" class="nova-menu min-w-[200px]">
              <template v-for="item in menuItems" :key="`m-${item.key}`">
                <RouterLink v-if="item.type === 'route'" :to="item.path" class="nova-menu__item text-muted-foreground" @click="moreOpen = false">{{ item.label }}</RouterLink>
                <a v-else :href="item.path" :target="item.target" rel="noopener noreferrer" class="nova-menu__item text-muted-foreground" @click="moreOpen = false">{{ item.label }}</a>
              </template>
              <RouterLink v-if="!userAuthStore.isAuthenticated" to="/guest/orders" class="nova-menu__item text-muted-foreground" @click="moreOpen = false">{{ t('navbar.guestOrders') }}</RouterLink>
              <RouterLink v-if="userAuthStore.isAuthenticated" to="/me" class="nova-menu__item text-muted-foreground" @click="moreOpen = false">{{ t('navbar.personalCenter') }}</RouterLink>
              <button v-if="userAuthStore.isAuthenticated" class="nova-menu__item text-left text-muted-foreground" @click="userAuthStore.logout(); moreOpen = false">{{ t('navbar.logout') }}</button>
              <button class="nova-menu__item text-left text-muted-foreground" @click="toggleTheme">{{ t('resellerConsole.common.toggleTheme') }}</button>
              <span class="nova-menu__label">{{ t('navbar.selectLanguage') }}</span>
              <button
                v-for="lang in languages"
                :key="`ml-${lang.code}`"
                class="nova-menu__item justify-between"
                :class="appStore.locale === lang.code ? 'text-[color:var(--accent)]' : 'text-muted-foreground'"
                @click="changeLanguage(lang.code); moreOpen = false"
              >
                {{ lang.name }}
                <span v-if="appStore.locale === lang.code" class="h-[7px] w-[7px] rounded-full bg-[color:var(--accent)]"></span>
              </button>
            </div>
          </div>
        </div>
      </div>
    </header>

    <!-- ==================== 页面内容 ==================== -->
    <main class="flex-1">
      <slot />
    </main>

    <!-- ==================== 页脚 ==================== -->
    <footer class="nova-band mt-[var(--gap-block)] border-t border-border">
      <div class="mx-auto grid w-full max-w-[1200px] grid-cols-1 gap-[32px] px-6 pb-10 pt-[56px] sm:grid-cols-2 lg:grid-cols-[1.8fr_repeat(3,1fr)]">
        <div>
          <RouterLink class="inline-flex items-center gap-2.5 text-[17px] font-semibold tracking-[-0.03em] text-foreground" to="/">
            <img v-if="brandLogo" :src="brandLogo" :alt="brandName" class="h-8 max-w-[160px] object-contain" />
            <template v-else>
              <span class="grid h-8 w-8 flex-none place-items-center rounded-[10px] bg-foreground text-[13px] font-semibold text-background">{{ brandInitial }}</span>
              <span>{{ brandName }}</span>
            </template>
          </RouterLink>
          <p class="mt-3.5 max-w-[38ch] text-[14px] leading-relaxed text-muted-foreground">
            {{ brandDescription || t('nova.footer.tagline') }}
          </p>
        </div>

        <div>
          <h4 class="nova-eyebrow mb-4">{{ t('nova.footer.shop') }}</h4>
          <RouterLink v-if="!isListMode" to="/products" class="nova-foot-link">{{ t('products.allCategories') }}</RouterLink>
          <RouterLink v-if="noticeEnabled" to="/notice" class="nova-foot-link">{{ t('nav.notice') }}</RouterLink>
          <RouterLink v-if="blogEnabled" to="/blog" class="nova-foot-link">{{ t('nav.blog') }}</RouterLink>
        </div>

        <div>
          <h4 class="nova-eyebrow mb-4">{{ t('nova.footer.account') }}</h4>
          <RouterLink to="/me" class="nova-foot-link">{{ t('navbar.personalCenter') }}</RouterLink>
          <RouterLink v-if="!userAuthStore.isAuthenticated" to="/guest/orders" class="nova-foot-link">{{ t('navbar.guestOrders') }}</RouterLink>
          <RouterLink to="/cart" class="nova-foot-link">{{ t('navbar.cart') }}</RouterLink>
        </div>

        <div>
          <h4 class="nova-eyebrow mb-4">{{ t('nova.footer.support') }}</h4>
          <RouterLink v-if="aboutEnabled" to="/about" class="nova-foot-link">{{ t('nav.about') }}</RouterLink>
          <a v-if="contact?.telegram" :href="contact.telegram" target="_blank" rel="noopener noreferrer" class="nova-foot-link">Telegram</a>
          <a v-if="contact?.whatsapp" :href="contact.whatsapp" target="_blank" rel="noopener noreferrer" class="nova-foot-link">WhatsApp</a>
          <RouterLink to="/terms" class="nova-foot-link">{{ t('footer.terms') }}</RouterLink>
          <RouterLink to="/privacy" class="nova-foot-link">{{ t('footer.privacy') }}</RouterLink>
          <a v-for="link in footerLinks" :key="link.name" :href="link.url || 'javascript:void(0)'" :target="link.url ? '_blank' : undefined" rel="noopener noreferrer" class="nova-foot-link">{{ link.name }}</a>
        </div>
      </div>

      <div class="mx-auto flex w-full max-w-[1200px] flex-wrap items-center justify-between gap-3.5 border-t border-border px-6 pb-[30px] pt-[18px] text-[13px] text-muted-foreground">
        <span>© {{ year }} {{ brandName }}</span>
        <span>{{ t('nova.footer.builtOn') }}</span>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Languages, LayoutGrid, Menu, Moon, ShoppingCart, Sun, X,
} from 'lucide-vue-next'
import { useAppStore } from '../../../stores/app'
import { useCartStore } from '../../../stores/cart'
import { useUserAuthStore } from '../../../stores/userAuth'
import { useNavConfig, type NavItem } from '../../../composables/useNavConfig'
import { useTheme } from '../../../utils/theme'
import { getImageUrl } from '../../../utils/image'
import { getLocalizedText } from '../../../utils/resellerSiteConfig'
// Inter 是 nova 设计本体的字体（参考站点即用 Inter）。
// 只在本模板外壳里引入，classic / vault / ember 用户不会加载这份字体资源。
import '@fontsource-variable/inter'
import '../styles/nova.css'

const { t } = useI18n()
const appStore = useAppStore()
const cartStore = useCartStore()
const userAuthStore = useUserAuthStore()
const { theme, toggleTheme } = useTheme()

const langOpen = ref(false)
const moreOpen = ref(false)
const stuck = ref(false)
const langEl = ref<HTMLElement | null>(null)
const moreEl = ref<HTMLElement | null>(null)

const year = new Date().getFullYear()

const brandName = computed(() => String(appStore.config?.brand?.site_name || '').trim() || 'Nova Store')
const brandInitial = computed(() => brandName.value.slice(0, 1).toUpperCase())
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
  return typeof desc === 'string' ? desc.trim() : ''
})

const { isListMode, blogEnabled, noticeEnabled, aboutEnabled, secondaryNavItems } = useNavConfig()

const menuItems = computed<NavItem[]>(() => {
  const items: NavItem[] = []
  // 列表模式下首页即商品列表，不再单列一项
  if (!isListMode.value) {
    items.push({ key: 'products', path: '/products', label: t('products.allCategories'), icon: LayoutGrid, type: 'route', target: '_self' })
  }
  items.push(...secondaryNavItems.value)
  return items
})

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

const contact = computed(() => appStore.config?.contact as { telegram?: string; whatsapp?: string } | undefined)
const cartCount = computed(() => cartStore.totalItems)

const languages = [
  { code: 'zh-CN', name: '简体中文' },
  { code: 'zh-TW', name: '繁體中文' },
  { code: 'en-US', name: 'English' },
]

const changeLanguage = (code: string) => {
  appStore.setLocale(code)
  langOpen.value = false
  moreOpen.value = false
}

const toggleLang = () => { langOpen.value = !langOpen.value; moreOpen.value = false }
const toggleMore = () => { moreOpen.value = !moreOpen.value; langOpen.value = false }

const onDocClick = (e: MouseEvent) => {
  const target = e.target as Node
  if (langOpen.value && langEl.value && !langEl.value.contains(target)) langOpen.value = false
  if (moreOpen.value && moreEl.value && !moreEl.value.contains(target)) moreOpen.value = false
}

const onScroll = () => { stuck.value = window.scrollY > 10 }

onMounted(() => {
  document.addEventListener('click', onDocClick)
  window.addEventListener('scroll', onScroll, { passive: true })
  onScroll()
  // Teleport 到 body 的浮层（Toast / ConfirmDialog / 公告弹窗 / Select 下拉）在
  // .nova-scope 之外，靠 body 上的这个 class 拿到 nova 配色，详见 styles/nova.css
  document.body.classList.add('nova-tokens')
})

onUnmounted(() => {
  document.removeEventListener('click', onDocClick)
  window.removeEventListener('scroll', onScroll)
  document.body.classList.remove('nova-tokens')
})
</script>

<style scoped>
/* 下拉浮层与页脚链接：只在 nova 模板内使用，随本组件一起按需加载 */
.nova-menu {
  position: absolute;
  right: 0;
  top: calc(100% + 8px);
  z-index: 60;
  display: flex;
  min-width: 176px;
  flex-direction: column;
  gap: 2px;
  padding: 8px;
  border: 1px solid var(--border);
  border-radius: var(--r-sm);
  background: var(--surface);
  box-shadow: var(--shadow-lg);
}

.nova-menu__label {
  padding: 6px 12px 2px;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.12em;
  color: var(--ink-3);
}

.nova-menu__item {
  display: flex;
  width: 100%;
  align-items: center;
  gap: 10px;
  padding: 9px 12px;
  border-radius: var(--r-xs);
  font-size: 14px;
  transition: background-color 160ms var(--ease);
}

.nova-menu__item:hover {
  background: var(--surface-2);
  color: var(--ink);
}

.nova-foot-link {
  display: block;
  padding: 5px 0;
  font-size: 14px;
  color: var(--ink-3);
  transition: color 160ms var(--ease);
}

.nova-foot-link:hover {
  color: var(--accent);
}
</style>
