<template>
  <div>
    <!-- 站长配置的横幅轮播（两种模式共用，顶部展示） -->
    <VaultBannerHero />

    <!-- ==================== 列表模式 ==================== -->
    <template v-if="isListMode">
      <section class="mx-auto w-full max-w-[1180px] px-4 py-6 sm:px-6">
        <div class="grid items-start gap-5 lg:grid-cols-[248px_1fr] lg:gap-7">
          <VaultCategorySidebar
            :category-groups="categoryGroups"
            :selected-category="selectedCategory"
            :expanded-parent-ids="expandedParentIds"
            @select="selectCategory"
            @toggle="toggleParentCategory"
          />

          <main class="min-w-0">
            <!-- 搜索 -->
            <div class="relative mb-4">
              <Search class="pointer-events-none absolute left-3.5 top-1/2 h-[18px] w-[18px] -translate-y-1/2 text-muted-foreground" />
              <input
                v-model="searchQuery"
                class="h-11 w-full rounded-xl border border-border bg-card pl-10 pr-10 text-[15px] text-foreground transition-colors placeholder:text-muted-foreground hover:border-hairline-strong"
                :placeholder="t('products.searchBoxPlaceholder')"
                :aria-label="t('products.searchLabel')"
              />
              <button v-if="searchQuery" type="button" class="absolute right-2.5 top-1/2 grid h-6 w-6 -translate-y-1/2 place-items-center rounded-full text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground" :aria-label="t('blog.searchClear')" @click="clearSearch"><X class="h-3.5 w-3.5" /></button>
            </div>

            <!-- 骨架 -->
            <div v-if="listLoading" class="space-y-2.5">
              <div v-for="i in 8" :key="i" class="h-[76px] rounded-lg border bg-card"></div>
            </div>

            <!-- 分组列表 -->
            <div v-else-if="listProductGroups.length" class="space-y-6">
              <div v-for="group in listProductGroups" :key="group.categoryId ?? 'uncategorized'">
                <div class="mb-2.5 flex items-center gap-2 px-0.5">
                  <span class="h-4 w-1 flex-none rounded-full bg-primary"></span>
                  <img v-if="group.categoryIcon" :src="getImageUrl(group.categoryIcon)" :alt="group.categoryName" loading="lazy" class="h-5 w-5 flex-none rounded object-cover" />
                  <span class="min-w-0 truncate text-sm font-bold text-foreground">{{ group.categoryName }}</span>
                  <span class="flex-none text-xs text-muted-foreground">({{ group.products.length }})</span>
                </div>
                <div class="space-y-2">
                  <VaultProductListItem
                    v-for="(product, idx) in group.products"
                    :key="product.id"
                    :product="product"
                    :index="idx"
                    @quick-buy="openQuickBuy"
                  />
                </div>
              </div>

              <nav v-if="listTotalPages > 1" class="mt-[30px] flex flex-wrap justify-center gap-2">
                <button class="grid h-[42px] min-w-[42px] place-items-center rounded-full border bg-card px-3 font-bold text-muted-foreground hover:border-primary hover:text-primary disabled:cursor-not-allowed disabled:opacity-40" :disabled="listCurrentPage <= 1" @click="listChangePage(listCurrentPage - 1)"><ChevronLeft class="h-[17px] w-[17px]" /></button>
                <button
                  v-for="p in listPageWindow"
                  :key="p"
                  class="grid h-[42px] min-w-[42px] place-items-center rounded-full border px-3 text-sm font-bold"
                  :class="p === listCurrentPage ? 'border-primary bg-primary text-white' : 'border-border bg-card text-muted-foreground hover:border-primary hover:text-primary'"
                  @click="listChangePage(p)"
                >{{ p }}</button>
                <button class="grid h-[42px] min-w-[42px] place-items-center rounded-full border bg-card px-3 font-bold text-muted-foreground hover:border-primary hover:text-primary disabled:cursor-not-allowed disabled:opacity-40" :disabled="listCurrentPage >= listTotalPages" @click="listChangePage(listCurrentPage + 1)"><ChevronRight class="h-[17px] w-[17px]" /></button>
              </nav>
            </div>

            <!-- 空态 -->
            <div v-else class="flex flex-col items-center gap-3 rounded-lg border border-dashed py-14 text-center text-muted-foreground">
              <component :is="(searchQuery || selectedCategory) ? SearchX : PackageOpen" class="h-12 w-12 opacity-60" />
              <p>{{ (searchQuery || selectedCategory) ? t('products.emptyFiltered') : t('products.empty') }}</p>
              <Button v-if="searchQuery || selectedCategory" variant="outline" size="sm" class="mt-2 rounded-full" @click="resetFilters">{{ t('products.clearFilters') }}</Button>
            </div>
          </main>
        </div>
      </section>
    </template>

    <!-- ==================== 卡片模式（默认） ==================== -->
    <template v-else>
      <!-- 分类块 -->
      <section v-if="topCategories.length" class="mx-auto w-full max-w-[1180px] px-4 py-9 sm:px-6">
        <div class="mb-[22px] flex flex-wrap items-end justify-between gap-4">
          <h2 class="text-[28px] font-extrabold">{{ t('vault.categoriesTitle') }}</h2>
          <Button as-child variant="ghost" size="sm" class="rounded-full"><RouterLink to="/products">{{ t('vault.allCategories') }} <ChevronRight /></RouterLink></Button>
        </div>
        <div class="grid gap-3.5 grid-cols-[repeat(auto-fill,minmax(150px,1fr))]">
          <RouterLink
            v-for="(cat, idx) in topCategories"
            :key="cat.id"
            class="relative flex min-h-[116px] flex-col justify-between gap-2.5 overflow-hidden rounded-lg p-[18px] transition hover:-translate-y-[3px] hover:shadow-[var(--shadow)]"
            :class="catColor(idx)"
            :to="`/categories/${cat.slug}`"
          >
            <span class="grid h-10 w-10 place-items-center rounded-xl bg-white/20">
              <img v-if="cat.icon" :src="getImageUrl(cat.icon)" :alt="catName(cat)" loading="lazy" class="h-6 w-6 object-contain" />
              <Tag v-else class="h-[22px] w-[22px]" />
            </span>
            <div class="min-w-0"><div class="line-clamp-2 break-words font-bold">{{ catName(cat) }}</div></div>
          </RouterLink>
        </div>
      </section>

      <!-- 热门商品 -->
      <section class="mx-auto w-full max-w-[1180px] px-4 py-9 sm:px-6">
        <div class="mb-[22px] flex flex-wrap items-end justify-between gap-4">
          <h2 class="text-[28px] font-extrabold">{{ t('home.featured.title') }}</h2>
          <Button as-child variant="outline" size="sm" class="rounded-full"><RouterLink to="/products">{{ t('home.featured.viewAll') }}</RouterLink></Button>
        </div>

        <div v-if="productsLoading" class="grid gap-4 grid-cols-[repeat(auto-fill,minmax(228px,1fr))]">
          <div v-for="i in 10" :key="i" class="h-[280px] rounded-lg border bg-card"></div>
        </div>
        <div v-else-if="products.length" class="grid gap-4 grid-cols-[repeat(auto-fill,minmax(228px,1fr))]">
          <VaultProductCard
            v-for="(product, idx) in products"
            :key="product.id"
            :product="product"
            :index="idx"
            @quick-buy="openQuickBuy"
          />
        </div>
        <div v-else class="flex flex-col items-center gap-3 rounded-lg border border-dashed py-14 text-center text-muted-foreground">
          <PackageOpen class="h-12 w-12 opacity-60" />
          <p>{{ t('home.featured.empty') }}</p>
        </div>
      </section>

      <!-- 最新动态 -->
      <section v-if="latestVisible && posts.length" class="mx-auto w-full max-w-[1180px] px-4 py-9 sm:px-6">
        <div class="mb-[22px] flex flex-wrap items-end justify-between gap-4">
          <h2 class="text-[28px] font-extrabold">{{ t('home.latest.title') }}</h2>
        </div>
        <div class="grid gap-4 grid-cols-[repeat(auto-fill,minmax(228px,1fr))]">
          <RouterLink
            v-for="post in posts"
            :key="post.id"
            class="block h-full rounded-lg border bg-card p-[22px] transition hover:-translate-y-[3px] hover:border-hairline-strong hover:shadow-[var(--shadow)]"
            :to="`/blog/${post.slug}`"
          >
            <span class="text-[12.5px] font-semibold text-muted-foreground">{{ formatDate(post.published_at) }}</span>
            <h3 class="mt-2 line-clamp-2 text-[17px] font-bold">{{ getLocalizedText(post.title) }}</h3>
            <p class="mt-2 line-clamp-2 text-sm text-muted-foreground">{{ getLocalizedText(post.summary) }}</p>
            <span class="mt-3.5 inline-flex items-center gap-1.5 text-sm font-bold text-primary">{{ t('blog.readMore') }} <ChevronRight class="h-4 w-4" /></span>
          </RouterLink>
        </div>
      </section>
    </template>

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
import { ChevronLeft, ChevronRight, PackageOpen, Search, SearchX, Tag, X } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { categoryAPI, postAPI, productAPI } from '../../api'
import { buildCategoryGroups, type PublicCategory } from '../../utils/category'
import { getImageUrl } from '../../utils/image'
import { useLocalized } from '../../composables/useProduct'
import { useProductList } from '../../composables/useProductList'
import { useProductListGroups } from '../../composables/useProductListGroups'
import { usePageSeo } from '../../composables/usePageSeo'
import { useAppStore } from '../../stores/app'
import VaultProductCard from './components/VaultProductCard.vue'
import VaultProductListItem from './components/VaultProductListItem.vue'
import VaultCategorySidebar from './components/VaultCategorySidebar.vue'
import VaultBannerHero from './components/VaultBannerHero.vue'
import ProductQuickBuy from '../../components/ProductQuickBuy.vue'
import AnnouncementModal from '../../components/AnnouncementModal.vue'
import { useAnnouncement, type HomeAnnouncement } from '../../composables/useAnnouncement'

const route = useRoute()
const { t } = useI18n()
const { getLocalizedText } = useLocalized()
const appStore = useAppStore()

const isListMode = computed(() => appStore.config?.template_mode === 'list')

// ==================== 快速购买 ====================
const quickBuyProduct = ref<any>(null)
const quickBuyVisible = ref(false)
const openQuickBuy = (product: any) => {
  quickBuyProduct.value = product
  quickBuyVisible.value = true
}

// ==================== 列表模式 ====================
const {
  loading: listLoading,
  products: listProducts,
  selectedCategory,
  searchQuery,
  currentPage: listCurrentPage,
  totalPages: listTotalPages,
  expandedParentIds,
  categoryGroups,
  categoryMap: listCategoryMap,
  selectCategory,
  toggleParentCategory,
  changePage: listChangePage,
  clearSearch,
  initialize: listInitialize,
  cleanup: listCleanup,
} = useProductList({ pageSize: 20, homeRouteName: 'home' })

const listProductGroups = useProductListGroups(listProducts, listCategoryMap)

const listPageWindow = computed(() => {
  const total = listTotalPages.value
  const cur = listCurrentPage.value
  const start = Math.max(1, cur - 2)
  const end = Math.min(total, start + 4)
  const realStart = Math.max(1, end - 4)
  const pages: number[] = []
  for (let p = realStart; p <= end; p++) pages.push(p)
  return pages
})

const resetFilters = () => {
  clearSearch()
  selectCategory(null)
}

// ==================== 卡片模式 ====================
const products = ref<any[]>([])
const productsLoading = ref(true)
const posts = ref<any[]>([])
const topCategories = ref<PublicCategory[]>([])

const catColors = [
  'bg-[color:var(--red)] text-white',
  'bg-[color:var(--teal)] text-white',
  'bg-[color:var(--plum)] text-white',
  'bg-[color:var(--gold)] text-[color:var(--on-gold)]',
  'bg-[color:var(--ink)] text-[color:var(--bg)]',
]
const catColor = (idx: number) => catColors[idx % catColors.length]
const catName = (cat: PublicCategory) => getLocalizedText(cat.name) || cat.slug || ''

const navBuiltin = computed(() => (appStore.config?.nav_config as { builtin?: Record<string, boolean> } | undefined)?.builtin)
const blogEnabled = computed(() => navBuiltin.value?.blog !== false)
const noticeEnabled = computed(() => navBuiltin.value?.notice !== false)
const latestVisible = computed(() => blogEnabled.value || noticeEnabled.value)

const formatDate = (value: string) => (value ? new Date(value).toLocaleDateString() : '')

// ==================== 公告弹窗（版本化可关闭，两种模式共用） ====================
const { shouldShow } = useAnnouncement()
const activeAnnouncement = ref<HomeAnnouncement | null>(null)
const announcementVisible = ref(false)
const showAnnouncementIfNeeded = () => {
  const announcement = appStore.config?.announcement as HomeAnnouncement | undefined
  if (announcement && shouldShow(announcement)) {
    activeAnnouncement.value = announcement
    announcementVisible.value = true
  }
}

let featuredRefreshTimer: number | null = null

const loadProducts = async (_silent = false) => {
	if (!_silent) productsLoading.value = true
  try {
    const res = await productAPI.list({ page: 1, page_size: 15 })
    products.value = res.data.data || []
  } catch (err) {
    console.error('Failed to load products:', err)
  } finally {
		if (!_silent) productsLoading.value = false
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
const seoCategoryName = computed(() => {
  if (!selectedCategory.value) return ''
  const cat = listCategoryMap.value.get(selectedCategory.value)
  return cat ? catName(cat) : ''
})
usePageSeo({
  canonicalPath: () => route.path,
  title: () => {
    if (route.name === 'category-products') return seoCategoryName.value || t('nav.products')
    if (route.name === 'products') return t('nav.products')
    return undefined
  },
})

onMounted(async () => {
  await appStore.loadConfig()
  if (isListMode.value) {
    await listInitialize()
  } else {
    await Promise.all([loadProducts(), loadCategories(), loadPosts()])
		if (typeof window !== 'undefined' && !featuredRefreshTimer) {
			featuredRefreshTimer = window.setInterval(() => {
				void loadProducts(true)
			}, 60_000)
		}
  }
  showAnnouncementIfNeeded()
})

onUnmounted(() => {
	listCleanup()
	if (featuredRefreshTimer) {
		clearInterval(featuredRefreshTimer)
		featuredRefreshTimer = null
	}
})
</script>
