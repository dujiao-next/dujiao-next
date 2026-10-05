<template>
  <div>
    <!-- ==================== 信任徽章行：浅灰底 4 项 ==================== -->
    <section class="atlas-trustbar">
      <div class="atlas-shell atlas-trustbar__row">
        <div v-for="item in trustItems" :key="item.key" class="atlas-trustbar__item">
          <component :is="item.icon" class="atlas-trustbar__icon" :stroke-width="1.5" />
          <div>
            <strong class="atlas-trustbar__title">{{ t(item.titleKey) }}</strong>
            <p class="atlas-trustbar__sub">{{ t(item.subKey) }}</p>
          </div>
        </div>
      </div>
    </section>

    <!-- ==================== 轮播 banner：1340x70 圆角 12 ==================== -->
    <div class="atlas-banner relative">
      <RouterLink to="/products" class="atlas-banner__slide">
        <span class="atlas-banner__title">{{ t('atlas.banner.title') }}</span>
        <span class="atlas-banner__cta">{{ t('atlas.banner.cta') }}</span>
      </RouterLink>
      <div class="atlas-banner__dots">
        <span
          v-for="n in 3"
          :key="n"
          class="atlas-banner__dot"
          :class="n === 1 && 'atlas-banner__dot--on'"
        ></span>
      </div>
    </div>

    <!-- ==================== 三栏主区：240 侧栏 + 1080 内容 ==================== -->
    <div class="atlas-shell">
      <div class="atlas-home-main">
        <!-- 左侧：推荐（红底表头 + 分类行，选中态跟随地址栏） -->
        <aside class="atlas-sidebar">
          <!-- 表头「推荐」：参考站这里也带图标。点击=清除分类筛选（回到全部/推荐） -->
          <button class="atlas-sidebar__head" type="button" @click="clearCategory">
            <Flame class="atlas-sidebar__headicon" :stroke-width="2" />
            <span class="atlas-sidebar__name">{{ t('atlas.sidebar.recommend') }}</span>
            <ChevronRight class="atlas-sidebar__chev h-4 w-4" />
          </button>

          <!-- 分类行：原地选中（不跳页），筛选结果直接反映到下方商品区 -->
          <button
            v-for="cat in rootCategories"
            :key="cat.id"
            class="atlas-sidebar__row"
            :class="{ 'atlas-sidebar__row--on': isActiveCategory(cat) }"
            type="button"
            :aria-pressed="isActiveCategory(cat)"
            @click="selectCategory(cat)"
          >
            <img v-if="catIcon(cat)" class="atlas-sidebar__icon" :src="catIcon(cat)" :alt="getLocalizedText(cat.name)" loading="lazy" />
            <span v-else class="atlas-sidebar__icon atlas-thumb"></span>
            <span class="atlas-sidebar__name">{{ getLocalizedText(cat.name) }}</span>
            <ChevronRight class="atlas-sidebar__chev h-4 w-4" />
          </button>
        </aside>

        <!-- 右侧 1080px -->
        <div class="atlas-main-col">
          <!-- 未筛选：推荐位 700 + 335，下排 3 × 335 -->
          <template v-if="!isFiltered">
            <div v-if="firstPromo" class="atlas-reco-top">
              <RouterLink class="atlas-reco-item" :to="firstPromo.to">
                <div class="atlas-reco-item__fallback">
                  <span class="atlas-reco-item__kicker">{{ t('atlas.reco.kicker') }}</span>
                  <span class="atlas-reco-item__title">{{ firstPromo.title }}</span>
                  <span class="atlas-reco-item__cta">{{ t('atlas.reco.cta') }}</span>
                </div>
              </RouterLink>

              <div class="flex flex-col gap-5">
                <RouterLink
                  v-for="(slot, i) in promoSlots.slice(1, 3)"
                  :key="`r-${i}`"
                  class="atlas-reco-item"
                  :to="slot.to"
                >
                  <div class="atlas-reco-item__fallback">
                    <span class="atlas-reco-item__kicker">{{ t('atlas.reco.kicker') }}</span>
                    <span class="atlas-reco-item__title" style="font-size: 20px">{{ slot.title }}</span>
                  </div>
                </RouterLink>
              </div>
            </div>

            <div v-if="promoSlots.length > 3" class="atlas-reco-bottom">
              <RouterLink
                v-for="(slot, i) in promoSlots.slice(3, 6)"
                :key="`b-${i}`"
                class="atlas-reco-item"
                :to="slot.to"
              >
                <div class="atlas-reco-item__fallback">
                  <span class="atlas-reco-item__title" style="font-size: 20px">{{ slot.title }}</span>
                </div>
              </RouterLink>
            </div>
          </template>

          <!-- 已筛选：把「当前在看什么」讲清楚，并提供清除入口 -->
          <div v-else class="atlas-filterbar">
            <div>
              <h1 class="atlas-filterbar__title">{{ filterTitle }}</h1>
              <p class="atlas-filterbar__sub">{{ t('atlas.home.resultCount', { count: totalCount }) }}</p>
            </div>
            <RouterLink to="/products" class="atlas-filterbar__clear">
              <X class="h-3.5 w-3.5" />
              {{ t('atlas.home.clearFilter') }}
            </RouterLink>
          </div>
        </div>
      </div>
    </div>

    <!-- ==================== 商品分区（module） ==================== -->
    <div class="atlas-module mt-10">
      <div class="atlas-module__content">
        <div class="atlas-module__descrow">
          <div>
            <h2 class="atlas-module__title">{{ isFiltered ? filterTitle : t('atlas.home.hot') }}</h2>
            <p class="atlas-module__desc">
              {{ isFiltered ? t('atlas.home.resultCount', { count: totalCount }) : t('atlas.home.hotSub') }}
            </p>
          </div>
          <RouterLink v-if="!isFiltered" to="/products" class="atlas-module__more">
            {{ t('atlas.section.more') }}
            <ChevronRight class="h-3 w-3" />
          </RouterLink>
        </div>

        <div v-if="loading" class="atlas-module__list">
          <div v-for="i in 8" :key="i" class="atlas-card overflow-hidden">
            <div class="atlas-skeleton aspect-[4/3] rounded-none"></div>
            <div class="space-y-2 p-3">
              <div class="atlas-skeleton h-3 w-2/3"></div>
              <div class="atlas-skeleton h-4 w-1/3"></div>
            </div>
          </div>
        </div>

        <div v-else-if="products.length" class="atlas-module__list">
          <AtlasProductCard
            v-for="product in products"
            :key="product.id"
            :product="product"
            @click="goToProduct"
          />
        </div>

        <EmptyState v-else variant="soft" size="md" icon="package" :title="t('products.empty')" />

        <!-- 分页：参考站是极简数字条 -->
        <nav v-if="totalPages > 1" class="atlas-pager">
          <button class="atlas-pager__btn" type="button" :disabled="page <= 1" @click="changePage(page - 1)">
            {{ t('products.previous') }}
          </button>
          <span class="atlas-pager__info">{{ page }} / {{ totalPages }}</span>
          <button class="atlas-pager__btn" type="button" :disabled="page >= totalPages" @click="changePage(page + 1)">
            {{ t('products.next') }}
          </button>
        </nav>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ChevronRight, Clock4, Flame, Headset, ShieldCheck, Truck, X } from 'lucide-vue-next'
import { useAppStore } from '../../stores/app'
import { useLocalized } from '../../composables/useProduct'
import { usePageSeo } from '../../composables/usePageSeo'
import { productAPI, categoryAPI } from '../../api/product'
import { getImageUrl } from '../../utils/image'
import type { PublicCategory } from '../../utils/category'
import AtlasProductCard from './components/AtlasProductCard.vue'
import EmptyState from '../../components/EmptyState.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const { getLocalizedText } = useLocalized()

const brandName = computed(() => String(appStore.config?.brand?.site_name || '').trim() || 'Atlas Store')

/* 信任徽章：参考站是 4 项、40px 图标、左图标右两行文字 */
const trustItems = [
  { key: 'fast', icon: Truck, titleKey: 'atlas.home.trust.fast', subKey: 'atlas.home.trust.fastSub' },
  { key: 'global', icon: ShieldCheck, titleKey: 'atlas.home.trust.global', subKey: 'atlas.home.trust.globalSub' },
  { key: 'secure', icon: Clock4, titleKey: 'atlas.home.trust.secure', subKey: 'atlas.home.trust.secureSub' },
  { key: 'service', icon: Headset, titleKey: 'atlas.home.trust.service', subKey: 'atlas.home.trust.serviceSub' },
]

const PAGE_SIZE = 12

const loading = ref(true)
const products = ref<any[]>([])
const categories = ref<PublicCategory[]>([])
const page = ref(1)
const totalPages = ref(1)
const totalCount = ref(0)

const rootCategories = computed(() =>
  categories.value.filter((c) => !c.parent_id || c.parent_id === 0),
)

const catIcon = (cat: PublicCategory) => (cat.icon ? getImageUrl(cat.icon) : '')

/**
 * 当前筛选状态 —— 完全由地址栏驱动。
 * 分类有两个来源，都要认：
 *   1) /categories/:slug 的路径参数（template_mode='list' 时由本组件渲染）
 *   2) /?category=slug 的查询参数（侧栏原地选中的入口，见 selectCategory）
 */
const activeSlug = computed(() =>
  String(route.params.slug || route.query.category || '').trim(),
)
const activeSearch = computed(() => String(route.query.search || '').trim())

const activeCategory = computed(() => {
  if (!activeSlug.value) return null
  return categories.value.find((c) => c.slug === activeSlug.value) || null
})

const isFiltered = computed(() => Boolean(activeSlug.value || activeSearch.value))

const filterTitle = computed(() => {
  if (activeCategory.value) return getLocalizedText(activeCategory.value.name)
  if (activeSearch.value) return t('atlas.home.searchResult', { keyword: activeSearch.value })
  return ''
})

/**
 * SEO 必须放在所有被引用的 computed 之后。
 * usePageSeo 内部走 @unhead/vue 的 useHead，它会在 setup 期间同步求值 title 回调；
 * 若此时闭包引用的 filterTitle 尚未初始化（TDZ），useHead 解析失败、返回的句柄为
 * undefined，随后作用域清理时调用 .dispose() 就会抛
 * 「Cannot read properties of undefined (reading 'dispose')」，整页被错误边界兜住。
 */
usePageSeo({ canonicalPath: () => route.path, title: () => filterTitle.value || brandName.value })

const isActiveCategory = (cat: PublicCategory) =>
  Boolean(activeSlug.value) && cat.slug === activeSlug.value

/**
 * 原地选中分类 —— 只改查询参数，不换路径。
 *
 * 换成路径（/categories/<slug>）会让路由去渲染另一个页面组件（Products.vue），
 * 表现就是「点一下跳到别的页面」；而同一个路径只改 query 时，Vue Router 会
 * 复用当前组件、不卸载不重挂，页面留在原地，筛选结果直接反映到下方商品区。
 * 顺带把状态放进地址栏，刷新/分享都能还原。
 */
const selectCategory = (cat: PublicCategory) => {
  router.replace({ query: { ...route.query, category: cat.slug } })
}

/** 清除分类筛选（回到全部 / 推荐） */
const clearCategory = () => {
  const next = { ...route.query }
  delete next.category
  router.replace({ query: next })
}

/**
 * 推荐位（参考站 .recommend-top / .recommend-bottom）。
 * 参考站放的是运营配的促销图；这里用本站的一级分类填充同样数量的卡位 ——
 * 几何与数量完全一致，内容来自本站数据，不引用参考站图片。
 */
const promoSlots = computed(() =>
  rootCategories.value.slice(0, 6).map((cat) => ({
    title: getLocalizedText(cat.name),
    to: `/categories/${cat.slug}`,
  })),
)

const firstPromo = computed(() => promoSlots.value[0])

const loadProducts = async () => {
  loading.value = true
  try {
    const params: Record<string, unknown> = { page: page.value, page_size: PAGE_SIZE }
    if (activeCategory.value) params.category_id = activeCategory.value.id
    if (activeSearch.value) params.search = activeSearch.value
    const res: any = await productAPI.list(params)
    products.value = res?.data?.data || []
    totalPages.value = res?.data?.pagination?.total_page || 1
    totalCount.value = res?.data?.pagination?.total || products.value.length
  } catch {
    products.value = []
    totalPages.value = 1
    totalCount.value = 0
  } finally {
    loading.value = false
  }
}

const changePage = async (next: number) => {
  if (next < 1 || next > totalPages.value) return
  page.value = next
  window.scrollTo({ top: 0, behavior: 'smooth' })
  await loadProducts()
}

const goToProduct = (slug: string) => router.push(`/products/${slug}`)

/**
 * 路由变化 → 重新过滤。
 * 覆盖三种入口：侧栏分类、顶部分类导航条、顶部搜索框。
 */
watch(
  () => [route.params.slug, route.query.category, route.query.search],
  async () => {
    page.value = 1
    // 分类数据可能还没到（直接打开 /categories/x 时），先等分类加载完再过滤
    if (activeSlug.value && !categories.value.length) await loadCategories()
    await loadProducts()
  },
)

const loadCategories = async () => {
  try {
    const res: any = await categoryAPI.list()
    categories.value = res?.data?.data || []
  } catch {
    categories.value = []
  }
}

onMounted(async () => {
  await loadCategories()
  await loadProducts()
})
</script>

<style scoped>
/* 筛选态的信息条：明确告诉用户「现在在看什么」并给清除入口 */
.atlas-filterbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 20px 24px;
  border-radius: var(--r-sidebar);
  background: var(--surface);
  box-shadow: var(--shadow-card);
}

.atlas-filterbar__title {
  font-family: var(--font-head);
  font-size: 22px;
  font-weight: 700;
  line-height: 1.2;
}

.atlas-filterbar__sub {
  margin-top: 4px;
  color: var(--ink-3);
  font-size: 13px;
}

.atlas-filterbar__clear {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  flex: none;
  height: 32px;
  padding: 0 12px;
  border: 1px solid var(--border-strong);
  border-radius: var(--r-sm);
  color: var(--ink-2);
  font-size: 13px;
  transition: all var(--dur) var(--ease);
}

.atlas-filterbar__clear:hover {
  border-color: var(--accent);
  color: var(--accent);
  background: var(--accent-soft);
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

.atlas-pager__btn:hover:not(:disabled) { border-color: var(--ink); }
.atlas-pager__btn:disabled { opacity: 0.4; cursor: not-allowed; }

.atlas-pager__info { color: var(--ink-3); font-size: 13px; }

@media (max-width: 1180px) {
  .atlas-trustbar__row {
    flex-wrap: wrap;
    gap: 12px 28px;
    justify-content: flex-start;
  }
}
</style>
