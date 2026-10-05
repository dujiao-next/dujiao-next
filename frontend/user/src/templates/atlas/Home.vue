<template>
  <div>
    <!-- ==================== Hero ==================== -->
    <!-- 参考站的 Hero 不是大图轮播，而是「品牌大字 + 一行副标题 + 三个信任徽章」，
         左对齐、贴在浅灰底上。这种克制的做法是它观感偏「工具站」的主因。 -->
    <section class="atlas-hero">
      <div class="atlas-shell py-12 sm:py-16">
        <h1 class="atlas-hero__title">{{ brandName }}</h1>
        <p class="atlas-hero__sub">{{ t('atlas.home.heroTagline') }}</p>

        <ul class="atlas-trust">
          <li v-for="item in trustItems" :key="item.key" class="atlas-trust__item">
            <component :is="item.icon" class="atlas-trust__icon" />
            <div>
              <div class="atlas-trust__title">{{ t(item.titleKey) }}</div>
              <div class="atlas-trust__sub">{{ t(item.subKey) }}</div>
            </div>
          </li>
        </ul>
      </div>
    </section>

    <div class="atlas-shell">
      <!-- ==================== 分类导航 ==================== -->
      <section v-if="rootCategories.length" class="atlas-block">
        <AtlasSectionHead
          :title="t('atlas.home.categories')"
          :subtitle="t('atlas.home.categoriesSub')"
          more-to="/products"
        />
        <div class="atlas-cats">
          <RouterLink
            v-for="cat in rootCategories"
            :key="cat.id"
            :to="`/categories/${cat.slug}`"
            class="atlas-cats__item"
          >
            <span class="atlas-cats__thumb">
              <img v-if="catIcon(cat)" :src="catIcon(cat)" :alt="getLocalizedText(cat.name)" loading="lazy" />
              <Package v-else class="h-6 w-6 text-[color:var(--ink-4)]" />
            </span>
            <span class="atlas-cats__name">{{ getLocalizedText(cat.name) }}</span>
          </RouterLink>
        </div>
      </section>

      <!-- ==================== 热门商品 ==================== -->
      <section class="atlas-block">
        <AtlasSectionHead
          :title="t('atlas.home.hot')"
          :subtitle="t('atlas.home.hotSub')"
          more-to="/products"
        />

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
          size="md"
          icon="package"
          :title="t('products.empty')"
        />
      </section>

      <!-- ==================== 全部商品入口 ==================== -->
      <div class="atlas-block flex justify-center">
        <RouterLink to="/products" class="atlas-btn">{{ t('atlas.home.viewAll') }}</RouterLink>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Package, ShieldCheck, Timer, Globe } from 'lucide-vue-next'
import { useAppStore } from '../../stores/app'
import { useLocalized } from '../../composables/useProduct'
import { usePageSeo } from '../../composables/usePageSeo'
import { productAPI, categoryAPI } from '../../api/product'
import { getImageUrl } from '../../utils/image'
import type { PublicCategory } from '../../utils/category'
import AtlasSectionHead from './components/AtlasSectionHead.vue'
import AtlasProductCard from './components/AtlasProductCard.vue'
import EmptyState from '../../components/EmptyState.vue'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const { getLocalizedText } = useLocalized()

const brandName = computed(() => String(appStore.config?.brand?.site_name || '').trim() || 'Atlas Store')

usePageSeo({ canonicalPath: () => '/', title: () => brandName.value })

const trustItems = [
  { key: 'fast', icon: Timer, titleKey: 'atlas.home.trust.fast', subKey: 'atlas.home.trust.fastSub' },
  { key: 'global', icon: Globe, titleKey: 'atlas.home.trust.global', subKey: 'atlas.home.trust.globalSub' },
  { key: 'secure', icon: ShieldCheck, titleKey: 'atlas.home.trust.secure', subKey: 'atlas.home.trust.secureSub' },
]

const loading = ref(true)
const products = ref<any[]>([])
const categories = ref<PublicCategory[]>([])

const rootCategories = computed(() =>
  categories.value.filter((c) => !c.parent_id || c.parent_id === 0),
)

const catIcon = (cat: PublicCategory) => (cat.icon ? getImageUrl(cat.icon) : '')

const goToProduct = (slug: string) => router.push(`/products/${slug}`)

onMounted(async () => {
  // 两个请求互不依赖，并行发；任一失败只影响对应区块
  const [productsRes, categoriesRes] = await Promise.allSettled([
    productAPI.list({ page: 1, page_size: 8 }),
    categoryAPI.list(),
  ])

  if (productsRes.status === 'fulfilled') {
    products.value = productsRes.value?.data?.data || []
  }
  if (categoriesRes.status === 'fulfilled') {
    categories.value = categoriesRes.value?.data?.data || []
  }
  loading.value = false
})
</script>

<style scoped>
.atlas-hero {
  background: var(--surface);
  border-bottom: 1px solid var(--border);
}

.atlas-hero__title {
  font-size: clamp(28px, 5vw, 44px);
  font-weight: 700;
  letter-spacing: -0.02em;
}

.atlas-hero__sub {
  margin-top: 8px;
  color: var(--ink-3);
  font-size: 15px;
}

.atlas-trust {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  margin-top: 28px;
}

@media (max-width: 640px) {
  .atlas-trust {
    grid-template-columns: 1fr;
  }
}

.atlas-trust__item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 14px;
  border-radius: var(--r-md);
  background: var(--bg);
}

.atlas-trust__icon {
  width: 20px;
  height: 20px;
  flex: none;
  color: var(--accent);
}

.atlas-trust__title {
  font-size: 13px;
  font-weight: 600;
}

.atlas-trust__sub {
  color: var(--ink-4);
  font-size: 11px;
}

/* 区块间距 */
.atlas-block {
  margin-top: var(--gap-block);
}

/* 商品网格：移动端 2 列，与参考站一致 */
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

@media (min-width: 1024px) {
  .atlas-grid {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }
}

/* 分类导航：参考站是一排带方形图标的入口 */
.atlas-cats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}

@media (min-width: 640px) {
  .atlas-cats {
    grid-template-columns: repeat(5, minmax(0, 1fr));
  }
}

@media (min-width: 1024px) {
  .atlas-cats {
    grid-template-columns: repeat(10, minmax(0, 1fr));
  }
}

.atlas-cats__item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 12px 6px;
  border-radius: var(--r-md);
  background: var(--surface);
  transition: box-shadow var(--dur) var(--ease);
}

.atlas-cats__item:hover {
  box-shadow: var(--shadow-md);
}

.atlas-cats__thumb {
  display: grid;
  place-items: center;
  width: 40px;
  height: 40px;
  border-radius: var(--r-sm);
  background: var(--surface-2);
  overflow: hidden;
}

.atlas-cats__thumb img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.atlas-cats__name {
  font-size: 12px;
  text-align: center;
  line-height: 1.3;
}
</style>
