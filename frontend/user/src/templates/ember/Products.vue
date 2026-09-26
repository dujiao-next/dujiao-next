<template>
  <div class="mx-auto w-full max-w-[1180px] px-4 pb-10 sm:px-6">
    <!-- 面包屑 -->
    <nav class="flex flex-wrap items-center gap-1.5 py-5 text-[13px] text-muted-foreground">
      <RouterLink class="transition-colors hover:text-foreground" to="/">{{ t('nav.home') }}</RouterLink>
      <ChevronRight class="h-3.5 w-3.5 flex-none opacity-60" />
      <span class="text-foreground">{{ currentCategoryName || t('nav.products') }}</span>
    </nav>

    <header class="mb-8 flex flex-col gap-2">
      <h1 class="text-[clamp(1.75rem,1.3rem+1.9vw,2.6rem)] font-semibold tracking-[-0.022em]">
        {{ currentCategoryName || t('ember.featured.title') }}
      </h1>
      <p class="text-[15px] text-muted-foreground">{{ t('ember.featured.subtitle') }}</p>
    </header>

    <div class="grid items-start gap-6 lg:grid-cols-[236px_minmax(0,1fr)]">
      <!-- ============ 分类 ============ -->
      <aside v-if="categoryGroups.length" class="min-w-0 lg:sticky lg:top-[92px]">
        <div class="rounded-[var(--r-lg)] border border-border bg-card p-3 sm:p-4">
          <div class="mb-3 hidden items-center gap-2 px-1 lg:flex">
            <span class="h-4 w-[3px] flex-none rounded-full bg-[color:var(--accent)]"></span>
            <h4 class="text-[13px] font-semibold">{{ t('products.categories') }}</h4>
          </div>

          <div class="flex gap-1.5 overflow-x-auto pb-0.5 lg:grid lg:gap-1 lg:overflow-visible lg:pb-0">
            <!-- 全部 -->
            <button
              type="button"
              class="flex-none whitespace-nowrap rounded-full px-3.5 py-2 text-[13.5px] transition-colors lg:w-full lg:rounded-[var(--r-sm)] lg:px-3 lg:py-2.5 lg:text-left"
              :class="selectedCategory === null
                ? 'bg-foreground font-medium text-background lg:bg-[color:var(--accent-soft)] lg:text-[color:var(--accent-strong)]'
                : 'bg-[color:var(--surface-2)] text-muted-foreground hover:text-foreground lg:bg-transparent lg:hover:bg-[color:var(--surface-2)]'"
              @click="selectCategory(null)"
            >{{ t('products.allCategories') }}</button>

            <template v-for="grp in categoryGroups" :key="grp.id">
              <div class="flex flex-none items-center gap-0.5 lg:w-full lg:min-w-0">
                <button
                  type="button"
                  class="flex min-w-0 max-w-[65vw] flex-none items-center gap-2.5 rounded-full px-3.5 py-2 text-[13.5px] transition-colors lg:w-full lg:max-w-none lg:flex-1 lg:rounded-[var(--r-sm)] lg:px-3 lg:py-2.5 lg:text-left"
                  :class="selectedCategory === grp.id
                    ? 'bg-foreground font-medium text-background lg:bg-[color:var(--accent-soft)] lg:text-[color:var(--accent-strong)]'
                    : 'bg-[color:var(--surface-2)] text-muted-foreground hover:text-foreground lg:bg-transparent lg:hover:bg-[color:var(--surface-2)]'"
                  @click="selectCategory(grp.id)"
                >
                  <img
                    v-if="grp.icon"
                    :src="getImageUrl(grp.icon)"
                    :alt="catName(grp)"
                    loading="lazy"
                    class="h-4 w-4 flex-none rounded object-cover"
                  />
                  <span class="min-w-0 truncate">{{ catName(grp) }}</span>
                </button>

                <button
                  v-if="grp.children.length"
                  type="button"
                  class="hidden h-9 w-9 flex-none place-items-center rounded-[var(--r-sm)] transition-colors hover:bg-[color:var(--surface-2)] lg:grid"
                  :class="expandedParentIds.includes(grp.id) ? 'text-[color:var(--accent)]' : 'text-muted-foreground'"
                  :aria-expanded="expandedParentIds.includes(grp.id)"
                  :aria-label="catName(grp)"
                  @click="toggleParentCategory(grp.id)"
                >
                  <ChevronDown
                    class="h-4 w-4 transition-transform"
                    :class="expandedParentIds.includes(grp.id) ? 'rotate-180' : ''"
                  />
                </button>
              </div>

              <!-- 子分类 -->
              <div v-if="expandedParentIds.includes(grp.id)" class="hidden lg:grid lg:gap-0.5 lg:pl-3">
                <button
                  v-for="child in grp.children"
                  :key="child.id"
                  type="button"
                  class="w-full truncate rounded-[var(--r-sm)] px-3 py-2 text-left text-[13px] transition-colors"
                  :class="selectedCategory === child.id
                    ? 'bg-[color:var(--accent-soft)] font-medium text-[color:var(--accent-strong)]'
                    : 'text-muted-foreground hover:bg-[color:var(--surface-2)] hover:text-foreground'"
                  @click="selectCategory(child.id)"
                >{{ catName(child) }}</button>
              </div>
            </template>
          </div>
        </div>
      </aside>

      <!-- ============ 商品 ============ -->
      <main class="min-w-0">
        <!-- 搜索 -->
        <div class="relative mb-5 max-w-[340px]">
          <Search class="pointer-events-none absolute left-4 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <input
            v-model="searchQuery"
            type="search"
            class="h-11 w-full rounded-full border border-border bg-card pl-10 pr-4 text-[14px] text-foreground outline-none transition-colors placeholder:text-muted-foreground focus:border-[color:var(--accent)] focus:ring-[var(--ring)]"
            :placeholder="t('products.searchBoxPlaceholder')"
            :aria-label="t('products.searchLabel')"
          />
        </div>

        <!-- 骨架 -->
        <div v-if="loading" class="grid gap-4 grid-cols-[repeat(auto-fill,minmax(238px,1fr))] max-[640px]:grid-cols-2 max-[640px]:gap-3">
          <div v-for="i in 9" :key="i" class="h-[280px] rounded-[var(--r-lg)] border border-border bg-card"></div>
        </div>

        <!-- 网格 -->
        <div v-else-if="products.length" class="grid gap-4 grid-cols-[repeat(auto-fill,minmax(238px,1fr))] max-[640px]:grid-cols-2 max-[640px]:gap-3">
          <EmberProductCard
            v-for="product in products"
            :key="product.id"
            :product="product"
            quick-buy-enabled
            @quick-buy="openQuickBuy"
          />
        </div>

        <!-- 空态 -->
        <div v-else class="flex flex-col items-center gap-3 rounded-[var(--r-lg)] border border-dashed border-border py-16 text-center">
          <component :is="(searchQuery || selectedCategory) ? SearchX : PackageOpen" class="h-11 w-11 text-muted-foreground" />
          <p class="font-medium">
            {{ (searchQuery || selectedCategory) ? t('products.emptyFiltered') : t('ember.featured.empty') }}
          </p>
          <p v-if="!searchQuery && !selectedCategory" class="max-w-[42ch] text-[13.5px] text-muted-foreground">
            {{ t('ember.featured.emptyHint') }}
          </p>
          <Button
            v-if="searchQuery || selectedCategory"
            variant="outline"
            size="sm"
            class="mt-2 rounded-full border-[color:var(--border-strong)]"
            @click="resetFilters"
          >{{ t('products.clearFilters') }}</Button>
        </div>

        <!-- 分页 -->
        <nav v-if="totalPages > 1" class="mt-10 flex flex-wrap items-center justify-center gap-2">
          <button
            class="grid h-10 w-10 place-items-center rounded-full border border-border bg-card text-muted-foreground transition-colors hover:border-[color:var(--border-strong)] hover:text-foreground disabled:cursor-not-allowed disabled:opacity-40"
            :disabled="currentPage <= 1"
            :aria-label="t('pagination.previous')"
            @click="changePage(currentPage - 1)"
          ><ChevronLeft class="h-4 w-4" /></button>

          <button
            v-for="p in pageWindow"
            :key="p"
            type="button"
            class="grid h-10 min-w-[40px] place-items-center rounded-full border px-3 text-[13.5px] tabular-nums transition-colors"
            :class="p === currentPage
              ? 'border-foreground bg-foreground text-background'
              : 'border-border bg-card text-muted-foreground hover:border-[color:var(--border-strong)] hover:text-foreground'"
            @click="changePage(p)"
          >{{ p }}</button>

          <button
            class="grid h-10 w-10 place-items-center rounded-full border border-border bg-card text-muted-foreground transition-colors hover:border-[color:var(--border-strong)] hover:text-foreground disabled:cursor-not-allowed disabled:opacity-40"
            :disabled="currentPage >= totalPages"
            :aria-label="t('pagination.next')"
            @click="changePage(currentPage + 1)"
          ><ChevronRight class="h-4 w-4" /></button>
        </nav>
      </main>
    </div>

    <!-- 快速购买 -->
    <ProductQuickBuy
      v-if="quickBuyProduct"
      :product="quickBuyProduct"
      :visible="quickBuyVisible"
      @update:visible="quickBuyVisible = $event"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import {
  ChevronDown, ChevronLeft, ChevronRight, PackageOpen, Search, SearchX,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { getImageUrl } from '../../utils/image'
import { useLocalized } from '../../composables/useProduct'
import { useProductList } from '../../composables/useProductList'
import { usePageSeo } from '../../composables/usePageSeo'
import EmberProductCard from './components/EmberProductCard.vue'
import ProductQuickBuy from '../../components/ProductQuickBuy.vue'

const route = useRoute()
const { t } = useI18n()
const { getLocalizedText } = useLocalized()

const {
  loading,
  products,
  categoryGroups,
  categoryMap,
  selectedCategory,
  expandedParentIds,
  searchQuery,
  currentPage,
  totalPages,
  selectCategory,
  toggleParentCategory,
  changePage,
  clearSearch,
  initialize,
  cleanup,
} = useProductList({ pageSize: 12, homeRouteName: 'products' })

const catName = (cat: any) => getLocalizedText(cat?.name) || cat?.slug || ''

/** 面包屑与标题显示当前分类名 */
const currentCategoryName = computed(() => {
  if (!selectedCategory.value) return ''
  const matched = categoryMap.value?.get(selectedCategory.value)
  return matched ? catName(matched) : ''
})

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

// ==================== 快速购买 ====================
const quickBuyProduct = ref<any>(null)
const quickBuyVisible = ref(false)
const openQuickBuy = (product: any) => {
  quickBuyProduct.value = product
  quickBuyVisible.value = true
}

usePageSeo({
  canonicalPath: () => route.path,
  title: () => t('nav.products'),
})

onMounted(() => {
  void initialize()
})

onUnmounted(() => {
  cleanup()
})
</script>
