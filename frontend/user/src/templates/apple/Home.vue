<template>
  <div>
    <!-- 后台用积木编辑器做的首页（posts: slug=home, layout=blocks） -->
    <BlockRenderer v-if="homeBlocks" :blocks="homeBlocks" />

    <!-- ==================== 列表模式 ==================== -->
    <template v-else-if="isListMode">

      <!-- 站长配置的横幅轮播（列表模式顶部展示） -->
      <VaultBannerHero />

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

    <!-- ==================== 卡片模式（Apple Store 风格） ==================== -->
    <div v-else class="st-root">
      <!-- 苹果官网式首页：一屏一个推广单元 + 两列商品单元 -->
      <HomeUnits :banners="bannerUnits" :products="(productUnits as any)" />

      <!-- 一个单元都没有时，给个占位，避免空白页 -->
      <div v-if="!bannerUnits.length && !productUnits.length" class="st-container hu-empty">
        <p>{{ t('home.featured.empty') }}</p>
      </div>


      <div class="pb-10"></div>
    </div>

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
  ChevronLeft, ChevronRight, PackageOpen, Search, SearchX,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { bannerAPI, postAPI, productAPI } from '../../api'
import { getImageUrl, getFirstImageUrl } from '../../utils/image'
import { useLocalized } from '../../composables/useProduct'
import { useProductList } from '../../composables/useProductList'
import { useProductListGroups } from '../../composables/useProductListGroups'
import { usePageSeo } from '../../composables/usePageSeo'
import { useAppStore } from '../../stores/app'
import VaultProductListItem from './components/VaultProductListItem.vue'
import VaultCategorySidebar from './components/VaultCategorySidebar.vue'
import HomeUnits from './components/store/HomeUnits.vue'
import ProductQuickBuy from '../../components/ProductQuickBuy.vue'
import AnnouncementModal from '../../components/AnnouncementModal.vue'
import { useAnnouncement, type HomeAnnouncement } from '../../composables/useAnnouncement'
import BlockRenderer from '../../components/blocks/BlockRenderer.vue'

const route = useRoute()
const { t } = useI18n()
const { getLocalizedText, siteCurrency, formatPrice } = useLocalized()
const appStore = useAppStore()

const isListMode = computed(() => appStore.config?.template_mode === 'list')

/** 积木式首页：后台建一条 slug=home 的页面即可接管首页 */
const homeBlocks = ref<any[] | null>(null)
const loadHomePage = async () => {
  try {
    const res = await postAPI.detail('home', { silentBusinessError: true })
    const page = res.data?.data
    const blocks = page?.layout === 'blocks' ? page?.content_blocks?.blocks : null
    if (Array.isArray(blocks) && blocks.length) homeBlocks.value = blocks
  } catch {
    /* 没有配置积木首页时保持原首页 */
  }
}

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

// ---------- 商品版图（首页四宫格） ----------
const products = ref<any[]>([])
const defaultTone = computed(() => {
  // 优先级：首页装修里显式设置的默认色 → 站点设置里的宫格底色 → 深色
  const layoutTone = (appStore.config as any)?.home_layout?.tone
  if (layoutTone === 'light' || layoutTone === 'dark') return layoutTone
  return appStore.config?.home_tiles_tone === 'light' ? 'light' : 'dark'
})

/** 后台「首页装修」里配置的商品版图：指定商品、顺序、每块深浅色 */
const productUnits = computed<any[]>(() => {
  const layout = (appStore.config as any)?.home_layout
  const tileConfig = Array.isArray(layout?.tiles) ? layout.tiles : (appStore.config as any)?.home_tiles
  const byId = new Map<number, any>(products.value.map((p: any) => [Number(p.id), p] as [number, any]))
  const productLink = (product: any) => `/products/${product.slug}`
  const toUnit = (product: any, tile?: any) => ({
    id: product.id,
    slug: product.slug,
    // 小标题由后台单独配置，留空则不显示（不再回退分类名）
    eyebrow: tile?.eyebrow_text || '',
    eyebrowColor: tile?.eyebrow_color || '',
    title: getLocalizedText(product.title),
    price: formatPrice(product.price_amount, siteCurrency.value),
    link: productLink(product),
    // 首页宣传图优先用后台单独上传的，没有才回退商品图
    image: tile?.image ? getImageUrl(tile.image) : getFirstImageUrl(product.images),
    mobileImage: tile?.mobile_image ? getImageUrl(tile.mobile_image) : '',
    // 两个按钮：后台可自定义文案与地址，留空则用默认「进一步了解 / 购买」并指向商品页
    ctaLabel: tile?.cta_text || t('storefront.learnMore'),
    ctaTo: tile?.cta_link || productLink(product),
    buyLabel: tile?.buy_text || t('storefront.buyNow'),
    buyTo: tile?.buy_link || productLink(product),
    tone: (tile?.tone || defaultTone.value) === 'light' ? 'light' : 'dark',
  })
  if (Array.isArray(tileConfig) && tileConfig.length) {
    const result: any[] = []
    for (const tile of tileConfig) {
      const product = byId.get(Number(tile?.product_id))
      if (product) result.push(toUnit(product, tile))
    }
    return result
  }
  return products.value.slice(0, 4).map((product: any) => toUnit(product))
})

const loadProducts = async () => {
  try {
    const res = await productAPI.list({ page: 1, page_size: 50 })
    products.value = res.data.data || []
  } catch {
    products.value = []
  }
}

// ---------- 苹果式首页单元 ----------
const bannerUnits = ref<Array<any>>([])
const loadBanners = async () => {
  try {
    const res = await bannerAPI.list({ position: 'home_hero' })
    const list = Array.isArray(res.data?.data) ? res.data.data : []
    bannerUnits.value = list.map((item: any, index: number) => ({
      id: item.id,
      title: getLocalizedText(item.title),
      subtitle: getLocalizedText(item.subtitle),
      image: getImageUrl(item.image),
      mobileImage: item.mobile_image ? getImageUrl(item.mobile_image) : '',
      to: item.link_value || '',
      // 「购买」优先用横幅单独配的地址，其次跟同一个商品页，最后去商店列表
      buyTo:
        item.buy_link ||
        (String(item.link_value || '').startsWith('/products/') ? item.link_value : '/products'),
      linkLabel: getLocalizedText(item.cta_text) || t('storefront.learnMore'),
      buyLabel: getLocalizedText(item.buy_text) || t('storefront.buyNow'),
      // 后台指定优先，否则第一条深色、其余浅色
      tone: item.tone === 'dark' || item.tone === 'light' ? item.tone : (index === 0 ? 'dark' : 'light'),
    }))
  } catch {
    bannerUnits.value = []
  }
}

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

// ==================== SEO ====================
const seoCategoryName = computed(() => {
  if (!selectedCategory.value) return ''
  const cat = listCategoryMap.value.get(selectedCategory.value)
  return cat ? getLocalizedText(cat.name) : ''
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
  void loadBanners()
  await loadHomePage()
  if (isListMode.value) {
    await listInitialize()
  } else {
    await loadProducts()
  }
  showAnnouncementIfNeeded()
})

onUnmounted(() => listCleanup())
</script>
