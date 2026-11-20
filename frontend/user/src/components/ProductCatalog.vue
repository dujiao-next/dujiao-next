<template>
  <div>
    <div class="container mx-auto px-4 lg:flex lg:items-start lg:gap-6">
      <!-- 大屏：左侧分类栏，跟随滚动。小屏：搜索框 + 分类开关，点开才展开列表 -->
      <aside class="catalog-aside lg:sticky lg:top-20 lg:w-60 lg:shrink-0 lg:overflow-y-auto lg:rounded-[22px] lg:bg-card lg:p-3">
        <div class="flex gap-2">
          <div class="relative min-w-0 flex-1">
            <Search class="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" aria-hidden="true" />
            <input
              v-model="searchQuery"
              type="search"
              class="catalog-search theme-float h-10 w-full rounded-full bg-card pl-10 pr-10 text-sm lg:bg-secondary lg:shadow-none text-foreground transition-shadow placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary/30"
              :placeholder="t('products.searchPlaceholder')"
              :aria-label="t('products.searchPlaceholder')"
              @keydown.enter="onSearch"
            />
            <button
              v-if="searchQuery"
              type="button"
              class="absolute inset-y-0 right-3 flex items-center text-muted-foreground transition-colors hover:text-foreground"
              :aria-label="t('products.clearFilters')"
              @click="clearSearch"
            >
              <X class="h-4 w-4" />
            </button>
          </div>
          <!-- 小屏：分类收在这个按钮后面，点开是完整列表，不用横向滑着找 -->
          <button
            type="button"
            class="theme-float inline-flex h-10 max-w-[45%] shrink-0 items-center gap-1.5 rounded-full px-4 text-sm font-medium transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary lg:hidden"
            :class="selectedCategory ? 'bg-foreground text-background' : 'bg-card text-foreground'"
            :aria-expanded="railOpen"
            @click="railOpen = !railOpen"
          >
            <span class="truncate">{{ selectedName || t('products.categoryLabel') }}</span>
            <ChevronDown class="h-4 w-4 shrink-0 transition-transform" :class="railOpen && 'rotate-180'" aria-hidden="true" />
          </button>
        </div>

        <nav
          class="catalog-rail mt-3 grid-cols-2 gap-0.5 lg:flex lg:flex-col"
          :class="railOpen ? 'grid' : 'hidden'"
          :aria-label="t('products.categoryLabel')"
        >
          <button type="button" :class="railClass(selectedCategory === null)" :aria-pressed="selectedCategory === null" @click="pickCategory(null)">
            <LayoutGrid v-if="hasCategoryIcons" class="h-5 w-5 shrink-0 p-0.5" aria-hidden="true" />
            {{ t('products.allCategories') }}
          </button>
          <template v-for="group in categoryGroups" :key="group.id">
            <button type="button" :class="railClass(activeGroup?.id === group.id)" :aria-pressed="activeGroup?.id === group.id" @click="pickCategory(group.id)">
              <img v-if="group.icon" :src="getImageUrl(group.icon)" alt="" loading="lazy" class="h-5 w-5 shrink-0 rounded-full object-cover" />
              <span v-else-if="hasCategoryIcons" class="h-5 w-5 shrink-0"></span>
              <span class="truncate">{{ getLocalizedText(group.name) }}</span>
            </button>
            <!-- 只展开当前一级分类的子分类 -->
            <div v-if="activeGroup?.id === group.id && group.children.length" class="col-span-2 my-1 ml-5 flex flex-col gap-0.5 border-l pl-2">
              <button
                v-for="child in group.children"
                :key="child.id"
                type="button"
                class="h-8 truncate rounded-lg px-2.5 text-left text-[13px] transition-colors focus-visible:outline-2 focus-visible:outline-primary"
                :class="selectedCategory === child.id ? 'bg-secondary font-semibold text-foreground' : 'text-muted-foreground hover:text-foreground'"
                :aria-pressed="selectedCategory === child.id"
                @click="pickCategory(child.id)"
              >
                {{ getLocalizedText(child.name) }}
              </button>
            </div>
          </template>
        </nav>
      </aside>

      <div class="min-w-0 flex-1 pt-4 lg:pt-0">
        <div v-if="loading" :class="isList ? 'theme-float rounded-[22px] bg-card p-2 md:p-3' : gridClass" aria-hidden="true">
          <template v-if="isList">
            <div v-for="i in 8" :key="i" class="flex items-center gap-3 px-2 py-2 md:px-3">
              <div class="h-12 w-12 shrink-0 rounded-xl theme-skeleton"></div>
              <div class="h-4 w-1/3 rounded theme-skeleton"></div>
              <div class="ml-auto h-5 w-20 rounded theme-skeleton"></div>
            </div>
          </template>
          <div v-for="i in 12" v-else :key="i" class="theme-float rounded-[22px] bg-card p-1.5">
            <div class="aspect-[16/10] rounded-2xl theme-skeleton"></div>
            <div class="space-y-2 px-2.5 pb-2 pt-3">
              <div class="h-4 w-full rounded theme-skeleton"></div>
              <div class="h-3 w-16 rounded theme-skeleton"></div>
              <div class="flex items-end justify-between pt-3">
                <div class="h-6 w-20 rounded theme-skeleton"></div>
                <div class="h-[34px] w-[34px] rounded-full theme-skeleton"></div>
              </div>
            </div>
          </div>
        </div>

        <template v-else-if="groups.length > 0">
          <!-- 列表模式：每个分类是一张价目表，表头一行、商品一行一条 -->
          <template v-if="isList">
            <section v-for="group in groups" :key="group.categoryId ?? 'uncategorized'" class="theme-float mb-5 rounded-[22px] bg-card p-2 last:mb-0 md:p-3">
              <div class="flex items-center gap-3 px-2 pb-2 pt-1.5 md:px-3 md:pt-2">
                <img v-if="group.categoryIcon" :src="getImageUrl(group.categoryIcon)" alt="" loading="lazy" class="h-8 w-8 shrink-0 rounded-[10px] object-cover" />
                <h2 class="min-w-0 truncate text-[17px] font-bold tracking-tight md:text-[19px]">{{ group.categoryName }}</h2>
                <span class="shrink-0 rounded-full bg-secondary px-2 text-xs font-semibold leading-[22px] tabular-nums text-muted-foreground">{{ group.products.length }}</span>
                <button
                  v-if="group.categoryId && selectedCategory !== group.categoryId"
                  type="button"
                  class="theme-pill-link ml-auto"
                  @click="selectCategory(group.categoryId)"
                >
                  {{ t('home.featured.viewAll') }}
                  <ChevronRight class="h-3.5 w-3.5" aria-hidden="true" />
                </button>
              </div>
              <div class="hidden items-center gap-x-4 px-3 pb-2 text-xs text-muted-foreground" :class="rowColsClass" aria-hidden="true">
                <span>{{ t('bottomNav.products') }}</span>
                <span>{{ t('productDetail.fulfillmentLabel') }}</span>
                <span>{{ t('quickBuy.stock') }}</span>
                <span class="text-right">{{ t('products.price') }}</span>
                <span></span>
              </div>
              <div class="flex flex-col gap-1.5">
                <ProductCard
                  v-for="product in group.products"
                  :key="product.id"
                  :product="product"
                  row
                  @click="goToProduct"
                  @quick-buy="openQuickBuy"
                />
              </div>
            </section>
          </template>

          <template v-else>
          <section v-for="group in groups" :key="group.categoryId ?? 'uncategorized'" class="mb-9 last:mb-0">
            <div class="mb-4 flex items-center gap-3">
              <img v-if="group.categoryIcon" :src="getImageUrl(group.categoryIcon)" alt="" loading="lazy" class="h-9 w-9 shrink-0 rounded-[11px] object-cover" />
              <h2 class="min-w-0 truncate text-[19px] font-bold tracking-tight md:text-[22px]">{{ group.categoryName }}</h2>
              <!-- 分页下一页里未必装得下整个分类，可点进去看全部 -->
              <button
                v-if="group.categoryId && selectedCategory !== group.categoryId"
                type="button"
                class="theme-pill-link ml-auto"
                @click="selectCategory(group.categoryId)"
              >
                {{ t('home.featured.viewAll') }}
                <ChevronRight class="h-3.5 w-3.5" aria-hidden="true" />
              </button>
            </div>
            <div :class="gridClass">
              <ProductCard
                v-for="(product, index) in group.products"
                :key="product.id"
                :product="product"
                :lead="index === 0 && group.products.length >= LEAD_MIN_GROUP_SIZE"
                @click="goToProduct"
                @quick-buy="openQuickBuy"
              />
            </div>
          </section>
          </template>

          <PaginationNav
            :current-page="currentPage"
            :total-pages="totalPages"
            :loading="loading"
            compact
            @change-page="changePage"
          />
        </template>

        <EmptyState
          v-else
          variant="soft"
          size="lg"
          :icon="(searchQuery || selectedCategory) ? 'search' : 'package'"
          :title="(searchQuery || selectedCategory) ? t('products.emptyFiltered') : t('products.empty')"
        >
          <template v-if="searchQuery || selectedCategory" #action>
            <Button variant="secondary" @click="clearSearch(); selectCategory(null)">
              {{ t('products.clearFilters') }}
            </Button>
          </template>
        </EmptyState>
      </div>
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
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ChevronDown, ChevronRight, LayoutGrid, Search, X } from 'lucide-vue-next'
import { getImageUrl } from '../utils/image'
import { useLocalized } from '../composables/useProduct'
import { useProductList } from '../composables/useProductList'
import { useProductListGroups } from '../composables/useProductListGroups'
import ProductCard, { rowColsClass } from './ProductCard.vue'
import ProductQuickBuy from './ProductQuickBuy.vue'
import PaginationNav from './PaginationNav.vue'
import EmptyState from './EmptyState.vue'
import { Button } from '@/components/ui/button'

const props = withDefaults(defineProps<{
  /** 取消分类筛选后回到的路由 */
  homeRouteName?: string
  /** grid：卡片网格；list：紧凑的横条，一屏能看更多商品 */
  layout?: 'grid' | 'list'
}>(), {
  homeRouteName: 'home',
  layout: 'grid',
})

const emit = defineEmits<{
  /** 当前选中分类的本地化名称（未选为空串），供页面设置 SEO 标题 */
  categoryName: [name: string]
}>()

const router = useRouter()
const { t } = useI18n()
const { getLocalizedText } = useLocalized()

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
  onSearch,
  initialize,
  cleanup,
} = useProductList({ pageSize: 24, homeRouteName: props.homeRouteName })

const groups = useProductListGroups(products, categoryMap)

// 大屏左侧有分类栏，网格比通栏时少一列
const isList = computed(() => props.layout === 'list')
const gridClass = 'grid grid-cols-2 gap-3 md:grid-cols-4 md:gap-4 lg:grid-cols-3 xl:grid-cols-5 2xl:grid-cols-6'
// 大卡占 2 列 2 行，分类里商品凑不满两行时旁边会留空，所以够多才放大首位
const LEAD_MIN_GROUP_SIZE = 7

// 只要有一个分类带图标，左侧栏就给所有项留出图标位，文字才能对齐
const hasCategoryIcons = computed(() => categoryGroups.value.some((group) => group.icon))

// 选中的是子分类时，高亮它所属的一级分类
const activeGroup = computed(() => {
  const id = selectedCategory.value
  if (!id) return undefined
  return categoryGroups.value.find((group) => group.id === id || group.children.some((child) => child.id === id))
})

const railClass = (active: boolean) => [
  'flex h-10 w-full items-center gap-2.5 rounded-xl px-3 text-left text-sm font-medium transition-colors focus-visible:outline-2 focus-visible:outline-primary',
  active ? 'bg-foreground text-background' : 'text-muted-foreground hover:bg-secondary hover:text-foreground',
]

const selectedName = computed(() => {
  const category = selectedCategory.value ? categoryMap.value.get(selectedCategory.value) : undefined
  return category ? getLocalizedText(category.name) : ''
})
watch(selectedName, (name) => emit('categoryName', name))

// 小屏的分类列表默认收起；选完就收起，选中带子分类的一级分类时留着让人继续选
const railOpen = ref(false)
const pickCategory = (id: number | null) => {
  selectCategory(id)
  railOpen.value = !!categoryGroups.value.find((group) => group.id === id)?.children.length
}
// 首页点分类会跳到商品页、组件重新挂载，上面的状态带不过去；停在带子分类的一级分类上时补开
watch(activeGroup, (group) => {
  if (group?.children.length && selectedCategory.value === group.id) railOpen.value = true
})

const quickBuyProduct = ref<any>(null)
const quickBuyVisible = ref(false)
const openQuickBuy = (product: any) => {
  quickBuyProduct.value = product
  quickBuyVisible.value = true
}

const goToProduct = (slug: string) => {
  router.push(`/products/${slug}`)
}

onMounted(initialize)
onUnmounted(cleanup)
</script>

<style scoped>
/* 分类栏只在大屏是一块浮起的面板；高度不超过视口，分类多时内部滚动 */
@media (min-width: 1024px) {
  .catalog-aside {
    max-height: calc(100vh - 6.5rem);
    box-shadow: 0 1px 2px rgba(0, 0, 0, 0.04), 0 10px 28px -14px rgba(0, 0, 0, 0.14);
  }
  .dark .catalog-aside {
    box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.06);
  }
}

/* 小屏展开后的分类列表是一块独立的浮起面板 */
@media (max-width: 1023.98px) {
  .catalog-rail {
    padding: 0.5rem;
    border-radius: 22px;
    background: var(--color-card);
    box-shadow: 0 1px 2px rgba(0, 0, 0, 0.04), 0 10px 28px -14px rgba(0, 0, 0, 0.14);
  }
  .dark .catalog-rail {
    box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.06);
  }
}

/* 已有自绘的清除按钮 */
.catalog-search::-webkit-search-cancel-button {
  display: none;
}
</style>
