<template>
  <div
    class="blog-page min-h-screen bg-background text-foreground pt-20 pb-16 relative overflow-hidden">
    <div class="container mx-auto px-4 relative z-10">
      <!-- Page Header -->
      <div class="mb-16 mt-12 text-center">
        <h1 class="text-4xl md:text-6xl font-black mb-6 tracking-tight text-foreground">{{ t('nav.blog') }}</h1>
        <p
          class="text-muted-foreground max-w-2xl mx-auto text-lg leading-relaxed border-b pb-8">
          {{ t('blog.subtitle') }}
        </p>
      </div>

      <!-- Search Box -->
      <div class="mb-12 max-w-xl mx-auto">
        <div
          class="relative bg-card backdrop-blur-md border rounded-2xl flex items-center px-5 py-3 transition-all focus-within:shadow-lg focus-within:border-primary">
          <Search class="w-5 h-5 text-muted-foreground shrink-0" />
          <input v-model="searchKeyword" type="text" :placeholder="t('blog.searchPlaceholder')"
            class="flex-1 bg-transparent border-none outline-none px-3 text-foreground placeholder:text-muted-foreground" />
          <Button v-if="searchKeyword" type="button" variant="ghost" size="sm" @click="searchKeyword = ''">
            {{ t('blog.searchClear') }}
          </Button>
        </div>
      </div>

      <!-- Category Filter -->
      <div v-if="categoryFilterEnabled && categories.length"
        class="mb-12 flex flex-wrap items-center justify-center gap-2">
        <Button size="sm" class="rounded-full" :variant="hasCategoryFilter ? 'outline' : 'default'"
          @click="clearCategory()">
          {{ t('blog.allCategories') }}
        </Button>
        <Button v-for="category in categories" :key="category.id" size="sm" class="rounded-full"
          :variant="selectedCategory === category.slug ? 'default' : 'outline'"
          @click="selectCategory(category.slug)">
          {{ getLocalizedText(category.name) }}
        </Button>
      </div>

      <!-- Loading State -->
      <div v-if="loading" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-8">
        <div v-for="i in 6" :key="i"
          class="bg-muted rounded-2xl h-[300px] animate-pulse border">
        </div>
      </div>

      <!-- Posts Grid -->
      <div v-else-if="posts.length > 0">
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-8">
          <article v-for="post in posts" :key="post.id"
            class="group bg-card backdrop-blur-xl border rounded-2xl overflow-hidden shadow-sm hover:bg-muted/50 transition-all duration-300 hover:-translate-y-1 hover:shadow-xl cursor-pointer flex flex-col"
            @click="goToPost(post.slug)">
            <!-- Thumbnail -->
            <div v-if="post.thumbnail" class="h-48 overflow-hidden relative">
              <img :src="getImageUrl(post.thumbnail)" :alt="getLocalizedText(post.title)"
                loading="lazy" class="w-full h-full object-cover transition-transform duration-500 group-hover:scale-110">
              <div class="absolute inset-0 bg-black/35"></div>
            </div>

            <div class="p-8 flex flex-col flex-1">
              <div class="flex items-center justify-between mb-6">
                <Badge :variant="post.type === 'blog' ? 'accent' : 'info'" class="uppercase tracking-wider">
                  {{ post.type === 'blog' ? t('nav.blog') : t('nav.notice') }}
                </Badge>
                <time class="text-xs text-muted-foreground font-mono">
                  {{ formatDate(post.published_at) }}
                </time>
              </div>

              <h2
                class="text-2xl font-bold mb-4 text-foreground transition-colors line-clamp-2 leading-tight">
                {{ getLocalizedText(post.title) }}
              </h2>

              <p class="text-muted-foreground text-sm mb-8 line-clamp-3 leading-relaxed flex-1">
                {{ getLocalizedText(post.summary) }}
              </p>

              <div
                class="flex items-center text-sm font-medium text-muted-foreground group-hover:text-foreground transition-colors mt-auto pt-6 border-t">
                {{ t('blog.readMore') }}
                <ArrowRight class="w-4 h-4 ml-2 transition-transform group-hover:translate-x-2" />
              </div>
            </div>
          </article>
        </div>

        <!-- Pagination -->
        <PaginationNav
          :current-page="currentPage"
          :total-pages="totalPages"
          @change-page="changePage"
        />
      </div>

      <!-- Empty State -->
      <EmptyState v-else variant="soft" size="lg" :title="emptyStateTitle">
        <template #icon>
          <BookOpen class="w-20 h-20 text-muted-foreground opacity-70" :stroke-width="1.5" />
        </template>
      </EmptyState>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ArrowRight, BookOpen, Search } from 'lucide-vue-next'
import { getImageUrl } from '../utils/image'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import EmptyState from '../components/EmptyState.vue'
import PaginationNav from '../components/PaginationNav.vue'
import { usePostList } from '../composables/usePostList'

const { t } = useI18n()
const route = useRoute()

const {
  loading, posts, currentPage, totalPages, searchKeyword,
  categories, selectedCategory, categoryFilterEnabled, hasCategoryFilter,
  selectCategory, clearCategory,
  getLocalizedText, formatDate, goToPost, changePage,
} = usePostList(
  'blog',
  // 分类页与主页共用同一组件，canonical 跟随实际路径，避免分类页指向列表页。
  { title: () => t('nav.blog'), canonicalPath: () => route.path },
  { categoryFilter: true },
)

const emptyStateTitle = computed(() => {
  if (searchKeyword.value.trim()) return t('blog.noResults')
  if (hasCategoryFilter.value) return t('blog.noCategoryResults')
  return t('blog.empty')
})
</script>

<style scoped>
.line-clamp-2 {
  overflow: hidden;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  line-clamp: 2;
}

.line-clamp-3 {
  overflow: hidden;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
  line-clamp: 3;
}
</style>
