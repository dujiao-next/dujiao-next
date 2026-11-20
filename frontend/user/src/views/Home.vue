<template>
  <div class="home-page min-h-screen bg-background text-foreground">
    <!-- Hero：banner，卡片模式下右侧并排最新动态 -->
    <section v-if="showHeroSection" class="relative z-10 pt-24">
      <div class="container mx-auto grid gap-4 px-4 lg:grid-cols-12">
        <div v-if="bannerLoading" class="theme-skeleton" :class="[bannerSizeClass, showNewsPanel ? 'lg:col-span-8' : 'lg:col-span-12']"></div>
        <div
          v-else
          class="relative isolate flex flex-col justify-end overflow-hidden bg-neutral-900 p-5 md:p-9"
          :class="[bannerSizeClass, showNewsPanel ? 'lg:col-span-8' : 'lg:col-span-12']"
          @touchstart="onBannerTouchStart"
          @touchend="onBannerTouchEnd"
        >
          <Transition name="banner-fade" mode="out-in">
            <img v-if="heroImage" :src="heroImage" :key="heroImage" alt="" class="absolute inset-0 -z-10 h-full w-full object-cover" />
          </Transition>
          <!-- 只压暗文字所在的左下角，图片其余部分保持原色 -->
          <div class="absolute inset-0 -z-10 bg-gradient-to-tr from-black/65 via-black/20 to-transparent"></div>

          <h1 class="hero-title line-clamp-2 max-w-[16em] text-2xl font-bold text-white md:text-[40px]">{{ heroTitle }}</h1>
          <p class="mt-2.5 hidden max-w-xl truncate text-[15px] text-white/80 md:block">{{ heroSubtitle }}</p>
          <button
            type="button"
            class="mt-3.5 h-9 w-fit rounded-full bg-neutral-50 px-4 text-sm font-semibold text-neutral-900 transition-colors hover:bg-neutral-200 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-white md:mt-5 md:h-10 md:px-5"
            @click="goToHeroLink"
          >
            {{ heroPrimaryButtonText }}
          </button>

          <!-- 当前项是一条随自动轮播走完的进度条 -->
          <div v-if="bannerCount > 1" class="absolute bottom-2 right-5 flex items-center gap-1.5 md:bottom-5 md:right-7">
            <button
              v-for="(_, index) in banners"
              :key="`hero-dot-${index}`"
              type="button"
              class="group/dot py-2"
              :aria-label="t('common.switchBanner', { n: index + 1 })"
              :aria-current="index === currentBannerIndex"
              @click="selectHeroBanner(index)"
            >
              <span
                class="block h-1.5 overflow-hidden rounded-full transition-all"
                :class="index === currentBannerIndex ? 'w-8 bg-neutral-50/35' : 'w-1.5 bg-neutral-50/50 group-hover/dot:bg-neutral-50/80'"
              >
                <span v-if="index === currentBannerIndex" :key="currentBannerIndex" class="hero-progress block h-full rounded-full bg-neutral-50"></span>
              </span>
            </button>
          </div>
        </div>

        <aside v-if="showNewsPanel" class="theme-float hidden flex-col rounded-[28px] bg-card p-6 pb-4 lg:col-span-4 lg:flex">
          <h2 class="flex items-baseline justify-between gap-3 text-[17px] font-bold">
            {{ t('home.latest.title') }}
            <span class="flex gap-1.5">
              <router-link v-if="blogEnabled" to="/blog" class="theme-pill-link">{{ t('nav.blog') }}<ChevronRight class="h-3.5 w-3.5" aria-hidden="true" /></router-link>
              <router-link v-if="noticeEnabled" to="/notice" class="theme-pill-link">{{ t('nav.notice') }}<ChevronRight class="h-3.5 w-3.5" aria-hidden="true" /></router-link>
            </span>
          </h2>
          <ol class="mt-2 flex flex-1 flex-col divide-y">
            <li v-for="post in posts" :key="post.id" class="flex flex-1 flex-col justify-center py-2.5">
              <time class="text-xs tabular-nums text-muted-foreground">{{ formatDate(post.published_at) }}</time>
              <router-link :to="`/blog/${post.slug}`" class="truncate text-[15px] font-semibold transition-colors hover:text-primary">
                {{ getLocalizedText(post.title) }}
              </router-link>
            </li>
          </ol>
        </aside>
      </div>
    </section>

    <ProductCatalog
      class="pb-14"
      :class="showHeroSection ? 'pt-5 lg:pt-6' : 'pt-24'"
      :layout="isListMode ? 'list' : 'grid'"
      @category-name="seoCategoryName = $event"
    />


    <!-- 大屏且有 banner 时最新动态已并入 hero 右侧，这里只给小屏 / 无 banner 时兜底 -->
    <div v-if="latestSectionVisible" :class="showNewsPanel && 'lg:hidden'">
    <hr class="theme-section-divider mx-4 md:mx-auto md:max-w-6xl" />

    <section class="relative z-10 py-12">
      <div class="container mx-auto px-4">
        <div class="mb-6 flex items-end justify-between gap-4">
          <div>
            <h2 class="theme-section-heading text-[1.7rem]">{{ t('home.latest.title') }}</h2>
            <p class="mt-1 text-sm text-muted-foreground">{{ t('home.latest.description') }}</p>
          </div>
          <div class="flex items-center gap-1.5">
            <router-link v-if="blogEnabled" to="/blog" class="theme-pill-link">{{ t('nav.blog') }}<ChevronRight class="h-3.5 w-3.5" aria-hidden="true" /></router-link>
            <router-link v-if="noticeEnabled" to="/notice" class="theme-pill-link">{{ t('nav.notice') }}<ChevronRight class="h-3.5 w-3.5" aria-hidden="true" /></router-link>
          </div>
        </div>

        <div v-if="posts.length > 0" class="grid grid-cols-1 gap-5 md:grid-cols-2 xl:grid-cols-4">
          <article
            v-for="post in posts"
            :key="post.id"
            class="theme-float group flex cursor-pointer flex-col rounded-[22px] bg-card p-5"
            @click="goToPost(post.slug)"
          >
            <div class="mb-2 text-xs text-muted-foreground">{{ formatDate(post.published_at) }}</div>
            <h3 class="line-clamp-2 text-base font-semibold">{{ getLocalizedText(post.title) }}</h3>
            <p class="mt-2 line-clamp-2 text-sm text-muted-foreground">{{ getLocalizedText(post.summary) }}</p>
            <div class="mt-auto pt-4"><span class="theme-pill-link group-hover:bg-foreground group-hover:text-background">{{ t('blog.readMore') }}<ChevronRight class="h-3.5 w-3.5" aria-hidden="true" /></span></div>
          </article>
        </div>
        <div v-else class="rounded-2xl border border-dashed py-12 text-center text-muted-foreground">
          {{ t('blog.empty') }}
        </div>
      </div>
    </section>
    </div>

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
import { ChevronRight } from 'lucide-vue-next'
import { postAPI } from '../api'
import { useLocalized } from '../composables/useProduct'
import { useBannerCarousel } from '../composables/useBannerCarousel'
import { usePageSeo } from '../composables/usePageSeo'
import { useAppStore } from '../stores/app'
import ProductCatalog from '../components/ProductCatalog.vue'
import AnnouncementModal from '../components/AnnouncementModal.vue'
import { useAnnouncement, type HomeAnnouncement } from '../composables/useAnnouncement'

const router = useRouter()
const { t } = useI18n()
const { getLocalizedText } = useLocalized()
const appStore = useAppStore()

const templateMode = computed(() => appStore.config?.template_mode || 'card')
const navBuiltin = computed(() => (appStore.config?.nav_config as { builtin?: Record<string, boolean> } | undefined)?.builtin)
const blogEnabled = computed(() => navBuiltin.value?.blog !== false)
const noticeEnabled = computed(() => navBuiltin.value?.notice !== false)
const latestSectionVisible = computed(() => blogEnabled.value || noticeEnabled.value)
const isListMode = computed(() => templateMode.value === 'list')

// ==================== Shared State ====================
const posts = ref<any[]>([])

const { shouldShow } = useAnnouncement()
const activeAnnouncement = ref<HomeAnnouncement | null>(null)
const announcementVisible = ref(false)

// ==================== Banner Carousel ====================
const {
  banners,
  bannerLoading,
  currentBannerIndex,
  bannerCount,
  showHeroSection,
  heroImage,
  heroTitle,
  heroSubtitle,
  heroPrimaryButtonText,
  loadBanners,
  selectHeroBanner,
  goToHeroLink,
  onBannerTouchStart,
  onBannerTouchEnd,
  stopHeroAutoPlay,
} = useBannerCarousel()

const showNewsPanel = computed(() => latestSectionVisible.value && posts.value.length > 0)
const bannerSizeClass = 'h-48 rounded-[22px] md:h-64 md:rounded-[28px] lg:h-80'

// ==================== SEO ====================
const route = useRoute()
const seoCategoryName = ref('')
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

const goToPost = (slug: string) => {
  router.push(`/blog/${slug}`)
}

const loadLatestPosts = async () => {
  if (!latestSectionVisible.value) return
  try {
    const params: Record<string, unknown> = { page: 1, page_size: 4 }
    if (blogEnabled.value && !noticeEnabled.value) params.type = 'blog'
    if (!blogEnabled.value && noticeEnabled.value) params.type = 'notice'
    const response = await postAPI.list(params)
    posts.value = response.data.data || []
  } catch (error) {
    console.error('Failed to load posts:', error)
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
  await Promise.all([loadBanners(), loadLatestPosts()])
  showAnnouncementIfNeeded()
})

onUnmounted(() => {
  stopHeroAutoPlay()
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

.hero-title {
  line-height: 1.1;
  letter-spacing: -0.03em;
  text-wrap: balance;
}

/* 时长需与 useBannerCarousel 的自动轮播间隔(5s)一致 */
.hero-progress {
  transform-origin: left;
  animation: hero-progress 5s linear both;
}
@keyframes hero-progress {
  from { transform: scaleX(0); }
}
@media (prefers-reduced-motion: reduce) {
  .hero-progress { animation: none; }
}
</style>
