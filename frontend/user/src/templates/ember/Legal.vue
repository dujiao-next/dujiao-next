<template>
  <div class="mx-auto w-full max-w-[880px] px-4 pb-10 pt-2 sm:px-6">
    <!-- 加载 -->
    <div v-if="loading" class="flex flex-col gap-4 py-16">
      <div class="mx-auto h-8 w-[40%] rounded-[var(--r-xs)] bg-[color:var(--surface-2)]"></div>
      <div class="h-[280px] rounded-[var(--r-lg)] border border-border bg-card"></div>
    </div>

    <div v-else class="mt-7 rounded-[var(--r-lg)] border border-border bg-card p-6 sm:p-9">
      <h1 class="mb-6 border-b border-border pb-5 text-[clamp(1.5rem,1.15rem+1.5vw,2.25rem)] font-semibold tracking-[-0.022em]">
        {{ title }}
      </h1>

      <div
        v-if="content"
        class="ember-prose max-w-[72ch] text-[15px] leading-[1.75] text-[color:var(--ink-2)]"
        v-html="content"
      ></div>

      <div v-else class="flex flex-col items-center gap-3 py-16 text-center">
        <FileText class="h-11 w-11 text-muted-foreground" />
        <p class="text-[14px] text-muted-foreground">{{ t('common.noContent') }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { FileText } from 'lucide-vue-next'
import { useLegal } from '../../composables/useLegal'

const { t } = useI18n()

const props = defineProps<{ type: 'terms' | 'privacy' }>()

const { loading, title, content } = useLegal(() => props.type)
</script>

<style scoped>
.ember-prose :deep(h1),
.ember-prose :deep(h2),
.ember-prose :deep(h3) {
  margin-block: 22px 10px;
  font-weight: 600;
  letter-spacing: -0.02em;
  color: var(--ink);
}
.ember-prose :deep(h1) { font-size: 1.35rem; }
.ember-prose :deep(h2) { font-size: 1.15rem; }
.ember-prose :deep(h3) { font-size: 1.02rem; }
.ember-prose :deep(p) { margin-block: 10px; }
.ember-prose :deep(ul),
.ember-prose :deep(ol) {
  margin-block: 10px;
  padding-left: 22px;
  list-style: revert;
}
.ember-prose :deep(li) { margin-block: 4px; }
.ember-prose :deep(a) { color: var(--accent); }
.ember-prose :deep(strong) { font-weight: 600; color: var(--ink); }
.ember-prose :deep(img) { border-radius: var(--r); margin-block: 16px; }
.ember-prose :deep(blockquote) {
  margin-block: 14px;
  padding-left: 16px;
  border-left: 3px solid var(--border-strong);
  color: var(--ink-3);
}
.ember-prose :deep(table) {
  width: 100%;
  border-collapse: collapse;
  margin-block: 16px;
  font-size: 0.9rem;
}
.ember-prose :deep(th),
.ember-prose :deep(td) {
  border: 1px solid var(--border);
  padding: 8px 12px;
  text-align: left;
}
.ember-prose :deep(th) { background: var(--surface-2); font-weight: 600; }
</style>
