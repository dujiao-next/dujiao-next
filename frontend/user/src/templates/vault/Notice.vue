<template>
  <div class="mx-auto w-full max-w-[1180px] px-6 pb-10 pt-2">
    <header class="my-6 flex flex-col items-start gap-2">
      <h1 class="text-[34px] font-extrabold">{{ t('nav.notice') }}</h1>
      <p class="text-muted-foreground">{{ t('notice.subtitle') }}</p>
    </header>

    <!-- Loading -->
    <div v-if="loading" class="mx-auto grid max-w-[880px] gap-3.5">
      <div v-for="i in 6" :key="i" class="h-[104px] rounded-lg border bg-card opacity-50"></div>
    </div>

    <!-- Notices -->
    <template v-else-if="notices.length > 0">
      <div class="mx-auto grid max-w-[880px] gap-3.5">
        <button
          v-for="notice in notices"
          :key="notice.id"
          type="button"
          class="group flex w-full items-center gap-[18px] rounded-lg border bg-card px-[22px] py-[18px] text-left transition hover:-translate-x-0.5 hover:border-hairline-strong hover:shadow-[var(--shadow)]"
          @click="goToNotice(notice.slug)"
        >
          <div
            class="relative grid h-14 w-14 flex-none place-items-center overflow-hidden rounded-md bg-secondary max-[640px]:hidden"
            :class="notice.thumbnail ? '' : 'bg-[color:var(--teal)]'"
          >
            <img v-if="notice.thumbnail" :src="getImageUrl(notice.thumbnail)" :alt="getLocalizedText(notice.title)" loading="lazy" class="absolute inset-0 h-full w-full object-cover" />
            <Bell v-else class="h-6 w-6 text-white/90" />
          </div>
          <div class="min-w-0 flex-1">
            <div class="mb-1.5 flex items-center gap-2.5">
              <Badge variant="warning" size="sm" class="rounded-full">{{ t('nav.notice') }}</Badge>
              <span class="text-xs font-semibold text-muted-foreground">{{ formatDate(notice.published_at) }}</span>
            </div>
            <h2 class="truncate text-lg font-bold">{{ getLocalizedText(notice.title) }}</h2>
            <p class="mt-0.5 truncate text-sm text-muted-foreground">{{ getLocalizedText(notice.summary) }}</p>
          </div>
          <ChevronRight class="h-5 w-5 flex-none text-muted-foreground transition group-hover:text-primary" />
        </button>
      </div>

      <div v-if="totalPages > 1" class="mt-8 flex items-center justify-center gap-3.5">
        <Button variant="outline" size="icon" class="rounded-full" :disabled="currentPage <= 1" @click="changePage(currentPage - 1)"><ChevronLeft /></Button>
        <span class="text-sm font-bold">{{ currentPage }} / {{ totalPages }}</span>
        <Button variant="outline" size="icon" class="rounded-full" :disabled="currentPage >= totalPages" @click="changePage(currentPage + 1)"><ChevronRight /></Button>
      </div>
    </template>

    <!-- Empty -->
    <div v-else class="flex flex-col items-center gap-3 rounded-xl border border-dashed py-16 text-center text-muted-foreground">
      <Bell class="h-10 w-10 opacity-60" />
      <p>{{ t('notice.empty') }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Bell, ChevronLeft, ChevronRight } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { getImageUrl } from '../../utils/image'
import { usePostList } from '../../composables/usePostList'

const { t } = useI18n()

const {
  loading, posts: notices, currentPage, totalPages,
  getLocalizedText, formatDate, goToPost: goToNotice, changePage,
} = usePostList('notice', { title: () => t('nav.notice'), canonicalPath: () => '/notice' })
</script>
