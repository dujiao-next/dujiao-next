<template>
  <div class="st-root st-page">
    <div class="st-container">
      <div class="notice-shell">
        <header class="st-page-head">
          <h1 class="st-page-title">{{ t('nav.notice') }}</h1>
          <p class="st-page-lead">{{ t('notice.subtitle') }}</p>
        </header>

        <!-- Loading -->
        <div v-if="loading" class="notice-list" aria-hidden="true">
          <div v-for="i in 6" :key="i" class="notice-row notice-skel">
            <span class="notice-skel-date"></span>
            <span class="notice-skel-title"></span>
            <span class="notice-skel-summary"></span>
          </div>
        </div>

        <!-- Notices -->
        <template v-else-if="notices.length > 0">
          <div class="notice-list">
            <button
              v-for="notice in notices"
              :key="notice.id"
              type="button"
              class="notice-row"
              @click="goToNotice(notice.slug)"
            >
              <span class="notice-date">{{ formatDate(notice.published_at) }}</span>
              <span class="notice-row-title">{{ getLocalizedText(notice.title) }}</span>
              <span v-if="getLocalizedText(notice.summary)" class="notice-summary">{{ getLocalizedText(notice.summary) }}</span>
            </button>
          </div>

          <nav v-if="totalPages > 1" class="notice-pager" :aria-label="t('pagination.label')">
            <button
              type="button"
              class="notice-pager-btn"
              :aria-label="t('pagination.previous')"
              :disabled="currentPage <= 1"
              @click="changePage(currentPage - 1)"
            >
              <ChevronLeft aria-hidden="true" />
            </button>
            <span class="notice-pager-count">{{ currentPage }} / {{ totalPages }}</span>
            <button
              type="button"
              class="notice-pager-btn"
              :aria-label="t('pagination.next')"
              :disabled="currentPage >= totalPages"
              @click="changePage(currentPage + 1)"
            >
              <ChevronRight aria-hidden="true" />
            </button>
          </nav>
        </template>

        <!-- Empty -->
        <div v-else class="notice-empty">
          <Bell class="notice-empty-icon" aria-hidden="true" />
          <p>{{ t('notice.empty') }}</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Bell, ChevronLeft, ChevronRight } from 'lucide-vue-next'
import { usePostList } from '../../composables/usePostList'

const { t } = useI18n()

const {
  loading, posts: notices, currentPage, totalPages,
  getLocalizedText, formatDate, goToPost: goToNotice, changePage,
} = usePostList('notice', { title: () => t('nav.notice'), canonicalPath: '/notice' })
</script>

<style scoped>
.notice-shell {
  --notice-hair: color-mix(in srgb, var(--st-text) 12%, transparent);
  max-width: 900px;
  margin-inline: auto;
}

/* ---------- 列表 ---------- */
.notice-list {
  margin-top: 40px;
  border-bottom: 1px solid var(--notice-hair);
}
.notice-row {
  display: block;
  width: 100%;
  padding: 26px 0;
  border: 0;
  border-top: 1px solid var(--notice-hair);
  border-radius: 0;
  background: none;
  color: var(--st-text);
  font: inherit;
  text-align: left;
  cursor: pointer;
}
.notice-date {
  display: block;
  margin-bottom: 8px;
  font-size: 12px;
  line-height: 1.3;
  letter-spacing: 0.01em;
  color: var(--st-text-3);
}
.notice-row-title {
  display: block;
  font-family: var(--st-font-display);
  font-size: 21px;
  line-height: 1.2;
  font-weight: 600;
  letter-spacing: -0.01em;
  color: var(--st-text);
  transition: color 200ms var(--st-ease);
}
@media (min-width: 735px) {
  .notice-row-title {
    font-size: 24px;
  }
}
.notice-summary {
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  overflow: hidden;
  margin-top: 8px;
  font-size: 15px;
  line-height: 1.5;
  color: var(--st-text-2);
}
.notice-row:focus-visible {
  outline: 2px solid var(--st-link);
  outline-offset: 4px;
}
@media (hover: hover) {
  .notice-row:hover .notice-row-title {
    color: var(--st-link);
  }
}

/* ---------- 骨架 ---------- */
.notice-skel {
  pointer-events: none;
}
.notice-skel-date,
.notice-skel-title,
.notice-skel-summary {
  display: block;
  border-radius: 4px;
  background: var(--st-bg-alt);
  animation: notice-pulse 1.4s ease-in-out infinite;
}
.notice-skel-date {
  width: 92px;
  height: 10px;
  margin-bottom: 14px;
}
.notice-skel-title {
  width: min(72%, 560px);
  height: 22px;
}
.notice-skel-summary {
  width: min(88%, 700px);
  height: 12px;
  margin-top: 12px;
}
@keyframes notice-pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.45;
  }
}

/* ---------- 分页 ---------- */
.notice-pager {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 14px;
  margin-top: 48px;
}
.notice-pager-btn {
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
.notice-pager-btn:hover:not(:disabled) {
  background: color-mix(in srgb, var(--st-text) 10%, transparent);
}
.notice-pager-btn:disabled {
  opacity: 0.28;
  cursor: default;
}
.notice-pager-btn svg {
  width: 16px;
  height: 16px;
  stroke-width: 2;
}
.notice-pager-count {
  font-size: 13px;
  color: var(--st-text-2);
  font-variant-numeric: tabular-nums;
}

/* ---------- 空态 ---------- */
.notice-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
  padding: 96px 0;
  text-align: center;
  color: var(--st-text-2);
}
.notice-empty-icon {
  width: 30px;
  height: 30px;
  opacity: 0.35;
  stroke-width: 1.6;
}
.notice-empty p {
  margin: 0;
  font-size: 17px;
  line-height: 1.5;
}

@media (prefers-reduced-motion: reduce) {
  .notice-skel-date,
  .notice-skel-title,
  .notice-skel-summary {
    animation: none;
  }
  .notice-row-title,
  .notice-pager-btn {
    transition: none;
  }
}
</style>
