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
        <!-- 左侧：推荐（红底表头 + 分类行） -->
        <aside class="atlas-sidebar">
          <div class="atlas-sidebar__head">
            <span class="atlas-sidebar__name">{{ t('atlas.sidebar.recommend') }}</span>
            <ChevronRight class="atlas-sidebar__chev h-4 w-4" />
          </div>
          <RouterLink
            v-for="cat in rootCategories"
            :key="cat.id"
            :to="`/categories/${cat.slug}`"
            class="atlas-sidebar__row"
          >
            <img v-if="catIcon(cat)" class="atlas-sidebar__icon" :src="catIcon(cat)" :alt="getLocalizedText(cat.name)" loading="lazy" />
            <span v-else class="atlas-sidebar__icon atlas-thumb"></span>
            <span class="atlas-sidebar__name">{{ getLocalizedText(cat.name) }}</span>
            <ChevronRight class="atlas-sidebar__chev h-4 w-4" />
          </RouterLink>
        </aside>

        <!-- 右侧：推荐位 700 + 335，下排 3 × 335 -->
        <div class="atlas-main-col">
          <div v-if="promoSlots.length" class="atlas-reco-top">
            <RouterLink
              v-for="(slot, i) in promoSlots.slice(0, 1)"
              :key="`t-${i}`"
              class="atlas-reco-item"
              :to="slot.to"
            >
              <div class="atlas-reco-item__fallback">
                <span class="atlas-reco-item__kicker">{{ t('atlas.reco.kicker') }}</span>
                <span class="atlas-reco-item__title">{{ slot.title }}</span>
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
        </div>
      </div>
    </div>

    <!-- ==================== 商品分区（module） ==================== -->
    <div class="atlas-module mt-10">
      <div class="atlas-module__content">
        <div class="atlas-module__descrow">
          <div>
            <h2 class="atlas-module__title">{{ t('atlas.home.hot') }}</h2>
            <p class="atlas-module__desc">{{ t('atlas.home.hotSub') }}</p>
          </div>
          <RouterLink to="/products" class="atlas-module__more">
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
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ChevronRight, Clock4, Headset, ShieldCheck, Truck } from 'lucide-vue-next'
import { useAppStore } from '../../stores/app'
import { useLocalized } from '../../composables/useProduct'
import { usePageSeo } from '../../composables/usePageSeo'
import { productAPI, categoryAPI } from '../../api/product'
import { getImageUrl } from '../../utils/image'
import type { PublicCategory } from '../../utils/category'
import AtlasProductCard from './components/AtlasProductCard.vue'
import EmptyState from '../../components/EmptyState.vue'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const { getLocalizedText } = useLocalized()

const brandName = computed(() => String(appStore.config?.brand?.site_name || '').trim() || 'Atlas Store')
usePageSeo({ canonicalPath: () => '/', title: () => brandName.value })

/* 信任徽章：参考站是 4 项、40px 图标、左图标右两行文字 */
const trustItems = [
  { key: 'fast', icon: Truck, titleKey: 'atlas.home.trust.fast', subKey: 'atlas.home.trust.fastSub' },
  { key: 'global', icon: ShieldCheck, titleKey: 'atlas.home.trust.global', subKey: 'atlas.home.trust.globalSub' },
  { key: 'secure', icon: Clock4, titleKey: 'atlas.home.trust.secure', subKey: 'atlas.home.trust.secureSub' },
  { key: 'service', icon: Headset, titleKey: 'atlas.home.trust.service', subKey: 'atlas.home.trust.serviceSub' },
]

const loading = ref(true)
const products = ref<any[]>([])
const categories = ref<PublicCategory[]>([])

const rootCategories = computed(() =>
  categories.value.filter((c) => !c.parent_id || c.parent_id === 0),
)

const catIcon = (cat: PublicCategory) => (cat.icon ? getImageUrl(cat.icon) : '')

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

const goToProduct = (slug: string) => router.push(`/products/${slug}`)

onMounted(async () => {
  const [productsRes, categoriesRes] = await Promise.allSettled([
    productAPI.list({ page: 1, page_size: 8 }),
    categoryAPI.list(),
  ])
  if (productsRes.status === 'fulfilled') products.value = productsRes.value?.data?.data || []
  if (categoriesRes.status === 'fulfilled') categories.value = categoriesRes.value?.data?.data || []
  loading.value = false
})
</script>

<style scoped>
/* 参考站该行内容为 1340 版心；窄屏时允许换行 */
@media (max-width: 1180px) {
  .atlas-trustbar__row {
    flex-wrap: wrap;
    gap: 12px 28px;
    justify-content: flex-start;
  }
}
</style>
