<template>
  <div class="mx-auto w-full max-w-[1180px] px-4 pb-10 pt-2 sm:px-6">
    <header class="my-6 flex flex-col items-start gap-2">
      <h1 class="text-[clamp(1.75rem,1.3rem+1.9vw,2.6rem)] font-semibold tracking-[-0.022em]">{{ t('nav.notice') }}</h1>
      <p class="text-[15px] text-muted-foreground">{{ t('notice.subtitle') }}</p>
    </header>

    <!-- 加载 -->
    <div v-if="loading" class="mx-auto grid max-w-[880px] gap-3">
      <div v-for="i in 6" :key="i" class="h-[96px] rounded-[var(--r-lg)] border border-border bg-card"></div>
    </div>

    <!-- 列表 -->
    <template v-else-if="posts.length > 0">
      <div class="mx-auto grid max-w-[880px] gap-3">
        <button
          v-for="notice in posts"
          :key="notice.id"
          type="button"
          class="group flex w-full items-center gap-4 rounded-[var(--r-lg)] border border-border bg-card px-5 py-4 text-left transition-all hover:-translate-y-0.5 hover:border-[color:var(--border-strong)] hover:shadow-[var(--shadow-sm)]"
          @click="goToPost(notice.slug)"
        >
          <div class="relative grid h-14 w-14 flex-none place-items-center overflow-hidden rounded-[var(--r-sm)] bg-[color:var(--surface-2)] max-[640px]:hidden">
            <img
              v-if="notice.thumbnail"
              :src="getImageUrl(notice.thumbnail)"
              :alt="getLocalizedText(notice.title)"
              loading="lazy"
              class="absolute inset-0 h-full w-full object-cover"
            />
            <Bell v-else class="h-5 w-5 text-muted-foreground" />
          </div>

          <div class="min-w-0 flex-1">
            <div class="mb-1.5 flex flex-wrap items-center gap-2.5">
              <span class="ember-pill ember-pill--warning">{{ t('nav.notice') }}</span>
              <span class="text-[12px] text-muted-foreground">{{ formatDate(notice.published_at) }}</span>
            </div>
            <h2 class="truncate text-[15px] font-semibold">{{ getLocalizedText(notice.title) }}</h2>
            <p v-if="notice.summary" class="mt-0.5 truncate text-[13px] text-muted-foreground">{{ getLocalizedText(notice.summary) }}</p>
          </div>

          <ChevronRight class="h-4 w-4 flex-none text-muted-foreground transition-transform group-hover:translate-x-0.5" />
        </button>
      </div>

      <nav v-if="totalPages > 1" class="mt-8 flex items-center justify-center gap-3">
        <button
          class="grid h-10 w-10 place-items-center rounded-full border border-border bg-card text-muted-foreground transition-colors hover:border-[color:var(--border-strong)] disabled:opacity-40"
          :disabled="currentPage <= 1"
          @click="changePage(currentPage - 1)"
        ><ChevronLeft class="h-4 w-4" /></button>
        <span class="text-[13px] tabular-nums text-muted-foreground">{{ currentPage }} / {{ totalPages }}</span>
        <button
          class="grid h-10 w-10 place-items-center rounded-full border border-border bg-card text-muted-foreground transition-colors hover:border-[color:var(--border-strong)] disabled:opacity-40"
          :disabled="currentPage >= totalPages"
          @click="changePage(currentPage + 1)"
        ><ChevronRight class="h-4 w-4" /></button>
      </nav>
    </template>

    <!-- 空态 -->
    <div v-else class="flex flex-col items-center gap-3 rounded-[var(--r-lg)] border border-dashed border-border py-16 text-center">
      <Bell class="h-11 w-11 text-muted-foreground" />
      <p class="text-[14px] text-muted-foreground">{{ t('notice.empty') }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Bell, ChevronLeft, ChevronRight } from 'lucide-vue-next'
import { getImageUrl } from '../../utils/image'
import { usePostList } from '../../composables/usePostList'

const { t } = useI18n()

const {
  loading, posts, currentPage, totalPages,
  getLocalizedText, formatDate, goToPost, changePage,
} = usePostList('notice', { title: () => t('nav.notice'), canonicalPath: '/notice' })
</script>
