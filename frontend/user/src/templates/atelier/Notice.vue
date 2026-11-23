<template>
  <div class="min-h-screen pb-16 pt-[88px] text-foreground">
    <div class="mx-auto w-full max-w-4xl px-4 sm:px-6">
      <div class="mb-7 border-b pb-5">
        <h1 class="text-[26px] font-bold tracking-[-0.025em] md:text-[30px]">{{ t('nav.notice') }}</h1>
        <p class="mt-1 text-sm text-muted-foreground">{{ t('notice.subtitle') }}</p>
      </div>

      <div v-if="loading" class="atelier-card divide-y overflow-hidden rounded-[16px] bg-card">
        <div v-for="i in 6" :key="i" class="flex items-center gap-4 px-5 py-4">
          <div class="h-10 w-10 shrink-0 rounded-[10px] theme-skeleton"></div>
          <div class="flex-1 space-y-2">
            <div class="h-4 w-2/3 rounded theme-skeleton"></div>
            <div class="h-3 w-1/2 rounded theme-skeleton"></div>
          </div>
          <div class="h-4 w-20 rounded theme-skeleton"></div>
        </div>
      </div>

      <div v-else-if="notices.length > 0">
        <div class="atelier-card divide-y overflow-hidden rounded-[16px] bg-card">
          <RouterLink
            v-for="notice in notices"
            :key="notice.id"
            :to="`/blog/${notice.slug}`"
            class="group flex items-center gap-4 px-5 py-4 transition-colors hover:bg-background"
          >
            <div class="hidden h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-[10px] border bg-background text-muted-foreground sm:flex">
              <img v-if="notice.thumbnail" :src="getImageUrl(notice.thumbnail)" alt="" loading="lazy" class="h-full w-full object-cover" />
              <Bell v-else class="h-4 w-4" aria-hidden="true" />
            </div>
            <div class="min-w-0 flex-1">
              <h2 class="truncate text-[15px] font-semibold text-foreground">{{ getLocalizedText(notice.title) }}</h2>
              <p v-if="getLocalizedText(notice.summary)" class="mt-0.5 truncate text-[13px] text-muted-foreground">
                {{ getLocalizedText(notice.summary) }}
              </p>
              <time class="mt-1 block font-mono text-xs text-muted-foreground md:hidden">
                {{ formatDate(notice.published_at) }}
              </time>
            </div>
            <time class="hidden shrink-0 font-mono text-xs tabular-nums text-muted-foreground md:block">
              {{ formatDate(notice.published_at) }}
            </time>
            <ChevronRight class="h-4 w-4 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5 group-hover:text-foreground" aria-hidden="true" />
          </RouterLink>
        </div>

        <PaginationNav :current-page="currentPage" :total-pages="totalPages" @change-page="changePage" />
      </div>

      <EmptyState v-else size="lg" :title="t('notice.empty')">
        <template #icon>
          <Bell class="h-14 w-14 text-muted-foreground opacity-65" :stroke-width="1.5" />
        </template>
      </EmptyState>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Bell, ChevronRight } from 'lucide-vue-next'
import { getImageUrl } from '../../utils/image'
import EmptyState from '../../components/EmptyState.vue'
import PaginationNav from '../../components/PaginationNav.vue'
import { usePostList } from '../../composables/usePostList'

const { t } = useI18n()

const {
  loading, posts: notices, currentPage, totalPages,
  getLocalizedText, formatDate, changePage,
} = usePostList('notice', { title: () => t('nav.notice'), canonicalPath: '/notice' })
</script>
