<template>
  <div
    class="blog-detail-page min-h-screen bg-background text-foreground pt-24 pb-16 relative overflow-hidden">
    <div class="container mx-auto px-4 max-w-4xl relative z-10">
      <!-- Loading State -->
      <div v-if="loading" class="animate-pulse space-y-8">
        <div class="h-8 bg-muted rounded w-1/3"></div>
        <div class="space-y-4">
          <div class="h-12 bg-muted rounded w-3/4"></div>
          <div class="h-6 bg-muted rounded w-1/2"></div>
        </div>
        <div class="h-96 bg-muted rounded-3xl"></div>
      </div>

      <!-- Post Content -->
      <article v-else-if="post">
        <!-- Breadcrumb：窄屏下标题独占一行（w-full 触发换行），宽屏才与层级同行截断，
             避免长标题与面包屑挤在一行时溢出。分隔用 gap 而非 space-x —— 换行后
             space-x 会给行首元素也留 margin，导致缩进错位。 -->
        <nav class="mb-8 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-muted-foreground font-medium">
          <router-link to="/" class="text-muted-foreground transition-colors hover:text-foreground">{{ t('nav.home')
          }}</router-link>
          <span>/</span>
          <router-link :to="backLink" class="text-muted-foreground transition-colors hover:text-foreground">{{ backLabel
          }}</router-link>
          <template v-if="categoryLink">
            <span>/</span>
            <router-link :to="categoryLink"
              class="text-muted-foreground transition-colors hover:text-foreground md:max-w-[160px] md:truncate">
              {{ categoryName }}
            </router-link>
          </template>
          <span class="hidden md:inline">/</span>
          <span class="w-full text-foreground md:w-auto md:max-w-[200px] md:truncate">{{ getLocalizedText(post.title)
          }}</span>
        </nav>

        <Card
          class="backdrop-blur-xl rounded-3xl p-8 md:p-12 shadow-2xl relative overflow-hidden">
          <!-- Featured Image -->
          <div v-if="post.thumbnail" class="mb-12 relative h-64 md:h-96 rounded-2xl overflow-hidden group">
            <img :src="getImageUrl(post.thumbnail)" :alt="getLocalizedText(post.title)"
              loading="lazy" class="w-full h-full object-cover">
            <div class="absolute inset-0 bg-black/20 dark:bg-black/35"></div>
          </div>

          <!-- Post Header -->
          <header class="mb-12 border-b pb-12">
            <div class="flex flex-wrap items-center gap-4 mb-6">
              <Badge :variant="post.type === 'blog' ? 'accent' : 'info'" class="uppercase tracking-wider">
                {{ post.type === 'blog' ? t('nav.blog') : t('nav.notice') }}
              </Badge>
              <time class="text-sm text-muted-foreground font-mono">
                {{ formatDate(post.published_at) }}
              </time>
            </div>

            <h1 class="text-3xl md:text-5xl font-black text-foreground mb-6 leading-tight tracking-tight">
              {{ getLocalizedText(post.title) }}
            </h1>

            <p v-if="post.summary" class="text-xl text-muted-foreground leading-relaxed font-light">
              {{ getLocalizedText(post.summary) }}
            </p>
          </header>

          <!-- Post Content -->
          <div v-html="processHtmlForDisplay(getLocalizedText(post.content))"
            class="prose prose-lg max-w-none dark:prose-invert theme-prose">
          </div>

          <!-- Related Products -->
          <section v-if="relatedProducts.length"
            class="mt-16 pt-12 border-t">
            <h2 class="text-2xl font-bold text-foreground mb-8 flex items-center gap-3">
              <span class="w-1.5 h-7 bg-primary rounded-full"></span>
              {{ t('blog.relatedProducts') }}
            </h2>
            <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-5">
              <router-link
                v-for="rp in relatedProducts"
                :key="rp.id"
                :to="`/products/${rp.slug}`"
                class="group flex items-center gap-4 bg-card backdrop-blur-md border rounded-2xl p-4 hover:shadow-xl hover:-translate-y-0.5 transition-all"
              >
                <div v-if="rp.image"
                  class="h-16 w-16 shrink-0 overflow-hidden rounded-xl bg-muted">
                  <img :src="getImageUrl(rp.image)" :alt="getLocalizedText(rp.title)" loading="lazy"
                    class="h-full w-full object-cover transition-transform group-hover:scale-110" />
                </div>
                <div class="min-w-0 flex-1">
                  <div class="font-semibold text-foreground truncate">{{ getLocalizedText(rp.title) }}</div>
                  <div class="mt-1 font-mono text-sm text-muted-foreground">
                    {{ formatPrice(rp.price_amount) }}
                  </div>
                </div>
              </router-link>
            </div>
          </section>

          <!-- Footer -->
          <footer class="mt-16 pt-12 border-t flex justify-center">
            <Button variant="secondary" as-child class="rounded-full h-11 px-6">
              <router-link :to="backLink" class="group">
                <ArrowLeft class="transition-transform group-hover:-translate-x-1" />
                <span class="font-medium">{{ backText }}</span>
              </router-link>
            </Button>
          </footer>
        </Card>
      </article>

      <!-- Error State -->
      <Card v-else
        class="text-center py-24 rounded-3xl backdrop-blur-sm">
        <AlertCircle class="w-20 h-20 mx-auto text-muted-foreground mb-6" :stroke-width="1.5" />
        <p class="text-muted-foreground text-xl mb-8">
          {{ t('blogDetail.notFound') }}
        </p>
        <Button as-child class="rounded-full h-11 px-8">
          <router-link to="/blog">{{ t('blogDetail.backToBlog') }}</router-link>
        </Button>
      </Card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ArrowLeft, AlertCircle } from 'lucide-vue-next'
import { getImageUrl } from '../utils/image'
import { processHtmlForDisplay } from '../utils/content'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { useBlogDetail } from '../composables/useBlogDetail'

const { t } = useI18n()

const {
  loading, post, relatedProducts, getLocalizedText, formatDate, formatPrice, backLink, backText,
  backLabel, categoryName, categoryLink,
} = useBlogDetail()
</script>
