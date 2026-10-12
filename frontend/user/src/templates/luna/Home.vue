<template>
  <div class="mx-auto w-full max-w-[1100px] px-3 pt-5 sm:px-6 sm:pt-8">
    <!-- 页面只有「选择分类 / 选择商品」两个二级标题，补一个仅供读屏与搜索引擎使用的一级标题 -->
    <h1 class="sr-only">{{ pageHeading }}</h1>
    <section class="luna-panel px-4 py-5 sm:px-6 sm:py-6">
      <!-- 选择分类：没有「全部商品」，默认选中第一个分类 -->
      <template v-if="categoryGroups.length">
        <h2 class="flex items-center gap-2 text-lg font-semibold text-foreground">
          <LayoutGrid class="h-5 w-5 flex-none text-primary" />
          {{ t('luna.chooseCategory') }}
        </h2>
        <div class="mt-4 flex flex-wrap gap-2.5">
          <button
            v-for="group in categoryGroups"
            :key="group.id"
            type="button"
            class="inline-flex h-12 min-w-[108px] max-w-full items-center justify-center gap-2 rounded-[10px] px-5 text-sm transition"
            :class="activeGroup?.id === group.id ? 'luna-gradient font-semibold' : 'bg-secondary text-foreground hover:bg-primary/10 hover:text-primary'"
            :aria-pressed="activeGroup?.id === group.id"
            @click="selectCategory(group.id)"
          >
            <img v-if="group.icon" :src="getImageUrl(group.icon)" alt="" loading="lazy" class="h-5 w-5 flex-none rounded object-cover" />
            <span class="truncate">{{ catName(group) }}</span>
          </button>
        </div>

        <!-- 子分类：点父分类看它和所有子分类的商品，点子分类只看该子分类 -->
        <div v-if="activeGroup?.children.length" class="mt-3 flex flex-wrap gap-2">
          <button
            v-for="child in activeGroup.children"
            :key="child.id"
            type="button"
            class="inline-flex h-8 max-w-full items-center rounded-full border px-3.5 text-[13px] transition-colors"
            :class="selectedCategory === child.id ? 'border-primary bg-primary/10 font-semibold text-primary' : 'border-border text-muted-foreground hover:border-primary/50 hover:text-primary'"
            :aria-pressed="selectedCategory === child.id"
            @click="selectCategory(selectedCategory === child.id ? activeGroup.id : child.id)"
          >
            <span class="truncate">{{ catName(child) }}</span>
          </button>
        </div>
      </template>

      <!-- 选择商品 -->
      <div :class="{ 'mt-5 border-t pt-5': categoryGroups.length }">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <h2 class="flex items-center gap-2 text-lg font-semibold text-foreground">
            <ShoppingBag class="h-5 w-5 flex-none text-primary" />
            {{ t('luna.chooseProduct') }}
          </h2>
          <div class="relative w-full sm:w-64">
            <Search class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <input
              v-model="searchQuery"
              class="h-9 w-full rounded-full border border-border bg-card pl-9 pr-9 text-sm text-foreground transition-colors placeholder:text-muted-foreground hover:border-hairline-strong focus:border-primary focus:outline-none"
              :placeholder="t('products.searchBoxPlaceholder')"
              :aria-label="t('products.searchLabel')"
            />
            <button v-if="searchQuery" type="button" class="absolute right-2 top-1/2 grid h-6 w-6 -translate-y-1/2 place-items-center rounded-full text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground" :aria-label="t('blog.searchClear')" @click="clearSearch"><X class="h-3.5 w-3.5" /></button>
          </div>
        </div>

        <!-- 骨架 -->
        <div v-if="loading" class="mt-4 grid grid-cols-1 gap-3" :class="gridClass">
          <div v-for="i in 6" :key="i" class="h-[100px] animate-pulse rounded-[10px] bg-secondary"></div>
        </div>

        <div v-else-if="products.length" class="mt-4 grid grid-cols-1 gap-3" :class="gridClass">
          <LunaProductCard v-for="product in products" :key="product.id" :product="product" @quick-buy="openQuickBuy" />
        </div>

        <!-- 空态 -->
        <div v-else class="mt-4 flex flex-col items-center gap-3 rounded-[10px] border border-dashed py-14 text-center text-sm text-muted-foreground">
          <component :is="searchQuery ? SearchX : PackageOpen" class="h-11 w-11 opacity-60" />
          <p>{{ searchQuery ? t('products.emptyFiltered') : t('products.empty') }}</p>
          <Button v-if="searchQuery" variant="outline" size="sm" class="rounded-full" @click="clearSearch">{{ t('products.clearFilters') }}</Button>
        </div>

        <PaginationNav :current-page="currentPage" :total-pages="totalPages" :loading="loading" :scroll-top="false" compact @change-page="changePage" />
      </div>
    </section>

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
import { LayoutGrid, PackageOpen, Search, SearchX, ShoppingBag, X } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { normalizeCategoryParentId, type PublicCategory } from '../../utils/category'
import { getImageUrl } from '../../utils/image'
import { useLocalized } from '../../composables/useProduct'
import { useProductList } from '../../composables/useProductList'
import { usePageSeo } from '../../composables/usePageSeo'
import { useAnnouncement, type HomeAnnouncement } from '../../composables/useAnnouncement'
import { useAppStore } from '../../stores/app'
import LunaProductCard from './components/LunaProductCard.vue'
import PaginationNav from '../../components/PaginationNav.vue'
import ProductQuickBuy from '../../components/ProductQuickBuy.vue'
import AnnouncementModal from '../../components/AnnouncementModal.vue'

const route = useRoute()
const { t } = useI18n()
const { getLocalizedText } = useLocalized()
const appStore = useAppStore()

// luna 的首页、/products、/categories/:slug 是同一个页面：分类 + 该分类下的商品
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
} = useProductList({ pageSize: 24, homeRouteName: 'home', defaultToFirstCategory: true })

const catName = (cat: PublicCategory) => getLocalizedText(cat.name) || cat.slug || ''

/** 当前高亮的顶级分类：选中的就是它，或选中的是它的子分类 */
const activeGroup = computed(() => {
  const selected = selectedCategory.value ? categoryMap.value.get(selectedCategory.value) : undefined
  if (!selected) return undefined
  const parentId = normalizeCategoryParentId(selected.parent_id)
  return categoryGroups.value.find((group) => group.id === (parentId > 0 ? parentId : selected.id))
})

// 后台「商品布局」：卡片模式多列，列表模式单列（模板里的 grid-cols-1 保证轨道是 minmax(0,1fr)，长商品名不会把列撑宽）
const gridClass = computed(() => appStore.config?.template_mode === 'list' ? '' : 'sm:grid-cols-2 lg:grid-cols-3')

// ==================== 快速购买 ====================
const quickBuyProduct = ref<any>(null)
const quickBuyVisible = ref(false)
const openQuickBuy = (product: any) => {
  quickBuyProduct.value = product
  quickBuyVisible.value = true
}

// ==================== 公告弹窗（版本化可关闭） ====================
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
const selectedCategoryName = computed(() => {
  const cat = selectedCategory.value ? categoryMap.value.get(selectedCategory.value) : undefined
  return cat ? catName(cat) : ''
})

/** 一级标题：分类页用分类名，首页用站点名 */
const pageHeading = computed(() => {
  if (route.name === 'category-products' && selectedCategoryName.value) return selectedCategoryName.value
  return String(appStore.config?.brand?.site_name || '').trim() || t('nav.products')
})

usePageSeo({
  canonicalPath: () => route.path,
  title: () => {
    if (route.name === 'category-products') return selectedCategoryName.value || t('nav.products')
    if (route.name === 'products') return t('nav.products')
    return undefined
  },
})

onMounted(async () => {
  await appStore.loadConfig()
  await initialize()
  showAnnouncementIfNeeded()
})

onUnmounted(() => cleanup())
</script>
