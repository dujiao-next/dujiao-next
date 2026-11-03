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
/* 数值照抄 apple.com.cn/iphone-duo 的 .localnav-wrap（实测）：
   1024×53、圆角 18、白 80% + blur(20px)、外圈 1px #e6e6e8；
   标题 19/600，按钮 28 高、12px、圆角 120、内边距 6×10。 */
.bd-nav-wrap {
  position: sticky;
  top: 44px;
  z-index: 30;
  padding: 6px 0;
  pointer-events: none;
}
.bd-nav {
  display: flex;
  align-items: center;
  width: min(1024px, calc(100vw - 20px));
  height: 53px;
  margin-inline: auto;
  padding: 0 20px;
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.8);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  box-shadow: 0 0 0 1px rgb(230, 230, 232);
  pointer-events: auto;
}
.bd-nav-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 19px;
  font-weight: 600;
  letter-spacing: 0.228px;
  line-height: 26px;
  color: #1d1d1f;
}
.bd-nav-actions {
  display: flex;
  gap: 10px;
  flex: none;
}
.bd-nav-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 28px;
  padding: 6px 10px;
  border: 0;
  border-radius: 120px;
  background: rgba(255, 255, 255, 0.8);
  color: #1d1d1f;
  font-size: 12px;
  font-weight: 400;
  line-height: 16px;
  text-decoration: none;
  cursor: pointer;
}
.bd-nav-btn--primary {
  background: #0071e3;
  color: #fff;
}
.bd-nav-btn--primary:hover {
  background: #0077ed;
}
@media (max-width: 734px) {
  .bd-nav-wrap {
    top: 48px;
  }
  .bd-nav {
    width: calc(100vw - 20px);
    padding: 0 12px;
  }
}
</style>
