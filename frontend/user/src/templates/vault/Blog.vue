<template>
  <div class="mx-auto w-full max-w-[1180px] px-6 pb-10 pt-2">
    <header class="mt-6 flex flex-col items-start gap-2">
      <h1 class="text-[34px] font-extrabold">{{ t('nav.blog') }}</h1>
      <p class="text-muted-foreground">{{ t('blog.subtitle') }}</p>
    </header>

    <div class="relative my-7 max-w-[520px]">
      <Search class="pointer-events-none absolute left-3.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
      <Input v-model="searchKeyword" type="text" class="h-11 pl-10 pr-[88px]" :placeholder="t('blog.searchPlaceholder')" />
      <Button v-if="searchKeyword" type="button" variant="ghost" size="sm" class="absolute right-1.5 top-1/2 h-8 -translate-y-1/2 rounded-full" @click="searchKeyword = ''">{{ t('blog.searchClear') }}</Button>
    </div>

    <!-- Category filter -->
    <div v-if="categoryFilterEnabled && categories.length" class="mb-7 flex flex-wrap items-center gap-2">
      <Button size="sm" class="rounded-full" :variant="hasCategoryFilter ? 'outline' : 'default'" @click="clearCategory()">
        {{ t('blog.allCategories') }}
      </Button>
      <Button v-for="category in categories" :key="category.id" size="sm" class="rounded-full"
        :variant="selectedCategory === category.slug ? 'default' : 'outline'"
        @click="selectCategory(category.slug)">
        {{ getLocalizedText(category.name) }}
      </Button>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="grid gap-5 grid-cols-[repeat(auto-fill,minmax(280px,1fr))]">
      <div v-for="i in 6" :key="i" class="h-[280px] rounded-xl border bg-card opacity-50"></div>
    </div>

    <!-- Posts -->
    <template v-else-if="posts.length > 0">
      <div class="grid gap-5 grid-cols-[repeat(auto-fill,minmax(280px,1fr))]">
        <RouterLink
          v-for="post in posts"
          :key="post.id"
          class="group flex flex-col rounded-xl border bg-card p-[22px] shadow-sm transition hover:-translate-y-1 hover:border-hairline-strong hover:shadow-[var(--shadow)]"
          :to="`/blog/${post.slug}`"
        >
          <div v-if="post.thumbnail" class="-mx-[22px] -mt-[22px] mb-4 h-[168px] overflow-hidden rounded-t-xl">
            <img :src="getImageUrl(post.thumbnail)" :alt="getLocalizedText(post.title)" loading="lazy" class="h-full w-full object-cover" />
          </div>
          <div class="mb-1 flex items-center gap-2.5">
            <Badge :variant="post.type === 'blog' ? 'info' : 'warning'" size="sm" class="rounded-full">{{ post.type === 'blog' ? t('nav.blog') : t('nav.notice') }}</Badge>
            <span class="text-xs font-semibold text-muted-foreground">{{ formatDate(post.published_at) }}</span>
          </div>
          <h3 class="text-lg font-bold">{{ getLocalizedText(post.title) }}</h3>
          <p class="mt-1.5 flex-1 text-sm leading-relaxed text-muted-foreground">{{ getLocalizedText(post.summary) }}</p>
          <span class="mt-3 inline-flex items-center gap-1.5 text-sm font-semibold text-primary">
            {{ t('blog.readMore') }} <ArrowRight class="h-4 w-4 transition group-hover:translate-x-0.5" />
          </span>
        </RouterLink>
      </div>

      <div v-if="totalPages > 1" class="mt-8 flex items-center justify-center gap-3.5">
        <Button variant="outline" size="icon" class="rounded-full" :disabled="currentPage <= 1" @click="changePage(currentPage - 1)"><ChevronLeft /></Button>
        <span class="text-sm font-bold">{{ currentPage }} / {{ totalPages }}</span>
        <Button variant="outline" size="icon" class="rounded-full" :disabled="currentPage >= totalPages" @click="changePage(currentPage + 1)"><ChevronRight /></Button>
      </div>
    </template>

    <!-- Empty -->
    <div v-else class="flex flex-col items-center gap-3 rounded-xl border border-dashed py-16 text-center text-muted-foreground">
      <BookOpen class="h-10 w-10 opacity-60" />
      <p>{{ emptyStateTitle }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ArrowRight, BookOpen, ChevronLeft, ChevronRight, Search } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { getImageUrl } from '../../utils/image'
import { usePostList } from '../../composables/usePostList'

const { t } = useI18n()
const route = useRoute()

const {
  loading, posts, currentPage, totalPages, searchKeyword,
  categories, selectedCategory, categoryFilterEnabled, hasCategoryFilter,
  selectCategory, clearCategory,
  getLocalizedText, formatDate, changePage,
} = usePostList(
  'blog',
  { title: () => t('nav.blog'), canonicalPath: () => route.path },
  { categoryFilter: true },
)

const emptyStateTitle = computed(() => {
  if (searchKeyword.value.trim()) return t('blog.noResults')
  if (hasCategoryFilter.value) return t('blog.noCategoryResults')
  return t('blog.empty')
})
</script>
