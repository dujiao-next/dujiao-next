<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <h1 class="text-2xl font-semibold">{{ t('admin.posts.title') }}</h1>
      <Button size="sm" class="w-full gap-2 sm:w-auto" @click="openCreate">
        <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
        </svg>
        {{ t('admin.posts.create') }}
      </Button>
    </div>

    <Tabs v-model="currentTab" class="w-full">
      <TabsList>
        <TabsTrigger value="blog">{{ t('admin.posts.tabs.blog') }}</TabsTrigger>
        <TabsTrigger value="notice">{{ t('admin.posts.tabs.notice') }}</TabsTrigger>
        <TabsTrigger value="page">{{ t('admin.posts.tabs.page') }}</TabsTrigger>
      </TabsList>
    </Tabs>

    <div class="rounded-xl border border-border bg-card overflow-x-auto">
      <Table class="min-w-[980px]">
        <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
          <TableRow>
            <TableHead class="px-6 py-3">{{ t('admin.posts.table.id') }}</TableHead>
            <TableHead v-if="currentTab === 'blog'" class="w-32 px-6 py-3">{{ t('admin.posts.table.category') }}</TableHead>
            <TableHead class="min-w-[280px] px-6 py-3">{{ t('admin.posts.table.title') }}</TableHead>
            <TableHead class="min-w-[220px] px-6 py-3">{{ t('admin.posts.table.slug') }}</TableHead>
            <TableHead class="min-w-[120px] px-6 py-3">{{ t('admin.posts.table.status') }}</TableHead>
            <TableHead class="min-w-[180px] px-6 py-3">{{ t('admin.posts.table.createdAt') }}</TableHead>
            <TableHead class="min-w-[140px] px-6 py-3 text-right">{{ t('admin.posts.table.action') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody class="divide-y divide-border">
          <TableRow v-if="loading">
            <TableCell :colspan="currentTab === 'blog' ? 7 : 6" class="p-0">
              <TableSkeleton :columns="currentTab === 'blog' ? 7 : 6" :rows="5" />
            </TableCell>
          </TableRow>
          <TableRow v-else-if="posts.length === 0">
            <TableCell :colspan="currentTab === 'blog' ? 7 : 6" class="px-6 py-8 text-center text-muted-foreground">{{ t('admin.posts.empty') }}</TableCell>
          </TableRow>
          <TableRow v-for="post in posts" :key="post.id" class="cursor-pointer hover:bg-muted/30" @click="openEdit(post)">
            <TableCell class="px-6 py-4">
              <IdCell :value="post.id" />
            </TableCell>
            <TableCell v-if="currentTab === 'blog'" class="px-6 py-4">
              <span class="text-sm text-muted-foreground">{{ getCategoryName(post.category_id) }}</span>
            </TableCell>
            <TableCell class="min-w-[280px] px-6 py-4">
              <div class="flex items-center gap-4">
                <div
                  v-if="post.thumbnail"
                  class="h-12 w-12 shrink-0 overflow-hidden rounded-lg border border-border bg-muted/40"
                >
                  <img :src="getImageUrl(post.thumbnail)" class="h-full w-full object-cover" />
                </div>
                <div class="break-words font-medium text-foreground">{{ getLocalizedText(post.title) }}</div>
              </div>
            </TableCell>
            <TableCell class="min-w-[220px] px-6 py-4 font-mono text-muted-foreground break-all">{{ post.slug }}</TableCell>
            <TableCell class="min-w-[120px] px-6 py-4">
              <span
                class="inline-flex rounded-full border px-2.5 py-1 text-xs"
                :class="post.is_published ? 'text-emerald-700 border-emerald-200 bg-emerald-50' : 'text-amber-700 border-amber-200 bg-amber-50'"
              >
                {{ post.is_published ? t('admin.posts.status.published') : t('admin.posts.status.draft') }}
              </span>
            </TableCell>
            <TableCell class="min-w-[180px] px-6 py-4 text-xs text-muted-foreground">{{ formatDate(post.created_at) }}</TableCell>
            <TableCell class="min-w-[140px] px-6 py-4 text-right">
              <div class="flex items-center justify-end gap-2" @click.stop>
                <Button size="icon-sm" variant="outline" @click="openEdit(post)">
                  <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" />
                  </svg>
                </Button>
                <Button size="icon-sm" variant="destructive" @click="handleDelete(post)">
                  <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                  </svg>
                </Button>
              </div>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>

      <ListPagination
        :page="pagination.page"
        :total-page="pagination.total_page"
        :total="pagination.total"
        :page-size="pagination.page_size"
        :page-size-options="pageSizeOptions"
        @change-page="changePage"
        @change-page-size="changePageSize"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { adminAPI } from '@/api/admin'
import type { AdminPost, LocalizedText } from '@/api/types'
import IdCell from '@/components/IdCell.vue'
import { getImageUrl } from '@/utils/image'
import { formatDate, getLocalizedText } from '@/utils/format'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import TableSkeleton from '@/components/TableSkeleton.vue'
import ListPagination from '@/components/ListPagination.vue'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { notifyError } from '@/utils/notify'
import { cachePost } from '@/lib/postCache'
import { confirmAction } from '@/utils/confirm'

interface CategoryItem {
  id: number
  name: LocalizedText
}

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const loading = ref(false)
const currentTab = ref(
  route.params.type === 'notice' || route.params.type === 'page' ? String(route.params.type) : 'blog',
)
const posts = ref<AdminPost[]>([])
const categories = ref<CategoryItem[]>([])
const pagination = reactive({ page: 1, page_size: 10, total: 0, total_page: 0 })
const pageSizeOptions = [10, 20, 50, 100]

const fetchPosts = async () => {
  loading.value = true
  try {
    const res = await adminAPI.getPosts({
      page: pagination.page,
      page_size: pagination.page_size,
      type: currentTab.value,
    })
    posts.value = res.data.data || []
    if (res.data.pagination) Object.assign(pagination, res.data.pagination)
  } catch {
    posts.value = []
  } finally {
    loading.value = false
  }
}

const fetchCategories = async () => {
  try {
    const res = await adminAPI.getPostCategories()
    categories.value = ((res.data?.data || []) as any[]).map((c) => ({ id: c.id, name: c.name }))
  } catch {
    categories.value = []
  }
}

const getCategoryName = (categoryId?: number | null) => {
  if (!categoryId) return '-'
  const cat = categories.value.find((c) => c.id === categoryId)
  return cat ? getLocalizedText(cat.name) : '-'
}

const changePage = (page: number) => {
  pagination.page = page
  fetchPosts()
}

const changePageSize = (size: number) => {
  if (size === pagination.page_size) return
  pagination.page_size = size
  pagination.page = 1
  fetchPosts()
}

const openCreate = () => {
  router.push(`/posts/${currentTab.value}/new`)
}

const openEdit = (post: AdminPost) => {
  // 把整篇数据放进内存缓存，编辑器直接取，不用再发请求
  cachePost(post)
  router.push(`/posts/${post.type || currentTab.value}/${post.id}`)
}

const handleDelete = async (post: AdminPost) => {
  const confirmed = await confirmAction({
    description: t('admin.posts.confirmDelete', { name: getLocalizedText(post.title) }),
    confirmText: t('admin.common.delete'),
    variant: 'destructive',
  })
  if (!confirmed) return
  try {
    await adminAPI.deletePost(post.id)
    fetchPosts()
  } catch (err) {
    notifyError(t('admin.posts.errors.deleteFailed', { message: (err as Error).message || '' }))
  }
}

watch(currentTab, (tab) => {
  pagination.page = 1
  fetchPosts()
  if (route.params.type !== tab) router.replace(`/posts/${tab}`)
})

watch(
  () => route.params.type,
  (type) => {
    if ((type === 'notice' || type === 'blog' || type === 'page') && type !== currentTab.value) {
      currentTab.value = String(type)
    }
  },
)

onMounted(() => {
  fetchCategories()
  fetchPosts()
  // 兼容旧链接：/posts/blog?post_id=12 → 直接进整页编辑器
  if (route.query.post_id) {
    router.replace(`/posts/${currentTab.value}/${route.query.post_id}`)
  }
})
</script>
