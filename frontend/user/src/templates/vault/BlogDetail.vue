<template>
  <div class="mx-auto w-full max-w-[860px] px-6 pb-10 pt-2">
    <!-- Loading -->
    <div v-if="loading" class="grid gap-4 pt-6">
      <div class="h-6 w-2/5 rounded-md border bg-card"></div>
      <div class="h-11 w-3/4 rounded-md border bg-card"></div>
      <div class="h-80 rounded-xl border bg-card"></div>
    </div>

    <!-- Post -->
    <article v-else-if="post">
      <!-- 窄屏下标题独占一行（w-full 触发换行），宽屏才与层级同行截断 -->
      <nav class="flex flex-wrap items-center gap-x-1.5 gap-y-1 py-[18px] text-sm font-semibold text-muted-foreground">
        <RouterLink to="/" class="hover:text-primary">{{ t('nav.home') }}</RouterLink>
        <ChevronRight class="h-4 w-4 flex-none" />
        <RouterLink :to="backLink" class="hover:text-primary">{{ backLabel }}</RouterLink>
        <template v-if="categoryLink">
          <ChevronRight class="h-4 w-4 flex-none" />
          <RouterLink :to="categoryLink" class="hover:text-primary md:max-w-[200px] md:truncate">{{ categoryName }}</RouterLink>
        </template>
        <ChevronRight class="hidden h-4 w-4 flex-none md:block" />
        <span class="w-full text-foreground md:w-auto md:max-w-[240px] md:truncate">{{ getLocalizedText(post.title) }}</span>
      </nav>

      <Card class="p-[30px]">
        <div v-if="post.thumbnail" class="mb-7 h-80 overflow-hidden rounded-md">
          <img :src="getImageUrl(post.thumbnail)" :alt="getLocalizedText(post.title)" loading="lazy" class="h-full w-full object-cover" />
        </div>

        <header class="mb-6 border-b pb-6">
          <div class="mb-3.5 flex items-center gap-3">
            <Badge :variant="post.type === 'blog' ? 'info' : 'warning'" size="sm" class="rounded-full">{{ post.type === 'blog' ? t('nav.blog') : t('nav.notice') }}</Badge>
            <span class="text-[13px] font-semibold text-muted-foreground">{{ formatDate(post.published_at) }}</span>
          </div>
          <h1 class="text-[34px] font-extrabold leading-tight">{{ getLocalizedText(post.title) }}</h1>
          <p v-if="post.summary" class="mt-3.5 text-[17px] leading-relaxed text-muted-foreground">{{ getLocalizedText(post.summary) }}</p>
        </header>

        <div
          class="prose max-w-none dark:prose-invert prose-a:text-primary prose-img:rounded-md"
          v-html="processHtmlForDisplay(getLocalizedText(post.content))"
        ></div>

        <!-- 相关商品 -->
        <section v-if="relatedProducts.length" class="mt-9 border-t pt-7">
          <h2 class="mb-[18px] flex items-center gap-3 text-[21px] font-bold">
            <span class="h-6 w-[5px] flex-none rounded-full bg-primary"></span>{{ t('blog.relatedProducts') }}
          </h2>
          <div class="grid gap-3.5 grid-cols-[repeat(auto-fill,minmax(220px,1fr))]">
            <RouterLink
              v-for="rp in relatedProducts"
              :key="rp.id"
              :to="`/products/${rp.slug}`"
              class="flex items-center gap-3.5 rounded-md border bg-secondary p-3 transition hover:-translate-y-0.5 hover:shadow-[var(--shadow)]"
            >
              <div v-if="rp.image" class="h-14 w-14 flex-none overflow-hidden rounded-sm">
                <img :src="getImageUrl(rp.image)" :alt="getLocalizedText(rp.title)" loading="lazy" class="h-full w-full object-cover" />
              </div>
              <div class="min-w-0">
                <div class="truncate font-bold">{{ getLocalizedText(rp.title) }}</div>
                <div class="mt-1 text-[13px] font-semibold text-primary">{{ formatPrice(rp.price_amount) }}</div>
              </div>
            </RouterLink>
          </div>
        </section>

        <footer class="mt-8 flex justify-center border-t pt-6">
          <Button as-child variant="outline" size="sm" class="rounded-full">
            <RouterLink :to="backLink"><ArrowLeft /> {{ backText }}</RouterLink>
          </Button>
        </footer>
      </Card>
    </article>

    <!-- Error -->
    <div v-else class="my-10 flex flex-col items-center gap-3 rounded-xl border border-dashed py-16 text-center text-muted-foreground">
      <AlertCircle class="h-10 w-10 opacity-60" />
      <p>{{ t('blogDetail.notFound') }}</p>
      <Button as-child class="mt-2 rounded-full">
        <RouterLink to="/blog">{{ t('blogDetail.backToBlog') }}</RouterLink>
      </Button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ArrowLeft, AlertCircle, ChevronRight } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { getImageUrl } from '../../utils/image'
import { processHtmlForDisplay } from '../../utils/content'
import { useBlogDetail } from '../../composables/useBlogDetail'

const { t } = useI18n()

const {
  loading, post, relatedProducts, getLocalizedText, formatDate, formatPrice, backLink, backText,
  backLabel, categoryName, categoryLink,
} = useBlogDetail()
</script>
