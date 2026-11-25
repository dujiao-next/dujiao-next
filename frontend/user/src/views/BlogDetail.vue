<template>
  <div class="min-h-screen bg-background pb-16 pt-24 text-foreground">
    <div class="container mx-auto max-w-3xl px-4">
      <div v-if="loading" class="space-y-5">
        <div class="theme-skeleton h-5 w-1/3 rounded"></div>
        <div class="theme-skeleton h-96 rounded-[22px]"></div>
      </div>

      <article v-else-if="post">
        <nav class="mb-4 flex items-center gap-2 text-sm text-muted-foreground">
          <router-link to="/" class="transition-colors hover:text-foreground">{{ t('nav.home') }}</router-link>
          <span aria-hidden="true">/</span>
          <router-link :to="backLink" class="transition-colors hover:text-foreground">{{ post.type === 'blog' ? t('nav.blog') : t('nav.notice') }}</router-link>
          <span aria-hidden="true">/</span>
          <span class="min-w-0 truncate text-foreground">{{ getLocalizedText(post.title) }}</span>
        </nav>

        <div class="theme-float rounded-[22px] bg-card p-5 md:rounded-[28px] md:p-10">
          <header>
            <p class="flex items-center gap-2 text-[13px] text-muted-foreground">
              <span class="rounded-full bg-secondary px-2.5 py-0.5 font-medium text-foreground">{{ post.type === 'blog' ? t('nav.blog') : t('nav.notice') }}</span>
              <time class="tabular-nums">{{ formatDate(post.published_at) }}</time>
            </p>
            <h1 class="post-title mt-4 text-[28px] font-bold md:text-[40px]">{{ getLocalizedText(post.title) }}</h1>
            <p v-if="post.summary" class="mt-4 text-[17px] leading-relaxed text-muted-foreground">{{ getLocalizedText(post.summary) }}</p>
          </header>

          <img
            v-if="post.thumbnail"
            :src="getImageUrl(post.thumbnail)"
            :alt="getLocalizedText(post.title)"
            class="mt-8 max-h-96 w-full rounded-2xl object-cover"
          />

          <div
            v-html="processHtmlForDisplay(getLocalizedText(post.content))"
            class="prose max-w-none theme-prose mt-8 border-t pt-8 dark:prose-invert md:prose-lg"
          ></div>

          <section v-if="relatedProducts.length" class="mt-10 border-t pt-8">
            <h2 class="text-[17px] font-bold">{{ t('blog.relatedProducts') }}</h2>
            <div class="mt-4 grid grid-cols-1 gap-2 sm:grid-cols-2">
              <router-link
                v-for="rp in relatedProducts"
                :key="rp.id"
                :to="`/products/${rp.slug}`"
                class="flex items-center gap-3 rounded-2xl bg-secondary/60 p-2 pr-4 transition-colors hover:bg-secondary focus-visible:outline-2 focus-visible:outline-primary"
              >
                <div v-if="rp.image" class="h-14 w-14 shrink-0 overflow-hidden rounded-xl bg-card">
                  <img :src="getImageUrl(rp.image)" alt="" loading="lazy" class="h-full w-full object-cover" />
                </div>
                <div class="min-w-0 flex-1" :class="!rp.image && 'pl-2'">
                  <div class="truncate text-[15px] font-semibold">{{ getLocalizedText(rp.title) }}</div>
                  <div class="mt-0.5 text-sm font-bold tabular-nums">{{ formatPrice(rp.price_amount) }}</div>
                </div>
                <ChevronRight class="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
              </router-link>
            </div>
          </section>

          <footer class="mt-10 border-t pt-6">
            <router-link :to="backLink" class="theme-pill-link pl-2.5 pr-3.5">
              <ChevronLeft class="h-3.5 w-3.5" aria-hidden="true" />{{ backText }}
            </router-link>
          </footer>
        </div>
      </article>

      <div v-else class="theme-float rounded-[22px] bg-card px-6 py-20 text-center">
        <AlertCircle class="mx-auto mb-5 h-14 w-14 text-muted-foreground" :stroke-width="1.5" />
        <p class="mb-6 text-lg text-muted-foreground">{{ t('blogDetail.notFound') }}</p>
        <Button as-child class="h-11 rounded-full bg-foreground px-6 font-semibold text-background hover:bg-foreground/85">
          <router-link to="/blog">{{ t('blogDetail.backToBlog') }}</router-link>
        </Button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { AlertCircle, ChevronLeft, ChevronRight } from 'lucide-vue-next'
import { getImageUrl } from '../utils/image'
import { processHtmlForDisplay } from '../utils/content'
import { Button } from '@/components/ui/button'
import { useBlogDetail } from '../composables/useBlogDetail'

const { t } = useI18n()

const {
  loading, post, relatedProducts, getLocalizedText, formatDate, formatPrice, backLink, backText,
} = useBlogDetail()
</script>

<style scoped>
.post-title {
  line-height: 1.15;
  letter-spacing: -0.02em;
  text-wrap: balance;
}
</style>
