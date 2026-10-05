<template>
  <div class="atlas-scope flex min-h-screen flex-col">
    <!-- ==================== 顶栏工具条 ==================== -->
    <!-- 参考站是「工具条 + 主导航」两层：工具条放语言/币种与账户类入口，
         主导航只放商品分类。分开的好处是主导航不被账户操作挤占。 -->
    <div class="atlas-utility">
      <div class="atlas-shell flex h-9 items-center gap-3 text-[12px]">
        <!-- 语言 / 币种 -->
        <div class="relative" ref="langEl">
          <button
            class="flex items-center gap-1.5 text-[color:var(--footer-ink)] transition-colors hover:text-[color:var(--footer-ink-strong)]"
            type="button"
            @click="toggleLang"
          >
            <Languages class="h-3.5 w-3.5" />
            <span>{{ currentLanguageName }}</span>
            <span class="opacity-40">|</span>
            <span>{{ currencyLabel }}</span>
          </button>
          <div v-if="langOpen" class="atlas-menu">
            <span class="atlas-menu__label">{{ t('navbar.selectLanguage') }}</span>
            <button
              v-for="lang in languages"
              :key="lang.code"
              class="atlas-menu__item justify-between"
              :class="appStore.locale === lang.code ? 'text-[color:var(--accent)]' : ''"
              @click="changeLanguage(lang.code)"
            >
              {{ lang.name }}
              <span v-if="appStore.locale === lang.code" class="h-1.5 w-1.5 rounded-full bg-[color:var(--accent)]"></span>
            </button>
          </div>
        </div>

        <div class="ml-auto flex items-center gap-4">
          <RouterLink v-if="affiliateEnabled" to="/affiliate" class="atlas-utility__link">
            {{ t('atlas.nav.affiliate') }}
          </RouterLink>
          <RouterLink to="/guest/orders" class="atlas-utility__link">
            {{ t('navbar.guestOrders') }}
          </RouterLink>
          <RouterLink v-if="!userAuthStore.isAuthenticated" to="/auth/login" class="atlas-utility__link">
            {{ t('atlas.nav.loginOrRegister') }}
          </RouterLink>
          <template v-else>
            <RouterLink to="/me" class="atlas-utility__link">{{ t('navbar.personalCenter') }}</RouterLink>
            <button class="atlas-utility__link" type="button" @click="userAuthStore.logout()">
              {{ t('navbar.logout') }}
            </button>
          </template>
        </div>
      </div>
    </div>

    <!-- ==================== 主导航 ==================== -->
    <header class="atlas-header sticky top-0 z-50">
      <div class="atlas-shell flex h-[64px] items-center gap-4">
        <!-- 品牌 -->
        <RouterLink
          class="flex flex-none items-center gap-2 text-[17px] font-bold tracking-[-0.02em] text-[color:var(--ink)]"
          to="/"
          :title="brandName"
        >
          <img v-if="brandLogo" :src="brandLogo" :alt="brandName" class="h-7 max-w-[120px] object-contain" />
          <template v-else>
            <span class="grid h-7 w-7 flex-none place-items-center rounded-[var(--r-sm)] bg-[color:var(--btn)] text-[13px] font-bold text-[color:var(--on-btn)]">
              {{ brandInitial }}
            </span>
            <span class="truncate">{{ brandName }}</span>
          </template>
        </RouterLink>

        <!-- 分类导航：参考站把热门品类直接铺在主导航里 -->
        <nav class="mx-auto flex items-center gap-0.5 max-[1024px]:hidden">
          <RouterLink
            v-if="!isListMode"
            to="/products"
            class="atlas-navlink"
            active-class="atlas-navlink--on"
          >{{ t('nav.home') }}</RouterLink>
          <RouterLink
            v-for="cat in topCategories"
            :key="cat.id"
            :to="`/categories/${cat.slug}`"
            class="atlas-navlink"
            active-class="atlas-navlink--on"
          >{{ getLocalizedText(cat.name) }}</RouterLink>
          <RouterLink
            v-for="item in secondaryNavItems"
            :key="item.key"
            :to="item.path"
            class="atlas-navlink"
            active-class="atlas-navlink--on"
          >{{ item.label }}</RouterLink>
        </nav>

        <!-- 右侧：购物车 + 移动端菜单 -->
        <div class="ml-auto flex items-center gap-2 max-[1024px]:ml-0">
          <RouterLink
            class="relative inline-flex h-9 items-center gap-2 rounded-[var(--r-sm)] bg-[color:var(--btn)] px-3.5 text-[13px] font-semibold text-[color:var(--on-btn)] transition-colors hover:bg-[color:var(--btn-hover)]"
            to="/cart"
            :aria-label="t('navbar.cart')"
          >
            <ShoppingCart class="h-4 w-4" />
            <span class="max-[420px]:hidden">{{ t('navbar.cart') }}</span>
            <span
              v-if="cartCount > 0"
              class="grid h-[18px] min-w-[18px] place-items-center rounded-[var(--r-pill)] bg-[color:var(--accent)] px-1 text-[11px] font-bold text-white"
            >{{ cartCount > 99 ? '99+' : cartCount }}</span>
          </RouterLink>

          <div class="relative hidden max-[1024px]:block" ref="moreEl">
            <button
              class="grid h-9 w-9 place-items-center rounded-[var(--r-sm)] text-[color:var(--ink-2)] transition-colors hover:bg-[color:var(--surface-2)]"
              type="button"
              :aria-label="t('navbar.more')"
              @click="toggleMore"
            >
              <X v-if="moreOpen" class="pointer-events-none h-[18px] w-[18px]" />
              <Menu v-else class="pointer-events-none h-[18px] w-[18px]" />
            </button>
            <div v-if="moreOpen" class="atlas-menu min-w-[220px]">
              <RouterLink v-if="!isListMode" to="/products" class="atlas-menu__item" @click="moreOpen = false">{{ t('nav.home') }}</RouterLink>
              <RouterLink
                v-for="cat in topCategories"
                :key="`m-${cat.id}`"
                :to="`/categories/${cat.slug}`"
                class="atlas-menu__item"
                @click="moreOpen = false"
              >{{ getLocalizedText(cat.name) }}</RouterLink>
              <template v-for="item in secondaryNavItems" :key="`ms-${item.key}`">
                <RouterLink v-if="item.type === 'route'" :to="item.path" class="atlas-menu__item" @click="moreOpen = false">{{ item.label }}</RouterLink>
                <a v-else :href="item.path" :target="item.target" rel="noopener noreferrer" class="atlas-menu__item" @click="moreOpen = false">{{ item.label }}</a>
              </template>
              <RouterLink to="/guest/orders" class="atlas-menu__item" @click="moreOpen = false">{{ t('navbar.guestOrders') }}</RouterLink>
              <RouterLink v-if="affiliateEnabled" to="/affiliate" class="atlas-menu__item" @click="moreOpen = false">{{ t('atlas.nav.affiliate') }}</RouterLink>
              <button class="atlas-menu__item text-left" @click="toggleTheme">{{ t('resellerConsole.common.toggleTheme') }}</button>
            </div>
          </div>
        </div>
      </div>
    </header>

    <!-- ==================== 页面内容 ==================== -->
    <main class="flex-1">
      <slot />
    </main>

    <!-- ==================== 页脚（纯黑） ==================== -->
    <footer class="mt-[var(--gap-block)] bg-[color:var(--footer-bg)]">
      <div class="atlas-shell grid grid-cols-1 gap-8 pb-10 pt-[52px] sm:grid-cols-2 lg:grid-cols-[1.8fr_repeat(3,1fr)]">
        <div>
          <RouterLink class="inline-flex items-center gap-2 text-[17px] font-bold text-[color:var(--footer-ink-strong)]" to="/">
            <img v-if="brandLogo" :src="brandLogo" :alt="brandName" class="h-7 max-w-[140px] object-contain" />
            <template v-else>
              <span class="grid h-7 w-7 flex-none place-items-center rounded-[var(--r-sm)] bg-white text-[13px] font-bold text-black">{{ brandInitial }}</span>
              <span>{{ brandName }}</span>
            </template>
          </RouterLink>
          <p class="mt-3 max-w-[38ch] text-[13px] leading-relaxed text-[color:var(--footer-ink)]">
            {{ brandDescription || t('atlas.footer.tagline') }}
          </p>
        </div>

        <div>
          <h4 class="atlas-foot-head">{{ t('atlas.footer.shop') }}</h4>
          <RouterLink v-if="!isListMode" to="/products" class="atlas-foot-link">{{ t('products.allCategories') }}</RouterLink>
          <RouterLink v-for="cat in topCategories" :key="`f-${cat.id}`" :to="`/categories/${cat.slug}`" class="atlas-foot-link">
            {{ getLocalizedText(cat.name) }}
          </RouterLink>
        </div>

        <div>
          <h4 class="atlas-foot-head">{{ t('atlas.footer.account') }}</h4>
          <RouterLink to="/me" class="atlas-foot-link">{{ t('navbar.personalCenter') }}</RouterLink>
          <RouterLink to="/guest/orders" class="atlas-foot-link">{{ t('navbar.guestOrders') }}</RouterLink>
          <RouterLink to="/cart" class="atlas-foot-link">{{ t('navbar.cart') }}</RouterLink>
        </div>

        <div>
          <h4 class="atlas-foot-head">{{ t('atlas.footer.support') }}</h4>
          <RouterLink v-if="aboutEnabled" to="/about" class="atlas-foot-link">{{ t('nav.about') }}</RouterLink>
          <RouterLink v-if="noticeEnabled" to="/notice" class="atlas-foot-link">{{ t('nav.notice') }}</RouterLink>
          <RouterLink v-if="blogEnabled" to="/blog" class="atlas-foot-link">{{ t('nav.blog') }}</RouterLink>
          <a v-if="contact?.telegram" :href="contact.telegram" target="_blank" rel="noopener noreferrer" class="atlas-foot-link">Telegram</a>
          <a v-if="contact?.whatsapp" :href="contact.whatsapp" target="_blank" rel="noopener noreferrer" class="atlas-foot-link">WhatsApp</a>
          <RouterLink to="/terms" class="atlas-foot-link">{{ t('footer.terms') }}</RouterLink>
          <RouterLink to="/privacy" class="atlas-foot-link">{{ t('footer.privacy') }}</RouterLink>
          <a
            v-for="link in footerLinks"
            :key="link.name"
            :href="link.url || 'javascript:void(0)'"
            :target="link.url ? '_blank' : undefined"
            rel="noopener noreferrer"
            class="atlas-foot-link"
          >{{ link.name }}</a>
        </div>
      </div>

      <div class="atlas-shell flex flex-wrap items-center justify-between gap-3 border-t border-[color:var(--footer-line)] py-4 text-[12px] text-[color:var(--footer-ink)]">
        <span>© {{ year }} {{ brandName }}</span>
        <span>{{ t('atlas.footer.builtOn') }}</span>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Languages, Menu, ShoppingCart, X } from 'lucide-vue-next'
import { useAppStore } from '../../../stores/app'
import { useCartStore } from '../../../stores/cart'
import { useUserAuthStore } from '../../../stores/userAuth'
import { useNavConfig } from '../../../composables/useNavConfig'
import { useLocalized } from '../../../composables/useProduct'
import { categoryAPI } from '../../../api/product'
import type { PublicCategory } from '../../../utils/category'
import { useTheme } from '../../../utils/theme'
import { getImageUrl } from '../../../utils/image'
import { getLocalizedText as getResellerText } from '../../../utils/resellerSiteConfig'
import '../styles/atlas.css'

const { t } = useI18n()
const appStore = useAppStore()
const cartStore = useCartStore()
const userAuthStore = useUserAuthStore()
const { getLocalizedText } = useLocalized()
const { toggleTheme } = useTheme()

const langOpen = ref(false)
const moreOpen = ref(false)
const langEl = ref<HTMLElement | null>(null)
const moreEl = ref<HTMLElement | null>(null)

const year = new Date().getFullYear()

const brandName = computed(() => String(appStore.config?.brand?.site_name || '').trim() || 'Atlas Store')
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

/**
 * 主导航里的热门品类。
 * 参考站把品类直链铺在主导航 —— 但真实站点往往有几十个分类，只取一级分类的前 8 个，
 * 其余走 /products 的侧栏，避免导航被撑爆。
 * 数据来自公开分类接口（与 /products 页同源），不读站点配置里的私有字段。
 */
const categories = ref<PublicCategory[]>([])
const topCategories = computed(() =>
  categories.value.filter((c) => !c.parent_id || c.parent_id === 0).slice(0, 8),
)

const affiliateEnabled = computed(() => appStore.config?.affiliate_enabled !== false)
const currencyLabel = computed(() => String(appStore.config?.currency || 'CNY').toUpperCase())

const languages = [
  { code: 'zh-CN', name: '简体中文' },
  { code: 'zh-TW', name: '繁體中文' },
  { code: 'en-US', name: 'English' },
]
const currentLanguageName = computed(
  () => languages.find((l) => l.code === appStore.locale)?.name || '简体中文',
)

const footerLinks = computed(() => {
  const links = appStore.config?.footer_links
  if (!Array.isArray(links)) return []
  return links
    .map((item: { name?: unknown; url?: unknown }) => ({
      name: typeof item?.name === 'string' ? item.name.trim() : getResellerText(item?.name as Record<string, string>, appStore.locale),
      url: String(item?.url || '').trim(),
    }))
    .filter((item) => item.name)
})

const contact = computed(() => appStore.config?.contact as { telegram?: string; whatsapp?: string } | undefined)
const cartCount = computed(() => cartStore.totalItems)

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

onMounted(async () => {
  document.addEventListener('click', onDocClick)
  // Teleport 到 body 的浮层（Toast / ConfirmDialog / 公告弹窗 / Select 下拉）在
  // .atlas-scope 之外，靠 body 上的这个 class 拿到 atlas 配色，详见 styles/atlas.css
  document.body.classList.add('atlas-tokens')

  // 分类只用于导航，失败时静默降级（导航仍保留「全部商品」入口）
  try {
    const res: any = await categoryAPI.list()
    const list = res?.data?.data
    if (Array.isArray(list)) categories.value = list
  } catch {
    /* 忽略：导航降级为仅「全部商品」 */
  }
})

onUnmounted(() => {
  document.removeEventListener('click', onDocClick)
  document.body.classList.remove('atlas-tokens')
})
</script>

<style scoped>
/* 顶栏工具条：参考站是深色窄条，视觉上把「账户类」操作和商品导航分开 */
.atlas-utility {
  background: #1a1a1a;
}

.atlas-utility__link {
  color: var(--footer-ink);
  transition: color var(--dur) var(--ease);
}

.atlas-utility__link:hover {
  color: var(--footer-ink-strong);
}

/* 主导航：白底 + 底部细线，sticky 后靠阴影区分层 */
.atlas-header {
  background: var(--surface);
  border-bottom: 1px solid var(--border);
}

.atlas-navlink {
  padding: 8px 10px;
  border-radius: var(--r-sm);
  color: var(--ink-2);
  font-size: 13.5px;
  font-weight: 500;
  white-space: nowrap;
  transition: color var(--dur) var(--ease), background-color var(--dur) var(--ease);
}

.atlas-navlink:hover {
  background: var(--surface-2);
  color: var(--ink);
}

.atlas-navlink--on {
  color: var(--accent);
  font-weight: 600;
}
</style>
