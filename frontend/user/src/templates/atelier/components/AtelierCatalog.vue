<template>
  <div>
    <div class="mx-auto w-full max-w-[1240px] px-4 sm:px-6">
      <!-- 顶部命令搜索中枢（彻底取消左侧边栏，无虚构数据，全部绑定真实站点配置与分类/搜索接口） -->
      <section class="mx-auto max-w-[780px] text-center" :class="compactHero ? 'pb-7 pt-2' : 'pb-9 pt-4 md:pt-8'">
        <div
          v-if="!compactHero"
          class="inline-flex items-center gap-2 rounded-full border bg-card/90 px-3 py-1 text-xs font-medium text-muted-foreground shadow-xs"
        >
          <span class="prism-pulse" aria-hidden="true"></span>
          <span>{{ t('atelier.trustAuto') }}</span>
        </div>

        <h1
          class="font-bold tracking-[-0.035em] text-foreground"
          :class="compactHero ? 'text-2xl sm:text-3xl' : 'mt-4 text-3xl leading-[1.15] sm:text-4xl md:text-[42px]'"
        >
          {{ heroHeadline }}
        </h1>

        <p v-if="heroSubline" class="mx-auto mt-2.5 max-w-[54ch] text-sm text-muted-foreground sm:text-[15px]">
          {{ heroSubline }}
        </p>

        <!-- 命令面板风格大搜索输入框 -->
        <div class="prism-search-hub mx-auto mt-6 flex max-w-[640px] items-center gap-2.5 rounded-[16px] px-3.5 py-1.5">
          <Search class="h-[17px] w-[17px] shrink-0 text-muted-foreground" aria-hidden="true" />
          <input
            ref="searchInputRef"
            v-model="searchQuery"
            type="search"
            class="at-search h-10 min-w-0 flex-1 border-none bg-transparent text-sm text-foreground outline-none placeholder:text-muted-foreground sm:text-[15px]"
            :placeholder="t('products.searchPlaceholder')"
            :aria-label="t('products.searchPlaceholder')"
            @keydown.enter="onSearch"
          />
          <button
            v-if="searchQuery"
            type="button"
            class="grid h-7 w-7 shrink-0 place-items-center rounded-lg text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
            :aria-label="t('products.clearFilters')"
            @click="clearSearch"
          >
            <X class="h-4 w-4" />
          </button>
          <button
            type="button"
            class="hidden h-7 shrink-0 items-center rounded-[7px] border bg-secondary px-2 font-mono text-[10.5px] font-semibold text-muted-foreground transition-colors hover:text-foreground sm:inline-flex"
            @click="onSearch"
          >
            ENTER
          </button>
        </div>

        <!-- 真实一级分类快捷胶囊带 -->
        <nav
          v-if="categoryGroups.length > 0"
          class="mt-4 flex flex-wrap items-center justify-center gap-2"
          :aria-label="t('products.categoryLabel')"
        >
          <button
            type="button"
            class="inline-flex h-8.5 items-center gap-1.5 rounded-full border px-3.5 text-xs font-semibold transition-all focus-visible:outline-2 focus-visible:outline-primary"
            :class="selectedCategory === null
              ? 'border-primary bg-primary text-primary-foreground shadow-xs'
              : 'bg-card text-muted-foreground hover:border-primary/40 hover:text-foreground'"
            :aria-pressed="selectedCategory === null"
            @click="selectCategory(null)"
          >
            <LayoutGrid class="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
            <span>{{ t('products.allCategories') }}</span>
          </button>

          <button
            v-for="group in categoryGroups"
            :key="group.id"
            type="button"
            class="inline-flex h-8.5 items-center gap-1.5 rounded-full border px-3.5 text-xs font-semibold transition-all focus-visible:outline-2 focus-visible:outline-primary"
            :class="activeGroup?.id === group.id
              ? 'border-primary bg-primary text-primary-foreground shadow-xs'
              : 'bg-card text-muted-foreground hover:border-primary/40 hover:text-foreground'"
            :aria-pressed="activeGroup?.id === group.id"
            @click="selectCategory(group.id)"
          >
            <img
              v-if="group.icon"
              :src="getImageUrl(group.icon)"
              alt=""
              loading="lazy"
              class="h-4 w-4 shrink-0 rounded-full object-cover"
            />
            <span class="max-w-[10rem] truncate">{{ getLocalizedText(group.name) }}</span>
          </button>
        </nav>

        <!-- 若当前选中的一级分类包含真实二级子分类，在下方展开一行精准筛选条 -->
        <div
          v-if="activeGroup && activeGroup.children.length > 0"
          class="mt-2.5 flex flex-wrap items-center justify-center gap-1.5"
        >
          <button
            type="button"
            class="inline-flex h-7 max-w-[14rem] items-center rounded-lg border px-2.5 text-[11.5px] font-semibold transition-colors"
            :class="selectedCategory === activeGroup.id
              ? 'border-primary/50 bg-primary/10 text-primary'
              : 'bg-card/70 text-muted-foreground hover:text-foreground'"
            @click="selectCategory(activeGroup.id)"
          >
            <span class="truncate">{{ getLocalizedText(activeGroup.name) }}</span>
          </button>
          <button
            v-for="child in activeGroup.children"
            :key="child.id"
            type="button"
            class="inline-flex h-7 max-w-[14rem] items-center rounded-lg border px-2.5 text-[11.5px] font-medium transition-colors"
            :class="selectedCategory === child.id
              ? 'border-primary/50 bg-primary/10 font-semibold text-primary'
              : 'bg-card/70 text-muted-foreground hover:border-hairline-strong hover:text-foreground'"
            :aria-pressed="selectedCategory === child.id"
            @click="selectCategory(child.id)"
          >
            <span class="truncate">{{ getLocalizedText(child.name) }}</span>
          </button>
        </div>
      </section>

      <!-- 插槽：首页在此插入 12 列不对称 Bento 精选展位（真实 Banner + 真实公告/文章） -->
      <slot name="showcase" />

      <!-- 实时商品货架顶栏：分类标题 + 接口总数 + 精密卡片/规格横表视图切换 -->
      <div ref="shelfRef" class="mb-4 mt-8 flex flex-wrap items-center justify-between gap-3">
        <div class="flex min-w-0 items-baseline gap-2.5">
          <h2 class="truncate text-lg font-bold tracking-[-0.02em] text-foreground">
            {{ selectedName || t('atelier.catalogHeading') }}
          </h2>
          <span v-if="!loading && displayTotal > 0" class="font-mono text-xs text-muted-foreground">
            {{ displayTotal }}
          </span>
        </div>

        <div class="inline-flex items-center rounded-[10px] border bg-card p-0.5 shadow-xs">
          <button
            type="button"
            class="inline-flex h-7.5 items-center gap-1.5 rounded-[8px] px-2.5 text-xs font-semibold transition-colors"
            :class="!isTableView ? 'bg-secondary text-foreground' : 'text-muted-foreground hover:text-foreground'"
            :aria-pressed="!isTableView"
            @click="viewMode = 'grid'"
          >
            <LayoutGrid class="h-3.5 w-3.5" aria-hidden="true" />
            <span>{{ t('atelier.viewCard') }}</span>
          </button>
          <button
            type="button"
            class="inline-flex h-7.5 items-center gap-1.5 rounded-[8px] px-2.5 text-xs font-semibold transition-colors"
            :class="isTableView ? 'bg-secondary text-foreground' : 'text-muted-foreground hover:text-foreground'"
            :aria-pressed="isTableView"
            @click="viewMode = 'list'"
          >
            <List class="h-3.5 w-3.5" aria-hidden="true" />
            <span>{{ t('atelier.viewTable') }}</span>
          </button>
        </div>
      </div>

      <!-- 骨架屏 -->
      <div
        v-if="loading"
        :class="isTableView ? 'prism-card overflow-hidden rounded-[16px] bg-card' : gridClass"
        aria-hidden="true"
      >
        <template v-if="isTableView">
          <div v-for="i in 8" :key="i" class="flex items-center gap-3.5 border-b px-5 py-3.5 last:border-b-0">
            <div class="h-10 w-10 shrink-0 rounded-[10px] theme-skeleton"></div>
            <div class="h-4 w-1/3 rounded theme-skeleton"></div>
            <div class="ml-auto h-5 w-20 rounded theme-skeleton"></div>
          </div>
        </template>
        <div v-for="i in 6" v-else :key="i" class="prism-card rounded-[16px] bg-card p-4.5">
          <div class="flex items-start gap-3.5">
            <div class="h-12 w-12 shrink-0 rounded-[12px] theme-skeleton"></div>
            <div class="min-w-0 flex-1 space-y-2">
              <div class="h-3 w-20 rounded theme-skeleton"></div>
              <div class="h-4 w-4/5 rounded theme-skeleton"></div>
            </div>
          </div>
          <div class="mt-3.5 h-3.5 w-full rounded theme-skeleton"></div>
          <div class="mt-2 h-3.5 w-2/3 rounded theme-skeleton"></div>
          <div class="mt-5 flex items-center justify-between border-t pt-3.5">
            <div class="h-6 w-20 rounded theme-skeleton"></div>
            <div class="h-8 w-20 rounded-[9px] theme-skeleton"></div>
          </div>
        </div>
      </div>

      <!-- 真实商品分组展示（统一分组标题，内部按视图模式切换规格横表或精密卡片网格） -->
      <template v-else-if="groups.length > 0">
        <section
          v-for="group in groups"
          :key="group.categoryId ?? 'uncategorized'"
          :class="isTableView ? 'mb-8 last:mb-0' : 'mb-9 last:mb-0'"
        >
          <div
            v-if="groups.length > 1 || !selectedCategory"
            class="flex items-center justify-between gap-3"
            :class="isTableView ? 'mb-3' : 'mb-3.5'"
          >
            <div class="flex min-w-0 items-center gap-2">
              <img
                v-if="group.categoryIcon"
                :src="getImageUrl(group.categoryIcon)"
                alt=""
                loading="lazy"
                class="h-6 w-6 shrink-0 rounded-[7px] border object-cover"
              />
              <span
                v-else
                class="atelier-stage grid h-6 w-6 shrink-0 place-items-center rounded-[7px] text-xs font-bold text-[hsl(var(--h)_48%_var(--at-glyph-l))]"
                :style="{ '--h': placeholderHue(group.categoryId ?? group.categoryName) }"
              >
                {{ Array.from(group.categoryName)[0] }}
              </span>
              <h3 class="truncate text-[15.5px] font-bold tracking-[-0.015em] text-foreground">
                {{ group.categoryName }}
              </h3>
              <span class="font-mono text-xs text-muted-foreground">{{ group.products.length }}</span>
            </div>
            <button
              v-if="group.categoryId && selectedCategory !== group.categoryId"
              type="button"
              class="inline-flex shrink-0 items-center gap-0.5 text-xs font-semibold text-muted-foreground transition-colors hover:text-primary"
              @click="selectCategory(group.categoryId)"
            >
              <span>{{ t('home.featured.viewAll') }}</span>
              <ChevronRight class="h-3.5 w-3.5" aria-hidden="true" />
            </button>
          </div>

          <div v-if="isTableView" class="prism-card overflow-hidden rounded-[16px] bg-card">
            <div
              class="hidden items-center gap-x-4 border-b bg-secondary/50 px-5 py-2.5 text-[11.5px] font-semibold tracking-[0.02em] text-muted-foreground"
              :class="atelierRowColsClass"
              aria-hidden="true"
            >
              <span>{{ t('bottomNav.products') }}</span>
              <span>{{ t('productDetail.fulfillmentLabel') }}</span>
              <span>{{ t('quickBuy.stock') }}</span>
              <span class="text-right">{{ t('products.price') }}</span>
              <span></span>
            </div>
            <div>
              <AtelierProductCard
                v-for="product in group.products"
                :key="product.id"
                :product="product"
                :category-name="group.categoryName"
                row
                @click="goToProduct"
                @quick-buy="openQuickBuy"
              />
            </div>
          </div>

          <div v-else :class="gridClass">
            <AtelierProductCard
              v-for="product in group.products"
              :key="product.id"
              :product="product"
              :category-name="group.categoryName"
              @click="goToProduct"
              @quick-buy="openQuickBuy"
            />
          </div>
        </section>

        <PaginationNav
          :current-page="currentPage"
          :total-pages="totalPages"
          :loading="loading"
          compact
          :scroll-top="false"
          @change-page="changePage"
        />
      </template>

      <EmptyState
        v-else
        size="lg"
        :icon="(searchQuery || selectedCategory) ? 'search' : 'package'"
        :title="(searchQuery || selectedCategory) ? t('products.emptyFiltered') : t('products.empty')"
      >
        <template v-if="searchQuery || selectedCategory" #action>
          <Button variant="secondary" class="rounded-[10px]" @click="clearSearch(); selectCategory(null)">
            {{ t('products.clearFilters') }}
          </Button>
        </template>
      </EmptyState>
    </div>

    <ProductQuickBuy
      v-if="quickBuyProduct"
      :product="quickBuyProduct"
      :visible="quickBuyVisible"
      @update:visible="quickBuyVisible = $event"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ChevronRight, LayoutGrid, List, Search, X } from 'lucide-vue-next'
import { getImageUrl, placeholderHue } from '../../../utils/image'
import { useLocalized } from '../../../composables/useProduct'
import { useProductList } from '../../../composables/useProductList'
import { useProductListGroups } from '../../../composables/useProductListGroups'
import { useFooterConfig } from '../../../composables/useFooterConfig'
import AtelierProductCard, { atelierRowColsClass } from './AtelierProductCard.vue'
import ProductQuickBuy from '../../../components/ProductQuickBuy.vue'
import PaginationNav from '../../../components/PaginationNav.vue'
import EmptyState from '../../../components/EmptyState.vue'
import { Button } from '@/components/ui/button'

const props = withDefaults(defineProps<{
  homeRouteName?: string
  layout?: 'grid' | 'list'
  /** 商品列表独立页使用紧凑搜索中枢头部 */
  compactHero?: boolean
  /** 可选自定义主标题（默认读取站点品牌名或商品页标题） */
  heading?: string
  /** 可选自定义副标题（默认读取站点真实简介） */
  subtitle?: string
}>(), {
  homeRouteName: 'home',
  layout: 'grid',
  compactHero: false,
  heading: '',
  subtitle: '',
})

const emit = defineEmits<{
  categoryName: [name: string]
}>()

const router = useRouter()
const { t } = useI18n()
const { brandSiteName, brandDescription } = useFooterConfig()
const shelfRef = ref<HTMLElement | null>(null)
const searchInputRef = ref<HTMLInputElement | null>(null)
const { getLocalizedText } = useLocalized()

const {
  loading,
  products,
  selectedCategory,
  searchQuery,
  currentPage,
  totalPages,
  total,
  categoryGroups,
  categoryMap,
  selectCategory,
  changePage,
  clearSearch,
  onSearch,
  initialize,
  cleanup,
} = useProductList({
  pageSize: 24,
  homeRouteName: props.homeRouteName,
  getScrollTop: () => Math.max(0, (shelfRef.value?.getBoundingClientRect().top ?? 0) + window.scrollY - 92),
})

const groups = useProductListGroups(products, categoryMap)
const displayTotal = computed(() => total.value || products.value.length)

// 初始视图跟随后台配置的 template_mode，同时允许用户自由切换精密卡片 / 规格横表
const viewMode = ref<'grid' | 'list'>(props.layout)
watch(() => props.layout, (val) => {
  viewMode.value = val
})
const isTableView = computed(() => viewMode.value === 'list')

const gridClass = 'grid grid-cols-1 gap-3.5 sm:grid-cols-2 lg:grid-cols-3'

const heroHeadline = computed(() => {
  if (props.heading) return props.heading
  return props.compactHero ? t('nav.products') : brandSiteName.value
})

const heroSubline = computed(() => {
  if (props.subtitle) return props.subtitle
  return brandDescription.value || t('atelier.heroSubtitle')
})

const activeGroup = computed(() => {
  const id = selectedCategory.value
  if (!id) return undefined
  return categoryGroups.value.find((group) => group.id === id || group.children.some((child) => child.id === id))
})

const selectedName = computed(() => {
  const category = selectedCategory.value ? categoryMap.value.get(selectedCategory.value) : undefined
  return category ? getLocalizedText(category.name) : ''
})
watch(selectedName, (name) => emit('categoryName', name))

const quickBuyProduct = ref<any>(null)
const quickBuyVisible = ref(false)
const openQuickBuy = (product: any) => {
  quickBuyProduct.value = product
  quickBuyVisible.value = true
}

const goToProduct = (slug: string) => {
  router.push(`/products/${slug}`)
}

const focusSearchInput = () => {
  searchInputRef.value?.scrollIntoView({ behavior: 'smooth', block: 'center' })
  searchInputRef.value?.focus()
}

onMounted(() => {
  initialize()
  window.addEventListener('atelier:focus-search', focusSearchInput)
  if (history.state?.focusSearch) {
    // 消费一次性聚焦意图，防止刷新或后退时重复触发
    history.replaceState({ ...history.state, focusSearch: false }, '')
    void nextTick(() => focusSearchInput())
  }
})

onUnmounted(() => {
  cleanup()
  window.removeEventListener('atelier:focus-search', focusSearchInput)
})
</script>

<style scoped>
.at-search::-webkit-search-cancel-button {
  display: none;
}
</style>
