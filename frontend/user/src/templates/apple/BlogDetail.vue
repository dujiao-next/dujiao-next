<template>
  <div class="bd">
    <!-- 苹果式悬浮胶囊：文章标题 + 探索 / 购买（介绍页专用） -->
    <div v-if="post" class="bd-nav-wrap">
      <div class="bd-nav">
        <span class="bd-nav-title">{{ getLocalizedText(post.title) }}</span>
        <div class="bd-nav-actions">
          <button type="button" class="bd-nav-btn" @click="scrollToContent">
            {{ t('productDetail.explore') }}
          </button>
          <RouterLink class="bd-nav-btn bd-nav-btn--primary" :to="buyLink">
            {{ t('productDetail.buyNow') }}
          </RouterLink>
        </div>
      </div>
    </div>

  <!-- 后台用积木编辑器拼的页面：直接按区块渲染 -->
  <BlockRenderer v-if="blockContent" :blocks="blockContent.blocks" />

  <div v-else class="mx-auto w-full max-w-[860px] px-6 pb-10 pt-2">
    <!-- Loading -->
    <div v-if="loading" class="grid gap-4 pt-6">
      <div class="h-6 w-2/5 rounded-md border bg-card"></div>
      <div class="h-11 w-3/4 rounded-md border bg-card"></div>
      <div class="h-80 rounded-xl border bg-card"></div>
    </div>

    <!-- Post -->
    <article v-else-if="post">
      <nav class="flex flex-wrap items-center gap-1.5 py-[18px] text-sm font-semibold text-muted-foreground">
        <RouterLink to="/" class="hover:text-primary">{{ t('nav.home') }}</RouterLink>
        <ChevronRight class="h-4 w-4 flex-none" />
        <RouterLink :to="backLink" class="hover:text-primary">{{ backText }}</RouterLink>
        <ChevronRight class="h-4 w-4 flex-none" />
        <span class="max-w-[240px] truncate text-foreground">{{ getLocalizedText(post.title) }}</span>
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
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { computed } from 'vue'
import { ArrowLeft, AlertCircle, ChevronRight } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { getImageUrl } from '../../utils/image'
import { processHtmlForDisplay } from '../../utils/content'
import { useBlogDetail } from '../../composables/useBlogDetail'
import BlockRenderer from '../../components/blocks/BlockRenderer.vue'

const { t } = useI18n()

const buyLink = computed(() => {
  const first = (relatedProducts.value || [])[0]
  return first ? `/products/${first.slug}` : '/products'
})

const scrollToContent = () => {
  const second = document.querySelectorAll('.blk-reveal')[1]
  if (second) {
    second.scrollIntoView({ behavior: 'smooth', block: 'start' })
    return
  }
  const article = document.querySelector('.bd article') || document.querySelector('.bd section')
  if (article) {
    article.scrollIntoView({ behavior: 'smooth', block: 'start' })
    return
  }
  window.scrollBy({ top: window.innerHeight * 0.9, behavior: 'smooth' })
}

const {
  loading, post, relatedProducts, getLocalizedText, formatDate, formatPrice, backLink, backText,
} = useBlogDetail()

/** 后台用积木编辑器保存的区块内容（posts.content_blocks） */
const blockContent = computed(() => {
  const value = post.value as any
  if (!value || value.layout !== 'blocks') return null
  const raw = value.content_blocks
  const blocks = raw && typeof raw === 'object' ? raw.blocks : null
  return Array.isArray(blocks) && blocks.length ? { blocks } : null
})
</script>

<style scoped>
/* 苹果式悬浮胶囊：文章标题 + 探索 / 购买 */
.bd-nav-wrap {
  position: sticky;
  top: 56px;
  z-index: 30;
  padding: 10px 22px;
  pointer-events: none;
}
@media (min-width: 1068px) {
  .bd-nav-wrap {
    max-width: 1040px;
    margin-inline: auto;
  }
}
.bd-nav {
  display: flex;
  align-items: center;
  gap: 16px;
  height: 56px;
  padding: 0 10px 0 24px;
  border-radius: 28px;
  background: var(--st-material);
  backdrop-filter: saturate(180%) blur(20px);
  -webkit-backdrop-filter: saturate(180%) blur(20px);
  border: 1px solid color-mix(in srgb, var(--st-text) 8%, transparent);
  box-shadow: 0 6px 24px rgba(0, 0, 0, 0.06);
  pointer-events: auto;
}
.bd-nav-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: var(--st-font-display);
  font-size: 19px;
  font-weight: 600;
  color: var(--st-text);
}
.bd-nav-actions {
  display: flex;
  gap: 8px;
  flex: none;
}
.bd-nav-btn {
  display: inline-flex;
  align-items: center;
  height: 36px;
  padding: 0 16px;
  border: 1px solid color-mix(in srgb, var(--st-text) 22%, transparent);
  border-radius: 980px;
  background: transparent;
  color: var(--st-text);
  font-size: 14px;
  text-decoration: none;
  cursor: pointer;
  transition: background 240ms var(--st-ease), border-color 240ms var(--st-ease);
}
.bd-nav-btn:hover {
  background: color-mix(in srgb, var(--st-text) 6%, transparent);
}
.bd-nav-btn--primary {
  border-color: var(--st-blue);
  background: var(--st-blue);
  color: #fff;
}
.bd-nav-btn--primary:hover {
  border-color: var(--st-blue-hover);
  background: var(--st-blue-hover);
}
@media (max-width: 734px) {
  .bd-nav-wrap {
    top: 52px;
    padding: 8px 12px;
  }
  .bd-nav {
    height: 48px;
    padding: 0 8px 0 16px;
    border-radius: 24px;
  }
  .bd-nav-title {
    font-size: 15px;
  }
  .bd-nav-btn {
    height: 32px;
    padding: 0 12px;
    font-size: 12px;
  }
}
@media (prefers-reduced-motion: reduce) {
  .bd-nav-btn {
    transition: none;
  }
}
</style>
