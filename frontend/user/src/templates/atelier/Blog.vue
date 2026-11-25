<template>
  <div class="min-h-screen pb-16 pt-[88px] text-foreground">
    <div class="mx-auto w-full max-w-[1240px] px-4 sm:px-6">
      <div class="mb-7 flex flex-wrap items-end justify-between gap-x-8 gap-y-4 border-b pb-5">
        <div>
          <h1 class="text-[26px] font-bold tracking-[-0.025em] md:text-[30px]">{{ t('nav.blog') }}</h1>
          <p class="mt-1 text-sm text-muted-foreground">{{ t('blog.subtitle') }}</p>
        </div>
        <label class="flex h-10 w-full items-center gap-2 rounded-full border bg-card px-3.5 shadow-xs transition-all focus-within:border-hairline-strong focus-within:ring-2 focus-within:ring-primary/20 sm:w-72">
          <Search class="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
          <input
            v-model="searchKeyword"
            type="text"
            :placeholder="t('blog.searchPlaceholder')"
            :aria-label="t('blog.searchPlaceholder')"
            class="min-w-0 flex-1 border-none bg-transparent text-[13.5px] text-foreground outline-none placeholder:text-muted-foreground"
          />
          <button
            v-if="searchKeyword"
            type="button"
            class="-mr-1 grid h-6 w-6 shrink-0 place-items-center rounded-full text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
            :aria-label="t('blog.searchClear')"
            @click="searchKeyword = ''"
          >
            <X class="h-3.5 w-3.5" />
          </button>
        </label>
      </div>

      <div v-if="loading" class="grid grid-cols-1 gap-5 md:grid-cols-2 lg:grid-cols-3">
        <div v-for="i in 6" :key="i" class="theme-skeleton h-[280px] rounded-[16px]"></div>
      </div>

      <div v-else-if="posts.length > 0">
        <div class="grid grid-cols-1 gap-5 md:grid-cols-2 lg:grid-cols-3">
          <article
            v-for="(post, index) in posts"
            :key="post.id"
            class="at-post-card atelier-card group flex cursor-pointer flex-col rounded-[16px] bg-card p-2"
            :class="index === 0 && 'md:col-span-2 md:flex-row md:p-2.5'"
            @click="goToPost(post.slug)"
          >
            <div
              class="atelier-stage flex shrink-0 items-center justify-center overflow-hidden rounded-[14px]"
              :class="index === 0 ? 'h-48 md:h-auto md:min-h-64 md:w-1/2' : 'h-44'"
              :style="{ '--h': placeholderHue(post.id) }"
            >
              <img
                v-if="post.thumbnail"
                :src="getImageUrl(post.thumbnail)"
                alt=""
                loading="lazy"
                class="h-full w-full object-cover transition-transform duration-500 group-hover:scale-[1.04]"
              />
              <span
                v-else
                class="select-none text-6xl font-extrabold tracking-[-0.05em] text-[hsl(var(--h)_42%_var(--at-glyph-l))] opacity-85"
                aria-hidden="true"
              >
                {{ Array.from(getLocalizedText(post.title).trim())[0] }}
              </span>
            </div>

            <div class="flex flex-1 flex-col p-4" :class="index === 0 && 'md:p-6'">
              <div class="flex items-center gap-2 text-xs text-muted-foreground">
                <span class="inline-flex h-5 items-center rounded-[5px] border bg-background px-1.5 text-[11px] font-semibold text-foreground/80">
                  {{ t('nav.blog') }}
                </span>
                <time class="font-mono tabular-nums">{{ formatDate(post.published_at) }}</time>
              </div>
              <h2
                class="mt-2.5 line-clamp-2 font-bold leading-snug tracking-[-0.02em] text-foreground"
                :class="index === 0 ? 'text-xl md:text-[24px]' : 'text-[16.5px]'"
              >
                <RouterLink :to="`/blog/${post.slug}`" class="focus-visible:outline-2 focus-visible:outline-primary" @click.stop>
                  {{ getLocalizedText(post.title) }}
                </RouterLink>
              </h2>
              <p class="mt-2 line-clamp-3 text-[13.5px] leading-relaxed text-muted-foreground">
                {{ getLocalizedText(post.summary) }}
              </p>
              <div class="mt-auto pt-4">
                <span class="inline-flex items-center gap-1 text-xs font-semibold text-foreground/80 transition-colors group-hover:text-foreground">
                  {{ t('blog.readMore') }}
                  <ArrowRight class="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5" aria-hidden="true" />
                </span>
              </div>
            </div>
          </article>
        </div>

        <PaginationNav :current-page="currentPage" :total-pages="totalPages" @change-page="changePage" />
      </div>

      <EmptyState v-else size="lg" :title="searchKeyword.trim() ? t('blog.noResults') : t('blog.empty')">
        <template #icon>
          <BookOpen class="h-14 w-14 text-muted-foreground opacity-65" :stroke-width="1.5" />
        </template>
      </EmptyState>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ArrowRight, BookOpen, Search, X } from 'lucide-vue-next'
import { getImageUrl, placeholderHue } from '../../utils/image'
import EmptyState from '../../components/EmptyState.vue'
import PaginationNav from '../../components/PaginationNav.vue'
import { usePostList } from '../../composables/usePostList'

const { t } = useI18n()

const {
  loading, posts, currentPage, totalPages, searchKeyword,
  getLocalizedText, formatDate, goToPost, changePage,
} = usePostList('blog', { title: () => t('nav.blog'), canonicalPath: '/blog' })
</script>

<style scoped>
.at-post-card {
  transition: transform 240ms var(--ui-ease-out), box-shadow 240ms var(--ui-ease-out), border-color 180ms ease;
}
.at-post-card:hover {
  transform: translateY(-2px);
  border-color: var(--at-line-hi);
  box-shadow: var(--at-shadow-card-hover);
}
@media (prefers-reduced-motion: reduce) {
  .at-post-card { transition: none; }
  .at-post-card:hover { transform: none; }
}
</style>
