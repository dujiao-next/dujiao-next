<template>
  <div class="luna-scope">
    <!-- 顶栏 -->
    <header class="sticky top-0 z-50 border-b border-white/40 bg-[color:var(--ui-bg-overlay)] backdrop-blur-md dark:border-white/5">
      <div class="mx-auto flex h-16 w-full max-w-[1100px] items-center gap-3 px-4 sm:gap-4 sm:px-6">
        <RouterLink class="inline-flex min-w-0 items-center gap-2.5 text-[17px] font-bold text-foreground" to="/" :title="brandName">
          <!-- 店名文字始终在（窄屏有 logo 时只对读屏可见），logo 不再重复朗读店名 -->
          <img v-if="brandLogo" :src="brandLogo" alt="" class="h-9 max-w-[120px] flex-none object-contain sm:max-w-[150px]" />
          <span class="truncate" :class="{ 'max-sm:sr-only': brandLogo }">{{ brandName }}</span>
        </RouterLink>

        <nav class="flex min-w-0 gap-0.5 max-[900px]:hidden">
          <template v-for="item in secondaryNavItems" :key="item.key">
            <RouterLink
              v-if="item.type === 'route'"
              :to="item.path"
              class="whitespace-nowrap rounded-full px-3 py-1.5 text-sm font-medium text-muted-foreground transition-colors hover:bg-primary/10 hover:text-primary"
              active-class="!text-primary"
            >{{ item.label }}</RouterLink>
            <a
              v-else
              :href="item.path"
              :target="item.target"
              rel="noopener noreferrer"
              class="whitespace-nowrap rounded-full px-3 py-1.5 text-sm font-medium text-muted-foreground transition-colors hover:bg-primary/10 hover:text-primary"
            >{{ item.label }}</a>
          </template>
        </nav>

        <div class="ml-auto flex flex-none items-center gap-1.5">
          <button class="grid h-9 w-9 flex-none place-items-center rounded-full text-muted-foreground transition-colors hover:bg-primary/10 hover:text-primary" type="button" :aria-label="t('resellerConsole.common.toggleTheme')" @click="toggleTheme">
            <Sun v-if="theme === 'dark'" class="h-[18px] w-[18px]" />
            <Moon v-else class="h-[18px] w-[18px]" />
          </button>

          <!-- 语言切换（桌面） -->
          <div ref="langEl" class="relative max-[900px]:hidden">
            <button class="grid h-9 w-9 flex-none place-items-center rounded-full text-muted-foreground transition-colors hover:bg-primary/10 hover:text-primary" type="button" :aria-label="t('navbar.selectLanguage')" :aria-expanded="langOpen" @click="toggleLang">
              <Languages class="h-[18px] w-[18px]" />
            </button>
            <div v-if="langOpen" class="absolute right-0 top-[calc(100%+8px)] z-[60] flex min-w-[160px] flex-col gap-0.5 luna-panel p-1.5 shadow-[var(--luna-shadow-lg)]">
              <button v-for="lang in languages" :key="lang.code" type="button" class="flex w-full items-center justify-between gap-2.5 rounded-lg px-3 py-2 text-left text-sm font-medium transition-colors hover:bg-secondary" :class="appStore.locale === lang.code ? 'text-primary' : 'text-muted-foreground'" @click="changeLanguage(lang.code)">
                {{ lang.name }}
                <Check v-if="appStore.locale === lang.code" class="h-4 w-4" />
              </button>
            </div>
          </div>

          <RouterLink class="relative grid h-9 w-9 flex-none place-items-center rounded-full text-muted-foreground transition-colors hover:bg-primary/10 hover:text-primary" to="/cart" :aria-label="t('navbar.cart')">
            <ShoppingCart class="h-[18px] w-[18px]" />
            <span v-if="cartCount > 0" class="absolute -right-0.5 -top-0.5 grid h-[18px] min-w-[18px] place-items-center rounded-full bg-destructive px-1 text-[11px] font-bold text-primary-foreground">{{ cartCount }}</span>
          </RouterLink>

          <!-- 登录 / 退出（桌面） -->
          <RouterLink v-if="!userAuthStore.isAuthenticated" class="rounded-full px-3 py-1.5 text-sm font-medium text-muted-foreground transition-colors hover:bg-primary/10 hover:text-primary max-[900px]:hidden" to="/auth/login">{{ t('navbar.login') }}</RouterLink>
          <button v-else type="button" class="grid h-9 w-9 flex-none place-items-center rounded-full text-muted-foreground transition-colors hover:bg-destructive/10 hover:text-destructive max-[900px]:hidden" :aria-label="t('navbar.logout')" :title="t('navbar.logout')" @click="userAuthStore.logout()"><LogOut class="h-[18px] w-[18px]" /></button>

          <!-- luna 的标志性渐变胶囊：游客查单 / 个人中心 -->
          <RouterLink class="luna-gradient inline-flex h-9 flex-none items-center gap-1.5 rounded-full px-4 text-sm font-semibold transition hover:brightness-105 max-[520px]:hidden" :to="accountEntry.path">
            <component :is="accountEntry.icon" class="h-4 w-4" />
            {{ accountEntry.label }}
          </RouterLink>

          <!-- 移动端：更多菜单 -->
          <div ref="moreEl" class="relative hidden max-[900px]:block">
            <button class="grid h-9 w-9 flex-none place-items-center rounded-full text-muted-foreground transition-colors hover:bg-primary/10 hover:text-primary" type="button" :aria-label="t('navbar.more')" :aria-expanded="moreOpen" @click="toggleMore">
              <Menu v-if="!moreOpen" class="pointer-events-none h-[18px] w-[18px]" />
              <X v-else class="pointer-events-none h-[18px] w-[18px]" />
            </button>
            <div v-if="moreOpen" class="absolute right-0 top-[calc(100%+8px)] z-[60] flex min-w-[176px] flex-col gap-0.5 luna-panel p-1.5 shadow-[var(--luna-shadow-lg)]">
              <RouterLink :to="accountEntry.path" class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2.5 text-sm font-medium text-primary transition-colors hover:bg-secondary" @click="moreOpen = false">{{ accountEntry.label }}</RouterLink>
              <template v-for="item in secondaryNavItems" :key="`m-${item.key}`">
                <RouterLink v-if="item.type === 'route'" :to="item.path" class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2.5 text-sm font-medium text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground" @click="moreOpen = false">{{ item.label }}</RouterLink>
                <a v-else :href="item.path" :target="item.target" rel="noopener noreferrer" class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2.5 text-sm font-medium text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground" @click="moreOpen = false">{{ item.label }}</a>
              </template>
              <RouterLink v-if="!userAuthStore.isAuthenticated" to="/auth/login" class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2.5 text-sm font-medium text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground" @click="moreOpen = false">{{ t('navbar.login') }}</RouterLink>
              <button v-else type="button" class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2.5 text-left text-sm font-medium text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground" @click="userAuthStore.logout(); moreOpen = false">{{ t('navbar.logout') }}</button>
              <span class="mt-1 border-t px-3 pb-0.5 pt-2 text-[11px] font-semibold text-muted-foreground">{{ t('navbar.selectLanguage') }}</span>
              <button v-for="lang in languages" :key="`ml-${lang.code}`" type="button" class="flex w-full items-center justify-between gap-2.5 rounded-lg px-3 py-2.5 text-left text-sm font-medium transition-colors hover:bg-secondary" :class="appStore.locale === lang.code ? 'text-primary' : 'text-muted-foreground'" @click="changeLanguage(lang.code); moreOpen = false">
                {{ lang.name }}
                <Check v-if="appStore.locale === lang.code" class="h-4 w-4" />
              </button>
            </div>
          </div>
        </div>
      </div>
    </header>

    <!-- 页面内容 -->
    <main class="flex-1">
      <slot />
    </main>

    <!-- 页脚：luna 是居中的单栏页脚 -->
    <footer class="px-4 pb-8 pt-10 text-center text-[13px] text-muted-foreground">
      <div class="mx-auto flex w-full max-w-[1100px] flex-col items-center gap-3">
        <div class="flex flex-wrap items-center justify-center gap-x-5 gap-y-1.5">
          <RouterLink to="/terms" class="transition-colors hover:text-primary">{{ t('footer.terms') }}</RouterLink>
          <RouterLink to="/privacy" class="transition-colors hover:text-primary">{{ t('footer.privacy') }}</RouterLink>
          <a v-for="link in footerLinks" :key="link.name" :href="link.url || 'javascript:void(0)'" :target="link.url ? '_blank' : undefined" rel="noopener noreferrer" class="transition-colors hover:text-primary">{{ link.name }}</a>
          <a v-if="contact?.telegram" :href="contact.telegram" target="_blank" rel="noopener noreferrer" class="inline-flex items-center gap-1 transition-colors hover:text-primary"><Send class="h-3.5 w-3.5" /> Telegram</a>
          <a v-if="contact?.whatsapp" :href="contact.whatsapp" target="_blank" rel="noopener noreferrer" class="inline-flex items-center gap-1 transition-colors hover:text-primary"><MessageCircle class="h-3.5 w-3.5" /> WhatsApp</a>
          <a v-if="contact?.email" :href="`mailto:${contact.email}`" class="inline-flex items-center gap-1 transition-colors hover:text-primary"><Mail class="h-3.5 w-3.5" /> {{ t('footer.email') }}</a>
          <a v-if="contact?.support_url" :href="contact.support_url" :target="contact.support_url.startsWith('https://') ? '_blank' : undefined" rel="noopener noreferrer" class="inline-flex items-center gap-1 transition-colors hover:text-primary"><Headphones class="h-3.5 w-3.5" /> {{ t('footer.supportUrl') }}</a>
        </div>
        <div class="flex flex-wrap items-center justify-center gap-x-3 gap-y-1">
          <span>© {{ year }} {{ brandName }}</span>
          <a href="https://github.com/dujiao-next" target="_blank" rel="noopener noreferrer" aria-label="Dujiao-Next on GitHub" class="inline-flex items-center gap-1 transition-colors hover:text-primary">
            <Github class="h-3.5 w-3.5" />
            <span>Dujiao-Next</span>
          </a>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Check, ClipboardList, Github, Headphones, Languages, LogOut, Mail, Menu, MessageCircle, Moon,
  Send, ShoppingCart, Sun, User, X,
} from 'lucide-vue-next'
import { useAppStore } from '../../../stores/app'
import { useCartStore } from '../../../stores/cart'
import { useUserAuthStore } from '../../../stores/userAuth'
import { useNavConfig } from '../../../composables/useNavConfig'
import { useTheme } from '../../../utils/theme'
import { getImageUrl } from '../../../utils/image'
import { getLocalizedText } from '../../../utils/resellerSiteConfig'
import '../styles/luna.css'

const { t } = useI18n()
const appStore = useAppStore()
const cartStore = useCartStore()
const userAuthStore = useUserAuthStore()
const { theme, toggleTheme } = useTheme()
// luna 的首页就是商品页，导航里不单列「商品」，只取内置（博客/公告/关于）+ 后台自定义项
const { secondaryNavItems } = useNavConfig()

const langOpen = ref(false)
const moreOpen = ref(false)
const langEl = ref<HTMLElement | null>(null)
const moreEl = ref<HTMLElement | null>(null)

const year = new Date().getFullYear()

const brandName = computed(() => String(appStore.config?.brand?.site_name || '').trim() || 'Dujiao-Next')
const brandLogo = computed(() => {
  const raw = String(appStore.config?.brand?.site_logo || '').trim()
  return raw ? getImageUrl(raw) : ''
})

/** 渐变胶囊入口：游客去查单，已登录去个人中心 */
const accountEntry = computed(() => userAuthStore.isAuthenticated
  ? { path: '/me', label: t('navbar.personalCenter'), icon: User }
  : { path: '/guest/orders', label: t('navbar.guestOrders'), icon: ClipboardList })

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

const toggleLang = () => { langOpen.value = !langOpen.value; moreOpen.value = false }
const toggleMore = () => { moreOpen.value = !moreOpen.value; langOpen.value = false }

const onDocClick = (e: MouseEvent) => {
  const target = e.target as Node
  if (langOpen.value && langEl.value && !langEl.value.contains(target)) langOpen.value = false
  if (moreOpen.value && moreEl.value && !moreEl.value.contains(target)) moreOpen.value = false
}

const onKeydown = (e: KeyboardEvent) => {
  if (e.key !== 'Escape' || (!langOpen.value && !moreOpen.value)) return
  langOpen.value = false
  moreOpen.value = false
}

onMounted(() => {
  document.addEventListener('click', onDocClick)
  document.addEventListener('keydown', onKeydown)
  // Teleport 到 body 的浮层在 .luna-scope 之外，靠 body 上的这个 class 拿到 luna 配色，详见 styles/luna.css
  document.body.classList.add('luna-tokens')
})
onUnmounted(() => {
  document.removeEventListener('click', onDocClick)
  document.removeEventListener('keydown', onKeydown)
  document.body.classList.remove('luna-tokens')
})
</script>
