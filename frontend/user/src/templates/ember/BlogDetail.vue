<template>
  <div class="mx-auto w-full max-w-[860px] px-4 pb-10 pt-2 sm:px-6">
    <!-- 加载 -->
    <div v-if="loading" class="grid gap-4 pt-6">
      <div class="h-5 w-2/5 rounded-[var(--r-xs)] bg-[color:var(--surface-2)]"></div>
      <div class="h-10 w-3/4 rounded-[var(--r-xs)] bg-[color:var(--surface-2)]"></div>
      <div class="h-72 rounded-[var(--r-lg)] border border-border bg-card"></div>
    </div>

    <!-- 正文 -->
    <article v-else-if="post">
      <nav class="flex flex-wrap items-center gap-1.5 py-4 text-[13px] text-muted-foreground">
        <RouterLink class="transition-colors hover:text-foreground" to="/">{{ t('nav.home') }}</RouterLink>
        <ChevronRight class="h-3.5 w-3.5 flex-none opacity-60" />
        <RouterLink class="transition-colors hover:text-foreground" :to="backLink">{{ backText }}</RouterLink>
        <ChevronRight class="h-3.5 w-3.5 flex-none opacity-60" />
        <span class="max-w-[240px] truncate text-foreground">{{ getLocalizedText(post.title) }}</span>
      </nav>

      <div class="rounded-[var(--r-lg)] border border-border bg-card p-6 sm:p-8">
        <div v-if="post.thumbnail" class="mb-7 aspect-[16/9] overflow-hidden rounded-[var(--r)] bg-[color:var(--surface-2)]">
          <img
            :src="getImageUrl(post.thumbnail)"
            :alt="getLocalizedText(post.title)"
            loading="lazy"
            class="h-full w-full object-cover"
          />
        </div>

        <header class="mb-6 border-b border-border pb-6">
          <div class="mb-3.5 flex flex-wrap items-center gap-3">
            <span class="ember-pill" :class="post.type === 'blog' ? 'ember-pill--accent' : 'ember-pill--warning'">
              {{ post.type === 'blog' ? t('nav.blog') : t('nav.notice') }}
            </span>
            <span class="text-[12.5px] text-muted-foreground">{{ formatDate(post.published_at) }}</span>
          </div>
          <h1 class="text-[clamp(1.5rem,1.15rem+1.5vw,2.25rem)] font-semibold leading-[1.2] tracking-[-0.025em]">
            {{ getLocalizedText(post.title) }}
          </h1>
          <p v-if="post.summary" class="mt-3.5 text-[16px] leading-[1.65] text-muted-foreground">
            {{ getLocalizedText(post.summary) }}
          </p>
        </header>

        <div
          class="ember-prose max-w-[72ch] text-[15.5px] leading-[1.78] text-[color:var(--ink-2)]"
          v-html="processHtmlForDisplay(getLocalizedText(post.content))"
        ></div>

        <!-- 相关商品 -->
        <section v-if="relatedProducts.length" class="mt-10 border-t border-border pt-7">
          <h2 class="mb-4 flex items-center gap-3 text-[19px] font-semibold tracking-[-0.02em]">
            <span class="h-[22px] w-[3px] flex-none rounded-full bg-[color:var(--accent)]"></span>
            {{ t('blog.relatedProducts') }}
          </h2>
          <div class="grid gap-3 grid-cols-[repeat(auto-fill,minmax(220px,1fr))]">
            <RouterLink
              v-for="rp in relatedProducts"
              :key="rp.id"
              :to="`/products/${rp.slug}`"
              class="flex items-center gap-3.5 rounded-[var(--r)] border border-[color:var(--border-soft)] bg-[color:var(--surface-2)] p-3 transition-all hover:-translate-y-0.5 hover:shadow-[var(--shadow-sm)]"
            >
              <div v-if="rp.image" class="h-14 w-14 flex-none overflow-hidden rounded-[var(--r-xs)]">
                <img :src="getImageUrl(rp.image)" :alt="getLocalizedText(rp.title)" loading="lazy" class="h-full w-full object-cover" />
              </div>
              <div class="min-w-0">
                <div class="truncate text-[14px] font-medium">{{ getLocalizedText(rp.title) }}</div>
                <div class="mt-1 text-[13px] font-semibold tabular-nums text-[color:var(--accent-strong)]">{{ formatPrice(rp.price_amount) }}</div>
              </div>
            </RouterLink>
          </div>
        </section>

        <footer class="mt-8 flex justify-center border-t border-border pt-6">
          <Button as-child variant="outline" size="sm" class="rounded-full border-[color:var(--border-strong)]">
            <RouterLink :to="backLink"><ArrowLeft class="mr-1.5 h-4 w-4" />{{ backText }}</RouterLink>
          </Button>
        </footer>
      </div>
    </article>

    <!-- 不存在 -->
    <div v-else class="my-10 flex flex-col items-center gap-3 rounded-[var(--r-lg)] border border-dashed border-border py-16 text-center">
      <AlertCircle class="h-11 w-11 text-muted-foreground" />
      <p class="text-[14px] text-muted-foreground">{{ t('blogDetail.notFound') }}</p>
      <Button as-child size="sm" class="mt-2 rounded-full">
        <RouterLink to="/blog">{{ t('blogDetail.backToBlog') }}</RouterLink>
      </Button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { AlertCircle, ArrowLeft, ChevronRight } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { getImageUrl } from '../../utils/image'
import { processHtmlForDisplay } from '../../utils/content'
import { useBlogDetail } from '../../composables/useBlogDetail'

const { t } = useI18n()

const {
  loading, post, relatedProducts, getLocalizedText, formatDate, formatPrice, backLink, backText,
} = useBlogDetail()
</script>

<style scoped>
.ember-prose :deep(img) {
  border-radius: var(--r);
  margin-block: 18px;
}
.ember-prose :deep(a) {
  color: var(--accent);
}
.ember-prose :deep(h2),
.ember-prose :deep(h3) {
  margin-block: 22px 10px;
  font-weight: 600;
  letter-spacing: -0.02em;
  color: var(--ink);
}
.ember-prose :deep(p) {
  margin-block: 10px;
}
.ember-prose :deep(ul),
.ember-prose :deep(ol) {
  margin-block: 10px;
  padding-left: 22px;
  list-style: revert;
}
.ember-prose :deep(blockquote) {
  margin-block: 14px;
  padding-left: 16px;
  border-left: 3px solid var(--border-strong);
  color: var(--ink-3);
}
</style>
