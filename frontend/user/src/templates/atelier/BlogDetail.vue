<template>
  <div class="min-h-screen pb-16 pt-[88px] text-foreground">
    <div class="mx-auto w-full max-w-3xl px-4 sm:px-6">
      <div v-if="loading" class="space-y-5">
        <div class="theme-skeleton h-5 w-1/3 rounded"></div>
        <div class="theme-skeleton h-96 rounded-[16px]"></div>
      </div>

      <article v-else-if="post">
        <nav class="mb-5 flex items-center gap-2 text-[13px] text-muted-foreground">
          <RouterLink to="/" class="transition-colors hover:text-foreground">{{ t('nav.home') }}</RouterLink>
          <span aria-hidden="true">/</span>
          <RouterLink :to="backLink" class="transition-colors hover:text-foreground">
            {{ post.type === 'blog' ? t('nav.blog') : t('nav.notice') }}
          </RouterLink>
          <span aria-hidden="true">/</span>
          <span class="min-w-0 truncate font-medium text-foreground">{{ getLocalizedText(post.title) }}</span>
        </nav>

        <div class="atelier-card rounded-[16px] bg-card p-6 md:p-10">
          <header>
            <div class="flex items-center gap-2.5 text-xs text-muted-foreground">
              <span class="inline-flex h-5.5 items-center rounded-[6px] border bg-background px-2 text-[11px] font-semibold text-foreground/85">
                {{ post.type === 'blog' ? t('nav.blog') : t('nav.notice') }}
              </span>
              <time class="font-mono tabular-nums">{{ formatDate(post.published_at) }}</time>
            </div>
            <h1 class="at-post-title mt-4 text-[26px] font-bold md:text-[36px]">
              {{ getLocalizedText(post.title) }}
            </h1>
            <p v-if="post.summary" class="mt-3.5 text-base leading-relaxed text-muted-foreground">
              {{ getLocalizedText(post.summary) }}
            </p>
          </header>

          <img
            v-if="post.thumbnail"
            :src="getImageUrl(post.thumbnail)"
            :alt="getLocalizedText(post.title)"
            class="mt-7 max-h-96 w-full rounded-[16px] border object-cover"
          />

          <div
            v-html="processHtmlForDisplay(getLocalizedText(post.content))"
            class="prose max-w-none theme-prose mt-7 border-t pt-7 dark:prose-invert md:prose-lg"
          ></div>

          <section v-if="relatedProducts.length" class="mt-9 border-t pt-7">
            <h2 class="text-base font-bold tracking-[-0.015em]">{{ t('blog.relatedProducts') }}</h2>
            <div class="mt-3.5 grid grid-cols-1 gap-2.5 sm:grid-cols-2">
              <RouterLink
                v-for="rp in relatedProducts"
                :key="rp.id"
                :to="`/products/${rp.slug}`"
                class="group flex items-center gap-3 rounded-[14px] border bg-background p-2.5 pr-3.5 transition-colors hover:border-hairline-strong"
              >
                <div v-if="rp.image" class="h-12 w-12 shrink-0 overflow-hidden rounded-[10px] border bg-card">
                  <img :src="getImageUrl(rp.image)" alt="" loading="lazy" class="h-full w-full object-cover" />
                </div>
                <div class="min-w-0 flex-1" :class="!rp.image && 'pl-1'">
                  <div class="truncate text-sm font-semibold text-foreground">{{ getLocalizedText(rp.title) }}</div>
                  <div class="mt-0.5 text-[13px] font-bold tabular-nums text-foreground">{{ formatPrice(rp.price_amount) }}</div>
                </div>
                <ChevronRight class="h-4 w-4 shrink-0 text-muted-foreground transition-transform group-hover:translate-x-0.5 group-hover:text-foreground" aria-hidden="true" />
              </RouterLink>
            </div>
          </section>

          <footer class="mt-9 border-t pt-6">
            <RouterLink :to="backLink" class="theme-pill-link pl-2.5 pr-3.5">
              <ChevronLeft class="h-3.5 w-3.5" aria-hidden="true" />{{ backText }}
            </RouterLink>
          </footer>
        </div>
      </article>

      <div v-else class="atelier-card rounded-[16px] bg-card px-6 py-20 text-center">
        <AlertCircle class="mx-auto mb-4 h-12 w-12 text-muted-foreground" :stroke-width="1.5" />
        <p class="mb-6 text-base text-muted-foreground">{{ t('blogDetail.notFound') }}</p>
        <Button as-child class="h-11 rounded-[11px] bg-primary px-6 font-semibold text-primary-foreground shadow-xs hover:bg-primary/90">
          <RouterLink to="/blog">{{ t('blogDetail.backToBlog') }}</RouterLink>
        </Button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { AlertCircle, ChevronLeft, ChevronRight } from 'lucide-vue-next'
import { getImageUrl } from '../../utils/image'
import { processHtmlForDisplay } from '../../utils/content'
import { Button } from '@/components/ui/button'
import { useBlogDetail } from '../../composables/useBlogDetail'

const { t } = useI18n()

const {
  loading, post, relatedProducts, getLocalizedText, formatDate, formatPrice, backLink, backText,
} = useBlogDetail()
</script>

<style scoped>
.at-post-title {
  line-height: 1.16;
  letter-spacing: -0.025em;
  text-wrap: balance;
}
</style>
