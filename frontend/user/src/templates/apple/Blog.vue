<template>
  <div class="st-root st-page">
    <div class="st-container">
      <div class="blog-shell">
        <header class="st-page-head">
          <h1 class="st-page-title">{{ t('nav.blog') }}</h1>
          <p class="st-page-lead">{{ t('blog.subtitle') }}</p>
        </header>

        <div class="blog-search">
          <Search class="blog-search-icon" aria-hidden="true" />
          <input
            v-model="searchKeyword"
            type="text"
            class="blog-search-input"
            :placeholder="t('blog.searchPlaceholder')"
            :aria-label="t('blog.searchPlaceholder')"
          />
          <button v-if="searchKeyword" type="button" class="blog-search-clear" @click="searchKeyword = ''">
            {{ t('blog.searchClear') }}
          </button>
        </div>

        <!-- Loading -->
        <div v-if="loading" class="blog-list" aria-hidden="true">
          <div v-for="i in 6" :key="i" class="blog-row blog-skel">
            <span class="blog-skel-date"></span>
            <span class="blog-skel-title"></span>
            <span class="blog-skel-summary"></span>
          </div>
        </div>

        <!-- Posts -->
        <template v-else-if="posts.length > 0">
          <div class="blog-list">
            <RouterLink v-for="post in posts" :key="post.id" class="blog-row" :to="`/blog/${post.slug}`">
              <span class="blog-date">{{ formatDate(post.published_at) }}</span>
              <h2 class="blog-row-title">{{ getLocalizedText(post.title) }}</h2>
              <p v-if="getLocalizedText(post.summary)" class="blog-summary">{{ getLocalizedText(post.summary) }}</p>
            </RouterLink>
          </div>

          <nav v-if="totalPages > 1" class="blog-pager" :aria-label="t('pagination.label')">
            <button
              type="button"
              class="blog-pager-btn"
              :aria-label="t('pagination.previous')"
              :disabled="currentPage <= 1"
              @click="changePage(currentPage - 1)"
            >
              <ChevronLeft aria-hidden="true" />
            </button>
            <span class="blog-pager-count">{{ currentPage }} / {{ totalPages }}</span>
            <button
              type="button"
              class="blog-pager-btn"
              :aria-label="t('pagination.next')"
              :disabled="currentPage >= totalPages"
              @click="changePage(currentPage + 1)"
            >
              <ChevronRight aria-hidden="true" />
            </button>
          </nav>
        </template>

        <!-- Empty -->
        <div v-else class="blog-empty">
          <BookOpen class="blog-empty-icon" aria-hidden="true" />
          <p>{{ searchKeyword.trim() ? t('blog.noResults') : t('blog.empty') }}</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { BookOpen, ChevronLeft, ChevronRight, Search } from 'lucide-vue-next'
import { usePostList } from '../../composables/usePostList'

const { t } = useI18n()

const {
  loading, posts, currentPage, totalPages, searchKeyword,
  getLocalizedText, formatDate, changePage,
} = usePostList('blog', { title: () => t('nav.blog'), canonicalPath: '/blog' })
</script>

<style scoped>
.blog-shell {
  --blog-hair: color-mix(in srgb, var(--st-text) 12%, transparent);
  max-width: 900px;
  margin-inline: auto;
}

/* ---------- 搜索 ---------- */
.blog-search {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  max-width: 520px;
  height: 44px;
  margin-top: 32px;
  padding: 0 16px 0 42px;
  border-radius: 980px;
  background: var(--st-bg-alt);
}
.blog-search:focus-within {
  outline: 2px solid var(--st-link);
  outline-offset: 2px;
}
.blog-search-icon {
  position: absolute;
  left: 16px;
  top: 50%;
  width: 16px;
  height: 16px;
  transform: translateY(-50%);
  color: var(--st-text-2);
  stroke-width: 2;
  pointer-events: none;
}
.blog-search-input {
  flex: 1;
  min-width: 0;
  height: 100%;
  border: 0;
  background: none;
  outline: none;
  font: inherit;
  font-size: 16px;
  color: var(--st-text);
}
.blog-search-input::placeholder {
  color: var(--st-text-3);
  opacity: 1;
}
.blog-search-clear {
  flex: none;
  padding: 0;
  border: 0;
  background: none;
  font: inherit;
  font-size: 13px;
  color: var(--st-text-2);
  cursor: pointer;
}
.blog-search-clear:hover {
  color: var(--st-text);
}

/* ---------- 列表 ---------- */
.blog-list {
  margin-top: 40px;
  border-bottom: 1px solid var(--blog-hair);
}
.blog-row {
  display: block;
  padding: 26px 0;
  border-top: 1px solid var(--blog-hair);
  color: var(--st-text);
  text-decoration: none;
}
.blog-date {
  display: block;
  margin-bottom: 8px;
  font-size: 12px;
  line-height: 1.3;
  letter-spacing: 0.01em;
  color: var(--st-text-3);
}
.blog-row-title {
  margin: 0;
  font-family: var(--st-font-display);
  font-size: 21px;
  line-height: 1.2;
  font-weight: 600;
  letter-spacing: -0.01em;
  color: var(--st-text);
  transition: color 200ms var(--st-ease);
}
@media (min-width: 735px) {
  .blog-row-title {
    font-size: 24px;
  }
}
.blog-summary {
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  overflow: hidden;
  margin: 8px 0 0;
  font-size: 15px;
  line-height: 1.5;
  color: var(--st-text-2);
}
.blog-row:focus-visible {
  outline: 2px solid var(--st-link);
  outline-offset: 4px;
  border-radius: 2px;
}
@media (hover: hover) {
  .blog-row:hover .blog-row-title {
    color: var(--st-link);
  }
}

/* ---------- 骨架 ---------- */
.blog-skel {
  pointer-events: none;
}
.blog-skel-date,
.blog-skel-title,
.blog-skel-summary {
  display: block;
  border-radius: 4px;
  background: var(--st-bg-alt);
  animation: blog-pulse 1.4s ease-in-out infinite;
}
.blog-skel-date {
  width: 92px;
  height: 10px;
  margin-bottom: 14px;
}
.blog-skel-title {
  width: min(72%, 560px);
  height: 22px;
}
.blog-skel-summary {
  width: min(88%, 700px);
  height: 12px;
  margin-top: 12px;
}
@keyframes blog-pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.45;
  }
}

/* ---------- 分页 ---------- */
.blog-pager {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 14px;
  margin-top: 48px;
}
.blog-pager-btn {
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  padding: 0;
  border: 0;
  border-radius: 999px;
  background: var(--st-bg-alt);
  color: var(--st-text);
  cursor: pointer;
  transition: background 200ms var(--st-ease), opacity 200ms var(--st-ease);
}
.blog-pager-btn:hover:not(:disabled) {
  background: color-mix(in srgb, var(--st-text) 10%, transparent);
}
.blog-pager-btn:disabled {
  opacity: 0.28;
  cursor: default;
}
.blog-pager-btn svg {
  width: 16px;
  height: 16px;
  stroke-width: 2;
}
.blog-pager-count {
  font-size: 13px;
  color: var(--st-text-2);
  font-variant-numeric: tabular-nums;
}

/* ---------- 空态 ---------- */
.blog-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
  padding: 96px 0;
  text-align: center;
  color: var(--st-text-2);
}
.blog-empty-icon {
  width: 30px;
  height: 30px;
  opacity: 0.35;
  stroke-width: 1.6;
}
.blog-empty p {
  margin: 0;
  font-size: 17px;
  line-height: 1.5;
}

@media (prefers-reduced-motion: reduce) {
  .blog-skel-date,
  .blog-skel-title,
  .blog-skel-summary {
    animation: none;
  }
  .blog-row-title,
  .blog-pager-btn {
    transition: none;
  }
}
</style>
