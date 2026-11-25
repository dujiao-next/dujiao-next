<template>
  <div class="min-h-screen bg-background pb-16 pt-24 text-foreground">
    <div class="container mx-auto max-w-4xl px-4">
      <div class="mb-6">
        <h1 class="text-[28px] font-bold tracking-tight">{{ t('nav.notice') }}</h1>
        <p class="mt-1 text-sm text-muted-foreground">{{ t('notice.subtitle') }}</p>
      </div>

      <div v-if="loading" class="theme-float flex flex-col gap-1.5 rounded-[22px] bg-card p-2 md:p-3">
        <div v-for="i in 6" :key="i" class="theme-skeleton h-[72px] rounded-2xl"></div>
      </div>

      <!-- 一张面板里逐行排开：日期列在左，标题摘要在右 -->
      <div v-else-if="notices.length > 0">
        <div class="theme-float flex flex-col gap-1.5 rounded-[22px] bg-card p-2 md:p-3">
          <router-link
            v-for="notice in notices"
            :key="notice.id"
            :to="`/blog/${notice.slug}`"
            class="group flex items-center gap-4 rounded-2xl bg-secondary/60 p-3 transition-colors hover:bg-secondary focus-visible:outline-2 focus-visible:outline-primary md:px-4"
          >
            <div class="hidden h-12 w-12 shrink-0 items-center justify-center overflow-hidden rounded-xl bg-card text-muted-foreground sm:flex">
              <img v-if="notice.thumbnail" :src="getImageUrl(notice.thumbnail)" alt="" loading="lazy" class="h-full w-full object-cover" />
              <Bell v-else class="h-5 w-5" aria-hidden="true" />
            </div>
            <div class="min-w-0 flex-1">
              <h2 class="truncate text-[15px] font-semibold md:text-base">{{ getLocalizedText(notice.title) }}</h2>
              <p v-if="getLocalizedText(notice.summary)" class="mt-0.5 truncate text-[13px] text-muted-foreground">{{ getLocalizedText(notice.summary) }}</p>
              <time class="mt-1 block text-xs tabular-nums text-muted-foreground md:hidden">{{ formatDate(notice.published_at) }}</time>
            </div>
            <time class="hidden shrink-0 text-[13px] tabular-nums text-muted-foreground md:block">{{ formatDate(notice.published_at) }}</time>
            <ChevronRight class="h-4 w-4 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5 group-hover:text-foreground" aria-hidden="true" />
          </router-link>
        </div>

        <PaginationNav
          :current-page="currentPage"
          :total-pages="totalPages"
          @change-page="changePage"
        />
      </div>

      <EmptyState v-else size="lg" :title="t('notice.empty')">
        <template #icon>
          <Bell class="h-16 w-16 text-muted-foreground opacity-70" :stroke-width="1.5" />
        </template>
      </EmptyState>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Bell, ChevronRight } from 'lucide-vue-next'
import { getImageUrl } from '../utils/image'
import EmptyState from '../components/EmptyState.vue'
import PaginationNav from '../components/PaginationNav.vue'
import { usePostList } from '../composables/usePostList'

const { t } = useI18n()

const {
  loading, posts: notices, currentPage, totalPages,
  getLocalizedText, formatDate, changePage,
} = usePostList('notice', { title: () => t('nav.notice'), canonicalPath: '/notice' })
</script>
