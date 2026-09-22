<template>
  <div class="products-page min-h-screen bg-background pb-16 pt-20 text-foreground">
    <div class="container mx-auto px-4">
      <section class="products-announcement mt-6 rounded-2xl border bg-card p-5 shadow-sm md:mt-8 md:p-6">
        <div class="mb-4 flex items-center gap-2 text-sm font-semibold text-foreground">
          <span class="grid h-8 w-8 place-items-center rounded-full bg-secondary text-primary"><Megaphone class="h-4 w-4" /></span>
          <span>{{ announcementTitle }}</span>
        </div>
        <div class="announcement-content text-sm leading-7 text-muted-foreground" v-html="announcementContent"></div>
      </section>

      <section class="category-card-grid mt-5 flex gap-2.5 overflow-x-auto pb-2 md:mt-6 md:flex-wrap md:gap-3">
        <button type="button" class="category-pill" :class="{ 'category-pill-active': selectedCategory === null }" :aria-pressed="selectedCategory === null" @click="selectCategory(null)">
          <span class="category-icon"><FolderOpen class="h-4 w-4" /></span>
          <span>{{ t('products.allCategories') }}</span>
          <span class="category-active-indicator" aria-hidden="true"></span>
        </button>
        <template v-for="group in categoryGroups" :key="group.id">
          <button type="button" class="category-pill" :class="{ 'category-pill-active': selectedCategory === group.id }" :aria-pressed="selectedCategory === group.id" @click="selectCategory(group.id)">
            <img v-if="group.icon" :src="getImageUrl(group.icon)" :alt="getLocalizedText(group.name)" class="category-icon object-cover" />
            <span v-else class="category-icon"><FolderOpen class="h-4 w-4" /></span>
            <span>{{ getLocalizedText(group.name) }}</span>
            <span class="category-active-indicator" aria-hidden="true"></span>
          </button>
          <button v-for="child in group.children" :key="child.id" type="button" class="category-pill" :class="{ 'category-pill-active': selectedCategory === child.id }" :aria-pressed="selectedCategory === child.id" @click="selectCategory(child.id)">
            <img v-if="child.icon" :src="getImageUrl(child.icon)" :alt="getLocalizedText(child.name)" class="category-icon object-cover" />
            <span v-else class="category-icon"><FolderOpen class="h-4 w-4" /></span>
            <span>{{ getLocalizedText(child.name) }}</span>
            <span class="category-active-indicator" aria-hidden="true"></span>
          </button>
        </template>
      </section>

      <div class="mb-5 mt-8 flex items-center gap-2 text-lg font-bold"><Package class="h-5 w-5 text-primary" /><span>{{ selectedCategoryTitle }}</span></div>
      <main>
        <div v-if="loading && !hasLoadedOnce" class="grid grid-cols-2 gap-3 md:grid-cols-3 md:gap-4 lg:grid-cols-4">
          <div v-for="i in 6" :key="i" class="overflow-hidden rounded-2xl border bg-card">
            <div class="h-36 theme-skeleton md:h-56"></div><div class="space-y-3 p-3 md:p-5"><div class="h-5 w-3/4 rounded theme-skeleton"></div><div class="h-3 w-full rounded theme-skeleton"></div></div>
          </div>
        </div>
        <div v-else-if="loading" class="category-switch-loading">
          <span class="h-5 w-5 animate-spin rounded-full border-2 border-primary/25 border-t-primary"></span>
          <span>{{ t('common.loading') }}</span>
        </div>
        <div v-else-if="products.length">
          <div class="grid grid-cols-2 gap-3 md:grid-cols-3 md:gap-4 lg:grid-cols-4">
            <ProductCard v-for="(product, idx) in products" :key="product.id" :product="product" :index="idx" :max-tags="isMobileGrid ? 1 : 2" :animation-step="50" @click="goToProduct" />
          </div>
          <PaginationNav :current-page="currentPage" :total-pages="totalPages" :loading="loading" @change-page="changePage" />
        </div>
        <EmptyState v-else variant="soft" size="lg" icon="package" :title="t('products.empty')" />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { FolderOpen, Megaphone, Package } from 'lucide-vue-next'
import { useAppStore } from '../stores/app'
import { useProductList } from '../composables/useProductList'
import { usePageSeo } from '../composables/usePageSeo'
import { useLocalized } from '../composables/useProduct'
import { getImageUrl } from '../utils/image'
import { sanitizeRichHtml } from '../utils/richContent'
import ProductCard from '../components/ProductCard.vue'
import PaginationNav from '../components/PaginationNav.vue'
import EmptyState from '../components/EmptyState.vue'

const router = useRouter()
const route = useRoute()
const { t } = useI18n()
const appStore = useAppStore()
const { getLocalizedText } = useLocalized()
const { loading, hasLoadedOnce, products, selectedCategory, currentPage, totalPages, categoryGroups, categoryMap, selectCategory, changePage, initialize, cleanup } = useProductList({ pageSize: 12, homeRouteName: 'products' })
const announcementCacheKey = 'storefront:last-announcement'
const readCachedAnnouncement = () => {
  try {
    const value = JSON.parse(localStorage.getItem(announcementCacheKey) || 'null')
    return value && typeof value === 'object' ? value : null
  } catch {
    return null
  }
}
const writeCachedAnnouncement = (announcement: any) => {
  try {
    localStorage.setItem(announcementCacheKey, JSON.stringify(announcement))
  } catch {
    // Storage may be unavailable in private/restricted browser contexts.
  }
}
const cachedAnnouncement = ref<any>(readCachedAnnouncement())
const currentAnnouncement = computed(() => appStore.config?.announcement || cachedAnnouncement.value)
watch(() => appStore.config?.announcement, (announcement) => {
  if (!announcement) return
  cachedAnnouncement.value = announcement
  writeCachedAnnouncement(announcement)
}, { immediate: true })
const announcementTitle = computed(() => getLocalizedText(currentAnnouncement.value?.title) || '站点公告')
const announcementContent = computed(() => sanitizeRichHtml(getLocalizedText(currentAnnouncement.value?.content) || '<p>欢迎访问雪糕数卡，请在购买前仔细阅读商品说明。</p>'))
const selectedCategoryTitle = computed(() => selectedCategory.value ? getLocalizedText(categoryMap.value.get(selectedCategory.value)?.name) : t('products.allCategories'))
usePageSeo({ canonicalPath: () => route.path, title: () => selectedCategoryTitle.value })
const goToProduct = (slug: string) => router.push(`/products/${slug}`)
const isMobileGrid = ref(window.innerWidth < 768)
const handleResize = () => { isMobileGrid.value = window.innerWidth < 768 }
onMounted(async () => { window.addEventListener('resize', handleResize, { passive: true }); await initialize() })
onUnmounted(() => { window.removeEventListener('resize', handleResize); cleanup() })
</script>

<style scoped>
.category-card-grid { scrollbar-width: none; }
.category-card-grid::-webkit-scrollbar { display: none; }
.category-pill { position:relative; display:flex; flex:none; align-items:center; gap:.55rem; min-height:2.75rem; padding:.35rem .85rem .35rem .4rem; border:1px solid hsl(var(--border)); border-radius:1rem; background:hsl(var(--card)); color:hsl(var(--muted-foreground)); font-size:.875rem; font-weight:600; transition:border-color .18s ease, background-color .18s ease, box-shadow .18s ease, color .18s ease, transform .18s ease; }
.category-pill:hover { border-color:hsl(var(--primary)/.45); color:hsl(var(--foreground)); transform:translateY(-1px); }
.category-pill-active { border-color:hsl(var(--primary)); background:hsl(var(--primary)/.12); color:hsl(var(--primary)); box-shadow:0 0 0 2px hsl(var(--primary)/.16); }
.category-icon { display:grid; width:2rem; height:2rem; flex:none; place-items:center; border-radius:.65rem; background:var(--ui-bg-soft); color:var(--ui-accent); }
.category-pill-active .category-icon { background:var(--ui-accent); color:var(--ui-text-on-accent); }
.category-active-indicator { position:absolute; right:.75rem; bottom:-1px; left:.75rem; height:3px; border-radius:999px 999px 0 0; background:hsl(var(--primary)); opacity:0; transform:scaleX(.45); transform-origin:center; }
.category-pill-active .category-active-indicator { opacity:1; transform:scaleX(1); }
.category-switch-loading { display:flex; min-height:7rem; align-items:center; justify-content:center; gap:.65rem; color:var(--ui-text-muted); font-size:.875rem; }
.announcement-content :deep(a) { color:hsl(var(--primary)); text-decoration:underline; text-underline-offset:3px; }
</style>
