<template>
  <div
    class="notice-page min-h-screen bg-background text-foreground pt-20 pb-16">
    <div class="container mx-auto px-4">
      <!-- Page Header -->
      <div class="mb-16 mt-12 text-center">
        <h1 class="text-4xl md:text-6xl font-black mb-6 tracking-tight text-foreground">{{ t('nav.notice') }}</h1>
        <p
          class="text-muted-foreground max-w-2xl mx-auto text-lg leading-relaxed border-b pb-8">
          {{ t('notice.subtitle') }}
        </p>
      </div>

      <!-- Loading State -->
      <div v-if="loading" class="space-y-4 max-w-4xl mx-auto">
        <div v-for="i in 6" :key="i"
          class="bg-muted rounded-2xl h-32 animate-pulse border">
        </div>
      </div>

      <!-- Notices List -->
      <div v-else-if="notices.length > 0" class="max-w-4xl mx-auto space-y-4">
        <article v-for="notice in notices" :key="notice.id"
          class="group rounded-2xl border bg-card backdrop-blur-xl p-6 md:p-8 shadow-sm transition-all duration-300 hover:-translate-x-1 hover:shadow-md cursor-pointer flex items-center gap-6"
          @click="goToNotice(notice.slug)">
          <!-- Icon Column -->
          <div
            class="hidden sm:flex flex-shrink-0 w-16 h-16 rounded-xl overflow-hidden bg-secondary border items-center justify-center text-primary group-hover:scale-105 transition-transform">
            <img v-if="notice.thumbnail" :src="getImageUrl(notice.thumbnail)" :alt="getLocalizedText(notice.title)"
              loading="lazy" class="w-full h-full object-cover">
            <Bell v-else class="w-8 h-8" />
          </div>

          <!-- Content -->
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-3 mb-2">
              <Badge variant="accent" size="xs" class="uppercase tracking-wider">
                {{ t('nav.notice') }}
              </Badge>
              <time class="text-xs text-muted-foreground font-mono">
                {{ formatDate(notice.published_at) }}
              </time>
            </div>

            <h2
              class="text-xl font-bold text-foreground group-hover:text-primary transition-colors truncate mb-1">
              {{ getLocalizedText(notice.title) }}
            </h2>

            <p class="text-muted-foreground text-sm line-clamp-1">
              {{ getLocalizedText(notice.summary) }}
            </p>
          </div>

          <!-- Arrow -->
          <ChevronRight
            class="flex-shrink-0 w-6 h-6 text-muted-foreground group-hover:text-foreground transition-all group-hover:translate-x-1 duration-300" />
        </article>

        <!-- Pagination -->
        <PaginationNav
          :current-page="currentPage"
          :total-pages="totalPages"
          @change-page="changePage"
        />
      </div>

      <!-- Empty State -->
      <EmptyState v-else variant="soft" size="lg" class="max-w-4xl mx-auto" :title="t('notice.empty')">
        <template #icon>
          <Bell class="w-20 h-20 text-muted-foreground opacity-70" :stroke-width="1.5" />
        </template>
      </EmptyState>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Bell, ChevronRight } from 'lucide-vue-next'
import { getImageUrl } from '../utils/image'
import { Badge } from '@/components/ui/badge'
import EmptyState from '../components/EmptyState.vue'
import PaginationNav from '../components/PaginationNav.vue'
import { usePostList } from '../composables/usePostList'

const { t } = useI18n()

const {
  loading, posts: notices, currentPage, totalPages,
  getLocalizedText, formatDate, goToPost: goToNotice, changePage,
} = usePostList('notice', { title: () => t('nav.notice'), canonicalPath: () => '/notice' })
</script>

<style scoped>
.line-clamp-1 {
  overflow: hidden;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 1;
  line-clamp: 1;
}
</style>
