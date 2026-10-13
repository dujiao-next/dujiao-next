<template>
  <div class="atlas-scope flex min-h-screen flex-col">
    <!-- ==================== 公告条（sticky 置顶） ==================== -->
    <div v-if="noticeOpen" class="atlas-notice">
      <div class="atlas-shell atlas-notice__inner">
        <span class="atlas-notice__badge">NOTICE</span>
        <span class="atlas-notice__text">{{ t('atlas.notice.text') }}</span>
        <button class="atlas-notice__close" type="button" :aria-label="t('atlas.notice.close')" @click="closeNotice">
          <X class="h-4 w-4" />
        </button>
      </div>
    </div>

    <!-- ==================== 主行：深色 72px ==================== -->
    <header class="atlas-header">
      <div class="atlas-shell atlas-header__row">
        <!-- 品牌：圆标 + 两行文字 -->
        <RouterLink class="atlas-brand" to="/" :title="brandName">
          <img v-if="brandLogo" class="atlas-brand__logo" :src="brandLogo" :alt="brandName" />
          <span v-else class="atlas-brand__mark">{{ brandInitial }}</span>
          <span class="atlas-brand__text">
            <span class="atlas-brand__name">{{ brandName }}</span>
            <span class="atlas-brand__sub">{{ brandSub }}</span>
          </span>
        </RouterLink>

        <!-- 搜索框：白底 40px 圆角 5，右侧红色按钮 -->
        <form class="atlas-search" @submit.prevent="submitSearch">
          <input
            v-model="keyword"
            class="atlas-search__input"
            type="search"
            :placeholder="t('atlas.search.placeholder')"
            :aria-label="t('atlas.search.placeholder')"
          />
          <button class="atlas-search__btn" type="submit" :aria-label="t('atlas.search.placeholder')">
            <Search class="h-[22px] w-[22px]" />
          </button>
        </form>

        <!-- 右侧：语言 / 推广合作 / 订单查询 / 登录注册 -->
        <div class="atlas-actions">
          <div class="relative" ref="langEl">
            <button class="atlas-lang" type="button" @click="toggleLang">
              <span>{{ currentLanguageName }}</span>
              <span class="mx-2 opacity-40">|</span>
              <span>{{ currencyLabel }}</span>
              <ChevronDown class="ml-2 h-4 w-4" />
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

          <RouterLink v-if="affiliateEnabled" to="/affiliate" class="atlas-cooperate">
            <Rocket class="h-4 w-4" />
            {{ t('atlas.nav.affiliate') }}
          </RouterLink>

          <RouterLink to="/guest/orders" class="atlas-ghost-btn">
            <SearchCheck class="h-[18px] w-[18px]" />
            {{ t('navbar.guestOrders') }}
          </RouterLink>

          <RouterLink v-if="!userAuthStore.isAuthenticated" to="/auth/login" class="atlas-ghost-btn">
            {{ t('atlas.nav.loginOrRegister') }}
          </RouterLink>
          <template v-else>
            <RouterLink to="/me" class="atlas-ghost-btn">{{ t('navbar.personalCenter') }}</RouterLink>
            <button class="atlas-ghost-btn" type="button" @click="userAuthStore.logout()">{{ t('navbar.logout') }}</button>
          </template>
        </div>
      </div>
    </header>

    <!-- ==================== 分类导航条：40px + 2px 红下边框 ====================
         这里的项一律是「页内锚点定位」，不再是跳到另一个菜单页的链接。
         购物车是唯一例外 —— 它是独立页面，页内没有对应区块。 -->
    <nav class="atlas-navbar">
      <div class="atlas-shell atlas-navbar__row">
        <button
          type="button"
          class="atlas-navbar__item"
          :class="{ 'atlas-navbar__item--on': isHomeActive }"
          @click="goHome"
        >
          {{ t('nav.home') }}
        </button>
        <button
          v-for="cat in navCategories"
          :key="cat.id"
          type="button"
          class="atlas-navbar__item"
          :class="{ 'atlas-navbar__item--on': isNavCategoryActive(cat) }"
          @click="goCategory(cat)"
        >{{ getLocalizedText(cat.name) }}</button>

        <!-- 参考站末尾有一个带图标 + HOT 的高亮入口；这里是「热门商品区」的锚点 -->
        <button
          type="button"
          class="atlas-navbar__item atlas-navbar__item--hot"
          @click="goHotProducts"
        >
          <Sparkles class="h-[18px] w-[18px]" />
          {{ t('atlas.nav.hot') }}
          <span class="atlas-navbar__hot">HOT</span>
        </button>

        <RouterLink to="/cart" class="atlas-navbar__item ml-auto">
          <ShoppingCart class="h-[17px] w-[17px]" />
          {{ t('navbar.cart') }}
          <span v-if="cartCount > 0" class="atlas-navbar__hot">{{ cartCount > 99 ? '99+' : cartCount }}</span>
        </RouterLink>
      </div>
    </nav>

    <!-- ==================== 页面内容 ==================== -->
    <main class="flex-1">
      <slot />
    </main>

    <!-- ==================== 页脚 ==================== -->
    <footer class="bg-[color:var(--dark)]">
      <div class="atlas-shell grid grid-cols-1 gap-8 pb-6 pt-8 sm:grid-cols-2 lg:grid-cols-[1.8fr_repeat(3,1fr)]">
        <div>
          <RouterLink class="inline-flex items-center gap-2 text-[17px] font-bold text-white" to="/">
            <img v-if="brandLogo" :src="brandLogo" :alt="brandName" class="h-7 max-w-[140px] object-contain" />
            <template v-else>
              <span class="grid h-7 w-7 flex-none place-items-center rounded-full bg-[color:var(--accent)] text-[13px] font-bold text-white">{{ brandInitial }}</span>
              <span>{{ brandName }}</span>
            </template>
          </RouterLink>
          <p class="mt-3 max-w-[38ch] text-[13px] leading-relaxed text-[color:var(--footer-ink)]">
            {{ brandDescription || t('atlas.footer.tagline') }}
          </p>
        </div>

        <div>
          <h4 class="atlas-foot-head">{{ t('atlas.footer.shop') }}</h4>
          <RouterLink to="/products" class="atlas-foot-link">{{ t('products.allCategories') }}</RouterLink>
          <!-- 分类改为「回首页 + 原地筛选」，与页头分类按钮同一套逻辑（goCategory）；
               原来跳 /categories/<slug> 会换到另一个页面（Products.vue） -->
          <button
            v-for="cat in navCategories"
            :key="`f-${cat.id}`"
            type="button"
            class="atlas-foot-link atlas-foot-link--btn"
            @click="goCategory(cat)"
          >{{ getLocalizedText(cat.name) }}</button>
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

      <div class="atlas-shell flex flex-wrap items-center justify-between gap-3 border-t border-white/10 py-3 text-[12px] text-[color:var(--footer-ink)]">
        <span>© {{ year }} {{ brandName }}</span>
        <span>{{ t('atlas.footer.builtOn') }}</span>
      </div>
    </footer>

    <!-- ==================== 底部 App 浮层 ==================== -->
    <div v-if="appFloatOpen" class="atlas-appfloat">
      <img v-if="brandLogo" class="atlas-appfloat__logo" :src="brandLogo" :alt="brandName" />
      <span v-else class="atlas-appfloat__mark">{{ brandInitial }}</span>
      <span class="atlas-appfloat__text">{{ brandName }}</span>
      <span class="atlas-appfloat__desc">{{ t('atlas.appfloat.desc') }}</span>
      <!-- 参考站这里指向 App 下载页；本站没有 App，配置里也没有对应字段，
           所以保留同样的卡位与按钮样式，点击给出提示而不臆造跳转地址。 -->
      <button class="atlas-appfloat__btn" type="button" @click="onAppCta">
        {{ t('atlas.appfloat.cta') }}
      </button>
      <button class="atlas-appfloat__close" type="button" :aria-label="t('atlas.appfloat.close')" @click="closeAppFloat">
        <X class="h-4 w-4" />
      </button>
    </div>

    <!-- ==================== 右侧浮层：回到顶部 ====================
         在线客服入口由全局客服挂件（SupportWidget）统一提供，
         这里不再重复放置，避免右下角出现两个客服按钮。 -->
    <div class="atlas-custfloat">
      <button class="atlas-custfloat__box atlas-custfloat__box--alt" type="button" @click="scrollToTop">
        <ArrowUp class="h-5 w-5" />
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  ArrowUp, ChevronDown, Rocket, Search, SearchCheck, ShoppingCart, Sparkles, X,
} from 'lucide-vue-next'
import { useAppStore } from '../../../stores/app'
import { useCartStore } from '../../../stores/cart'
import { useUserAuthStore } from '../../../stores/userAuth'
import { useNavConfig } from '../../../composables/useNavConfig'
import { useLocalized } from '../../../composables/useProduct'
import { toast } from '../../../composables/useToast'
import { categoryAPI } from '../../../api/product'
import { getImageUrl } from '../../../utils/image'
import { scrollToAnchor } from '../../../utils/anchorScroll'
import { getLocalizedText as getResellerText } from '../../../utils/resellerSiteConfig'
import type { PublicCategory } from '../../../utils/category'
import { ATLAS_ANCHOR, atlasStickyOffset } from '../anchor'
import '../styles/atlas.css'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const cartStore = useCartStore()
const userAuthStore = useUserAuthStore()
const { getLocalizedText } = useLocalized()

const langOpen = ref(false)
const langEl = ref<HTMLElement | null>(null)
const keyword = ref('')

const NOTICE_KEY = 'dj-atlas-notice-closed'
const APP_FLOAT_KEY = 'dj-atlas-appfloat-closed'

const noticeOpen = ref(true)
const appFloatOpen = ref(true)

const year = new Date().getFullYear()

const brandName = computed(() => String(appStore.config?.brand?.site_name || '').trim() || 'Atlas Store')
const brandInitial = computed(() => brandName.value.slice(0, 1).toUpperCase())
// 参考站品牌是「主名 + 副名」两行（如 账号星球 / AccountBoy）。
// 公开配置的 brand 只有 site_name，副名走 i18n，不臆造配置字段。
const brandSub = computed(() => t('atlas.brand.sub'))
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

const { blogEnabled, noticeEnabled, aboutEnabled } = useNavConfig()

/**
 * 导航条里的分类。
 * 参考站把一级分类直链铺在导航条（末尾另有一个带图标 + HOT 的入口）。
 * 真实站点分类可能很多，只取一级分类前 9 个，其余走 /products 的侧栏。
 */
const categories = ref<PublicCategory[]>([])
const navCategories = computed(() =>
  categories.value.filter((c) => !c.parent_id || c.parent_id === 0).slice(0, 9),
)

const affiliateEnabled = computed(() => appStore.config?.affiliate?.enabled === true)
const currencyLabel = computed(() => String(appStore.config?.currency || 'CNY').toUpperCase())
const contact = computed(() => appStore.config?.contact as { telegram?: string; whatsapp?: string } | undefined)
const cartCount = computed(() => cartStore.totalItems)

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

const changeLanguage = (code: string) => {
  appStore.setLocale(code)
  langOpen.value = false
}
const toggleLang = () => { langOpen.value = !langOpen.value }

const submitSearch = () => {
  const q = keyword.value.trim()
  router.push(q ? { path: '/products', query: { search: q } } : { path: '/products' })
}

/* =====================================================================
   顶部导航的页内定位
   ---------------------------------------------------------------------
   导航条在 atlas 的每个页面都渲染，但锚点区块只存在于首页（Home.vue）。
   所以这里分两种情况：当前页面已有该区块 → 只滚；没有 → 先把路径导回
   首页，等 Home 挂载出锚点后再滚。
   ===================================================================== */

/** 当前页面是否已经渲染了该锚点 */
const hasAnchor = (id: string) => Boolean(document.getElementById(id))

/**
 * 首页按钮高亮：只在「首页 + 未选任何分类」时。
 *
 * 原来只判断 route.path === '/'，于是选了分类之后「首页」仍是红底 ——
 * 实测会出现「首页」和刚点的分类按钮**同时 --on**，看不出当前在看哪个。
 */
const isHomeActive = computed(() => {
  if (route.path !== '/') return false
  return !String(route.params.slug || route.query.category || '').trim()
})

/** 导航条分类高亮：与 Home 的 activeSlug 口径一致，路径参数与查询参数都要认 */
const isNavCategoryActive = (cat: PublicCategory) => {
  const slug = String(route.params.slug || route.query.category || '').trim()
  return Boolean(slug) && slug === cat.slug
}

/**
 * 分类按钮：原地筛选 + 滚动到分类区，不再跳到别的菜单页。
 *
 * 原来写的是 RouterLink → /categories/<slug>：
 *   · template_mode='card' 时那条路由由 Products.vue 渲染，直接换了一个页面；
 *   · 即便由 Home 渲染，也是换路由记录而不是原地更新。
 * 现在只改 query —— Home 的 activeSlug 同时认 params.slug 与 query.category，
 * 同一路由记录下只改 query 时 Vue Router 复用组件、不卸载不重挂，页面留在原地，
 * 筛选结果直接反映到分类区下方的商品区。
 *
 * 这里**不做任何滚动**：选分类只是换个筛选条件，视口该留在用户当前看的地方。
 * （曾经在末尾调 scrollToAnchor(catalog) 把视口拉到分类区，表现就是「一点分类
 * 页面就自己往下跳」。）
 */
const goCategory = async (cat: PublicCategory) => {
  const query = { ...route.query, category: cat.slug }

  // Home 里 activeSlug = params.slug || query.category，params 优先。
  // 当前路径带 :slug 时改 query 筛不动，必须把 path 导回 '/'。
  // 另外当前页面压根没有分类区（商品详情页、购物车等）时也得先回首页。
  const canFilterInPlace = !route.params.slug && hasAnchor(ATLAS_ANCHOR.catalog)
  if (canFilterInPlace) await router.replace({ query })
  else await router.push({ path: '/', query })
}

/** 首页按钮：清除分类/搜索筛选并回到页面顶部。
 *
 * 原来只在 route.path !== '/' 时才 push，且不清 query —— 于是「先点分类、再点首页」
 * 时路径已经是 '/'，什么也不做，表现为「点首页没反应」。现在无论当前在哪，都清掉
 * 分类与搜索查询参数并回到顶部，等价于「回到未筛选的首页」。 */
const goHome = async () => {
  const query = { ...route.query }
  delete query.category
  delete query.search
  const hasFilter = Boolean(route.query.category || route.query.search)
  if (route.path !== '/' || route.params.slug) {
    await router.push({ path: '/', query })
  } else if (hasFilter) {
    await router.replace({ query })
  }
  scrollToTop()
}

/** HOT 按钮：定位到商品区（未筛选是整宽热门分区，已筛选是主列里的商品网格） */
const goHotProducts = async () => {
  if (!hasAnchor(ATLAS_ANCHOR.products)) await router.push({ path: '/', query: { ...route.query } })
  scrollToAnchor(ATLAS_ANCHOR.products, { offset: atlasStickyOffset })
}

const closeNotice = () => {
  noticeOpen.value = false
  try { localStorage.setItem(NOTICE_KEY, '1') } catch { /* 忽略 */ }
}
const closeAppFloat = () => {
  appFloatOpen.value = false
  try { localStorage.setItem(APP_FLOAT_KEY, '1') } catch { /* 忽略 */ }
}
const onAppCta = () => toast.info(t('atlas.appfloat.noApp'))
const scrollToTop = () => window.scrollTo({ top: 0, behavior: 'smooth' })

const onDocClick = (e: MouseEvent) => {
  const target = e.target as Node
  if (langOpen.value && langEl.value && !langEl.value.contains(target)) langOpen.value = false
}

onMounted(async () => {
  document.addEventListener('click', onDocClick)
  // Teleport 到 body 的浮层（Toast / ConfirmDialog / 公告弹窗 / Select 下拉）在
  // .atlas-scope 之外，靠 body 上的这个 class 拿到 atlas 配色，详见 styles/atlas.css
  document.body.classList.add('atlas-tokens')

  try {
    if (localStorage.getItem(NOTICE_KEY) === '1') noticeOpen.value = false
    if (localStorage.getItem(APP_FLOAT_KEY) === '1') appFloatOpen.value = false
  } catch { /* 忽略 */ }

  // 分类只用于导航，失败时静默降级（导航仍保留首页与购物车）
  try {
    const res: any = await categoryAPI.list()
    const list = res?.data?.data
    if (Array.isArray(list)) categories.value = list
  } catch { /* 忽略 */ }
})

onUnmounted(() => {
  document.removeEventListener('click', onDocClick)
  document.body.classList.remove('atlas-tokens')
})
</script>

<style scoped>
.atlas-notice__text {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}
</style>
