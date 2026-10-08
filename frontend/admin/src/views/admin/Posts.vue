<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { adminAPI } from '@/api/admin'
import type { AdminPost, AdminProduct, LocalizedText } from '@/api/types'
import MediaPicker from '@/components/admin/MediaPicker.vue'
import RichEditor from '@/components/RichEditor.vue'
import IdCell from '@/components/IdCell.vue'
import { getImageUrl } from '@/utils/image'
import { formatDate, getLocalizedText } from '@/utils/format'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { Dialog, DialogDescription, DialogHeader, DialogScrollContent, DialogTitle } from '@/components/ui/dialog'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import TableSkeleton from '@/components/TableSkeleton.vue'
import ListPagination from '@/components/ListPagination.vue'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { notifyError, notifySuccess } from '@/utils/notify'
import { confirmAction } from '@/utils/confirm'
import { useFormValidation, rules } from '@/composables/useFormValidation'

interface RelatedProductRef {
  id: number
  slug: string
  title: LocalizedText
  image?: string
}

interface CategoryItem {
  id: number
  name: LocalizedText
  depth: number
  selectable: boolean
}

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const loading = ref(false)
const showModal = ref(false)
const isEditing = ref(false)
const currentTab = ref(route.params.type === 'notice' ? 'notice' : 'blog')
// 列表筛选：公告没有分类维度，分类筛选仅在博客文章下生效
const categoryFilter = ref('all')
const statusFilter = ref('all')
const currentLang = ref('zh-CN')
const submitting = ref(false)

watch(currentTab, (tab) => {
  pagination.page = 1
  if (tab === 'notice') categoryFilter.value = 'all'
  fetchPosts()
  if (route.params.type !== tab) {
    router.replace(`/posts/${tab}`)
  }
})

const languages = computed(() => [
  { code: 'zh-CN', name: t('admin.common.lang.zhCN') },
  { code: 'zh-TW', name: t('admin.common.lang.zhTW') },
  { code: 'en-US', name: t('admin.common.lang.enUS') },
])

const posts = ref<AdminPost[]>([])
const categories = ref<CategoryItem[]>([])
const pagination = reactive({
  page: 1,
  page_size: 10,
  total: 0,
  total_page: 0,
})

// --- Batch ---
const selectedIds = ref<Set<number>>(new Set())
const batchOperating = ref(false)
const batchCategoryId = ref('')

const allSelected = computed(() => posts.value.length > 0 && posts.value.every((p) => selectedIds.value.has(p.id)))

const toggleSelectAll = () => {
  selectedIds.value = allSelected.value ? new Set() : new Set(posts.value.map((p) => p.id))
}

const toggleSelect = (id: number) => {
  const next = new Set(selectedIds.value)
  next.has(id) ? next.delete(id) : next.add(id)
  selectedIds.value = next
}
const form = reactive({
  id: 0,
  title: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' } as Record<string, string>,
  slug: '',
  summary: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' } as Record<string, string>,
  content: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' } as Record<string, string>,
  type: 'blog',
  thumbnail: '',
  is_published: true,
  category_id: null as number | null,
  product_ids: [] as number[],
})

const relatedProducts = ref<RelatedProductRef[]>([])
const productSearch = ref('')
const productSearchResults = ref<RelatedProductRef[]>([])
const productSearchLoading = ref(false)
let productSearchTimer: ReturnType<typeof setTimeout> | null = null

const resetRelatedProductsState = () => {
  relatedProducts.value = []
  productSearch.value = ''
  productSearchResults.value = []
  productSearchLoading.value = false
}

const fetchPostRelatedProducts = async (postId: number) => {
  try {
    const res = await adminAPI.getPostRelatedProducts(postId)
    relatedProducts.value = (res.data.data || []) as RelatedProductRef[]
    form.product_ids = relatedProducts.value.map((p) => p.id)
  } catch {
    relatedProducts.value = []
    form.product_ids = []
  }
}

const runProductSearch = async () => {
  const keyword = productSearch.value.trim()
  if (!keyword) {
    productSearchResults.value = []
    productSearchLoading.value = false
    return
  }
  productSearchLoading.value = true
  try {
    const res = await adminAPI.getProducts({ search: keyword, page: 1, page_size: 20 })
    const list = (res.data.data || []) as AdminProduct[]
    productSearchResults.value = list.map((p) => ({
      id: p.id,
      slug: p.slug,
      title: p.title,
      image: p.images?.[0],
    }))
  } catch {
    productSearchResults.value = []
  } finally {
    productSearchLoading.value = false
  }
}

watch(productSearch, () => {
  if (productSearchTimer) clearTimeout(productSearchTimer)
  productSearchTimer = setTimeout(runProductSearch, 300)
})

const addRelatedProduct = (product: RelatedProductRef) => {
  if (form.product_ids.includes(product.id)) return
  relatedProducts.value.push(product)
  form.product_ids.push(product.id)
}

const removeRelatedProduct = (id: number) => {
  relatedProducts.value = relatedProducts.value.filter((p) => p.id !== id)
  form.product_ids = form.product_ids.filter((pid) => pid !== id)
}

const { errors, validate, clearErrors } = useFormValidation({
  slug: [rules.required('This field is required')],
  type: [rules.required('This field is required')],
  title: [rules.required('This field is required')],
})

const getCurrentLangName = () => {
  return languages.value.find((item) => item.code === currentLang.value)?.name || t('admin.common.lang.zhCN')
}

const fetchPosts = async () => {
  loading.value = true
  try {
    const res = await adminAPI.getPosts({
      page: pagination.page,
      page_size: pagination.page_size,
      type: currentTab.value,
      category_id: categoryFilter.value === 'all' ? undefined : categoryFilter.value,
      is_published: statusQueryValue(),
    })
    posts.value = res.data.data || []
    if (res.data.pagination) {
      Object.assign(pagination, res.data.pagination)
    }
  } catch {
    posts.value = []
  } finally {
    loading.value = false
  }
}

const buildCategoryOptions = (flat: any[]): CategoryItem[] => {
  const ids = new Set<number>(flat.map((c: any) => c.id))
  const idMap = new Map(flat.map((c: any, i: number) => [c.id, i]))
  const childrenMap = new Map<number | null, any[]>()
  for (const c of flat) {
    const pid = c.parent_id && ids.has(c.parent_id) ? c.parent_id : null
    const list = childrenMap.get(pid) || []
    list.push(c)
    childrenMap.set(pid, list)
  }
  for (const list of childrenMap.values()) {
    list.sort((a: any, b: any) => (idMap.get(a.id) ?? 0) - (idMap.get(b.id) ?? 0))
  }
  const result: CategoryItem[] = []
  const visited = new Set<number>()
  const walk = (cat: any, depth: number) => {
    if (visited.has(cat.id)) return
    visited.add(cat.id)
    const childCount = (childrenMap.get(cat.id) || []).length
    result.push({
      id: cat.id,
      name: cat.name,
      depth: Math.min(depth, 1),
      selectable: childCount === 0 && cat.is_active,
    })
    for (const child of childrenMap.get(cat.id) || []) {
      walk(child, depth + 1)
    }
  }
  const roots = (childrenMap.get(null) || []).sort((a: any, b: any) => (idMap.get(a.id) ?? 0) - (idMap.get(b.id) ?? 0))
  for (const root of roots) walk(root, 0)
  for (const c of flat) {
    if (!visited.has(c.id)) walk(c, 0)
  }
  return result
}

const fetchCategories = async () => {
  try {
    const res = await adminAPI.getPostCategories()
    const flat: any[] = (res.data?.data || [])
    categories.value = buildCategoryOptions(flat)
  } catch { categories.value = [] }
}

const getCategoryName = (categoryId?: number | null) => {
  if (!categoryId) return '-'
  const cat = categories.value.find(c => c.id === categoryId)
  return cat ? getLocalizedText(cat.name) : '-'
}

const getCategoryPath = (categoryId: number) => {
  const cat = categories.value.find(c => c.id === categoryId)
  return cat ? getLocalizedText(cat.name) : ''
}

// 发布状态三态：'all' 不下发参数，'published' → is_published=1，'draft' → is_published=0
const statusQueryValue = () => {
  if (statusFilter.value === 'published') return '1'
  if (statusFilter.value === 'draft') return '0'
  return undefined
}

const handleFilterChange = () => {
  pagination.page = 1
  fetchPosts()
}

const resetFilters = () => {
  categoryFilter.value = 'all'
  statusFilter.value = 'all'
  handleFilterChange()
}

const handleBatchStatus = async (isPublished: boolean) => {
  const ids = Array.from(selectedIds.value)
  if (!ids.length) return
  batchOperating.value = true
  try {
    const res = await adminAPI.batchUpdatePostStatus(ids, isPublished)
    const data = res.data.data as { success_count?: number } | null
    notifySuccess(t('admin.posts.batch.statusResult', { success: data?.success_count || 0, total: ids.length }))
    selectedIds.value = new Set()
    fetchPosts()
  } catch (err: any) { notifyError(err?.response?.data?.message || err?.message) } finally { batchOperating.value = false }
}

const handleBatchCategory = async () => {
  const ids = Array.from(selectedIds.value)
  if (!ids.length || !batchCategoryId.value) return
  batchOperating.value = true
  try {
    const res = await adminAPI.batchUpdatePostCategory(ids, Number(batchCategoryId.value))
    const data = res.data.data as { success_count?: number } | null
    notifySuccess(t('admin.posts.batch.categoryResult', { success: data?.success_count || 0, total: ids.length }))
    selectedIds.value = new Set()
    batchCategoryId.value = ''
    fetchPosts()
  } catch (err: any) { notifyError(err?.response?.data?.message || err?.message) } finally { batchOperating.value = false }
}

const handleBatchDelete = async () => {
  const ids = Array.from(selectedIds.value)
  if (!ids.length) return
  const confirmed = await confirmAction({ description: t('admin.posts.batch.deleteConfirm', { count: ids.length }), confirmText: t('admin.common.delete'), variant: 'destructive' })
  if (!confirmed) return
  batchOperating.value = true
  try {
    const res = await adminAPI.batchDeletePosts(ids)
    const data = res.data.data as { success_count?: number } | null
    notifySuccess(t('admin.posts.batch.deleteResult', { success: data?.success_count || 0, total: ids.length }))
    selectedIds.value = new Set()
    fetchPosts()
  } catch (err: any) { notifyError(err?.response?.data?.message || err?.message) } finally { batchOperating.value = false }
}

const changePage = (page: number) => {
  pagination.page = page
  fetchPosts()
}

const pageSizeOptions = [10, 20, 50, 100]

const changePageSize = (size: number) => {
  if (size === pagination.page_size) return
  pagination.page_size = size
  pagination.page = 1
  fetchPosts()
}

const openCreateModal = () => {
  isEditing.value = false
  currentLang.value = 'zh-CN'
  clearErrors()
  resetRelatedProductsState()
  Object.assign(form, {
    id: 0,
    title: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' },
    slug: '',
    summary: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' },
    content: { 'zh-CN': '', 'zh-TW': '', 'en-US': '' },
    type: currentTab.value,
    thumbnail: '',
    is_published: true,
    category_id: null,
    product_ids: [],
  })
  showModal.value = true
}

const openEditModal = (post: AdminPost) => {
  isEditing.value = true
  currentLang.value = 'zh-CN'
  resetRelatedProductsState()
  Object.assign(form, {
    id: post.id,
    title: post.title || { 'zh-CN': '', 'zh-TW': '', 'en-US': '' },
    slug: post.slug,
    summary: post.summary || { 'zh-CN': '', 'zh-TW': '', 'en-US': '' },
    content: post.content || { 'zh-CN': '', 'zh-TW': '', 'en-US': '' },
    type: post.type,
    thumbnail: post.thumbnail,
    is_published: post.is_published,
    category_id: post.category_id ?? null,
    product_ids: [],
  })
  showModal.value = true
  if (post.type === 'blog') {
    fetchPostRelatedProducts(post.id)
  }
}

const closeModal = () => {
  showModal.value = false
  clearErrors()
}

const handleSubmit = async () => {
  if (!validate({ slug: form.slug, type: form.type, title: form.title['zh-CN'] } as Record<string, unknown>)) return
  submitting.value = true
  try {
    const { product_ids, ...rest } = form
    const payload: Partial<AdminPost> & { product_ids?: number[] } = { ...rest }
    payload.category_id = form.type === 'blog' ? form.category_id : null
    payload.product_ids = form.type === 'blog' ? [...product_ids] : []
    if (isEditing.value) {
      await adminAPI.updatePost(form.id, payload)
    } else {
      await adminAPI.createPost(payload)
    }
    closeModal()
    fetchPosts()
  } catch (err) {
    notifyError(t('admin.posts.errors.operationFailed', { message: (err as Error).message || '' }))
  } finally {
    submitting.value = false
  }
}

const handleDelete = async (post: AdminPost) => {
  const confirmed = await confirmAction({ description: t('admin.posts.confirmDelete', { name: getLocalizedText(post.title) }), confirmText: t('admin.common.delete'), variant: 'destructive' })
  if (!confirmed) return
  try {
    await adminAPI.deletePost(post.id)
    fetchPosts()
  } catch (err) {
    notifyError(t('admin.posts.errors.deleteFailed', { message: (err as Error).message || '' }))
  }
}

const openEditById = async (rawId: unknown) => {
  const id = Number(rawId)
  if (!Number.isFinite(id) || id <= 0) return
  try {
    const res = await adminAPI.getPost(id)
    openEditModal(res.data.data)
  } catch (err) {
    notifyError(t('admin.posts.errors.operationFailed', { message: (err as Error).message || '' }))
  }
}

onMounted(() => {
  fetchCategories()
  fetchPosts()
  if (route.query.post_id) {
    openEditById(route.query.post_id)
  }
})

watch(
  () => route.params.type,
  (type) => {
    if (type === 'notice' || type === 'blog') {
      currentTab.value = type
    }
  },
)

watch(
  () => route.query.post_id,
  (value) => {
    if (value) {
      openEditById(value)
    }
  }
)
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <h1 class="text-2xl font-semibold">{{ t('admin.posts.title') }}</h1>
      <Button size="sm" class="w-full gap-2 sm:w-auto" @click="openCreateModal">
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
      </TabsList>
    </Tabs>

    <div class="rounded-xl border border-border bg-card p-4 shadow-sm">
      <div class="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center">
        <!-- 公告没有分类维度，分类筛选仅在博客文章下提供 -->
        <div v-if="currentTab === 'blog'" class="w-full md:w-56">
          <Select v-model="categoryFilter" @update:modelValue="handleFilterChange">
            <SelectTrigger class="h-9 w-full">
              <SelectValue :placeholder="t('admin.posts.filters.categoryPlaceholder')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{{ t('admin.posts.filters.categoryAll') }}</SelectItem>
              <SelectItem
                v-for="cat in categories"
                :key="cat.id"
                :value="String(cat.id)"
                :disabled="!cat.selectable"
                :class="cat.depth > 0 ? 'pl-6' : ''"
              >
                {{ cat.depth > 0 ? getLocalizedText(cat.name) : getCategoryPath(cat.id) }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div class="w-full md:w-48">
          <Select v-model="statusFilter" @update:modelValue="handleFilterChange">
            <SelectTrigger class="h-9 w-full">
              <SelectValue :placeholder="t('admin.posts.filters.statusPlaceholder')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{{ t('admin.posts.filters.statusAll') }}</SelectItem>
              <SelectItem value="published">{{ t('admin.posts.filters.statusPublished') }}</SelectItem>
              <SelectItem value="draft">{{ t('admin.posts.filters.statusDraft') }}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <Button variant="outline" size="sm" class="h-9 w-full sm:w-auto" @click="resetFilters">
          {{ t('admin.common.reset') }}
        </Button>
      </div>
    </div>

    <!-- Batch action bar -->
    <div v-if="selectedIds.size > 0" class="flex flex-wrap items-center gap-3 rounded-xl border border-primary/20 bg-primary/5 px-4 py-3">
      <span class="text-sm font-medium">{{ t('admin.posts.batch.selected', { count: selectedIds.size }) }}</span>
      <div class="flex flex-wrap items-center gap-2">
        <Button size="sm" variant="outline" :disabled="batchOperating" @click="handleBatchStatus(true)">{{ t('admin.posts.batch.publish') }}</Button>
        <Button size="sm" variant="outline" :disabled="batchOperating" @click="handleBatchStatus(false)">{{ t('admin.posts.batch.draft') }}</Button>
        <!-- 公告没有分类维度，移动分类仅在博客文章下提供 -->
        <div v-if="currentTab === 'blog'" class="flex items-center gap-1">
          <Select v-model="batchCategoryId">
            <SelectTrigger class="h-8 w-[160px] text-xs"><SelectValue :placeholder="t('admin.posts.batch.categoryPlaceholder')" /></SelectTrigger>
            <SelectContent>
              <SelectItem
                v-for="cat in categories"
                :key="cat.id"
                :value="String(cat.id)"
                :disabled="!cat.selectable"
                :class="cat.depth > 0 ? 'pl-6' : ''"
              >
                {{ cat.depth > 0 ? getLocalizedText(cat.name) : getCategoryPath(cat.id) }}
              </SelectItem>
            </SelectContent>
          </Select>
          <Button size="sm" variant="outline" :disabled="batchOperating || !batchCategoryId" @click="handleBatchCategory">{{ t('admin.posts.batch.moveCategory') }}</Button>
        </div>
        <Button size="sm" variant="destructive" :disabled="batchOperating" @click="handleBatchDelete">{{ t('admin.posts.batch.delete') }}</Button>
      </div>
      <button class="ml-auto text-xs text-muted-foreground hover:text-foreground" @click="selectedIds = new Set()">{{ t('admin.posts.batch.clearSelection') }}</button>
    </div>

    <div class="rounded-xl border border-border bg-card overflow-x-auto">
      <Table class="min-w-[1020px]">
        <TableHeader class="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
          <TableRow>
            <TableHead class="w-10 px-3 py-3">
              <Checkbox :model-value="allSelected ? true : (selectedIds.size > 0 ? 'indeterminate' : false)" @update:model-value="toggleSelectAll" />
            </TableHead>
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
            <TableCell :colspan="currentTab === 'blog' ? 8 : 7" class="p-0">
              <TableSkeleton :columns="currentTab === 'blog' ? 8 : 7" :rows="5" />
            </TableCell>
          </TableRow>
          <TableRow v-else-if="posts.length === 0">
            <TableCell :colspan="currentTab === 'blog' ? 8 : 7" class="px-6 py-8 text-center text-muted-foreground">{{ t('admin.posts.empty') }}</TableCell>
          </TableRow>
          <TableRow v-for="post in posts" :key="post.id" class="hover:bg-muted/30">
            <TableCell class="w-10 px-3 py-4">
              <Checkbox :model-value="selectedIds.has(post.id)" @update:model-value="() => toggleSelect(post.id)" />
            </TableCell>
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
              <div class="flex items-center justify-end gap-2">
                <Button size="icon-sm" variant="outline" @click="openEditModal(post)">
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

    <Dialog v-model:open="showModal" @update:open="(value) => { if (!value) closeModal() }">
      <DialogScrollContent class="w-[calc(100vw-1rem)] max-w-5xl p-4 sm:p-6" @interact-outside="(e) => e.preventDefault()">
        <DialogHeader>
          <DialogTitle>{{ isEditing ? t('admin.posts.modal.editTitle') : t('admin.posts.modal.createTitle') }}</DialogTitle>
          <DialogDescription>{{ isEditing ? t('admin.posts.modal.editDescription') : t('admin.posts.modal.createDescription') }}</DialogDescription>
        </DialogHeader>
        <form class="space-y-6" @submit.prevent="handleSubmit">
          <div class="border-b border-border">
            <div class="flex gap-4 overflow-x-auto pb-1">
              <button
                v-for="lang in languages"
                :key="lang.code"
                type="button"
                class="shrink-0 border-b-2 px-4 py-2 text-sm font-medium transition-colors"
                :class="currentLang === lang.code ? 'border-primary text-foreground' : 'border-transparent text-muted-foreground hover:text-foreground'"
                @click="currentLang = lang.code"
              >
                {{ lang.name }}
              </button>
            </div>
          </div>

          <div class="grid grid-cols-1 gap-4 md:grid-cols-2 md:gap-6">
            <div class="col-span-1 md:col-span-2">
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">
                {{ t('admin.posts.form.title', { lang: getCurrentLangName() }) }}
              </label>
              <Input v-model="form.title[currentLang]" />
              <p v-if="errors.title" class="text-xs text-destructive mt-1">{{ errors.title }}</p>
            </div>

            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.posts.form.slug') }}</label>
              <Input v-model="form.slug" :placeholder="t('admin.posts.form.slugPlaceholder')" />
              <p v-if="errors.slug" class="text-xs text-destructive mt-1">{{ errors.slug }}</p>
            </div>

            <div>
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.posts.form.type') }}</label>
              <Select v-model="form.type">
                <SelectTrigger class="h-9 w-full">
                  <SelectValue :placeholder="t('admin.posts.form.typeBlog')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="blog">{{ t('admin.posts.form.typeBlog') }}</SelectItem>
                  <SelectItem value="notice">{{ t('admin.posts.form.typeNotice') }}</SelectItem>
                </SelectContent>
              </Select>
              <p v-if="errors.type" class="text-xs text-destructive mt-1">{{ errors.type }}</p>
            </div>

            <div v-if="form.type === 'blog'">
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.posts.form.category') }}</label>
              <Select :model-value="form.category_id != null ? String(form.category_id) : '__none__'" @update:model-value="(v) => { form.category_id = v === '__none__' ? null : Number(v) || null }">
                <SelectTrigger class="h-9 w-full">
                  <SelectValue :placeholder="t('admin.posts.form.categoryPlaceholder')" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="__none__">{{ t('admin.posts.form.noCategory') }}</SelectItem>
                  <SelectItem v-for="cat in categories" :key="cat.id" :value="String(cat.id)" :disabled="!cat.selectable" :class="cat.depth > 0 ? 'pl-6' : ''">{{ cat.depth > 0 ? getLocalizedText(cat.name) : getCategoryPath(cat.id) }}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div class="col-span-1 md:col-span-2">
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">
                {{ t('admin.posts.form.summary', { lang: getCurrentLangName() }) }}
              </label>
              <Textarea v-model="form.summary[currentLang]" rows="2" :placeholder="t('admin.posts.form.summaryPlaceholder')" />
            </div>

            <div class="col-span-1 md:col-span-2">
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">{{ t('admin.posts.form.thumbnail') }}</label>
              <MediaPicker v-model="form.thumbnail" scene="post" />
            </div>

            <div class="col-span-1 md:col-span-2">
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">
                {{ t('admin.posts.form.content', { lang: getCurrentLangName() }) }}
              </label>
              <RichEditor :model-value="form.content[currentLang] || ''" @update:model-value="(v: string) => form.content[currentLang] = v" :placeholder="t('admin.posts.form.contentPlaceholder')" />
            </div>

            <div v-if="form.type === 'blog'" class="col-span-1 md:col-span-2 border-t border-border pt-4">
              <label class="mb-1.5 block text-xs font-medium text-muted-foreground">
                {{ t('admin.posts.form.relatedProducts') }}
              </label>
              <p class="mb-3 text-xs text-muted-foreground">
                {{ t('admin.posts.form.relatedProductsHint') }}
              </p>

              <div v-if="relatedProducts.length" class="mb-3 space-y-2">
                <div
                  v-for="p in relatedProducts"
                  :key="p.id"
                  class="flex items-center gap-3 rounded-md border border-border bg-muted/30 px-3 py-2"
                >
                  <div
                    v-if="p.image"
                    class="h-10 w-10 shrink-0 overflow-hidden rounded border border-border bg-muted/40"
                  >
                    <img :src="getImageUrl(p.image)" class="h-full w-full object-cover" />
                  </div>
                  <div class="min-w-0 flex-1">
                    <div class="truncate text-sm font-medium text-foreground">{{ getLocalizedText(p.title) }}</div>
                    <div class="truncate font-mono text-xs text-muted-foreground">/{{ p.slug }}</div>
                  </div>
                  <Button type="button" size="sm" variant="outline" @click="removeRelatedProduct(p.id)">
                    {{ t('admin.posts.form.relatedProductsRemove') }}
                  </Button>
                </div>
              </div>
              <p v-else class="mb-3 text-sm text-muted-foreground">
                {{ t('admin.posts.form.relatedProductsEmpty') }}
              </p>

              <Input v-model="productSearch" :placeholder="t('admin.posts.form.relatedProductsSearch')" />
              <div v-if="productSearch.trim()" class="mt-2 max-h-60 overflow-y-auto rounded-md border border-border">
                <div v-if="productSearchLoading" class="px-3 py-3 text-center text-xs text-muted-foreground">
                  ...
                </div>
                <div
                  v-else-if="!productSearchResults.length"
                  class="px-3 py-3 text-center text-sm text-muted-foreground"
                >
                  {{ t('admin.posts.form.relatedProductsNoResults') }}
                </div>
                <div
                  v-for="p in productSearchResults"
                  :key="p.id"
                  class="flex items-center gap-3 border-b border-border px-3 py-2 last:border-b-0 hover:bg-muted/40"
                >
                  <div
                    v-if="p.image"
                    class="h-10 w-10 shrink-0 overflow-hidden rounded border border-border bg-muted/40"
                  >
                    <img :src="getImageUrl(p.image)" class="h-full w-full object-cover" />
                  </div>
                  <div class="min-w-0 flex-1">
                    <div class="truncate text-sm font-medium text-foreground">{{ getLocalizedText(p.title) }}</div>
                    <div class="truncate font-mono text-xs text-muted-foreground">/{{ p.slug }}</div>
                  </div>
                  <Button
                    v-if="!form.product_ids.includes(p.id)"
                    type="button"
                    size="sm"
                    variant="outline"
                    @click="addRelatedProduct(p)"
                  >
                    {{ t('admin.posts.form.relatedProductsAdd') }}
                  </Button>
                  <span v-else class="text-xs text-muted-foreground">✓</span>
                </div>
              </div>
            </div>

            <div class="col-span-1 flex items-center gap-2 border-t border-border pt-4 md:col-span-2">
              <Switch v-model="form.is_published" />
              <span class="text-sm text-muted-foreground">{{ t('admin.posts.form.publishNow') }}</span>
            </div>
          </div>

          <div class="flex flex-col-reverse gap-3 border-t border-border pt-6 sm:flex-row sm:justify-end">
            <Button type="button" variant="outline" class="w-full sm:w-auto" @click="closeModal">{{ t('admin.common.cancel') }}</Button>
            <Button type="submit" class="w-full sm:w-auto" :disabled="submitting">
              {{ isEditing ? t('admin.posts.actions.saveChanges') : t('admin.posts.actions.publishNow') }}
            </Button>
          </div>
        </form>
      </DialogScrollContent>
    </Dialog>
  </div>
</template>
