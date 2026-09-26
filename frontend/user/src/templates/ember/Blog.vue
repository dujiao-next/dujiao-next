<template>
  <div class="mx-auto w-full max-w-[1180px] px-4 pb-10 pt-2 sm:px-6">
    <header class="mt-6 flex flex-col items-start gap-2">
      <h1 class="text-[clamp(1.75rem,1.3rem+1.9vw,2.6rem)] font-semibold tracking-[-0.022em]">{{ t('nav.blog') }}</h1>
      <p class="text-[15px] text-muted-foreground">{{ t('blog.subtitle') }}</p>
    </header>

    <!-- 搜索 -->
    <div class="relative my-7 w-full max-w-[420px]">
      <Search class="pointer-events-none absolute left-4 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
      <input
        v-model="searchKeyword"
        type="text"
        class="h-11 w-full rounded-full border border-border bg-card pl-10 pr-20 text-[14px] outline-none transition-colors placeholder:text-muted-foreground focus:border-[color:var(--accent)] focus:ring-[var(--ring)]"
        :placeholder="t('blog.searchPlaceholder')"
      />
      <button
        v-if="searchKeyword"
        type="button"
        class="absolute right-2 top-1/2 -translate-y-1/2 rounded-full px-3 py-1.5 text-[12.5px] text-muted-foreground transition-colors hover:bg-[color:var(--surface-2)] hover:text-foreground"
        @click="searchKeyword = ''"
      >{{ t('blog.searchClear') }}</button>
    </div>

    <!-- 加载 -->
    <div v-if="loading" class="grid gap-4 grid-cols-[repeat(auto-fill,minmax(272px,1fr))]">
      <div v-for="i in 6" :key="i" class="h-[300px] rounded-[var(--r-lg)] border border-border bg-card"></div>
    </div>

    <!-- 列表 -->
    <template v-else-if="posts.length > 0">
      <div class="grid gap-4 grid-cols-[repeat(auto-fill,minmax(272px,1fr))]">
        <RouterLink
          v-for="post in posts"
          :key="post.id"
          class="group flex flex-col overflow-hidden rounded-[var(--r-lg)] border border-border bg-card transition-all hover:-translate-y-1 hover:border-[color:var(--border-strong)] hover:shadow-[var(--shadow)]"
          :to="`/blog/${post.slug}`"
        >
          <div v-if="post.thumbnail" class="aspect-[16/10] overflow-hidden bg-[color:var(--surface-2)]">
            <img
              :src="getImageUrl(post.thumbnail)"
              :alt="getLocalizedText(post.title)"
              loading="lazy"
              class="h-full w-full object-cover transition-transform duration-500 group-hover:scale-[1.04]"
            />
          </div>

          <div class="flex flex-1 flex-col p-5">
            <div class="mb-2 flex flex-wrap items-center gap-2.5">
              <span class="ember-pill" :class="post.type === 'blog' ? 'ember-pill--accent' : 'ember-pill--warning'">
                {{ post.type === 'blog' ? t('nav.blog') : t('nav.notice') }}
              </span>
              <span class="text-[12px] text-muted-foreground">{{ formatDate(post.published_at) }}</span>
            </div>

            <h3 class="line-clamp-2 text-[15px] font-semibold leading-[1.4]">{{ getLocalizedText(post.title) }}</h3>
            <p class="mt-2 line-clamp-3 flex-1 text-[13px] leading-[1.6] text-muted-foreground">{{ getLocalizedText(post.summary) }}</p>

            <span class="mt-4 inline-flex items-center gap-1.5 text-[13px] font-medium text-[color:var(--accent)]">
              {{ t('blog.readMore') }}
              <ArrowRight class="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5" />
            </span>
          </div>
        </RouterLink>
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
      <BookOpen class="h-11 w-11 text-muted-foreground" />
      <p class="text-[14px] text-muted-foreground">{{ searchKeyword.trim() ? t('blog.noResults') : t('blog.empty') }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ArrowRight, BookOpen, ChevronLeft, ChevronRight, Search } from 'lucide-vue-next'
import { getImageUrl } from '../../utils/image'
import { usePostList } from '../../composables/usePostList'

const { t } = useI18n()

const {
  loading, posts, currentPage, totalPages, searchKeyword,
  getLocalizedText, formatDate, changePage,
} = usePostList('blog', { title: () => t('nav.blog'), canonicalPath: '/blog' })
</script>
