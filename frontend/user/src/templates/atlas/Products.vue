<template>
  <div class="atlas-shell py-6">
    <!-- 面包屑：参考站列表页顶部是「首页 / 分类」，不做居中大标题 -->
    <nav class="atlas-crumb">
      <RouterLink to="/">{{ t('nav.home') }}</RouterLink>
      <span>/</span>
      <span v-if="activeCategoryName">{{ activeCategoryName }}</span>
      <span v-else>{{ t('products.allCategories') }}</span>
    </nav>

    <div class="mt-4 flex flex-col gap-6 lg:flex-row">
      <!-- ==================== 左侧分类栏 ==================== -->
      <aside class="atlas-side">
        <h3 class="atlas-side__title">{{ t('products.categories') }}</h3>
        <button
          class="atlas-side__item"
          :class="!selectedCategoryId && 'atlas-side__item--on'"
          type="button"
          @click="selectCategory(null)"
        >{{ t('products.allCategories') }}</button>

        <template v-for="group in categoryGroups" :key="group.id">
          <button
            class="atlas-side__item"
            :class="selectedCategoryId === group.id && 'atlas-side__item--on'"
            type="button"
            @click="selectCategory(group.id)"
          >
            <img v-if="catIcon(group)" class="atlas-side__icon" :src="catIcon(group)" :alt="getLocalizedText(group.name)" loading="lazy" />
            <span>{{ getLocalizedText(group.name) }}</span>
          </button>

          <button
            v-for="child in group.children"
            :key="child.id"
            class="atlas-side__item atlas-side__item--child"
            :class="selectedCategoryId === child.id && 'atlas-side__item--on'"
            type="button"
            @click="selectCategory(child.id)"
          >
            <img v-if="catIcon(child)" class="atlas-side__icon" :src="catIcon(child)" :alt="getLocalizedText(child.name)" loading="lazy" />
            <span>{{ getLocalizedText(child.name) }}</span>
          </button>
        </template>
      </aside>

      <!-- ==================== 商品网格 ==================== -->
      <main class="min-w-0 flex-1">
        <div v-if="activeCategoryName" class="mb-4">
          <h1 class="atlas-section-title">{{ activeCategoryName }}</h1>
        </div>

        <div v-if="loading" class="atlas-grid">
          <div v-for="i in 8" :key="i" class="atlas-card overflow-hidden">
            <div class="atlas-skeleton aspect-[4/3] rounded-none"></div>
            <div class="space-y-2 p-3">
              <div class="atlas-skeleton h-3 w-2/3"></div>
              <div class="atlas-skeleton h-4 w-1/3"></div>
            </div>
          </div>
        </div>

        <div v-else-if="products.length" class="atlas-grid">
          <AtlasProductCard
            v-for="product in products"
            :key="product.id"
            :product="product"
            @click="goToProduct"
          />
        </div>

        <EmptyState
          v-else
          variant="soft"
          size="lg"
          icon="package"
          :title="t('products.empty')"
        />

        <!-- 分页：参考站用极简数字条，不做大圆角胶囊 -->
        <nav v-if="totalPages > 1" class="atlas-pager">
          <button
            class="atlas-pager__btn"
            type="button"
            :disabled="currentPage <= 1"
            @click="changePage(currentPage - 1)"
          >{{ t('products.previous') }}</button>
          <span class="atlas-pager__info">{{ currentPage }} / {{ totalPages }}</span>
          <button
            class="atlas-pager__btn"
            type="button"
            :disabled="currentPage >= totalPages"
            @click="changePage(currentPage + 1)"
          >{{ t('products.next') }}</button>
        </nav>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useLocalized } from '../../composables/useProduct'
import { usePageSeo } from '../../composables/usePageSeo'
import { productAPI, categoryAPI } from '../../api/product'
import { getImageUrl } from '../../utils/image'
import { buildCategoryGroups, type PublicCategory, type CategoryGroup } from '../../utils/category'
import AtlasProductCard from './components/AtlasProductCard.vue'
import EmptyState from '../../components/EmptyState.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const { getLocalizedText } = useLocalized()

const PAGE_SIZE = 12

const loading = ref(true)
const products = ref<any[]>([])
const categories = ref<PublicCategory[]>([])
const selectedCategoryId = ref<number | null>(null)
const currentPage = ref(1)
const totalPages = ref(1)

const categoryGroups = computed<CategoryGroup[]>(() => buildCategoryGroups(categories.value))

/* 分类图标：参考站侧栏每一行左侧都有一个 24x24 图标 */
const catIcon = (cat: PublicCategory) => (cat.icon ? getImageUrl(cat.icon) : '')

const activeCategoryName = computed(() => {
  if (!selectedCategoryId.value) return ''
  const found = categories.value.find((c) => c.id === selectedCategoryId.value)
  return found ? getLocalizedText(found.name) : ''
})

usePageSeo({
  canonicalPath: () => route.path,
  title: () => activeCategoryName.value || t('nav.products'),
})

const loadProducts = async () => {
  loading.value = true
  try {
    const params: Record<string, unknown> = { page: currentPage.value, page_size: PAGE_SIZE }
    if (selectedCategoryId.value) params.category_id = selectedCategoryId.value
    const res: any = await productAPI.list(params)
    products.value = res?.data?.data || []
    totalPages.value = res?.data?.pagination?.total_page || 1
  } catch {
    products.value = []
    totalPages.value = 1
  } finally {
    loading.value = false
  }
}

const selectCategory = async (id: number | null) => {
  selectedCategoryId.value = id
  currentPage.value = 1
  // 分类切换同步到地址栏，保证可分享/可刷新（与 classic 的行为一致）
  const slug = id ? categories.value.find((c) => c.id === id)?.slug : ''
  router.replace(slug ? `/categories/${slug}` : '/products')
  await loadProducts()
}

const changePage = async (page: number) => {
  if (page < 1 || page > totalPages.value) return
  currentPage.value = page
  window.scrollTo({ top: 0, behavior: 'smooth' })
  await loadProducts()
}

const goToProduct = (slug: string) => router.push(`/products/${slug}`)

onMounted(async () => {
  // 先取分类，再按 URL 里的 slug 定位分类，最后加载商品 —— 顺序不能反
  try {
    const res: any = await categoryAPI.list()
    categories.value = res?.data?.data || []
  } catch {
    categories.value = []
  }

  const slug = String(route.params.slug || '').trim()
  if (slug) {
    const matched = categories.value.find((c) => c.slug === slug)
    if (matched) selectedCategoryId.value = matched.id
  }

  await loadProducts()
})
</script>

<style scoped>
.atlas-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

@media (min-width: 640px) {
  .atlas-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 16px;
  }
}

@media (min-width: 1280px) {
  .atlas-grid {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }
}

/* 分类侧栏：白卡 + 极简条目，选中态用红字而不是红底 */
.atlas-side {
  flex: none;
  align-self: flex-start;
  width: 100%;
  padding: 12px;
  border-radius: var(--r-lg);
  background: var(--surface);
}

@media (min-width: 1024px) {
  .atlas-side {
    width: 208px;
    position: sticky;
    top: 80px;
  }
}

.atlas-side__title {
  margin-bottom: 8px;
  padding: 0 8px;
  color: var(--ink-4);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.1em;
  text-transform: uppercase;
}

.atlas-side__item {
  display: flex;
  align-items: center;
  width: 100%;
  padding: 7px 8px;
  border-radius: var(--r-sm);
  color: var(--ink-2);
  font-size: 13px;
  text-align: left;
  transition: background-color var(--dur) var(--ease), color var(--dur) var(--ease);
}

/* 行内分类图标：24x24，位于名称左侧（与首页侧栏、参考站一致） */
.atlas-side__icon {
  width: 24px;
  height: 24px;
  flex: none;
  margin-right: 8px;
  border-radius: var(--r-xs);
  object-fit: contain;
}

.atlas-side__item:hover {
  background: var(--surface-2);
  color: var(--ink);
}

.atlas-side__item--child {
  padding-left: 18px;
  color: var(--ink-3);
  font-size: 12.5px;
}

.atlas-side__item--on {
  background: var(--accent-soft);
  color: var(--accent);
  font-weight: 600;
}

/* 分页 */
.atlas-pager {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  margin-top: 28px;
}

.atlas-pager__btn {
  min-height: 34px;
  padding: 0 14px;
  border: 1px solid var(--border-strong);
  border-radius: var(--r-sm);
  background: var(--surface);
  font-size: 13px;
  transition: border-color var(--dur) var(--ease);
}

.atlas-pager__btn:hover:not(:disabled) {
  border-color: var(--ink);
}

.atlas-pager__btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.atlas-pager__info {
  color: var(--ink-3);
  font-size: 13px;
}
</style>
