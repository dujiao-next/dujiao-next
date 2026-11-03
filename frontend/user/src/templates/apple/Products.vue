<template>
  <div class="st-root pv">
    <!-- 商店页头（苹果 /store：左侧大标题 + 右侧说明） -->
    <StoreHero
      :title="pageTitle"
      :subtitle="storeLead"
      :links="storeLinks"
    />

    <!-- 分类轨道 -->
    <FamilyRail v-if="railItems.length > 1" :items="railItems" />

    <!-- 分类 chips + 搜索 -->
    <div class="st-container pv-toolbar pv-toolbar--end">
      <label class="pv-search">
        <Search class="pv-search-icon" />
        <input
          v-model="searchQuery"
          type="search"
          :placeholder="t('products.searchBoxPlaceholder')"
          :aria-label="t('products.searchLabel')"
        />
        <button v-if="searchQuery" type="button" class="pv-search-clear" :aria-label="t('blog.searchClear')" @click="clearSearch">
          <X />
        </button>
      </label>
    </div>

    <!-- 商品 -->
    <section class="st-container pv-body">
      <div v-if="loading" class="pv-grid">
        <div v-for="i in 9" :key="i" class="pv-skel" aria-hidden="true"></div>
      </div>

      <template v-else-if="products.length">
        <div class="pv-grid">
          <ProductTile
            v-for="product in products"
            :key="product.id"
            :slug="product.slug"
            :title="getLocalizedText(product.title)"
            :price="formatPrice(product.price_amount, siteCurrency)"
            :eyebrow="product.category ? getLocalizedText(product.category.name) : (product.tags?.[0] || '')"
            :image="getFirstImageUrl(product.images)"
          />
        </div>

        <nav v-if="totalPages > 1" class="pv-pager">
          <button type="button" class="pv-page" :disabled="currentPage <= 1" @click="changePage(currentPage - 1)">
            <ChevronLeft />
          </button>
          <button
            v-for="p in pageWindow"
            :key="p"
            type="button"
            class="pv-page"
            :class="{ 'is-active': p === currentPage }"
            @click="changePage(p)"
          >{{ p }}</button>
          <button type="button" class="pv-page" :disabled="currentPage >= totalPages" @click="changePage(currentPage + 1)">
            <ChevronRight />
          </button>
        </nav>
      </template>

      <div v-else class="pv-empty">
        <component :is="searchQuery || selectedCategory ? SearchX : PackageOpen" class="pv-empty-icon" />
        <p>{{ (searchQuery || selectedCategory) ? t('products.emptyFiltered') : t('products.empty') }}</p>
        <button v-if="searchQuery || selectedCategory" type="button" class="pv-clear" @click="resetFilters">
          {{ t('products.clearFilters') }}
        </button>
      </div>
    </section>

    <!-- 服务保障 -->
    <ServiceShelf
      class="pv-extras"
      :title="t('storefront.helpTitle')"
      :secondary="t('storefront.helpSub')"
      :items="serviceTiles"
    />

    <!-- 最新动态 -->
    <section v-if="latestVisible && posts.length" class="st-container st-shelf">
      <div class="st-shelf-head">
        <h2 class="st-shelf-title">{{ t('storefront.newsTitle') }}</h2>
        <p class="st-shelf-sub">{{ t('storefront.newsSub') }}</p>
      </div>
      <div class="st-tiles">
        <RouterLink
          v-for="post in posts"
          :key="post.id"
          class="st-tile"
          :to="`/blog/${post.slug}`"
        >
          <span class="st-tile-desc">{{ formatDate(post.published_at) }}</span>
          <span class="st-tile-title">{{ getLocalizedText(post.title) }}</span>
          <span class="st-tile-desc line-clamp-3">{{ getLocalizedText(post.summary) }}</span>
        </RouterLink>
      </div>
    </section>

    <div class="pb-10"></div>

    <ProductQuickBuy
      v-if="quickBuyProduct"
      :product="quickBuyProduct"
      :visible="quickBuyVisible"
      @update:visible="quickBuyVisible = $event"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ChevronLeft, ChevronRight, PackageOpen, Search, SearchX, X, LayoutGrid, Tag, ShoppingBag, Zap, ShieldCheck, MessageCircle } from 'lucide-vue-next'
import { ref } from 'vue'
import ServiceShelf, { type ServiceTile } from './components/store/ServiceShelf.vue'
import { postAPI } from '../../api'
import { useAppStore } from '../../stores/app'
import { useProductList } from '../../composables/useProductList'
import StoreHero from './components/store/StoreHero.vue'
import FamilyRail, { type RailItem } from './components/store/FamilyRail.vue'
import { usePageSeo } from '../../composables/usePageSeo'
import { getImageUrl } from '../../utils/image'
import { useLocalized } from '../../composables/useProduct'
import { getFirstImageUrl } from '../../utils/image'
import type { PublicCategory } from '../../utils/category'
import ProductTile from './components/store/ProductTile.vue'
import ProductQuickBuy from '../../components/ProductQuickBuy.vue'

const { t } = useI18n()
const route = useRoute()
const { getLocalizedText, siteCurrency, formatPrice } = useLocalized()

const quickBuyProduct = ref<any>(null)
const quickBuyVisible = ref(false)

const {
  loading,
  products,
  selectedCategory,
  searchQuery,
  currentPage,
  totalPages,
  categoryGroups,
  categoryMap,
  selectCategory,
  changePage,
  clearSearch,
  initialize,
  cleanup,
} = useProductList({ pageSize: 12, homeRouteName: 'products' })

const catName = (cat: PublicCategory) => getLocalizedText(cat.name) || cat.slug || ''

/** 商店页右侧说明与链接 */
const storeLead = computed(() => t('products.lead'))
const storeLinks = computed(() => [
  { label: t('storefront.allProducts'), to: '/products', external: false },
  { label: t('storefront.contact'), to: '/about', external: false },
])

/** 分类轨道：全部 + 一级分类 */
const railItems = computed<RailItem[]>(() => {
  const items: RailItem[] = [
    { key: 'all', label: t('storefront.allProducts'), to: '/products', fallback: LayoutGrid },
  ]
  for (const group of categoryGroups.value) {
    items.push({
      key: group.id ?? group.slug,
      label: catName(group),
      to: `/categories/${group.slug}`,
      icon: group.icon ? getImageUrl(group.icon) : undefined,
      fallback: Tag,
    })
  }
  return items
})

const selectedCategoryName = computed(() => {
  if (!selectedCategory.value) return ''
  const cat = categoryMap.value.get(selectedCategory.value)
  return cat ? catName(cat) : ''
})

const pageTitle = computed(() => {
  if (route.name === 'category-products') return selectedCategoryName.value || t('nav.products')
  return t('products.allCategories')
})

// 分页窗口：当前页前后各 2 页
const pageWindow = computed(() => {
  const total = totalPages.value
  const cur = currentPage.value
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

usePageSeo({
  canonicalPath: () => route.path,
  title: () => pageTitle.value,
})

const appStore = useAppStore()

// ---------- 服务保障 ----------
const supportHref = computed(() => {
  const c = appStore.config?.contact as { telegram?: string; whatsapp?: string; email?: string; support_url?: string } | undefined
  if (c?.support_url) return c.support_url
  if (c?.telegram) return c.telegram
  if (c?.email) return `mailto:${c.email}`
  return '/about'
})
const serviceTiles = computed<ServiceTile[]>(() => [
  { key: 'buy', icon: ShoppingBag, title: t('storefront.tileBuyTitle'), desc: t('storefront.tileBuyDesc'), to: '/notice' },
  { key: 'delivery', icon: Zap, title: t('storefront.tileDeliveryTitle'), desc: t('storefront.tileDeliveryDesc'), to: '/notice' },
  { key: 'refund', icon: ShieldCheck, title: t('storefront.tileRefundTitle'), desc: t('storefront.tileRefundDesc'), to: '/terms' },
  {
    key: 'contact',
    icon: MessageCircle,
    title: t('storefront.tileContactTitle'),
    desc: t('storefront.tileContactDesc'),
    to: supportHref.value,
    external: /^(https?:|mailto:)/.test(supportHref.value),
  },
])

// ---------- 最新动态 ----------
const navBuiltin = computed(() => (appStore.config?.nav_config as { builtin?: Record<string, boolean> } | undefined)?.builtin)
const blogEnabled = computed(() => navBuiltin.value?.blog !== false)
const noticeEnabled = computed(() => navBuiltin.value?.notice !== false)
const latestVisible = computed(() => blogEnabled.value || noticeEnabled.value)
const posts = ref<any[]>([])
const formatDate = (value: string) => (value ? new Date(value).toLocaleDateString() : '')
const loadPosts = async () => {
  if (!latestVisible.value) return
  try {
    const params: Record<string, unknown> = { page: 1, page_size: 3 }
    if (blogEnabled.value && !noticeEnabled.value) params.type = 'blog'
    if (!blogEnabled.value && noticeEnabled.value) params.type = 'notice'
    const res = await postAPI.list(params)
    posts.value = res.data.data || []
  } catch {
    posts.value = []
  }
}

onMounted(() => {
  void initialize()
  void loadPosts()
})
onUnmounted(() => cleanup())
</script>

<style scoped>
.pv {
  background: var(--st-bg);
  color: var(--st-text);
  padding-bottom: 80px;
}

/* ---------- 页头 ---------- */
.pv-head {
  padding-top: 48px;
  padding-bottom: 8px;
}
.pv-eyebrow {
  margin: 0 0 8px;
  font-size: 17px;
  font-weight: 600;
  color: var(--st-text-2);
}
.pv-title {
  margin: 0;
  font-family: var(--st-font-display);
  font-size: 32px;
  line-height: 1.08;
  font-weight: 600;
  letter-spacing: -0.01em;
}
@media (min-width: 735px) {
  .pv-title {
    font-size: 64px;
    letter-spacing: -0.015em;
  }
}
.pv-lead {
  margin: 14px 0 0;
  font-size: 19px;
  color: var(--st-text-2);
}
@media (min-width: 735px) {
  .pv-lead {
    font-size: 21px;
  }
}

/* ---------- 工具条 ---------- */
.pv-toolbar--end {
  justify-content: flex-end;
}
.pv-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  padding-top: 28px;
  padding-bottom: 20px;
}
.pv-chips {
  display: flex;
  gap: 8px;
  overflow-x: auto;
  scrollbar-width: none;
  flex: 1;
  min-width: 0;
  padding-bottom: 2px;
}
.pv-chips::-webkit-scrollbar {
  display: none;
}
.pv-chip {
  flex: none;
  padding: 8px 16px;
  border: 0;
  border-radius: 980px;
  background: var(--st-bg-alt);
  color: var(--st-text);
  font-size: 14px;
  line-height: 1.2;
  cursor: pointer;
  white-space: nowrap;
  transition: background 320ms var(--st-ease), color 320ms var(--st-ease);
}
.pv-chip:hover {
  background: #e8e8ed;
}
.pv-chip.is-active {
  background: var(--st-text);
  color: var(--st-bg);
}

.pv-search {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 38px;
  padding: 0 14px;
  border-radius: 980px;
  background: var(--st-bg-alt);
  flex: none;
  width: 100%;
}
@media (min-width: 735px) {
  .pv-search {
    width: 240px;
  }
}
.pv-search-icon {
  width: 16px;
  height: 16px;
  color: var(--st-text-2);
  flex: none;
}
.pv-search input {
  flex: 1;
  min-width: 0;
  border: 0;
  background: none;
  outline: none;
  font-size: 14px;
  color: var(--st-text);
}
.pv-search input::-webkit-search-cancel-button {
  display: none;
}
.pv-search-clear {
  display: grid;
  place-items: center;
  width: 18px;
  height: 18px;
  border: 0;
  background: none;
  color: var(--st-text-2);
  cursor: pointer;
}
.pv-search-clear svg {
  width: 14px;
  height: 14px;
}

/* ---------- 网格 ---------- */
.pv-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}
@media (min-width: 735px) {
  .pv-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 16px;
  }
}
@media (min-width: 1069px) {
  .pv-grid {
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 20px;
  }
}
.pv-skel {
  aspect-ratio: 4 / 5;
  border-radius: 18px;
  background: var(--st-bg-alt);
  animation: pv-pulse 1.4s ease-in-out infinite;
}
@keyframes pv-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

/* ---------- 分页 ---------- */
.pv-extras {
  margin-top: 24px;
}
.pv-pager {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 4px;
  padding-top: 40px;
}
.pv-page {
  display: grid;
  place-items: center;
  min-width: 36px;
  height: 36px;
  padding: 0 8px;
  border: 0;
  border-radius: 980px;
  background: none;
  color: var(--st-text-2);
  font-size: 14px;
  cursor: pointer;
}
.pv-page:hover:not(:disabled) {
  background: var(--st-bg-alt);
}
.pv-page.is-active {
  background: var(--st-text);
  color: var(--st-bg);
}
.pv-page:disabled {
  opacity: 0.3;
  cursor: default;
}
.pv-page svg {
  width: 16px;
  height: 16px;
}

/* ---------- 空态 ---------- */
.pv-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 80px 0;
  text-align: center;
  color: var(--st-text-2);
}
.pv-empty-icon {
  width: 44px;
  height: 44px;
  opacity: 0.4;
}
.pv-clear {
  margin-top: 8px;
  padding: 8px 18px;
  border: 0;
  border-radius: 980px;
  background: var(--st-blue);
  color: #fff;
  font-size: 14px;
  cursor: pointer;
}
</style>
