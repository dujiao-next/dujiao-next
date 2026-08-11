<template>
  <div class="home-page min-h-screen bg-background text-foreground">

    <!-- ==================== LIST MODE ==================== -->
    <template v-if="templateMode === 'list'">
      <!-- Hero Banner (shared with card mode) -->
      <section v-if="showHeroSection" class="relative z-10 border-b pt-16">
          <div class="relative overflow-hidden bg-card"
            @touchstart="onBannerTouchStart"
            @touchend="onBannerTouchEnd">
            <Transition name="banner-fade" mode="out-in">
              <img v-if="!bannerLoading && heroImage" :src="heroImage" :key="heroImage" loading="eager" fetchpriority="high" decoding="async" class="absolute inset-0 h-full w-full object-cover" @load="handleHeroImageLoad" @error="handleHeroImageError" />
            </Transition>
            <div class="absolute inset-0 bg-black/50"></div>
            <div class="relative container mx-auto px-4">
            <div v-if="heroVisualLoading" class="relative flex min-h-[200px] flex-col justify-between p-5 sm:min-h-[240px] sm:p-6 md:min-h-[320px] md:p-10">
              <div class="space-y-3">
                <div class="h-5 w-24 theme-skeleton rounded-full" style="background: rgba(255,255,255,0.35)"></div>
                <div class="h-8 max-w-3xl theme-skeleton rounded-xl md:h-10" style="background: rgba(255,255,255,0.35)"></div>
                <div class="h-4 max-w-2xl theme-skeleton rounded-lg" style="background: rgba(255,255,255,0.3)"></div>
              </div>
            </div>
            <div v-else class="relative flex min-h-[200px] flex-col justify-between p-5 sm:min-h-[240px] sm:p-6 md:min-h-[320px] md:p-10">
              <div v-if="bannerCount > 1" class="mb-3 flex items-center justify-end gap-2">
                <button type="button"
                  class="inline-flex h-8 w-8 items-center justify-center rounded-full border border-white/30 bg-black/20 text-white transition hover:bg-black/35 md:h-9 md:w-9"
                  @click="handlePrevHeroBanner" :aria-label="t('common.previousBanner')">
                  <ChevronLeft class="h-3.5 w-3.5" />
                </button>
                <button type="button"
                  class="inline-flex h-8 w-8 items-center justify-center rounded-full border border-white/30 bg-black/20 text-white transition hover:bg-black/35 md:h-9 md:w-9"
                  @click="handleNextHeroBanner" :aria-label="t('common.nextBanner')">
                  <ChevronRight class="h-3.5 w-3.5" />
                </button>
              </div>
              <div class="space-y-2 sm:space-y-3">
                <span :class="heroBadgeClass">
                  <span class="h-2 w-2 rounded-full bg-emerald-300"></span>
                  {{ heroBadge }}
                </span>
                <h1 class="max-w-4xl text-xl font-semibold tracking-[-0.02em] text-white sm:text-2xl md:text-3xl">
                  {{ heroTitle }}
                </h1>
                <p class="max-w-3xl text-xs leading-relaxed text-gray-100 sm:text-sm">
                  {{ heroSubtitle }}
                </p>
              </div>
              <div v-if="bannerCount > 1" class="mt-4 flex items-center gap-2">
                <button v-for="(_, bIdx) in banners" :key="`list-dot-${bIdx}`" type="button"
                  class="h-2 rounded-full transition-all"
                  :class="bIdx === currentBannerIndex ? 'w-6 bg-white' : 'w-2 bg-white/45 hover:bg-white/70'"
                  @click="selectHeroBanner(bIdx)"></button>
              </div>
            </div>
            </div>
          </div>
      </section>

      <!-- Main: Left Categories + Right Product List -->
      <section class="relative z-10 pb-6" :class="showHeroSection ? 'pt-6' : 'pt-24'">
        <div class="container mx-auto px-4">
          <div class="flex flex-col lg:flex-row gap-6">

            <CategorySidebar
              :categories="listCategoryGroups"
              :selected-category="listSelectedCategory"
              :expanded-parent-ids="listExpandedParentIds"
              :show-drawer="listShowFilterDrawer"
              compact
              @select-category="listSelectCategory"
              @toggle-parent="listToggleParentCategory"
              @update:show-drawer="listShowFilterDrawer = $event"
            />

            <!-- Right: Product List -->
            <main class="flex-1 min-w-0">
              <!-- Search Bar -->
              <div class="relative mb-4">
                <div class="absolute inset-y-0 left-3.5 flex items-center pointer-events-none">
                  <Search class="w-4 h-4 text-muted-foreground" />
                </div>
                <input
                  v-model="listSearchQuery"
                  type="text"
                  class="w-full h-10 pl-10 pr-10 rounded-xl border bg-card text-sm focus:outline-none focus:ring-2 focus:ring-primary/30 text-foreground placeholder:text-muted-foreground transition-shadow"
                  :placeholder="t('products.searchBoxPlaceholder')"
                  @keydown.enter="listOnSearch"
                />
                <button
                  v-if="listSearchQuery"
                  type="button"
                  class="absolute inset-y-0 right-3 flex items-center text-muted-foreground hover:text-foreground transition-colors"
                  @click="listClearSearch"
                >
                  <X class="w-4 h-4" />
                </button>
              </div>

              <!-- Loading Skeleton -->
              <div v-if="listLoading" class="space-y-6">
                <div v-for="i in 3" :key="i">
                  <div class="flex items-center gap-2 mb-3 px-0.5">
                    <div class="h-5 w-5 rounded theme-skeleton"></div>
                    <div class="h-4 w-28 rounded theme-skeleton"></div>
                  </div>
                  <div class="space-y-2">
                    <div v-for="j in 3" :key="j"
                      class="bg-card rounded-xl border flex items-center h-[72px]">
                      <div class="w-14 h-14 m-2 rounded-lg theme-skeleton flex-shrink-0"></div>
                      <div class="flex-1 px-3 py-2 space-y-2">
                        <div class="h-3.5 w-1/3 rounded theme-skeleton"></div>
                        <div class="h-3 w-1/4 rounded theme-skeleton"></div>
                      </div>
                      <div class="px-4 py-2">
                        <div class="h-4 w-14 rounded theme-skeleton"></div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <!-- Grouped Product List -->
              <div v-else-if="listProductGroups.length > 0" class="space-y-6">
                <div v-for="group in listProductGroups" :key="group.categoryId ?? 'uncategorized'">
                  <!-- Category Header -->
                  <div class="flex items-center gap-2 mb-3 px-0.5">
                    <span class="w-1 h-5 rounded-full bg-primary flex-shrink-0"></span>
                    <img v-if="group.categoryIcon" :src="getImageUrl(group.categoryIcon)"
                      :alt="group.categoryName" loading="lazy" class="h-5 w-5 rounded object-cover flex-shrink-0" />
                    <span class="text-sm font-semibold text-foreground truncate">{{ group.categoryName }}</span>
                    <span class="text-xs text-muted-foreground">({{ group.products.length }})</span>
                  </div>
                  <!-- Products in this category -->
                  <div class="space-y-2">
                    <ProductListItem
                      v-for="(product, idx) in group.products"
                      :key="product.id"
                      :product="product"
                      :index="idx"
                      :animation-step="20"
                      @click="goToProduct"
                      @quick-buy="openQuickBuy"
                    />
                  </div>
                </div>

                <PaginationNav
                  :current-page="listCurrentPage"
                  :total-pages="listTotalPages"
                  :loading="listLoading"
                  compact
                  @change-page="listChangePage"
                />
              </div>

              <!-- Empty State -->
              <EmptyState v-else variant="soft" icon="package" :title="t('products.empty')" />
            </main>
          </div>
        </div>
      </section>
    </template>

    <!-- ==================== CARD MODE (default) ==================== -->
    <template v-else>
    <section v-if="showHeroSection" class="relative z-10 border-b pt-16">
        <div class="relative overflow-hidden bg-card"
          @touchstart="onBannerTouchStart"
          @touchend="onBannerTouchEnd">
          <!-- Banner image with fade transition -->
          <Transition name="banner-fade" mode="out-in">
            <img v-if="!bannerLoading && heroImage" :src="heroImage" :key="heroImage" loading="eager" fetchpriority="high" decoding="async" class="absolute inset-0 h-full w-full object-cover" @load="handleHeroImageLoad" @error="handleHeroImageError" />
          </Transition>
          <div class="absolute inset-0 bg-black/50"></div>

          <div class="relative container mx-auto px-4">
            <div v-if="heroVisualLoading" class="relative flex min-h-[260px] flex-col justify-between p-5 sm:min-h-[320px] sm:p-6 md:min-h-[420px] md:p-12">
            <div class="mb-4 flex items-center justify-end">
              <span :class="heroBadgeClass">
                {{ t('common.loading') }}
              </span>
            </div>

            <div class="space-y-4">
              <div class="h-6 w-28 theme-skeleton rounded-full" style="background: rgba(255,255,255,0.35)"></div>
              <div class="h-10 max-w-4xl theme-skeleton rounded-xl md:h-14" style="background: rgba(255,255,255,0.35)"></div>
              <div class="h-5 max-w-3xl theme-skeleton rounded-lg" style="background: rgba(255,255,255,0.3)"></div>
            </div>

            <div class="flex flex-wrap items-center gap-3 pt-6">
              <div class="h-11 w-36 theme-skeleton rounded-lg" style="background: rgba(255,255,255,0.35)"></div>
              <div class="h-11 w-28 theme-skeleton rounded-lg" style="background: rgba(255,255,255,0.25)"></div>
            </div>
          </div>

          <div v-else class="relative flex min-h-[260px] items-center p-5 sm:min-h-[320px] sm:p-6 md:min-h-[400px] md:p-12">
            <div v-if="bannerCount > 1" class="absolute right-5 top-5 flex items-center gap-2 sm:right-6 sm:top-6 md:right-12 md:top-10">
              <button
                type="button"
                class="inline-flex h-9 w-9 items-center justify-center rounded-full border border-white/30 bg-black/20 text-white transition hover:bg-black/35 md:h-10 md:w-10"
                @click="handlePrevHeroBanner"
                :aria-label="t('common.previousBanner')"
              >
                <ChevronLeft class="h-4 w-4" />
              </button>
              <button
                type="button"
                class="inline-flex h-9 w-9 items-center justify-center rounded-full border border-white/30 bg-black/20 text-white transition hover:bg-black/35 md:h-10 md:w-10"
                @click="handleNextHeroBanner"
                :aria-label="t('common.nextBanner')"
              >
                <ChevronRight class="h-4 w-4" />
              </button>
            </div>

            <div class="max-w-4xl">
              <div class="space-y-3 sm:space-y-4">
                <span :class="heroBadgeClass">
                  <span class="h-2 w-2 rounded-full bg-emerald-300"></span>
                  {{ heroBadge }}
                </span>
                <h1 class="text-2xl font-semibold tracking-[-0.02em] text-white sm:text-3xl md:text-[2.85rem]">
                  {{ heroTitle }}
                </h1>
                <p class="max-w-3xl text-xs leading-relaxed text-gray-100 sm:text-sm md:text-base">
                  {{ heroSubtitle }}
                </p>
              </div>

              <div class="mt-7 flex flex-wrap items-center gap-3 sm:mt-8">
                <button
                  type="button"
                  @click="goToHeroLink"
                  class="inline-flex min-h-[40px] items-center gap-2 rounded-lg bg-white px-4 py-2.5 text-sm font-semibold text-gray-900 transition hover:-translate-y-0.5 hover:shadow-lg sm:min-h-[44px] sm:px-5 sm:py-3"
                >
                  {{ heroPrimaryButtonText }}
                  <ArrowRight class="h-4 w-4" />
                </button>
                <router-link
                  v-if="!hasHeroLink"
                  to="/products"
                  class="inline-flex min-h-[40px] items-center rounded-lg border border-white/30 px-4 py-2.5 text-sm font-medium text-white transition hover:border-white hover:bg-white/10 sm:min-h-[44px] sm:px-5 sm:py-3"
                >
                  {{ t('home.featured.viewAll') }}
                </router-link>
              </div>

              <div v-if="bannerCount > 1" class="mt-6 flex items-center gap-2">
                <button
                  v-for="(_, index) in banners"
                  :key="`hero-dot-${index}`"
                  type="button"
                  class="h-2.5 rounded-full transition-all"
                  :class="index === currentBannerIndex ? 'w-7 bg-white' : 'w-2.5 bg-white/45 hover:bg-white/70'"
                  @click="selectHeroBanner(index)"
                  :aria-label="t('common.switchBanner', { n: index + 1 })"
                ></button>
              </div>
            </div>
          </div>
          </div>
        </div>
    </section>

    <section id="featured" class="relative z-10 pb-6" :class="showHeroSection ? 'pt-8' : 'pt-28 md:pt-32'">
      <div class="container mx-auto px-4">
        <div class="mb-5 flex items-end justify-between gap-4">
          <div>
            <h2 class="theme-section-heading text-3xl md:text-4xl">{{ t('home.featured.title') }}</h2>
            <p class="mt-2 text-sm text-muted-foreground">{{ t('home.featured.description') }}</p>
          </div>
          <router-link
                v-if="!hasHeroLink"
                to="/products"
            class="text-sm font-semibold text-muted-foreground transition-colors hover:text-foreground"
          >
            {{ t('home.featured.viewAll') }}
          </router-link>
        </div>

        <div v-if="featuredLoading" class="grid grid-cols-2 gap-3 md:grid-cols-3 md:gap-4 lg:grid-cols-4 xl:grid-cols-5" aria-hidden="true">
          <div v-for="i in 5" :key="`featured-skeleton-${i}`" class="overflow-hidden rounded-xl border bg-card">
            <div class="aspect-[16/9] theme-skeleton"></div>
            <div class="space-y-3 p-4">
              <div class="h-4 w-4/5 rounded theme-skeleton"></div>
              <div class="h-4 w-2/5 rounded theme-skeleton"></div>
            </div>
          </div>
        </div>
        <div v-else-if="products.length > 0" class="grid grid-cols-2 gap-3 md:gap-4 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
          <ProductCard
            v-for="(product, idx) in products"
            :key="product.id"
            :product="product"
            :index="idx"
            :animation-step="60"
            @click="goToProduct"
            @quick-buy="openQuickBuy"
          />
        </div>
        <div v-else class="rounded-2xl border border-dashed py-16 text-center text-muted-foreground theme-slide-up">
          <PackageOpen class="mx-auto h-16 w-16 mb-4 text-muted-foreground opacity-50" :stroke-width="1.5" />
          {{ t('home.featured.empty') }}
        </div>
      </div>
    </section>

    <template v-if="latestSectionVisible">
    <hr class="theme-section-divider mx-4 md:mx-auto md:max-w-6xl" />

    <section id="latest" class="relative z-10 pb-8 pt-6">
      <div class="container mx-auto px-4">
        <div class="mb-4">
          <h2 class="theme-section-heading text-[1.7rem]">{{ t('home.latest.title') }}</h2>
          <p class="mt-1 text-sm text-muted-foreground">{{ t('home.latest.description') }}</p>
        </div>

        <div v-if="latestLoading" class="grid max-w-[1120px] grid-cols-1 items-start gap-4 md:grid-cols-3" aria-hidden="true">
          <div v-for="i in 3" :key="`latest-skeleton-${i}`" class="overflow-hidden rounded-2xl border bg-card">
            <div class="aspect-[16/7] theme-skeleton"></div>
            <div class="p-4">
              <div class="h-3 w-32 rounded theme-skeleton"></div>
              <div class="mt-2 h-5 w-4/5 rounded theme-skeleton"></div>
              <div class="mt-3 h-4 w-full rounded theme-skeleton"></div>
              <div class="mt-2 h-4 w-full rounded theme-skeleton"></div>
              <div class="mt-2 h-4 w-2/3 rounded theme-skeleton"></div>
            </div>
          </div>
        </div>
        <div v-else-if="posts.length > 0" class="grid max-w-[1120px] grid-cols-1 items-start gap-4 md:grid-cols-3">
          <article
            v-for="post in posts"
            :key="post.id"
            class="group cursor-pointer overflow-hidden rounded-2xl border bg-card shadow-sm transition hover:-translate-y-0.5 hover:border-primary/25 hover:shadow-md"
            @click="goToPost(post.slug)"
          >
            <img v-if="post.thumbnail" :src="getImageUrl(post.thumbnail)" :alt="getLocalizedText(post.title)" loading="lazy" decoding="async" class="aspect-[16/7] w-full object-cover transition-transform duration-500 group-hover:scale-[1.02]" />
            <div class="p-4">
              <div class="flex items-center gap-1.5 text-xs text-muted-foreground">
                <span class="font-semibold text-foreground/70">{{ post.type === 'notice' ? t('nav.notice') : t('nav.blog') }}</span>
                <span aria-hidden="true">·</span>
                <time :datetime="post.published_at">{{ formatDate(post.published_at) }}</time>
              </div>
              <h3 class="mt-1.5 line-clamp-2 text-base font-semibold">{{ getLocalizedText(post.title) }}</h3>
              <p class="mt-2 line-clamp-3 text-sm leading-6 text-muted-foreground">
                {{ getPostPreview(post) }}
                <span class="inline-flex items-center gap-1 font-semibold text-primary">
                  {{ t('blog.readMore') }}
                  <ArrowRight class="h-3.5 w-3.5" />
                </span>
              </p>
            </div>
          </article>
        </div>
        <div v-else class="rounded-2xl border border-dashed py-12 text-center text-muted-foreground">
          {{ t('blog.empty') }}
        </div>
      </div>
    </section>
    </template>
    </template>

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
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ArrowRight, ChevronLeft, ChevronRight, PackageOpen, Search, X } from 'lucide-vue-next'
import { postAPI, productAPI } from '../api'
import { getImageUrl } from '../utils/image'
import { useLocalized } from '../composables/useProduct'
import { useBannerCarousel } from '../composables/useBannerCarousel'
import { useProductList } from '../composables/useProductList'
import { useProductListGroups } from '../composables/useProductListGroups'
import { usePageSeo } from '../composables/usePageSeo'
import { useAppStore } from '../stores/app'
import ProductCard from '../components/ProductCard.vue'
import ProductListItem from '../components/ProductListItem.vue'
import ProductQuickBuy from '../components/ProductQuickBuy.vue'
import CategorySidebar from '../components/CategorySidebar.vue'
import PaginationNav from '../components/PaginationNav.vue'
import EmptyState from '../components/EmptyState.vue'
import AnnouncementModal from '../components/AnnouncementModal.vue'
import { useAnnouncement, type HomeAnnouncement } from '../composables/useAnnouncement'

const router = useRouter()
const { t } = useI18n()
const { getLocalizedText } = useLocalized()
const appStore = useAppStore()

// 英雄横幅上的玻璃态徽章(白字深底,跟随图片而非主题)
const heroBadgeClass = 'inline-flex items-center gap-2 rounded-md border border-white/25 bg-black/55 px-2.5 py-0.5 text-xs font-semibold uppercase tracking-wider text-white backdrop-blur-sm'

const templateMode = computed(() => appStore.config?.template_mode || 'card')
const navBuiltin = computed(() => (appStore.config?.nav_config as { builtin?: Record<string, boolean> } | undefined)?.builtin)
const blogEnabled = computed(() => navBuiltin.value?.blog !== false)
const noticeEnabled = computed(() => navBuiltin.value?.notice !== false)
const latestSectionVisible = computed(() => blogEnabled.value || noticeEnabled.value)

// ==================== Shared State ====================
const products = ref<any[]>([])
const posts = ref<any[]>([])
const featuredLoading = ref(true)
const latestLoading = ref(true)
const quickBuyProduct = ref<any>(null)
const quickBuyVisible = ref(false)

const { shouldShow } = useAnnouncement()
const activeAnnouncement = ref<HomeAnnouncement | null>(null)
const announcementVisible = ref(false)

const openQuickBuy = (product: any) => {
  quickBuyProduct.value = product
  quickBuyVisible.value = true
}

// ==================== Banner Carousel ====================
const {
  banners,
  bannerLoading,
  heroVisualLoading,
  currentBannerIndex,
  bannerCount,
  showHeroSection,
  heroImage,
  handleHeroImageLoad,
  handleHeroImageError,
  heroBadge,
  heroTitle,
  heroSubtitle,
  hasHeroLink,
  heroPrimaryButtonText,
  loadBanners,
  handleNextHeroBanner,
  handlePrevHeroBanner,
  selectHeroBanner,
  goToHeroLink,
  onBannerTouchStart,
  onBannerTouchEnd,
  stopHeroAutoPlay,
} = useBannerCarousel()

// ==================== List Mode ====================
const {
  loading: listLoading,
  products: listProducts,
  selectedCategory: listSelectedCategory,
  searchQuery: listSearchQuery,
  currentPage: listCurrentPage,
  totalPages: listTotalPages,
  showFilterDrawer: listShowFilterDrawer,
  expandedParentIds: listExpandedParentIds,
  categoryGroups: listCategoryGroups,
  categoryMap: listCategoryMap,
  selectCategory: listSelectCategory,
  toggleParentCategory: listToggleParentCategory,
  changePage: listChangePage,
  clearSearch: listClearSearch,
  onSearch: listOnSearch,
  initialize: listInitialize,
  cleanup: listCleanup,
} = useProductList({ pageSize: 20, homeRouteName: 'home' })

const listProductGroups = useProductListGroups(listProducts, listCategoryMap)

// ==================== SEO ====================
const route = useRoute()
const seoCategoryName = computed(() => {
  if (!listSelectedCategory.value) return ''
  const cat = listCategoryMap.value.get(listSelectedCategory.value)
  return cat ? getLocalizedText(cat.name) : ''
})
usePageSeo({
  canonicalPath: () => route.path,
  title: () => {
    if (route.name === 'category-products') {
      return seoCategoryName.value || t('nav.products')
    }
    if (route.name === 'products') return t('nav.products')
    return undefined
  },
})

// ==================== Card Mode ====================
const formatDate = (dateString: string) => {
  if (!dateString) return ''
  return new Date(dateString).toLocaleDateString()
}

const getPostPreview = (post: any) => {
  const summary = getLocalizedText(post?.summary).replace(/\s+/g, ' ').trim()
  if (!summary) return ''
  const isChinese = String(appStore.locale || '').toLowerCase().startsWith('zh')
  const maxLength = isChinese ? 56 : 108
  if (summary.length <= maxLength) return `${summary}…`
  let preview = summary.slice(0, maxLength).trim()
  if (!isChinese) preview = preview.replace(/\s+\S*$/, '')
  return `${preview}…`
}

const goToProduct = (slug: string) => {
  router.push(`/products/${slug}`)
}

const goToPost = (slug: string) => {
  router.push(`/blog/${slug}`)
}

const loadFeaturedProducts = async () => {
  try {
    const response = await productAPI.list({ page: 1, page_size: 15 })
    products.value = response.data.data || []
  } catch (error) {
    console.error('Failed to load products:', error)
  } finally {
    featuredLoading.value = false
  }
}

const loadLatestPosts = async () => {
  if (!latestSectionVisible.value) {
    latestLoading.value = false
    return
  }
  try {
    const params: Record<string, unknown> = { page: 1, page_size: 3 }
    if (blogEnabled.value && !noticeEnabled.value) params.type = 'blog'
    if (!blogEnabled.value && noticeEnabled.value) params.type = 'notice'
    const response = await postAPI.list(params)
    posts.value = response.data.data || []
  } catch (error) {
    console.error('Failed to load posts:', error)
  } finally {
    latestLoading.value = false
  }
}

// ==================== Lifecycle ====================
const showAnnouncementIfNeeded = () => {
  const announcement = appStore.config?.announcement as HomeAnnouncement | undefined
  if (announcement && shouldShow(announcement)) {
    activeAnnouncement.value = announcement
    announcementVisible.value = true
  }
}

onMounted(async () => {
  await appStore.loadConfig()
  if (templateMode.value === 'list') {
    await Promise.all([loadBanners(), listInitialize()])
  } else {
    await Promise.all([loadBanners(), loadFeaturedProducts(), loadLatestPosts()])
  }
  showAnnouncementIfNeeded()
})

onUnmounted(() => {
  stopHeroAutoPlay()
  listCleanup()
})
</script>

<style scoped>
.banner-fade-enter-active,
.banner-fade-leave-active {
  transition: opacity 300ms ease;
}
.banner-fade-enter-from,
.banner-fade-leave-to {
  opacity: 0;
}
</style>
