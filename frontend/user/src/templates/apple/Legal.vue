<template>
  <div class="st-root st-page">
    <div class="st-container">
      <div class="legal-shell">
        <div v-if="loading" class="legal-loading">
          <Loader2 class="legal-spinner" aria-hidden="true" />
        </div>

        <template v-else>
          <header class="st-page-head">
            <h1 class="st-page-title">{{ title }}</h1>
          </header>

          <hr class="st-hairline legal-rule" />

          <div v-if="content" class="legal-prose" v-html="content"></div>

          <div v-else class="legal-empty">
            <FileText class="legal-empty-icon" aria-hidden="true" />
            <p>{{ t('common.noContent') }}</p>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { FileText, Loader2 } from 'lucide-vue-next'
import { useLegal } from '../../composables/useLegal'

const { t } = useI18n()

const props = defineProps<{
  type: 'terms' | 'privacy'
}>()

const { loading, title, content } = useLegal(() => props.type)
</script>

<style scoped>
.legal-shell {
  --legal-hair: color-mix(in srgb, var(--st-text) 12%, transparent);
  max-width: 720px;
  margin-inline: auto;
}

.legal-rule {
  margin-top: 16px;
  background: var(--legal-hair);
}

/* ---------- 正文 ---------- */
.legal-prose {
  padding-top: 40px;
  font-size: 17px;
  line-height: 1.6;
  color: var(--st-text);
}
.legal-prose :deep(p) {
  margin: 0 0 1em;
  text-wrap: pretty;
}
.legal-prose :deep(p:last-child) {
  margin-bottom: 0;
}
.legal-prose :deep(h1),
.legal-prose :deep(h2),
.legal-prose :deep(h3),
.legal-prose :deep(h4) {
  font-family: var(--st-font-display);
  font-weight: 600;
  letter-spacing: -0.01em;
  color: var(--st-text);
  scroll-margin-top: 88px;
}
.legal-prose :deep(h1) {
  margin: 48px 0 16px;
  font-size: 40px;
  line-height: 1.1;
  letter-spacing: -0.015em;
}
.legal-prose :deep(h2) {
  margin: 48px 0 14px;
  padding-top: 32px;
  border-top: 1px solid var(--legal-hair);
  font-size: 28px;
  line-height: 1.15;
}
@media (min-width: 735px) {
  .legal-prose :deep(h2) {
    font-size: 32px;
  }
}
.legal-prose :deep(h3) {
  margin: 32px 0 10px;
  font-size: 24px;
  line-height: 1.2;
}
.legal-prose :deep(h4) {
  margin: 24px 0 8px;
  font-size: 19px;
  line-height: 1.3;
}
.legal-prose :deep(h1:first-child),
.legal-prose :deep(h2:first-child),
.legal-prose :deep(h3:first-child) {
  margin-top: 0;
  padding-top: 0;
  border-top: 0;
}
.legal-prose :deep(a) {
  color: var(--st-link);
  text-decoration: underline;
  text-decoration-color: color-mix(in srgb, var(--st-link) 45%, transparent);
  text-decoration-thickness: 1px;
  text-underline-offset: 2px;
}
.legal-prose :deep(a:hover) {
  text-decoration-color: var(--st-link);
}
.legal-prose :deep(strong),
.legal-prose :deep(b) {
  font-weight: 600;
}
.legal-prose :deep(ul),
.legal-prose :deep(ol) {
  margin: 0 0 1em;
  padding-left: 1.4em;
}
.legal-prose :deep(ul) {
  list-style: disc;
}
.legal-prose :deep(ol) {
  list-style: decimal;
}
.legal-prose :deep(li) {
  margin: 0.35em 0;
  padding-left: 0.1em;
}
.legal-prose :deep(li)::marker {
  color: var(--st-text-3);
}
.legal-prose :deep(hr) {
  height: 0;
  margin: 40px 0;
  border: 0;
  border-top: 1px solid var(--legal-hair);
}
.legal-prose :deep(blockquote) {
  margin: 24px 0;
  padding-left: 20px;
  border-left: 1px solid var(--legal-hair);
  color: var(--st-text-2);
}
.legal-prose :deep(img) {
  display: block;
  max-width: 100%;
  height: auto;
  margin: 24px 0;
  border-radius: 14px;
}
.legal-prose :deep(table) {
  width: 100%;
  margin: 24px 0;
  border-collapse: collapse;
  font-size: 15px;
  line-height: 1.45;
}
.legal-prose :deep(th),
.legal-prose :deep(td) {
  padding: 10px 12px;
  border-bottom: 1px solid var(--legal-hair);
  text-align: left;
  vertical-align: top;
}
.legal-prose :deep(th) {
  font-weight: 600;
}
.legal-prose :deep(code) {
  padding: 0.12em 0.35em;
  border-radius: 4px;
  background: var(--st-bg-alt);
  font-size: 0.88em;
}
.legal-prose :deep(pre) {
  margin: 24px 0;
  padding: 16px;
  border-radius: 10px;
  background: var(--st-bg-alt);
  overflow-x: auto;
  font-size: 14px;
  line-height: 1.5;
}
.legal-prose :deep(pre code) {
  padding: 0;
  background: none;
  font-size: inherit;
}

/* ---------- 加载 / 空态 ---------- */
.legal-loading {
  display: grid;
  place-items: center;
  min-height: 360px;
  color: var(--st-text-2);
}
.legal-spinner {
  width: 28px;
  height: 28px;
  animation: legal-spin 1s linear infinite;
}
@keyframes legal-spin {
  to {
    transform: rotate(360deg);
  }
}

.legal-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 72px 0;
  text-align: center;
  color: var(--st-text-2);
}
.legal-empty-icon {
  width: 28px;
  height: 28px;
  opacity: 0.35;
  stroke-width: 1.6;
}
.legal-empty p {
  margin: 0;
  font-size: 15px;
  line-height: 1.5;
}

@media (prefers-reduced-motion: reduce) {
  .legal-spinner {
    animation: none;
  }
}
</style>
