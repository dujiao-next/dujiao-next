<template>
  <section v-if="showHeroSection" class="mx-auto w-full max-w-[1180px] px-4 pt-6 sm:px-6">
    <div
      class="relative overflow-hidden rounded-[var(--r-xl)] border border-border bg-card shadow-[var(--shadow)]"
      @touchstart="onBannerTouchStart"
      @touchend="onBannerTouchEnd"
    >
      <Transition name="ember-banner" mode="out-in">
        <img
          v-if="!bannerLoading && heroImage"
          :key="heroImage"
          :src="heroImage"
          :alt="heroTitle"
          class="absolute inset-0 h-full w-full object-cover"
        />
      </Transition>

      <!-- 遮罩：左上深、右下浅，保证文字可读 -->
      <div class="absolute inset-0 bg-gradient-to-r from-black/70 via-black/45 to-black/20"></div>

      <!-- 骨架 -->
      <div
        v-if="bannerLoading"
        class="relative flex min-h-[190px] flex-col justify-end gap-3 p-5 sm:min-h-[240px] sm:p-7 md:min-h-[300px] md:p-9"
      >
        <div class="h-6 w-24 animate-pulse rounded-full bg-white/30"></div>
        <div class="h-8 w-3/4 max-w-[520px] animate-pulse rounded-[var(--r-sm)] bg-white/30"></div>
        <div class="h-4 w-1/2 max-w-[360px] animate-pulse rounded bg-white/25"></div>
      </div>

      <div
        v-else
        class="relative flex min-h-[190px] flex-col justify-between gap-4 p-5 sm:min-h-[240px] sm:p-7 md:min-h-[300px] md:p-9"
      >
        <!-- 左右切换 -->
        <div v-if="bannerCount > 1" class="flex items-center justify-end gap-2">
          <button
            type="button"
            class="grid h-9 w-9 place-items-center rounded-full border border-white/30 bg-black/25 text-white transition-colors hover:bg-black/45"
            :aria-label="t('common.previousBanner')"
            @click="handlePrevHeroBanner"
          ><ChevronLeft class="h-4 w-4" /></button>
          <button
            type="button"
            class="grid h-9 w-9 place-items-center rounded-full border border-white/30 bg-black/25 text-white transition-colors hover:bg-black/45"
            :aria-label="t('common.nextBanner')"
            @click="handleNextHeroBanner"
          ><ChevronRight class="h-4 w-4" /></button>
        </div>

        <div class="mt-auto flex max-w-[640px] flex-col gap-2.5">
          <span class="inline-flex w-fit items-center gap-1.5 rounded-full border border-white/25 bg-black/45 px-3 py-1 text-[12px] font-medium text-white backdrop-blur-sm">
            <Zap class="h-3.5 w-3.5" /> {{ heroBadge }}
          </span>
          <h2 class="text-[clamp(1.25rem,1.1rem+0.8vw,2rem)] font-semibold leading-[1.2] tracking-[-0.025em] text-white">
            {{ heroTitle }}
          </h2>
          <p class="line-clamp-2 max-w-[52ch] text-[13.5px] leading-[1.6] text-white/85 sm:text-[15px]">
            {{ heroSubtitle }}
          </p>
          <div class="flex flex-wrap items-center gap-3 pt-1.5">
            <button
              type="button"
              class="inline-flex min-h-[42px] items-center gap-2 rounded-full bg-white px-5 py-2.5 text-[14px] font-medium text-[color:var(--ink)] transition-transform hover:scale-[1.03]"
              @click="goToHeroLink"
            >
              {{ heroPrimaryButtonText }}
              <ArrowRight class="h-4 w-4" />
            </button>
          </div>
        </div>

        <!-- 指示点 -->
        <div v-if="bannerCount > 1" class="flex items-center gap-2">
          <button
            v-for="(_, idx) in banners"
            :key="`ember-banner-dot-${idx}`"
            type="button"
            class="h-1.5 rounded-full transition-all"
            :class="idx === currentBannerIndex ? 'w-6 bg-white' : 'w-1.5 bg-white/45 hover:bg-white/70'"
            :aria-label="t('common.switchBanner', { n: idx + 1 })"
            @click="selectHeroBanner(idx)"
          ></button>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowRight, ChevronLeft, ChevronRight, Zap } from 'lucide-vue-next'
import { useBannerCarousel } from '../../../composables/useBannerCarousel'

const { t } = useI18n()

const {
  banners,
  bannerLoading,
  currentBannerIndex,
  bannerCount,
  showHeroSection,
  heroImage,
  heroBadge,
  heroTitle,
  heroSubtitle,
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

onMounted(() => { void loadBanners() })
onUnmounted(() => stopHeroAutoPlay())
</script>

<style scoped>
.ember-banner-enter-active,
.ember-banner-leave-active {
  transition: opacity 320ms cubic-bezier(0.16, 1, 0.3, 1);
}
.ember-banner-enter-from,
.ember-banner-leave-to {
  opacity: 0;
}
</style>
