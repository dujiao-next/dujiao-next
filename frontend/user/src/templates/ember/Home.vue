<template>
  <div>
    <!-- ==================== 站长配置的横幅轮播 ==================== -->
    <EmberBannerHero />

    <!-- ==================== Hero（卡片模式） ==================== -->
    <section v-if="!isListMode" class="relative overflow-hidden pb-10 pt-12 sm:pt-16">
      <div class="ember-mesh" aria-hidden="true">
        <span class="ember-mesh__blob ember-mesh__blob--a"></span>
        <span class="ember-mesh__blob ember-mesh__blob--b"></span>
        <span class="ember-mesh__grid"></span>
        <span class="ember-mesh__fade"></span>
      </div>

      <div class="relative z-[1] mx-auto grid w-full max-w-[1180px] items-center gap-10 px-4 sm:px-6 lg:grid-cols-[1.04fr_0.96fr] lg:gap-14">
        <div class="flex flex-col items-start gap-5">
          <span class="ember-pill ember-pill--accent">{{ t('ember.hero.badge') }}</span>

          <h1 class="max-w-[15ch] text-[clamp(2.05rem,1.45rem+2.55vw,3.35rem)] font-semibold tracking-[-0.022em] text-foreground">
            {{ t('ember.hero.titleLead') }}<span class="text-[color:var(--accent)]">{{ t('ember.hero.titleHighlight') }}</span>
          </h1>

          <p class="max-w-[52ch] text-[clamp(1rem,0.95rem+0.42vw,1.18rem)] leading-[1.62] text-muted-foreground">
            {{ t('ember.hero.lede') }}
          </p>

          <div class="flex flex-wrap gap-3 max-[640px]:w-full">
            <Button as-child size="lg" class="h-12 rounded-full px-6 text-[15px] font-medium max-[640px]:w-full">
              <RouterLink to="/products">{{ t('ember.hero.ctaPrimary') }}</RouterLink>
            </Button>
            <Button as-child variant="outline" size="lg" class="h-12 rounded-full border-[color:var(--border-strong)] px-6 text-[15px] font-medium max-[640px]:w-full">
              <RouterLink to="/guest/orders">{{ t('ember.hero.ctaSecondary') }}</RouterLink>
            </Button>
          </div>

          <ul class="flex flex-wrap gap-x-5 gap-y-2 text-[13px] text-muted-foreground">
            <li v-for="point in trustPoints" :key="point" class="inline-flex items-center gap-2">
              <Check class="h-3.5 w-3.5 flex-none text-[color:var(--success)]" />
              <span>{{ point }}</span>
            </li>
          </ul>
        </div>

        <!-- 热销预览 -->
        <div class="rounded-[var(--r-xl)] border border-border bg-card p-5 shadow-[var(--shadow-lg)] max-[640px]:hidden">
          <div class="flex items-baseline gap-3 border-b border-[color:var(--border-soft)] pb-3">
            <span class="text-[13.5px] font-semibold">{{ t('ember.hero.hotTitle') }}</span>
            <RouterLink class="ml-auto text-[12.5px] text-[color:var(--accent)] hover:text-[color:var(--accent-strong)]" to="/products">
              {{ t('ember.hero.hotMore') }}
            </RouterLink>
          </div>

          <div v-if="productsLoading" class="flex flex-col gap-1 pt-1">
            <div v-for="i in 4" :key="i" class="h-[52px] rounded-[var(--r-sm)] bg-[color:var(--surface-2)]"></div>
          </div>

          <div v-else-if="hotProducts.length" class="flex flex-col gap-1 pt-1">
            <RouterLink
              v-for="product in hotProducts"
              :key="product.id"
              class="grid grid-cols-[44px_minmax(0,1fr)_auto] items-center gap-3 rounded-[var(--r-sm)] p-2 transition-colors hover:bg-[color:var(--surface-2)]"
              :to="`/products/${product.slug}`"
            >
              <img
                v-if="hotImage(product)"
                :src="hotImage(product)"
                :alt="getLocalizedText(product.title)"
                loading="lazy"
                class="h-11 w-11 rounded-[var(--r-xs)] object-cover"
              />
              <span v-else class="grid h-11 w-11 place-items-center rounded-[var(--r-xs)] bg-[color:var(--surface-2)] text-muted-foreground">
                <Package class="h-4 w-4" />
              </span>
              <span class="min-w-0">
                <span class="block truncate text-[13.5px] font-medium">{{ getLocalizedText(product.title) }}</span>
                <span class="block text-[11.5px] text-muted-foreground">
                  {{ getLocalizedText(product.category?.name) || t('products.stockStatus.inStock') }}
                </span>
              </span>
              <span class="text-[13.5px] font-semibold tabular-nums tracking-[-0.02em]">
                {{ formatPrice(effectivePrice(product), siteCurrency) }}
              </span>
            </RouterLink>
          </div>

          <p v-else class="py-6 text-center text-[13px] text-muted-foreground">{{ t('ember.hero.hotEmpty') }}</p>
        </div>
      </div>
    </section>

    <!-- ==================== 分类块（卡片模式） ==================== -->
    <section v-if="!isListMode && topCategories.length" class="mx-auto w-full max-w-[1180px] px-4 py-9 sm:px-6">
      <header class="mb-6 flex flex-wrap items-end justify-between gap-4">
        <h2 class="text-[clamp(1.35rem,1.1rem+1vw,1.9rem)] font-semibold tracking-[-0.022em]">
          {{ t('ember.categories.title') }}
        </h2>
        <Button as-child variant="ghost" size="sm" class="rounded-full">
          <RouterLink to="/products">{{ t('ember.categories.all') }} <ChevronRight class="h-4 w-4" /></RouterLink>
        </Button>
      </header>

      <div class="grid gap-3 grid-cols-[repeat(auto-fill,minmax(160px,1fr))]">
        <RouterLink
          v-for="(cat, idx) in topCategories"
          :key="cat.id"
          class="relative flex min-h-[112px] flex-col justify-between gap-2.5 overflow-hidden rounded-[var(--r-lg)] border p-4 transition-all hover:-translate-y-1 hover:shadow-[var(--shadow)]"
          :class="categoryTone(idx)"
          :to="`/categories/${cat.slug}`"
        >
          <span class="grid h-10 w-10 place-items-center rounded-[var(--r-sm)] bg-white/25">
            <img
              v-if="cat.icon"
              :src="getImageUrl(cat.icon)"
              :alt="catName(cat)"
              loading="lazy"
              class="h-5 w-5 object-contain"
            />
            <Tag v-else class="h-[18px] w-[18px]" />
          </span>
          <span class="line-clamp-2 break-words text-[14px] font-medium">{{ catName(cat) }}</span>
        </RouterLink>
      </div>
    </section>

    <!-- ==================== 列表模式：搜索 + 分类 + 列表 ==================== -->
    <section v-if="isListMode" class="mx-auto w-full max-w-[1180px] px-4 py-6 sm:px-6">
      <div class="grid items-start gap-6 lg:grid-cols-[236px_minmax(0,1fr)]">
        <!-- 分类 -->
        <aside v-if="listCategoryGroups.length" class="min-w-0 lg:sticky lg:top-[92px]">
          <div class="rounded-[var(--r-lg)] border border-border bg-card p-3 sm:p-4">
            <div class="mb-3 hidden items-center gap-2 px-1 lg:flex">
              <span class="h-4 w-[3px] flex-none rounded-full bg-[color:var(--accent)]"></span>
              <h4 class="text-[13px] font-semibold">{{ t('products.categories') }}</h4>
            </div>
            <div class="flex gap-1.5 overflow-x-auto pb-0.5 lg:grid lg:gap-1 lg:overflow-visible lg:pb-0">
              <button
                type="button"
                class="flex-none whitespace-nowrap rounded-full px-3.5 py-2 text-[13.5px] transition-colors lg:w-full lg:rounded-[var(--r-sm)] lg:px-3 lg:py-2.5 lg:text-left"
                :class="listSelectedCategory === null
                  ? 'bg-foreground font-medium text-background lg:bg-[color:var(--accent-soft)] lg:text-[color:var(--accent-strong)]'
                  : 'bg-[color:var(--surface-2)] text-muted-foreground hover:text-foreground lg:bg-transparent lg:hover:bg-[color:var(--surface-2)]'"
                @click="listSelectCategory(null)"
              >{{ t('products.allCategories') }}</button>

              <template v-for="grp in listCategoryGroups" :key="grp.id">
                <div class="flex flex-none items-center gap-0.5 lg:w-full lg:min-w-0">
                  <button
                    type="button"
                    class="flex min-w-0 max-w-[65vw] flex-none items-center gap-2.5 rounded-full px-3.5 py-2 text-[13.5px] transition-colors lg:w-full lg:max-w-none lg:flex-1 lg:rounded-[var(--r-sm)] lg:px-3 lg:py-2.5 lg:text-left"
                    :class="listSelectedCategory === grp.id
                      ? 'bg-foreground font-medium text-background lg:bg-[color:var(--accent-soft)] lg:text-[color:var(--accent-strong)]'
                      : 'bg-[color:var(--surface-2)] text-muted-foreground hover:text-foreground lg:bg-transparent lg:hover:bg-[color:var(--surface-2)]'"
                    @click="listSelectCategory(grp.id)"
                  >
                    <img
                      v-if="grp.icon"
                      :src="getImageUrl(grp.icon)"
                      :alt="listCatName(grp)"
                      loading="lazy"
                      class="h-4 w-4 flex-none rounded object-cover"
                    />
                    <span class="min-w-0 truncate">{{ listCatName(grp) }}</span>
                  </button>
                  <button
                    v-if="grp.children.length"
                    type="button"
                    class="hidden h-9 w-9 flex-none place-items-center rounded-[var(--r-sm)] transition-colors hover:bg-[color:var(--surface-2)] lg:grid"
                    :class="listExpandedParentIds.includes(grp.id) ? 'text-[color:var(--accent)]' : 'text-muted-foreground'"
                    :aria-expanded="listExpandedParentIds.includes(grp.id)"
                    :aria-label="listCatName(grp)"
                    @click="listToggleParentCategory(grp.id)"
                  >
                    <ChevronDown
                      class="h-4 w-4 transition-transform"
                      :class="listExpandedParentIds.includes(grp.id) ? 'rotate-180' : ''"
                    />
                  </button>
                </div>

                <div v-if="listExpandedParentIds.includes(grp.id)" class="hidden lg:grid lg:gap-0.5 lg:pl-3">
                  <button
                    v-for="child in grp.children"
                    :key="child.id"
                    type="button"
                    class="w-full truncate rounded-[var(--r-sm)] px-3 py-2 text-left text-[13px] transition-colors"
                    :class="listSelectedCategory === child.id
                      ? 'bg-[color:var(--accent-soft)] font-medium text-[color:var(--accent-strong)]'
                      : 'text-muted-foreground hover:bg-[color:var(--surface-2)] hover:text-foreground'"
                    @click="listSelectCategory(child.id)"
                  >{{ listCatName(child) }}</button>
                </div>
              </template>
            </div>
          </div>
        </aside>

        <!-- 列表 -->
        <main class="min-w-0">
          <div class="relative mb-5 max-w-[340px]">
            <Search class="pointer-events-none absolute left-4 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <input
              v-model="listSearchQuery"
              type="search"
              class="h-11 w-full rounded-full border border-border bg-card pl-10 pr-4 text-[14px] outline-none transition-colors placeholder:text-muted-foreground focus:border-[color:var(--accent)] focus:ring-[var(--ring)]"
              :placeholder="t('products.searchBoxPlaceholder')"
              :aria-label="t('products.searchLabel')"
            />
          </div>

          <div v-if="listLoading" class="flex flex-col gap-2.5">
            <div v-for="i in 8" :key="i" class="h-[76px] rounded-[var(--r-lg)] border border-border bg-card"></div>
          </div>

          <div v-else-if="listProducts.length" class="flex flex-col gap-2.5">
            <EmberProductListItem
              v-for="product in listProducts"
              :key="product.id"
              :product="product"
              @quick-buy="openQuickBuy"
            />
          </div>

          <div v-else class="flex flex-col items-center gap-3 rounded-[var(--r-lg)] border border-dashed border-border py-16 text-center">
            <PackageOpen class="h-11 w-11 text-muted-foreground" />
            <p class="font-medium">
              {{ (listSearchQuery || listSelectedCategory) ? t('products.emptyFiltered') : t('ember.featured.empty') }}
            </p>
            <Button
              v-if="listSearchQuery || listSelectedCategory"
              variant="outline"
              size="sm"
              class="mt-2 rounded-full border-[color:var(--border-strong)]"
              @click="listResetFilters"
            >{{ t('products.clearFilters') }}</Button>
          </div>

          <nav v-if="listTotalPages > 1" class="mt-8 flex flex-wrap items-center justify-center gap-2">
            <button
              class="grid h-10 w-10 place-items-center rounded-full border border-border bg-card text-muted-foreground transition-colors hover:border-[color:var(--border-strong)] disabled:opacity-40"
              :disabled="listCurrentPage <= 1"
              :aria-label="t('pagination.previous')"
              @click="listChangePage(listCurrentPage - 1)"
            ><ChevronLeft class="h-4 w-4" /></button>
            <span class="grid h-10 place-items-center px-3 text-[13.5px] tabular-nums text-muted-foreground">
              {{ listCurrentPage }} / {{ listTotalPages }}
            </span>
            <button
              class="grid h-10 w-10 place-items-center rounded-full border border-border bg-card text-muted-foreground transition-colors hover:border-[color:var(--border-strong)] disabled:opacity-40"
              :disabled="listCurrentPage >= listTotalPages"
              :aria-label="t('pagination.next')"
              @click="listChangePage(listCurrentPage + 1)"
            ><ChevronRight class="h-4 w-4" /></button>
          </nav>
        </main>
      </div>
    </section>

    <!-- ==================== 商品（卡片模式） ==================== -->
    <section v-else class="mx-auto w-full max-w-[1180px] px-4 py-9 sm:px-6">
      <header class="mb-8 flex flex-wrap items-end justify-between gap-4">
        <div class="flex max-w-[720px] flex-col gap-2">
          <h2 class="text-[clamp(1.5rem,1.15rem+1.5vw,2.25rem)] font-semibold tracking-[-0.02em]">
            {{ t('ember.featured.title') }}
          </h2>
          <p class="text-[15px] leading-[1.6] text-muted-foreground">{{ t('ember.featured.subtitle') }}</p>
        </div>
        <Button as-child variant="ghost" size="sm" class="rounded-full">
          <RouterLink to="/products">{{ t('ember.hero.hotMore') }} <ChevronRight class="h-4 w-4" /></RouterLink>
        </Button>
      </header>

      <div v-if="productsLoading" class="grid gap-4 grid-cols-[repeat(auto-fill,minmax(238px,1fr))] max-[640px]:grid-cols-2 max-[640px]:gap-3">
        <div v-for="i in 8" :key="i" class="h-[280px] rounded-[var(--r-lg)] border border-border bg-card"></div>
      </div>

      <div v-else-if="products.length" class="grid gap-4 grid-cols-[repeat(auto-fill,minmax(238px,1fr))] max-[640px]:grid-cols-2 max-[640px]:gap-3">
        <EmberProductCard
          v-for="product in products"
          :key="product.id"
          :product="product"
          quick-buy-enabled
          @quick-buy="openQuickBuy"
        />
      </div>

      <div v-else class="flex flex-col items-center gap-3 rounded-[var(--r-lg)] border border-dashed border-border py-16 text-center">
        <PackageOpen class="h-11 w-11 text-muted-foreground" />
        <p class="font-medium">{{ t('ember.featured.empty') }}</p>
        <p class="max-w-[42ch] text-[13.5px] text-muted-foreground">{{ t('ember.featured.emptyHint') }}</p>
      </div>
    </section>

    <!-- ==================== 最新动态 ==================== -->
    <section v-if="latestVisible && posts.length" class="ember-band border-y border-border">
      <div class="mx-auto w-full max-w-[1180px] px-4 py-12 sm:px-6">
        <header class="mb-6 flex flex-wrap items-end justify-between gap-4">
          <h2 class="text-[clamp(1.35rem,1.1rem+1vw,1.9rem)] font-semibold tracking-[-0.022em]">
            {{ t('ember.latest.title') }}
          </h2>
        </header>

        <div class="grid gap-4 grid-cols-[repeat(auto-fill,minmax(248px,1fr))]">
          <RouterLink
            v-for="post in posts"
            :key="post.id"
            class="flex h-full flex-col rounded-[var(--r-lg)] border border-border bg-card p-5 transition-all hover:-translate-y-1 hover:border-[color:var(--border-strong)] hover:shadow-[var(--shadow)]"
            :to="`/blog/${post.slug}`"
          >
            <span class="text-[12px] text-muted-foreground">{{ formatDate(post.published_at) }}</span>
            <h3 class="mt-2 line-clamp-2 text-[15px] font-semibold leading-[1.4]">{{ getLocalizedText(post.title) }}</h3>
            <p class="mt-2 line-clamp-2 flex-1 text-[13px] leading-[1.6] text-muted-foreground">{{ getLocalizedText(post.summary) }}</p>
            <span class="mt-3.5 inline-flex items-center gap-1.5 text-[13px] font-medium text-[color:var(--accent)]">
              {{ t('blog.readMore') }} <ChevronRight class="h-3.5 w-3.5" />
            </span>
          </RouterLink>
        </div>
      </div>
    </section>

    <!-- ==================== 特性 ==================== -->
    <section class="mx-auto w-full max-w-[1180px] px-4 py-12 sm:px-6">
      <div class="grid gap-4 grid-cols-[repeat(auto-fit,minmax(240px,1fr))]">
        <article
          v-for="feature in features"
          :key="feature.title"
          class="flex flex-col gap-3 rounded-[var(--r-lg)] border border-border bg-card p-6 transition-transform duration-300 hover:-translate-y-1"
        >
          <span class="grid h-10 w-10 place-items-center rounded-[var(--r-sm)] bg-[color:var(--accent-soft)] text-[color:var(--accent-strong)]">
            <component :is="feature.icon" class="h-5 w-5" />
          </span>
          <h3 class="text-[15px] font-semibold tracking-[-0.02em]">{{ feature.title }}</h3>
          <p class="text-[13.5px] leading-[1.65] text-muted-foreground">{{ feature.body }}</p>
        </article>
      </div>
    </section>

    <!-- 快速购买 -->
    <ProductQuickBuy
      v-if="quickBuyProduct"
      :product="quickBuyProduct"
      :visible="quickBuyVisible"
      @update:visible="quickBuyVisible = $event"
    />

    <AnnouncementModal
      v-if="activeAnnouncement"
      :announcement="activeAnnouncement"
      :visible="announcementVisible"
      @update:visible="announcementVisible = $event"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  Check, ChevronDown, ChevronLeft, ChevronRight, CreditCard, Package, PackageOpen, Receipt, Search,
  ShieldCheck, Tag, Zap,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { categoryAPI, postAPI, productAPI } from '../../api'
import { buildCategoryGroups, type PublicCategory } from '../../utils/category'
import { getFirstImageUrl, getImageUrl } from '../../utils/image'
import { useLocalized, useProductLabels } from '../../composables/useProduct'
import { useProductList } from '../../composables/useProductList'
import { useNavConfig } from '../../composables/useNavConfig'
import { usePageSeo } from '../../composables/usePageSeo'
import { useAnnouncement, type HomeAnnouncement } from '../../composables/useAnnouncement'
import { useAppStore } from '../../stores/app'
import EmberProductCard from './components/EmberProductCard.vue'
import EmberProductListItem from './components/EmberProductListItem.vue'
import EmberBannerHero from './components/EmberBannerHero.vue'
import ProductQuickBuy from '../../components/ProductQuickBuy.vue'
import AnnouncementModal from '../../components/AnnouncementModal.vue'

const route = useRoute()
const { t } = useI18n()
const { getLocalizedText, siteCurrency, formatPrice } = useLocalized()
const { hasPromotionPrice, getPromotionPriceAmount } = useProductLabels()
const appStore = useAppStore()
const { isListMode, blogEnabled, noticeEnabled } = useNavConfig()

const products = ref<any[]>([])
const productsLoading = ref(true)
const posts = ref<any[]>([])
const topCategories = ref<PublicCategory[]>([])

// ==================== 列表模式（template_mode === 'list'）====================
// 该模式下 /products 与首页同页，因此首页需自带搜索 + 分类 + 列表 + 分页
const {
  loading: listLoading,
  products: listProducts,
  selectedCategory: listSelectedCategory,
  searchQuery: listSearchQuery,
  currentPage: listCurrentPage,
  totalPages: listTotalPages,
  expandedParentIds: listExpandedParentIds,
  categoryGroups: listCategoryGroups,
  selectCategory: listSelectCategory,
  toggleParentCategory: listToggleParentCategory,
  changePage: listChangePage,
  clearSearch: listClearSearch,
  initialize: listInitialize,
  cleanup: listCleanup,
} = useProductList({ pageSize: 20, homeRouteName: 'home' })

const listCatName = (cat: any) => getLocalizedText(cat?.name) || cat?.slug || ''

const listResetFilters = () => {
  listClearSearch()
  listSelectCategory(null)
}

// ==================== 快速购买 ====================
const quickBuyProduct = ref<any>(null)
const quickBuyVisible = ref(false)
const openQuickBuy = (product: any) => {
  quickBuyProduct.value = product
  quickBuyVisible.value = true
}

const trustPoints = computed(() => [
  t('ember.hero.trustAuto'),
  t('ember.hero.trustGuest'),
  t('ember.hero.trustTrace'),
])

const features = computed(() => [
  { icon: Zap, title: t('ember.features.autoTitle'), body: t('ember.features.autoBody') },
  { icon: ShieldCheck, title: t('ember.features.secureTitle'), body: t('ember.features.secureBody') },
  { icon: Receipt, title: t('ember.features.traceTitle'), body: t('ember.features.traceBody') },
  { icon: CreditCard, title: t('ember.features.multiTitle'), body: t('ember.features.multiBody') },
])

/** 右侧预览只展示前 4 件 */
const hotProducts = computed(() => products.value.slice(0, 4))

const effectivePrice = (product: any) =>
  hasPromotionPrice(product) ? getPromotionPriceAmount(product) : product?.price_amount

const hotImage = (product: any) => getFirstImageUrl(product?.images)

const catName = (cat: PublicCategory) => getLocalizedText(cat.name) || cat.slug || ''

/** 分类卡配色：只用中性灰阶 + 系统蓝，不引入额外色相 */
const categoryTones = [
  'bg-[color:var(--ink)] text-background border-transparent',
  'bg-[color:var(--accent)] text-white border-transparent',
  'bg-[color:var(--surface-2)] text-foreground',
  'bg-[color:var(--surface-sunken)] text-foreground',
]
const categoryTone = (idx: number) => categoryTones[idx % categoryTones.length]

const latestVisible = computed(() => blogEnabled.value || noticeEnabled.value)

const formatDate = (value: string) => (value ? new Date(value).toLocaleDateString() : '')

// ==================== 数据加载 ====================
const loadProducts = async () => {
  productsLoading.value = true
  try {
    const res = await productAPI.list({ page: 1, page_size: 8 })
    products.value = res.data.data || []
  } catch (err) {
    console.error('Failed to load products:', err)
    products.value = []
  } finally {
    productsLoading.value = false
  }
}

const loadCategories = async () => {
  try {
    const res = await categoryAPI.list()
    topCategories.value = buildCategoryGroups(res.data.data || []).slice(0, 10)
  } catch (err) {
    console.error('Failed to load categories:', err)
  }
}

const loadPosts = async () => {
  if (!latestVisible.value) return
  try {
    const params: Record<string, unknown> = { page: 1, page_size: 3 }
    if (blogEnabled.value && !noticeEnabled.value) params.type = 'blog'
    if (!blogEnabled.value && noticeEnabled.value) params.type = 'notice'
    const res = await postAPI.list(params)
    posts.value = res.data.data || []
  } catch (err) {
    console.error('Failed to load posts:', err)
  }
}

// ==================== SEO ====================
usePageSeo({
  canonicalPath: () => route.path,
  title: () => (route.name === 'products' ? t('nav.products') : undefined),
})

// ==================== 公告弹窗 ====================
const { shouldShow } = useAnnouncement()
const activeAnnouncement = ref<HomeAnnouncement | null>(null)
const announcementVisible = ref(false)

onMounted(async () => {
  await appStore.loadConfig()

  if (isListMode.value) {
    await listInitialize()
  } else {
    await Promise.all([loadProducts(), loadCategories(), loadPosts()])
  }

  const announcement = appStore.config?.announcement as HomeAnnouncement | undefined
  if (announcement && shouldShow(announcement)) {
    activeAnnouncement.value = announcement
    announcementVisible.value = true
  }
})

onUnmounted(() => {
  listCleanup()
})
</script>
