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

    <!-- ==================== 轮播 banner：1340x70 圆角 12 ====================
         整块 banner 就是「立即选购」按钮。
         原来是 RouterLink → /products（跳到商品列表页），现在改成页内滚动：
         文案是「全量商品 即刻选购」，对应的就是本页下方的商品区。 -->
    <div class="atlas-banner relative">
      <button type="button" class="atlas-banner__slide" @click="goShopNow">
        <span class="atlas-banner__title">{{ t('atlas.banner.title') }}</span>
        <span class="atlas-banner__cta">{{ t('atlas.banner.cta') }}</span>
      </button>
      <div class="atlas-banner__dots">
        <span
          v-for="n in 3"
          :key="n"
          class="atlas-banner__dot"
          :class="n === 1 && 'atlas-banner__dot--on'"
        ></span>
      </div>
    </div>

    <!-- ==================== 三栏主区：240 侧栏 + 1080 内容 ====================
         这里是顶部分类按钮的落点（ATLAS_ANCHOR.catalog）。
         为什么锚在「三栏主区」而不是侧栏本身：筛选会让推荐位消失、主列换成紧凑
         网格，区块高度变化很大；而三栏主区上方（信任徽章 + banner）高度固定，
         滚到它的顶部落点才是稳的。 -->
    <div :id="ATLAS_ANCHOR.catalog" class="atlas-shell">
      <div class="atlas-home-main">
        <!-- 左侧：推荐（红底表头 + 分类行，选中态跟随地址栏） -->
        <aside class="atlas-sidebar">
          <!-- 表头「推荐」：参考站这里也带图标。点击=清除分类筛选（回到全部/推荐） -->
          <button
            class="atlas-sidebar__head"
            :class="{ 'atlas-sidebar__head--on': !isFiltered }"
            type="button"
            :title="t('atlas.home.clearFilter')"
            @click="clearCategory"
          >
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
              <AtlasPromoCard
                large
                :to="firstPromo.to"
                :title="firstPromo.title"
                :image="firstPromo.image"
                :desc="firstPromo.desc"
                :cta="t('atlas.reco.cta')"
              />

              <div class="flex flex-col gap-5">
                <AtlasPromoCard
                  v-for="(slot, i) in promoSlots.slice(1, 3)"
                  :key="`r-${i}`"
                  :to="slot.to"
                  :title="slot.title"
                  :image="slot.image"
                  :desc="slot.desc"
                />
              </div>
            </div>

            <div v-if="promoSlots.length > 3" class="atlas-reco-bottom">
              <AtlasPromoCard
                v-for="(slot, i) in promoSlots.slice(3, 6)"
                :key="`b-${i}`"
                :to="slot.to"
                :title="slot.title"
                :image="slot.image"
                :desc="slot.desc"
              />
            </div>
          </template>

          <!-- 已筛选：筛选条 + 商品网格就地渲染在主列内。
               为什么必须放主列里：商品列表原本在三栏行之外，而 flex 行的高度由
               最高的子项（侧栏）决定，主列被筛选条压短后就会在它下方留下
               与侧栏等高的空白（实测 438px），列表看起来「跑到页面下方」。 -->
          <template v-else>
            <!-- 筛选态不再渲染标题/计数条：侧栏选中项已经说明「在看哪个分类」，
                 再放一条同样的标题就是重复占位。清除筛选 = 再点一次选中的分类。 -->
            <AtlasGoodsModule
              :id="ATLAS_ANCHOR.products"
              compact
              :products="products"
              :loading="loading"
              :page="page"
              :total-pages="totalPages"
              @select="goToProduct"
              @change-page="changePage"
            />
          </template>
        </div>
      </div>
    </div>

    <!-- ==================== 未筛选时的整宽商品分区（参考站版式，4 列） ====================
         已筛选态不在这里渲染 —— 它收进主列内部（见上方），否则会被侧栏撑开的
         行高推下去。 -->
    <AtlasGoodsModule
      v-if="!isFiltered"
      :id="ATLAS_ANCHOR.products"
      :title="t('atlas.home.hot')"
      :desc="t('atlas.home.hotSub')"
      :products="products"
      :loading="loading"
      :page="page"
      :total-pages="totalPages"
      @select="goToProduct"
      @change-page="changePage"
    >
      <template #more>
        <RouterLink to="/products" class="atlas-module__more">
          {{ t('atlas.section.more') }}
          <ChevronRight class="h-3 w-3" />
        </RouterLink>
      </template>
    </AtlasGoodsModule>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ChevronRight, Clock4, Flame, Headset, ShieldCheck, Truck } from 'lucide-vue-next'
import { useAppStore } from '../../stores/app'
import { useLocalized } from '../../composables/useProduct'
import { usePageSeo } from '../../composables/usePageSeo'
import { productAPI, categoryAPI } from '../../api/product'
import { getImageUrl } from '../../utils/image'
import { scrollToAnchor } from '../../utils/anchorScroll'
import type { PublicCategory } from '../../utils/category'
import { ATLAS_ANCHOR, atlasStickyOffset } from './anchor'
import AtlasGoodsModule from './components/AtlasGoodsModule.vue'
import AtlasPromoCard from './components/AtlasPromoCard.vue'

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

/**
 * 每页固定 3 行。两种状态的列数都是 4（整宽分区 / 主列紧凑态一致），
 * 所以 4 列 × 3 行 = 12 条，超出部分由后端分页到下一页。
 */
const PAGE_SIZE = 12

const loading = ref(true)
const products = ref<any[]>([])
const categories = ref<PublicCategory[]>([])
const page = ref(1)
const totalPages = ref(1)

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
  // 再点一次已选中的分类 = 取消筛选（筛选态没有单独的「清除」按钮）
  if (isActiveCategory(cat)) {
    clearCategory()
    return
  }
  router.replace({ query: { ...route.query, category: cat.slug } })
}

/** 清除分类筛选（回到全部 / 推荐） */
const clearCategory = () => {
  const next = { ...route.query }
  delete next.category
  router.replace({ query: next })
}

/**
 * 推荐位（参考站 .recommend-top / .recommend-bottom 的 6 个卡位：700 + 335×2 + 335×3）。
 * 参考站这里放的是运营配图；本站没有配图来源，按需求「直接使用商品图片作为图标」，
 * 于是取商品来填这 6 个卡位 —— 几何与数量不变，且每张卡都有图。
 *
 * 取法：**每个一级分类各取 1 个商品**，而不是取列表前 6 个。
 * 列表前 6 个常常是同一系列（同一分类的多个规格），那样 6 张推荐位会长得几乎一样。
 */
const promoSlots = ref<Array<{ to: string; title: string; image: string; desc: string }>>([])
const firstPromo = computed(() => promoSlots.value[0])

const toSlot = (p: any) => {
  const tags = Array.isArray(p?.tags) ? p.tags : []
  return {
    to: `/products/${p?.slug}`,
    title: getLocalizedText(p?.title),
    image: p?.images?.[0] ? getImageUrl(p.images[0]) : '',
    desc: tags
      .map((tag: any) => (typeof tag === 'string' ? tag : getLocalizedText(tag?.name ?? tag)))
      .filter((tag: string) => tag)
      .slice(0, 3)
      .join(','),
  }
}

const buildPromoSlots = async () => {
  const cats = rootCategories.value.slice(0, 6)
  const picked: any[] = []

  if (cats.length) {
    const results = await Promise.allSettled(
      cats.map((cat) => productAPI.list({ page: 1, page_size: 1, category_id: cat.id })),
    )
    for (const r of results) {
      if (r.status !== 'fulfilled') continue
      const p = (r.value as any)?.data?.data?.[0]
      if (p) picked.push(p)
    }
  }

  // 某些分类没有商品时，用已加载的列表补齐，保证 6 个卡位不空
  for (const p of products.value) {
    if (picked.length >= 6) break
    if (!picked.some((x) => x.id === p.id)) picked.push(p)
  }

  promoSlots.value = picked.slice(0, 6).map(toSlot)
}

const loadProducts = async () => {
  loading.value = true
  try {
    const params: Record<string, unknown> = { page: page.value, page_size: PAGE_SIZE }
    if (activeCategory.value) params.category_id = activeCategory.value.id
    if (activeSearch.value) params.search = activeSearch.value
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

const changePage = async (next: number) => {
  if (next < 1 || next > totalPages.value) return
  page.value = next
  window.scrollTo({ top: 0, behavior: 'smooth' })
  await loadProducts()
}

const goToProduct = (slug: string) => router.push(`/products/${slug}`)

/**
 * banner「立即选购」→ 页内商品区。
 *
 * 原来是 RouterLink → /products：那会换路由记录，在 template_mode='card' 下
 * 直接渲染成 Products.vue，表现就是「点一下跳到另一个菜单页」。
 * 现在只滚动，不改地址、不改筛选状态 —— banner 文案是「全量商品 即刻选购」，
 * 对应本页下方的商品区，用户的筛选状态不该被一个浏览动作重置。
 */
const goShopNow = () => scrollToAnchor(ATLAS_ANCHOR.products, { offset: atlasStickyOffset })

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
  // 推荐位需要「每个一级分类各取一个」，依赖分类与商品都就绪
  await buildPromoSlots()
})
</script>

<style scoped>
/* 筛选态已不再显示独立的标题/计数条（与侧栏选中项重复），
   所以 .atlas-filterbar* 的样式随之下线；
   商品网格与分页的样式在 AtlasGoodsModule 组件内。 */

@media (max-width: 1180px) {
  .atlas-trustbar__row {
    flex-wrap: wrap;
    gap: 12px 28px;
    justify-content: flex-start;
  }
}
</style>
